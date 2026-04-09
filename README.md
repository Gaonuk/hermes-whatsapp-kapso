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

The bridge sends only the **final text response** from the agent — tool calls, intermediate reasoning, and streaming chunks are not forwarded to WhatsApp.

## Supported message types

| Type | Support | What the agent sees |
|------|---------|---------------------|
| Text | Full | Full message text |
| Voice (with transcription) | Full | `[voice] transcribed text` |
| Voice (no transcription) | Partial | `[audio] (audio/ogg)` |
| Image | Partial | `[image] caption (mime) media_url` |
| Document | Partial | `[document] filename (mime) media_url` |
| Video | Partial | `[video] caption (mime) media_url` |
| Location | Full | `[location] name address (lat, lng)` |

Images, documents, and videos are forwarded as text descriptions with media URLs — the agent cannot visually inspect them since we use the text-only chat completions endpoint.

## Setup guide (Tailscale on Hetzner VPS)

This is a step-by-step guide to deploy the bridge on a Hetzner VPS using Tailscale Funnel for sub-second webhook delivery.

### 1. Install Tailscale

```bash
curl -fsSL https://tailscale.com/install.sh | sh
sudo tailscale up
sudo tailscale set --operator=$USER
```

### 2. Install Hermes Agent

```bash
bash <(curl -s https://install.nous.ai/hermes)
hermes setup
```

If running on a headless server, enable linger so the gateway service persists:

```bash
sudo loginctl enable-linger $USER
hermes gateway restart
```

Verify hermes-agent is running:

```bash
curl http://127.0.0.1:8642/health
```

### 3. Install the bridge

```bash
# Install Go if not present
sudo apt install -y golang-go

# Clone and build
git clone https://github.com/Gaonuk/hermes-whatsapp-kapso.git
cd hermes-whatsapp-kapso
go mod tidy
go build ./cmd/hermes-whatsapp-bridge
go build ./cmd/hermes-whatsapp-cli
```

### 4. Get your Kapso credentials

