package transcribe

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"
)

func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	var he *httpError
	if !errors.As(err, &he) {
		return false
	}
	return he.StatusCode == 429 || he.StatusCode >= 500
}

type retryTranscriber struct {
	inner     Transcriber
	attempts  int
	base      time.Duration
	factor    float64
	jitter    float64
	timeout   time.Duration
	sleepFunc func(time.Duration)
}

func newRetryTranscriber(inner Transcriber, timeout time.Duration) *retryTranscriber {
	return &retryTranscriber{
		inner:     inner,
		attempts:  3,
		base:      1 * time.Second,
		factor:    2.0,
		jitter:    0.25,
		timeout:   timeout,
		sleepFunc: time.Sleep,
	}
}

func (r *retryTranscriber) Transcribe(ctx context.Context, audio []byte, mimeType string) (string, error) {
	if r.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.timeout)
		defer cancel()
	}

	if err := ctx.Err(); err != nil {
		return "", err
	}

	delay := r.base
	var lastErr error

	for i := 0; i < r.attempts; i++ {
		text, err := r.inner.Transcribe(ctx, audio, mimeType)
		if err == nil {
			return text, nil
		}
		lastErr = err

		if !isRetryable(err) || i == r.attempts-1 {
			break
		}

		if ctx.Err() != nil {
			break
		}

		jitterAmount := time.Duration(float64(delay) * r.jitter * rand.Float64()) //nolint:gosec
		r.sleepFunc(delay + jitterAmount)
		delay = time.Duration(float64(delay) * r.factor)
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", ctxErr
	}

	if !isRetryable(lastErr) {
		return "", lastErr
	}

	return "", fmt.Errorf("transcribe failed after %d attempts: %w", r.attempts, lastErr)
}
