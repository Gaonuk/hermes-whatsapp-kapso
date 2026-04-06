package transcribe

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rgaona/hermes-whatsapp-kapso/internal/config"
)

// localWhisper implements Transcriber using local ffmpeg + whisper-cli subprocesses.
type localWhisper struct {
	BinaryPath string
	ModelPath  string
	Language   string
	execCmd    func(ctx context.Context, name string, args ...string) *exec.Cmd
}

func newLocalWhisper(cfg config.TranscribeConfig) (*localWhisper, error) {
	if cfg.ModelPath == "" {
		return nil, fmt.Errorf("local provider requires model_path (set KAPSO_TRANSCRIBE_MODEL_PATH)")
	}

	binaryPath := cfg.BinaryPath
	if binaryPath == "" {
		binaryPath = "whisper-cli"
	}

	return &localWhisper{
		BinaryPath: binaryPath,
		ModelPath:  cfg.ModelPath,
		Language:   cfg.Language,
		execCmd:    exec.CommandContext,
	}, nil
}

func (p *localWhisper) Transcribe(ctx context.Context, audio []byte, _ string) (string, error) {
	dir, err := os.MkdirTemp("", "kapso-whisper-*")
	if err != nil {
		return "", fmt.Errorf("local provider: create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	rawPath := filepath.Join(dir, "audio.ogg")
	if err := os.WriteFile(rawPath, audio, 0o600); err != nil {
		return "", fmt.Errorf("local provider: write audio file: %w", err)
	}

	wavPath := filepath.Join(dir, "audio.wav")
	ffmpegCmd := p.execCmd(ctx, "ffmpeg",
		"-y",
		"-loglevel", "error",
		"-i", rawPath,
		"-acodec", "pcm_s16le",
		"-ac", "1",
		"-ar", "16000",
		wavPath,
	)
	if out, err := ffmpegCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg conversion failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	outputPrefix := filepath.Join(dir, "transcript")
	args := []string{
		"-m", p.ModelPath,
		"-f", wavPath,
		"-otxt",
		"-of", outputPrefix,
	}
	if p.Language != "" {
		args = append(args, "-l", p.Language)
	}

	whisperCmd := p.execCmd(ctx, p.BinaryPath, args...)
	if out, err := whisperCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("whisper-cli transcription failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	txtPath := outputPrefix + ".txt"
	raw, err := os.ReadFile(txtPath)
	if err != nil {
		return "", fmt.Errorf("local provider: read transcript file: %w", err)
	}

	return strings.TrimSpace(string(raw)), nil
}
