# CLAUDE.md

## Project

Go daemon bridging WhatsApp (Kapso Cloud API) → Hermes Agent (NousResearch/hermes-agent). Two binaries: `hermes-whatsapp-bridge` (main daemon) and `hermes-whatsapp-cli` (send messages/health checks).

## Commands

```bash
just build                            # Build both binaries
just test                             # Run tests
just check                            # Run tests + vet + fmt check
just lint                             # Run golangci-lint
```

## Structure

```
cmd/hermes-whatsapp-bridge/main.go    # Daemon entrypoint
cmd/hermes-whatsapp-cli/main.go       # CLI entrypoint
internal/config/                      # TOML + env config (3-tier: defaults < file < env)
internal/kapso/                       # Kapso HTTP client + webhook types
internal/gateway/                     # Gateway interface + Hermes HTTP implementation
internal/delivery/                    # Message source abstraction (poller + webhook)
internal/security/                    # Allowlist, rate limiting, roles, session isolation
internal/transcribe/                  # Audio transcription (OpenAI, Groq, Deepgram, local)
internal/tailscale/                   # Auto Tailscale Funnel for webhooks
internal/preflight/                   # Pre-deployment validation checks
internal/commands/                    # Bridge-level command system
```

## Conventions

- **Go 1.22**, minimal deps (BurntSushi/toml only)
- Standard `log` package, no frameworks
- Table-driven tests with dependency injection (e.g., mockable `now()`)
- Errors wrapped with `fmt.Errorf` for context
- Context-based cancellation for all goroutines
- Interfaces for extensibility (`delivery.Source`, `gateway.Gateway`)
- No globals — all state in structs
- CGO disabled in builds

## CI/CD

- GitHub Actions: CI on push/PR (build, test, vet, fmt check)
- Releases: tag `v*` triggers GoReleaser → prebuilt binaries on GitHub Releases

## Config

Config file: `~/.config/hermes-whatsapp/config.toml`
Required env vars: `KAPSO_API_KEY`, `KAPSO_PHONE_NUMBER_ID`
Gateway env vars: `HERMES_URL` (default: http://127.0.0.1:8642), `HERMES_TOKEN`, `HERMES_MODEL`, `HERMES_SYSTEM_PROMPT`
Delivery modes: `polling` (default), `tailscale`, `domain`
