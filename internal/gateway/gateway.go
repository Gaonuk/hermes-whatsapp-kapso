package gateway

import (
	"context"

	"github.com/rgaona/hermes-whatsapp-kapso/internal/config"
)

// Gateway is the abstraction for AI agent backends.
type Gateway interface {
	// Connect establishes a connection to the backend.
	Connect(ctx context.Context) error

	// SendAndReceive sends a message and blocks until the agent's reply is
	// available. The returned string is the raw agent response text.
	SendAndReceive(ctx context.Context, req *Request) (string, error)

	// Close tears down the connection.
	Close() error
}

// Request carries all fields a gateway implementation might need to format and
// route a message.
type Request struct {
	SessionKey     string // agent session to target
	IdempotencyKey string // dedup key (typically the WhatsApp message ID)
	From           string // sender phone number (E.164)
	FromName       string // sender display name
	Role           string // sender role (admin, member, etc.)
	Text           string // raw message text
}

// New creates the appropriate Gateway for the configured type.
func New(cfg config.GatewayConfig) (Gateway, error) {
	return NewHermes(cfg), nil
}
