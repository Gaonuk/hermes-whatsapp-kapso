# hermes-whatsapp-kapso

WhatsApp bridge for [Hermes Agent](https://github.com/NousResearch/hermes-agent) via [Kapso](https://kapso.ai) WhatsApp Cloud API.

Ported from [openclaw-kapso-whatsapp](https://github.com/Enriquefft/openclaw-kapso-whatsapp) — replacing the OpenClaw/ZeroClaw WebSocket gateway with Hermes Agent's OpenAI-compatible HTTP API.

## How it works

```
WhatsApp → Kapso Cloud API → hermes-whatsapp-bridge → Hermes Agent
                                      ↓
                         Extract text, transcribe audio,
                         check security, dispatch commands
                                      ↓
                         Format response, send reply back through Kapso
```

The bridge receives WhatsApp messages through Kapso (via polling or webhooks), applies security checks and rate limiting, and forwards them to a running Hermes Agent instance. Agent replies are formatted for WhatsApp and sent back to the user.

## Quick start

### Prerequisites

- Go 1.22+
- A [Kapso](https://kapso.ai) account with WhatsApp Cloud API access
- A running [hermes-agent](https://github.com/NousResearch/hermes-agent) instance

### Install from source

```bash
git clone https://github.com/Gaonuk/hermes-whatsapp-kapso.git
cd hermes-whatsapp-kapso
go build ./cmd/hermes-whatsapp-bridge
go build ./cmd/hermes-whatsapp-cli
```

### Install from release

```bash
curl -fsSL https://raw.githubusercontent.com/Gaonuk/hermes-whatsapp-kapso/main/scripts/install.sh | bash
```

### Configure

```bash
export KAPSO_API_KEY="your-kapso-api-key"
export KAPSO_PHONE_NUMBER_ID="your-phone-number-id"
export HERMES_URL="http://127.0.0.1:8642"      # hermes-agent API server
# export HERMES_TOKEN="optional-bearer-token"   # if API_SERVER_KEY is set
# export HERMES_MODEL="hermes-agent"            # model name (default)
# export HERMES_SYSTEM_PROMPT="You are..."      # optional system prompt
```

Or use a config file at `~/.config/hermes-whatsapp/config.toml`:

```toml
[kapso]
api_key = "your-kapso-api-key"
phone_number_id = "your-phone-number-id"

[gateway]
url = "http://127.0.0.1:8642"
# token = "optional-bearer-token"
# model = "hermes-agent"
# system_prompt = "You are a helpful assistant."
# session_key = "main"
# error_message = "Sorry, I ran into an issue processing your message."

[delivery]
mode = "polling"         # polling | tailscale | domain
poll_interval = 30       # seconds

[security]
mode = "allowlist"       # allowlist | open
session_isolation = true
rate_limit = 10
rate_window = 60

[security.roles]
admin = ["+15551234567"]
member = ["+15559876543"]
```

### Run

```bash
# Verify configuration
./hermes-whatsapp-cli preflight

# Start the bridge
./hermes-whatsapp-bridge
```

## Features

### Message delivery modes

| Mode | Description | Latency |
|------|-------------|---------|
| `polling` (default) | Queries Kapso API every N seconds | ~30s |
| `tailscale` | Auto-creates encrypted tunnel via Tailscale Funnel | Sub-second |
| `domain` | Your own reverse proxy points to the webhook server | Sub-second |

### Security

- **Sender allowlist** — only authorized phone numbers can interact
- **Per-sender rate limiting** — token bucket prevents abuse
- **Role-based access** — phone numbers mapped to roles (admin, member, etc.)
- **Session isolation** — each sender gets a separate conversation context

### Audio transcription

Supports multiple providers for voice message transcription:

| Provider | Env var | Notes |
|----------|---------|-------|
| OpenAI Whisper | `KAPSO_TRANSCRIBE_API_KEY` | Default model: whisper-1 |
| Groq | `KAPSO_TRANSCRIBE_API_KEY` | Default model: whisper-large-v3 |
| Deepgram | `KAPSO_TRANSCRIBE_API_KEY` | Default model: nova-3 |
| Local whisper-cli | — | Requires ffmpeg + whisper-cli in PATH |

Set `KAPSO_TRANSCRIBE_PROVIDER` to one of: `openai`, `groq`, `deepgram`, `local`.

### Bridge commands

Configure bridge-level commands that are intercepted before reaching the agent:

```toml
[commands]
prefix = "!"
timeout = 30

[commands.definitions.status]
type = "shell"
description = "Show bridge status"
shell = "echo 'Bridge is running'"

[commands.definitions.ask]
type = "agent"
description = "Ask the agent directly"
prompt = "{args}"
```

## CLI

```bash
# Send a message
hermes-whatsapp-cli send --to +15551234567 --text "Hello!"

# Check webhook server health
hermes-whatsapp-cli status

# Run preflight checks
hermes-whatsapp-cli preflight
```

## Environment variables

| Variable | Description | Default |
|----------|-------------|---------|
| `KAPSO_API_KEY` | Kapso API key (required) | — |
| `KAPSO_PHONE_NUMBER_ID` | WhatsApp phone number ID (required) | — |
| `HERMES_URL` | Hermes agent API URL | `http://127.0.0.1:8642` |
| `HERMES_TOKEN` | Bearer token for hermes-agent auth | — |
| `HERMES_MODEL` | Model name for chat completions | `hermes-agent` |
| `HERMES_SYSTEM_PROMPT` | System prompt prepended to conversations | — |
| `HERMES_SESSION_KEY` | Base session key | `main` |
| `KAPSO_MODE` | Delivery mode: polling, tailscale, domain | `polling` |
| `KAPSO_POLL_INTERVAL` | Polling interval in seconds | `30` |
| `KAPSO_SECURITY_MODE` | Security mode: allowlist, open | `allowlist` |
| `KAPSO_ALLOWED_NUMBERS` | Comma-separated phone numbers | — |
| `KAPSO_TRANSCRIBE_PROVIDER` | Transcription provider | — |
| `KAPSO_TRANSCRIBE_API_KEY` | Transcription API key | — |

## Development

```bash
just build    # Build both binaries
just test     # Run tests
just check    # Run tests + vet + fmt check
just lint     # Run golangci-lint
```

## License

MIT — see [LICENSE](LICENSE).

## Credits

Ported from [openclaw-kapso-whatsapp](https://github.com/Enriquefft/openclaw-kapso-whatsapp) by [@Enriquefft](https://github.com/Enriquefft).
