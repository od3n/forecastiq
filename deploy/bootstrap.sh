#!/usr/bin/env bash
# deploy/bootstrap.sh — Idempotent VPS provisioning for ForecastIQ.
# Run as root on a fresh Hetzner CX32 (Ubuntu 22.04+).
# Safe to re-run: every step is guarded.
#
# Reference: docs/delivery/04-infrastructure-as-code.md §3
# Reference: docs/architecture/06-deployment-architecture.md §3
#
# Usage: curl -sSL <raw-url> | bash   (or: scp + bash deploy/bootstrap.sh)
set -euo pipefail

echo "=== ForecastIQ VPS Bootstrap ==="
echo "Host: $(hostname) | Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo ""

# ── 1. System packages ───────────────────────────────────────────────────────
echo "[1/10] Installing system packages..."
apt-get update -qq
apt-get install -y -qq \
  caddy \
  postgresql-client \
  rclone \
  fail2ban \
  ufw \
  jq \
  curl \
  ca-certificates \
  gnupg

# Grafana Agent (official APT repo)
if ! command -v grafana-agent &>/dev/null; then
  mkdir -p /etc/apt/keyrings
  curl -fsSL https://apt.grafana.com/gpg.key | gpg --dearmor -o /etc/apt/keyrings/grafana.gpg
  echo "deb [signed-by=/etc/apt/keyrings/grafana.gpg] https://apt.grafana.com stable main" \
    > /etc/apt/sources.list.d/grafana.list
  apt-get update -qq
  apt-get install -y -qq grafana-agent
fi
echo "  Packages OK"

# ── 2. App user ──────────────────────────────────────────────────────────────
echo "[2/10] Creating app user..."
if ! id forecastiq &>/dev/null; then
  useradd --system --shell /usr/sbin/nologin --home-dir /opt/forecastiq forecastiq
fi
echo "  User: forecastiq (nologin)"

# ── 3. Volume ────────────────────────────────────────────────────────────────
echo "[3/10] Checking volume mount..."
VOLUME_MOUNT="/var/lib/forecastiq"
if mountpoint -q "$VOLUME_MOUNT"; then
  echo "  Volume already mounted at $VOLUME_MOUNT"
else
  echo "  WARNING: $VOLUME_MOUNT is not a mountpoint."
  echo "  Ensure the Hetzner volume is attached and mounted (fstab entry)."
  echo "  Creating directory as fallback for bootstrap completion..."
  mkdir -p "$VOLUME_MOUNT"
fi

# ── 4. Directories + permissions ─────────────────────────────────────────────
echo "[4/10] Creating directories..."
mkdir -p /opt/forecastiq/releases
mkdir -p "$VOLUME_MOUNT/payloads"
mkdir -p "$VOLUME_MOUNT/backups"
mkdir -p /etc/forecastiq
mkdir -p /var/log/caddy

# Create 'current' symlink placeholder if it doesn't exist
if [ ! -L /opt/forecastiq/current ]; then
  ln -sfn /opt/forecastiq/releases /opt/forecastiq/current
fi

chown -R forecastiq:forecastiq /opt/forecastiq
chown -R forecastiq:forecastiq "$VOLUME_MOUNT/payloads"
chown -R forecastiq:forecastiq "$VOLUME_MOUNT/backups"

# Secrets file (empty template if missing)
if [ ! -f /etc/forecastiq/secrets.env ]; then
  cat > /etc/forecastiq/secrets.env <<'EOF'
# ForecastIQ production secrets — managed by operator.
# Reference: docs/security/04-secrets-management.md
# FIQ_DATABASE_URL=postgres://...
# FIQ_PROVIDER_OPENWEATHER_API_KEY=...
# FIQ_SUPABASE_SERVICE_ROLE_KEY=...
# FIQ_AUTH_JWKS_URL=...
# FIQ_AUTH_ISSUER=...
# FIQ_AUTH_AUDIENCE=...
EOF
fi
chmod 0600 /etc/forecastiq/secrets.env
chown root:root /etc/forecastiq/secrets.env
echo "  Directories OK"

