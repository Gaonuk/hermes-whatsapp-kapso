// Package commands implements the bridge-level command system.
package commands

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/rgaona/hermes-whatsapp-kapso/internal/config"
	"github.com/rgaona/hermes-whatsapp-kapso/internal/gateway"
	"github.com/rgaona/hermes-whatsapp-kapso/internal/kapso"
)

const maxOutputLen = 4000

// Dispatcher parses and executes bridge commands.
type Dispatcher struct {
	prefix  string
	timeout time.Duration
	defs    map[string]config.CommandDef
}

// New creates a Dispatcher from config.
func New(cfg config.CommandsConfig) *Dispatcher {
	return &Dispatcher{
		prefix:  cfg.Prefix,
		timeout: time.Duration(cfg.Timeout) * time.Second,
		defs:    cfg.Definitions,
	}
}

// Prefix returns the configured command prefix string.
func (d *Dispatcher) Prefix() string { return d.prefix }

// IsCommand reports whether text is a command invocation.
func (d *Dispatcher) IsCommand(text string) bool {
	if d.prefix == "" || len(d.defs) == 0 {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(text), d.prefix)
}

// Parse extracts the command name and free-form args from text.
func (d *Dispatcher) Parse(text string) (name, args string, ok bool) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, d.prefix) {
		return "", "", false
	}
	rest := strings.TrimSpace(text[len(d.prefix):])
	parts := strings.SplitN(rest, " ", 2)
	name = strings.ToLower(parts[0])
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}
	return name, args, true
}

// Exists reports whether a command with the given name is defined.
func (d *Dispatcher) Exists(name string) bool {
	if name == "help" {
		return true
	}
	_, ok := d.defs[name]
	return ok
}

// CanRun reports whether the role is permitted to run the named command.
func (d *Dispatcher) CanRun(name, role string) bool {
	if name == "help" {
		return true
	}
	def, ok := d.defs[name]
	if !ok {
		return false
	}
	if len(def.Roles) == 0 {
		return true
	}
	for _, r := range def.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// Ack returns the pre-execution acknowledgment message for a command.
func (d *Dispatcher) Ack(name string) string {
	def, ok := d.defs[name]
	if !ok {
		return ""
	}
	return def.Ack
}

// Handle executes the named command and returns the reply text.
func (d *Dispatcher) Handle(
	ctx context.Context,
	name, args, role, sessionKey string,
	gw gateway.Gateway,
	req *gateway.Request,
	_ *kapso.Client,
) string {
	if name == "help" {
		return d.helpText(role)
	}

	def, ok := d.defs[name]
	if !ok {
		return fmt.Sprintf("Unknown command. Send %shelp for available commands.", d.prefix)
	}

	switch def.Type {
	case "shell":
		return d.runShell(ctx, def, args)
	case "agent":
		return d.runAgent(ctx, def, args, role, sessionKey, gw, req)
	default:
		log.Printf("commands: unknown command type %q for %q", def.Type, name)
		return fmt.Sprintf("Command %q has an invalid type %q.", name, def.Type)
	}
}

func (d *Dispatcher) runShell(ctx context.Context, def config.CommandDef, args string) string {
	timeout := d.timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	tCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(tCtx, "sh", "-c", def.Shell)
	cmd.Env = append(os.Environ(), "ARGS="+args)
	cmd.WaitDelay = 2 * time.Second

	out, err := cmd.CombinedOutput()

	if tCtx.Err() == context.DeadlineExceeded {
		return fmt.Sprintf("Command timed out after %s.", timeout)
	}

	result := strings.TrimSpace(string(out))
	if result == "" && err != nil {
		return fmt.Sprintf("Command failed: %v", err)
	}
	if len(result) > maxOutputLen {
		result = result[:maxOutputLen] + "\n… (truncated)"
	}
	return result
}

func (d *Dispatcher) runAgent(
	ctx context.Context,
	def config.CommandDef,
	args, role, sessionKey string,
	gw gateway.Gateway,
	req *gateway.Request,
) string {
	prompt := strings.ReplaceAll(def.Prompt, "{args}", args)
	reply, err := gw.SendAndReceive(ctx, &gateway.Request{
		SessionKey:     sessionKey,
		IdempotencyKey: req.IdempotencyKey + "-cmd",
		From:           req.From,
		FromName:       req.FromName,
		Role:           role,
		Text:           prompt,
	})
	if err != nil {
		log.Printf("commands: agent command failed: %v", err)
		return fmt.Sprintf("Agent error: %v", err)
	}
	return reply
}

func (d *Dispatcher) helpText(role string) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("*Available commands* (prefix: %s)", d.prefix))

	var names []string
	for name := range d.defs {
		if d.CanRun(name, role) {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	for _, name := range names {
		def := d.defs[name]
		desc := def.Description
		if desc == "" {
			desc = def.Type + " command"
		}
		lines = append(lines, fmt.Sprintf("%s%s — %s", d.prefix, name, desc))
	}

	if len(names) == 0 {
		return "No commands available for your role."
	}

	lines = append(lines, fmt.Sprintf("\nSend %s<command> [optional instructions]", d.prefix))
	return strings.Join(lines, "\n")
}
