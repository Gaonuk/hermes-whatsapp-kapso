package security

import (
	"strings"
	"sync"
	"time"

	"github.com/rgaona/hermes-whatsapp-kapso/internal/config"
)

// Verdict represents the outcome of a guard check.
type Verdict int

const (
	Allow Verdict = iota
	Deny
	RateLimited
)

type bucket struct {
	tokens    int
	windowEnd time.Time
}

// Guard enforces sender allowlist, rate limiting, role resolution, and session isolation.
type Guard struct {
	mode        string
	phoneTo     map[string]string
	defaultRole string
	denyMessage string
	rateLimit   int
	rateWindow  time.Duration
	isolate     bool
	now         func() time.Time
	mu          sync.Mutex
	buckets     map[string]*bucket
}

// New creates a Guard from the security config.
func New(cfg config.SecurityConfig) *Guard {
	phoneTo := make(map[string]string)
	for role, phones := range cfg.Roles {
		for _, phone := range phones {
			n := normalize(phone)
			if _, exists := phoneTo[n]; !exists {
				phoneTo[n] = role
			}
		}
	}

	return &Guard{
		mode:        cfg.Mode,
		phoneTo:     phoneTo,
		defaultRole: cfg.DefaultRole,
		denyMessage: cfg.DenyMessage,
		rateLimit:   cfg.RateLimit,
		rateWindow:  time.Duration(cfg.RateWindow) * time.Second,
		isolate:     cfg.SessionIsolation,
		now:         time.Now,
		buckets:     make(map[string]*bucket),
	}
}

// Check returns Allow, Deny, or RateLimited for the given sender phone number.
func (g *Guard) Check(from string) Verdict {
	n := normalize(from)

	if g.mode == "allowlist" {
		if _, ok := g.phoneTo[n]; !ok {
			return Deny
		}
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.now()
	b, ok := g.buckets[n]
	if !ok || now.After(b.windowEnd) {
		g.buckets[n] = &bucket{
			tokens:    g.rateLimit - 1,
			windowEnd: now.Add(g.rateWindow),
		}
		return Allow
	}

	if b.tokens <= 0 {
		return RateLimited
	}
	b.tokens--
	return Allow
}

// Role returns the sender's role.
func (g *Guard) Role(from string) string {
	n := normalize(from)
	if role, ok := g.phoneTo[n]; ok {
		return role
	}
	return g.defaultRole
}

// DenyMessage returns the configured denial message.
func (g *Guard) DenyMessage() string {
	return g.denyMessage
}

// SessionKey returns a per-sender session key if isolation is enabled.
func (g *Guard) SessionKey(baseKey, from string) string {
	if !g.isolate {
		return baseKey
	}
	n := normalize(from)
	return baseKey + "-wa-" + n
}

func normalize(phone string) string {
	if phone == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(phone))

	for _, r := range phone {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}

	return b.String()
}
