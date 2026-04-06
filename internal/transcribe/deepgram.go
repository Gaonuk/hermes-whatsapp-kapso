package transcribe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// deepgramProvider implements Transcriber for the Deepgram API.
type deepgramProvider struct {
	APIKey     string
	Model      string
	Language   string
	HTTPClient *http.Client
	BaseURL    string
}

func (p *deepgramProvider) client() *http.Client {
	if p.HTTPClient != nil {
		return p.HTTPClient
	}
	return http.DefaultClient
}

type deepgramResponse struct {
	Results struct {
		Channels []struct {
			Alternatives []struct {
				Transcript string `json:"transcript"`
			} `json:"alternatives"`
		} `json:"channels"`
	} `json:"results"`
}

func (p *deepgramProvider) Transcribe(ctx context.Context, audio []byte, mimeType string) (string, error) {
	norm := NormalizeMIME(mimeType)

	q := url.Values{}
	q.Set("model", p.Model)
	q.Set("smart_format", "true")
	if p.Language != "" {
		q.Set("language", p.Language)
	}

	baseURL := p.BaseURL
	if baseURL == "" {
		baseURL = "https://api.deepgram.com/v1/listen"
	}
	endpoint := baseURL + "?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(audio))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+p.APIKey)
	req.Header.Set("Content-Type", norm)

	resp, err := p.client().Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", &httpError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	var result deepgramResponse
	if err = json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse deepgram response: %w", err)
	}

	channels := result.Results.Channels
	if len(channels) == 0 || len(channels[0].Alternatives) == 0 {
		return "", fmt.Errorf("deepgram response missing channels/alternatives")
	}

	return channels[0].Alternatives[0].Transcript, nil
}
