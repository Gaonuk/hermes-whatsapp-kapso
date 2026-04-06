package transcribe

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/rgaona/hermes-whatsapp-kapso/internal/config"
)

// Transcriber converts audio bytes to text.
type Transcriber interface {
	Transcribe(ctx context.Context, audio []byte, mimeType string) (string, error)
}

// New constructs a Transcriber from the provided config.
// Returns (nil, nil) when no provider is configured.
func New(cfg config.TranscribeConfig) (Transcriber, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))

	if provider == "" {
		log.Printf("transcription disabled (no provider configured)")
		return nil, nil
	}

	var p Transcriber

	switch provider {
	case "openai":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("provider %q requires KAPSO_TRANSCRIBE_API_KEY", provider)
		}
		model := cfg.Model
		if model == "" {
			model = "whisper-1"
		}
		p = &openAIWhisper{
			BaseURL:           "https://api.openai.com/v1",
			APIKey:            cfg.APIKey,
			Model:             model,
			Language:          cfg.Language,
			NoSpeechThreshold: cfg.NoSpeechThreshold,
			Debug:             cfg.Debug,
		}

	case "groq":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("provider %q requires KAPSO_TRANSCRIBE_API_KEY", provider)
		}
		model := cfg.Model
		if model == "" {
			model = "whisper-large-v3"
		}
		p = &openAIWhisper{
			BaseURL:           "https://api.groq.com/openai/v1",
			APIKey:            cfg.APIKey,
			Model:             model,
			Language:          cfg.Language,
			NoSpeechThreshold: cfg.NoSpeechThreshold,
			Debug:             cfg.Debug,
		}

	case "deepgram":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("provider %q requires KAPSO_TRANSCRIBE_API_KEY", provider)
		}
		model := cfg.Model
		if model == "" {
			model = "nova-3"
		}
		p = &deepgramProvider{
			APIKey:   cfg.APIKey,
			Model:    model,
			Language: cfg.Language,
		}

	case "local":
		binaryPath := cfg.BinaryPath
		if binaryPath == "" {
			binaryPath = "whisper-cli"
		}
		if _, err := exec.LookPath(binaryPath); err != nil {
			return nil, fmt.Errorf("local provider binary %q not found: %w", binaryPath, err)
		}
		if _, err := exec.LookPath("ffmpeg"); err != nil {
			return nil, fmt.Errorf("local provider requires ffmpeg in PATH: %w", err)
		}
		lp, err := newLocalWhisper(cfg)
		if err != nil {
			return nil, err
		}
		if cfg.CacheTTL > 0 {
			return newCacheTranscriber(lp, time.Duration(cfg.CacheTTL)*time.Second), nil
		}
		return lp, nil

	default:
		return nil, fmt.Errorf("unknown transcription provider %q (valid: openai, groq, deepgram, local)", provider)
	}

	timeout := time.Duration(cfg.Timeout) * time.Second
	wrapped := newRetryTranscriber(p, timeout)
	if cfg.CacheTTL > 0 {
		return newCacheTranscriber(wrapped, time.Duration(cfg.CacheTTL)*time.Second), nil
	}
	return wrapped, nil
}
