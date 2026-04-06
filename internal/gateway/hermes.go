package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rgaona/hermes-whatsapp-kapso/internal/config"
)

// OpenAI-compatible chat completion types for hermes-agent API.

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type chatCompletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Choices []struct {
		Message      chatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
}

// Hermes implements Gateway for the hermes-agent runtime via its
// OpenAI-compatible HTTP API (/v1/chat/completions).
//
// Each sender gets an isolated conversation history tracked via the
// X-Hermes-Session-Id header, so hermes-agent maintains per-user context.
type Hermes struct {
	baseURL      string
	token        string
	model        string
	systemPrompt string
	sessionKey   string

	client *http.Client

	// Per-sender conversation history for stateful interactions.
	mu       sync.Mutex
	sessions map[string][]chatMessage
}

// NewHermes creates a Hermes gateway from config.
func NewHermes(cfg config.GatewayConfig) *Hermes {
	return &Hermes{
		baseURL:      strings.TrimRight(cfg.URL, "/"),
		token:        cfg.Token,
		model:        cfg.Model,
		systemPrompt: cfg.SystemPrompt,
		sessionKey:   cfg.SessionKey,
		client: &http.Client{
			Timeout: 10 * time.Minute,
		},
		sessions: make(map[string][]chatMessage),
	}
}

// Connect verifies the hermes-agent is reachable by hitting the health endpoint.
func (h *Hermes) Connect(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.baseURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("create health request: %w", err)
	}
	if h.token != "" {
		req.Header.Set("Authorization", "Bearer "+h.token)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("connect to hermes-agent at %s: %w", h.baseURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("hermes-agent health check failed (status %d)", resp.StatusCode)
	}

	log.Printf("connected to hermes-agent at %s", h.baseURL)
	return nil
}

// SendAndReceive sends a message to hermes-agent via the chat completions API
// and returns the agent's reply text.
func (h *Hermes) SendAndReceive(ctx context.Context, req *Request) (string, error) {
	sessionKey := req.SessionKey
	if sessionKey == "" {
		sessionKey = h.sessionKey
	}

	// Build the message with sender metadata.
	userText := fmt.Sprintf("From: %s (%s) [role: %s]\n%s",
		req.From, req.FromName, req.Role, req.Text)

	// Get or initialize conversation history for this session.
	h.mu.Lock()
	history, exists := h.sessions[sessionKey]
	if !exists {
		history = []chatMessage{}
		if h.systemPrompt != "" {
			history = append(history, chatMessage{
				Role:    "system",
				Content: h.systemPrompt,
			})
		}
	}

	// Append the new user message.
	history = append(history, chatMessage{
		Role:    "user",
		Content: userText,
	})

	// Cap history to prevent unbounded growth (keep system prompt + last 50 messages).
	maxHistory := 51
	if h.systemPrompt != "" {
		maxHistory = 52
	}
	if len(history) > maxHistory {
		if h.systemPrompt != "" {
			// Keep system prompt + last 50 messages.
			history = append(history[:1], history[len(history)-50:]...)
		} else {
			history = history[len(history)-50:]
		}
	}

	// Copy for the request (release lock before HTTP call).
	messages := make([]chatMessage, len(history))
	copy(messages, history)
	h.mu.Unlock()

	// Build the chat completion request.
	completionReq := chatCompletionRequest{
		Model:    h.model,
		Messages: messages,
		Stream:   false,
	}

	body, err := json.Marshal(completionReq)
	if err != nil {
		return "", fmt.Errorf("marshal chat completion request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, h.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create chat completion request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if h.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+h.token)
	}
	// Use session ID header for hermes-agent session tracking.
	httpReq.Header.Set("X-Hermes-Session-Id", sessionKey)

	resp, err := h.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("send chat completion: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read chat completion response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("hermes-agent error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var completionResp chatCompletionResponse
	if err := json.Unmarshal(respBody, &completionResp); err != nil {
		return "", fmt.Errorf("parse chat completion response: %w", err)
	}

	if len(completionResp.Choices) == 0 {
		return "", fmt.Errorf("hermes-agent returned no choices")
	}

	replyText := completionResp.Choices[0].Message.Content

	// Store assistant reply in conversation history.
	h.mu.Lock()
	history = append(history, chatMessage{
		Role:    "assistant",
		Content: replyText,
	})
	h.sessions[sessionKey] = history
	h.mu.Unlock()

	return replyText, nil
}

// Close is a no-op for the HTTP-based hermes gateway.
func (h *Hermes) Close() error {
	return nil
}