# ── 5. systemd unit ──────────────────────────────────────────────────────────
echo "[5/10] Installing systemd unit..."
cp /opt/forecastiq/releases/deploy/systemd/forecastiq.service /etc/systemd/system/forecastiq.service 2>/dev/null \
  || echo "  (unit file will be installed by first deploy)"
systemctl daemon-reload
systemctl enable forecastiq 2>/dev/null || true
echo "  systemd unit enabled (not started until first deploy)"

# ── 6. Caddyfile ─────────────────────────────────────────────────────────────
echo "[6/10] Installing Caddyfile..."
if [ -f /opt/forecastiq/releases/deploy/caddy/Caddyfile ]; then
  cp /opt/forecastiq/releases/deploy/caddy/Caddyfile /etc/caddy/Caddyfile
  caddy validate --config /etc/caddy/Caddyfile
else
  echo "  (Caddyfile will be installed by first deploy)"
fi
systemctl enable caddy
systemctl restart caddy 2>/dev/null || true
echo "  Caddy OK"

# ── 7. Firewall ──────────────────────────────────────────────────────────────
echo "[7/10] Configuring firewall (ufw)..."
ufw --force reset >/dev/null 2>&1
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp   comment "SSH"
ufw allow 80/tcp   comment "HTTP (Caddy redirect)"
ufw allow 443/tcp  comment "HTTPS (Caddy)"
# Metrics port localhost-only (grafana-agent scrapes same host)
ufw deny 9090/tcp  comment "Metrics (localhost only via bind)"
ufw --force enable
echo "  Firewall OK (22, 80, 443 open; 9090 denied external)"

# ── 8. fail2ban ──────────────────────────────────────────────────────────────
echo "[8/10] Configuring fail2ban..."
cat > /etc/fail2ban/jail.local <<'EOF'
[sshd]
enabled = true
port = ssh
filter = sshd
logpath = /var/log/auth.log
maxretry = 5
bantime = 3600
findtime = 600
EOF
systemctl enable fail2ban
systemctl restart fail2ban
echo "  fail2ban OK (SSH jail active)"

# ── 9. Cron entries (placeholders for WP-24) ─────────────────────────────────
echo "[9/10] Setting up cron placeholders..."
CRON_FILE="/etc/cron.d/forecastiq"
if [ ! -f "$CRON_FILE" ]; then
  cat > "$CRON_FILE" <<'EOF'
# ForecastIQ scheduled tasks (WP-24 will populate these)
SHELL=/bin/bash
PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

# Nightly backup (WP-24): 02:30 UTC
# 30 2 * * * root /opt/forecastiq/releases/deploy/scripts/backup.sh >> /var/log/forecastiq-backup.log 2>&1

# Monthly restore test (WP-24): 1st of month, 04:00 UTC
# 0 4 1 * * root /opt/forecastiq/releases/deploy/scripts/restore-test.sh >> /var/log/forecastiq-restore-test.log 2>&1
EOF
fi
echo "  Cron placeholders OK (backup/restore commented until WP-24)"

# ── 10. Verification ─────────────────────────────────────────────────────────
echo ""
echo "=== Bootstrap Verification ==="
echo "  User forecastiq:  $(id forecastiq 2>/dev/null && echo OK || echo MISSING)"
echo "  Volume mount:     $(mountpoint -q /var/lib/forecastiq && echo MOUNTED || echo NOT_MOUNTED)"
echo "  Releases dir:     $([ -d /opt/forecastiq/releases ] && echo OK || echo MISSING)"
echo "  Secrets file:     $([ -f /etc/forecastiq/secrets.env ] && echo OK || echo MISSING)"
echo "  Caddy enabled:    $(systemctl is-enabled caddy 2>/dev/null || echo MISSING)"
echo "  FIQ unit:         $(systemctl is-enabled forecastiq 2>/dev/null || echo NOT_YET)"
echo "  UFW active:       $(ufw status | head -1)"
echo "  fail2ban:         $(systemctl is-active fail2ban 2>/dev/null || echo INACTIVE)"
echo ""
echo "=== Bootstrap Complete ==="
echo "Next: deploy the first release, then 'systemctl start forecastiq'."