From [kapso.ai](https://kapso.ai):
- **API Key** — found in your account settings
- **Phone Number ID** — the WhatsApp phone number ID linked to your account

### 5. Generate a webhook verify token

```bash
openssl rand -hex 32
```

Save this — you'll set it as `KAPSO_WEBHOOK_VERIFY_TOKEN` and paste it in Kapso when registering the webhook.

### 6. Configure and run

```bash
export KAPSO_API_KEY="your-kapso-api-key"
export KAPSO_PHONE_NUMBER_ID="your-phone-number-id"
export KAPSO_MODE="tailscale"
export KAPSO_WEBHOOK_VERIFY_TOKEN="your-generated-token"
export KAPSO_ALLOWED_NUMBERS="+15551234567"  # your WhatsApp number
export HERMES_URL="http://127.0.0.1:8642"

./hermes-whatsapp-bridge
```

The bridge will output something like:

```
connected to hermes-agent at http://127.0.0.1:8642
tailscale funnel started on port 18790 → https://your-vps.tailXXXXX.ts.net/webhook
register this webhook URL in Kapso: https://your-vps.tailXXXXX.ts.net/webhook
webhook server listening on [::]:18790
```

### 7. Register the webhook in Kapso

In Kapso's dashboard:
1. Go to webhook settings
2. Set the webhook URL to the one printed by the bridge (e.g. `https://your-vps.tailXXXXX.ts.net/webhook`)
3. Set the verify token to the value of `KAPSO_WEBHOOK_VERIFY_TOKEN`
4. Kapso will give you a **webhook secret** — set it as an env var:

```bash
export KAPSO_WEBHOOK_SECRET="the-secret-kapso-gave-you"
```

Then restart the bridge with all env vars set.

### 8. Test it

Send a WhatsApp message to your Kapso number. You should see the message flow through in the bridge logs and get a reply from hermes-agent.

### 9. Run as a background service (recommended)

Instead of running the bridge in the foreground, install it as a systemd user service so it runs in the background, auto-restarts on failure, and survives reboots:

```bash
# Install the service
./scripts/install-service.sh

# Edit your env file with your credentials
nano ~/.config/hermes-whatsapp/env

# Start it
systemctl --user start hermes-whatsapp-bridge

# Check status
systemctl --user status hermes-whatsapp-bridge

# View logs (live)
journalctl --user -u hermes-whatsapp-bridge -f

# Enable linger so it survives reboots
sudo loginctl enable-linger $USER
```

To stop or restart:

```bash
systemctl --user stop hermes-whatsapp-bridge
systemctl --user restart hermes-whatsapp-bridge
```

## Quick start (polling mode)

If you don't need webhooks, polling mode works with zero networking setup:

```bash
export KAPSO_API_KEY="your-key"
export KAPSO_PHONE_NUMBER_ID="your-phone-id"
export KAPSO_ALLOWED_NUMBERS="+15551234567"
export HERMES_URL="http://127.0.0.1:8642"

./hermes-whatsapp-bridge
```

Messages are checked every 30 seconds (configurable via `KAPSO_POLL_INTERVAL`).

## Configuration

### Config file

`~/.config/hermes-whatsapp/config.toml` (or set `HERMES_CONFIG` to a custom path):

```toml
[kapso]
api_key = "your-kapso-api-key"
phone_number_id = "your-phone-number-id"

[gateway]
url = "http://127.0.0.1:8642"
# token = "optional-bearer-token"     # if hermes-agent has API_SERVER_KEY set
# model = "hermes-agent"              # model name sent in chat completions
# system_prompt = "You are a helpful assistant."
# session_key = "main"
# error_message = "Sorry, I ran into an issue processing your message."

[delivery]
mode = "tailscale"       # polling | tailscale | domain
# poll_interval = 30     # seconds (polling mode only)
# poll_fallback = false  # also poll when using webhook modes

[webhook]
# addr = ":18790"                     # webhook listen address
# verify_token = "your-token"         # for Meta webhook verification handshake
# secret = "kapso-provided-secret"    # for HMAC signature validation

[security]
mode = "allowlist"       # allowlist | open
session_isolation = true
rate_limit = 10
rate_window = 60

[security.roles]
admin = ["+15551234567"]
member = ["+15559876543"]

[transcribe]
# provider = "groq"                   # openai | groq | deepgram | local
# api_key = "your-transcribe-key"
# model = "whisper-large-v3"
# language = "en"
```

### Environment variables

All env vars override config file values.

#### Required

| Variable | Description |
|----------|-------------|
| `KAPSO_API_KEY` | Kapso API key |
| `KAPSO_PHONE_NUMBER_ID` | WhatsApp phone number ID |

#### Gateway (Hermes Agent)

| Variable | Description | Default |
|----------|-------------|---------|
| `HERMES_URL` | Hermes agent API URL | `http://127.0.0.1:8642` |
| `HERMES_TOKEN` | Bearer token for hermes-agent auth | — |
| `HERMES_MODEL` | Model name for chat completions | `hermes-agent` |
| `HERMES_SYSTEM_PROMPT` | System prompt prepended to conversations | — |
| `HERMES_SESSION_KEY` | Base session key | `main` |
| `HERMES_ERROR_MESSAGE` | Message sent when agent fails | `Sorry, I ran into an issue...` |

#### Delivery

| Variable | Description | Default |
|----------|-------------|---------|
| `KAPSO_MODE` | Delivery mode: `polling`, `tailscale`, `domain` | `polling` |
| `KAPSO_POLL_INTERVAL` | Polling interval in seconds | `30` |
| `KAPSO_POLL_FALLBACK` | Also poll when using webhook modes (`true`/`false`) | `false` |
| `KAPSO_WEBHOOK_ADDR` | Webhook listen address | `:18790` |
| `KAPSO_WEBHOOK_VERIFY_TOKEN` | Token for webhook verification handshake | — |
| `KAPSO_WEBHOOK_SECRET` | Secret for HMAC webhook signature validation | — |

#### Security

| Variable | Description | Default |
|----------|-------------|---------|
| `KAPSO_SECURITY_MODE` | `allowlist` or `open` | `allowlist` |
| `KAPSO_ALLOWED_NUMBERS` | Comma-separated phone numbers (added to default role) | — |
| `KAPSO_SESSION_ISOLATION` | Per-sender session isolation (`true`/`false`) | `true` |
| `KAPSO_RATE_LIMIT` | Max messages per rate window | `10` |
| `KAPSO_RATE_WINDOW` | Rate limit window in seconds | `60` |
| `KAPSO_DEFAULT_ROLE` | Default role for allowed numbers | `member` |
| `KAPSO_DENY_MESSAGE` | Message sent to unauthorized senders | `Sorry, you are not authorized...` |

#### Transcription

| Variable | Description | Default |
|----------|-------------|---------|
| `KAPSO_TRANSCRIBE_PROVIDER` | `openai`, `groq`, `deepgram`, or `local` | — (disabled) |
| `KAPSO_TRANSCRIBE_API_KEY` | API key for cloud transcription providers | — |
| `KAPSO_TRANSCRIBE_MODEL` | Model override | Provider-specific default |
| `KAPSO_TRANSCRIBE_LANGUAGE` | Language hint (e.g. `en`, `es`) | — (auto-detect) |
| `KAPSO_TRANSCRIBE_MAX_AUDIO_SIZE` | Max audio file size in bytes | `26214400` (25MB) |

## Features

### Message delivery modes

| Mode | Description | Latency | Setup |
|------|-------------|---------|-------|
| `polling` (default) | Queries Kapso API every N seconds | ~30s | None |
| `tailscale` | Auto-creates encrypted tunnel via Tailscale Funnel | Sub-second | Tailscale installed |
| `domain` | Your own reverse proxy points to the webhook server | Sub-second | DNS + reverse proxy |

### Security

- **Sender allowlist** — only authorized phone numbers can interact
- **Per-sender rate limiting** — token bucket prevents abuse (default: 10 messages per 60 seconds)
- **Role-based access** — phone numbers mapped to roles (admin, member, etc.)
- **Session isolation** — each sender gets a separate conversation context
- **Webhook signature validation** — HMAC-SHA256 verification of incoming webhooks
- **SSRF protection** — media downloads restricted to Kapso/WhatsApp/Facebook CDN domains

### Audio transcription

Supports multiple providers for voice message transcription:

| Provider | Default model | Notes |
|----------|---------------|-------|
| OpenAI Whisper | `whisper-1` | Most accurate |
| Groq | `whisper-large-v3` | Fastest |
| Deepgram | `nova-3` | Smart formatting |
| Local whisper-cli | — | Requires ffmpeg + whisper-cli in PATH |

Transcription includes retry with exponential backoff and optional caching.

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
