package transcribe

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

type cacheEntry struct {
	text   string
	expiry time.Time
}

type cacheTranscriber struct {
	inner   Transcriber
	ttl     time.Duration
	nowFunc func() time.Time
	mu      sync.Mutex
	items   map[string]cacheEntry
}

func newCacheTranscriber(inner Transcriber, ttl time.Duration) *cacheTranscriber {
	return &cacheTranscriber{
		inner:   inner,
		ttl:     ttl,
		nowFunc: time.Now,
		items:   make(map[string]cacheEntry),
	}
}

func (c *cacheTranscriber) cacheKey(audio []byte) string {
	h := sha256.Sum256(audio)
	return hex.EncodeToString(h[:])
}

func (c *cacheTranscriber) Transcribe(ctx context.Context, audio []byte, mimeType string) (string, error) {
	key := c.cacheKey(audio)
	now := c.nowFunc()

	c.mu.Lock()
	entry, ok := c.items[key]
	if ok && now.Before(entry.expiry) {
		c.mu.Unlock()
		return entry.text, nil
	}
	c.mu.Unlock()

	text, err := c.inner.Transcribe(ctx, audio, mimeType)
	if err != nil {
		return "", err
	}

	expiry := c.nowFunc().Add(c.ttl)
	c.mu.Lock()
	c.items[key] = cacheEntry{text: text, expiry: expiry}
	c.mu.Unlock()

	return text, nil
}
