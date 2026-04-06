package transcribe

import "strings"

// NormalizeMIME strips MIME type parameters and maps variants to canonical forms.
func NormalizeMIME(mimeType string) string {
	if mimeType == "" {
		return ""
	}
	norm, _, _ := strings.Cut(mimeType, ";")
	norm = strings.ToLower(strings.TrimSpace(norm))

	switch norm {
	case "audio/opus":
		return "audio/ogg"
	}
	return norm
}

func mimeToFilename(norm string) string {
	switch norm {
	case "audio/ogg":
		return "audio.ogg"
	case "audio/mpeg":
		return "audio.mp3"
	case "audio/mp4":
		return "audio.mp4"
	case "audio/wav", "audio/x-wav":
		return "audio.wav"
	case "audio/webm":
		return "audio.webm"
	case "audio/flac":
		return "audio.flac"
	default:
		return "audio.bin"
	}
}
