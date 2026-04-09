#!/usr/bin/env bash
set -euo pipefail

# Install hermes-whatsapp-bridge as a user-level systemd service.
# Run this as a normal user (not root).

SERVICE_NAME="hermes-whatsapp-bridge"
SERVICE_DIR="${HOME}/.config/systemd/user"
ENV_FILE="${HOME}/.config/hermes-whatsapp/env"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DEPLOY_DIR="${SCRIPT_DIR}/../deploy"

# --- Ensure systemd user dir exists ---
mkdir -p "$SERVICE_DIR"

# --- Create env file template if missing ---
mkdir -p "$(dirname "$ENV_FILE")"
if [ ! -f "$ENV_FILE" ]; then
  cat > "$ENV_FILE" <<'ENVEOF'
# Required
KAPSO_API_KEY=
KAPSO_PHONE_NUMBER_ID=

# Delivery (polling | tailscale | domain)
# KAPSO_MODE=polling

# Webhook (for tailscale/domain modes)
# KAPSO_WEBHOOK_VERIFY_TOKEN=
# KAPSO_WEBHOOK_SECRET=

# Gateway
# HERMES_URL=http://127.0.0.1:8642
# HERMES_TOKEN=
# HERMES_MODEL=hermes-agent

# Security
# KAPSO_ALLOWED_NUMBERS=+15551234567
# KAPSO_SECURITY_MODE=allowlist

# Transcription
# KAPSO_TRANSCRIBE_PROVIDER=
# KAPSO_TRANSCRIBE_API_KEY=
ENVEOF
  chmod 600 "$ENV_FILE"
  echo "Created env file template: $ENV_FILE"
  echo "  -> Fill in your values before starting the service."
else
  echo "Env file already exists: $ENV_FILE"
fi

# --- Install service file ---
cp "$DEPLOY_DIR/${SERVICE_NAME}.service" "$SERVICE_DIR/${SERVICE_NAME}.service"
echo "Installed service file: $SERVICE_DIR/${SERVICE_NAME}.service"

# --- Reload and enable ---
systemctl --user daemon-reload
systemctl --user enable "$SERVICE_NAME"

echo
echo "Service installed. Next steps:"
echo "  1. Edit your env file:  nano $ENV_FILE"
echo "  2. Start the service:   systemctl --user start $SERVICE_NAME"
echo "  3. Check status:        systemctl --user status $SERVICE_NAME"
echo "  4. View logs:           journalctl --user -u $SERVICE_NAME -f"
echo
echo "To survive reboots, enable linger:"
echo "  sudo loginctl enable-linger \$USER"
