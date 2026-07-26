#!/usr/bin/env bash
# deploy/bootstrap.sh — Idempotent VPS provisioning for ForecastIQ.
# Run as root on a fresh Hetzner CX32 (Ubuntu 22.04+).
# Safe to re-run: every step is guarded.
#
# Reference: docs/delivery/04-infrastructure-as-code.md §3
# Reference: docs/architecture/06-deployment-architecture.md §3
#
# Usage: bash deploy/bootstrap.sh   (as root; review before piping from curl)
#
# Environment (optional):
#   FIQ_DOMAIN     API domain served by Caddy (default api.forecastiq.example)
#   DEPLOY_PUBKEY  SSH public key installed for the 'deploy' user
set -euo pipefail

if [ "${EUID:-$(id -u)}" -ne 0 ]; then
  echo "ERROR: bootstrap.sh must run as root." >&2
  exit 1
fi

FIQ_DOMAIN="${FIQ_DOMAIN:-api.forecastiq.example}"
DEPLOY_PUBKEY="${DEPLOY_PUBKEY:-}"

echo "=== ForecastIQ VPS Bootstrap ==="
echo "Host: $(hostname) | Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "Domain: ${FIQ_DOMAIN}"
echo ""

# ── 1. System packages ──────────────────────────────────────────────
echo "[1/11] Installing system packages..."
apt-get update -qq
apt-get install -y -qq \
  postgresql-client \
  rclone \
  fail2ban \
  ufw \
  jq \
  curl \
  ca-certificates \
  gnupg

# Caddy (official cloudsmith APT repo — not in the Ubuntu jammy archives).
if ! command -v caddy &>/dev/null; then
  mkdir -p /etc/apt/keyrings
  curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' \
    | gpg --dearmor -o /etc/apt/keyrings/caddy-stable.gpg
  curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' \
    | sed 's|^deb |deb [signed-by=/etc/apt/keyrings/caddy-stable.gpg] |' \
    > /etc/apt/sources.list.d/caddy-stable.list
  apt-get update -qq
  apt-get install -y -qq caddy
fi

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

# ── 2. Users ────────────────────────────────────────────────────────
echo "[2/11] Creating users..."
if ! id forecastiq &>/dev/null; then
  useradd --system --shell /usr/sbin/nologin --home-dir /opt/forecastiq forecastiq
fi
# Deploy user: owns the release tree; privileged steps go only through the
# scoped sudo wrappers installed in step 5 (never full root).
if ! id deploy &>/dev/null; then
  useradd --create-home --shell /bin/bash deploy
fi
if [ -n "$DEPLOY_PUBKEY" ]; then
  install -d -m 0700 -o deploy -g deploy /home/deploy/.ssh
  touch /home/deploy/.ssh/authorized_keys
  if ! grep -qsF "$DEPLOY_PUBKEY" /home/deploy/.ssh/authorized_keys; then
    echo "$DEPLOY_PUBKEY" >> /home/deploy/.ssh/authorized_keys
  fi
  chown deploy:deploy /home/deploy/.ssh/authorized_keys
  chmod 0600 /home/deploy/.ssh/authorized_keys
else
  echo "  NOTE: DEPLOY_PUBKEY not set — add /home/deploy/.ssh/authorized_keys manually."
fi
echo "  Users: forecastiq (nologin), deploy (ssh)"

# ── 3. Volume ───────────────────────────────────────────────────────
echo "[3/11] Checking volume mount..."
VOLUME_MOUNT="/var/lib/forecastiq"
if mountpoint -q "$VOLUME_MOUNT"; then
  echo "  Volume already mounted at $VOLUME_MOUNT"
else
  echo "  WARNING: $VOLUME_MOUNT is not a mountpoint."
  echo "  Ensure the Hetzner volume is attached and mounted (fstab entry)."
  echo "  Creating directory as fallback for bootstrap completion..."
  mkdir -p "$VOLUME_MOUNT"
fi

# ── 4. Directories + permissions ────────────────────────────────────
echo "[4/11] Creating directories..."
mkdir -p /opt/forecastiq/releases
mkdir -p "$VOLUME_MOUNT/payloads"
mkdir -p "$VOLUME_MOUNT/backups"
mkdir -p /etc/forecastiq
mkdir -p /var/log/caddy

# The release tree belongs to the deploy user (rsync target + symlink flips);
# runtime data belongs to the app user. Never re-chown releases to forecastiq
# on re-run — that would strip the deployer's write access.
chown deploy:deploy /opt/forecastiq /opt/forecastiq/releases
chown -R forecastiq:forecastiq "$VOLUME_MOUNT/payloads"
chown -R forecastiq:forecastiq "$VOLUME_MOUNT/backups"
# Caddy runs as user 'caddy' and must own its access-log directory.
chown caddy:caddy /var/log/caddy

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

# ── 5. Scoped sudo wrappers for the deploy user ─────────────────────
echo "[5/11] Installing deploy sudo wrappers..."
cat > /usr/local/bin/forecastiq-install-configs <<'EOF'
#!/usr/bin/env bash
# Installs the current release's systemd unit + Caddyfile, then reloads.
# Root-owned wrapper: the only way 'deploy' touches /etc (via sudoers).
set -euo pipefail
RELEASE="/opt/forecastiq/current"
install -m 0644 "$RELEASE/deploy/systemd/forecastiq.service" /etc/systemd/system/forecastiq.service
install -m 0644 "$RELEASE/deploy/caddy/Caddyfile" /etc/caddy/Caddyfile
caddy validate --config /etc/caddy/Caddyfile
systemctl daemon-reload
systemctl reload caddy 2>/dev/null || systemctl restart caddy
EOF
cat > /usr/local/bin/forecastiq-migrate <<'EOF'
#!/usr/bin/env bash
# Runs database migrations for the current release with production secrets.
# Root-owned wrapper: 'deploy' cannot read /etc/forecastiq/secrets.env itself.
# Args are fixed to 'migrate up' — rollbacks are an operator action.
set -euo pipefail
set -a
. /etc/forecastiq/secrets.env
set +a
exec /opt/forecastiq/current/forecastiq migrate up
EOF
chmod 0755 /usr/local/bin/forecastiq-install-configs /usr/local/bin/forecastiq-migrate
chown root:root /usr/local/bin/forecastiq-install-configs /usr/local/bin/forecastiq-migrate

cat > /etc/sudoers.d/forecastiq-deploy <<'EOF'
# Deploy user: exactly the three privileged deploy steps, nothing else.
deploy ALL=(root) NOPASSWD: /usr/local/bin/forecastiq-install-configs
deploy ALL=(root) NOPASSWD: /usr/local/bin/forecastiq-migrate
deploy ALL=(root) NOPASSWD: /usr/bin/systemctl restart forecastiq
EOF
chmod 0440 /etc/sudoers.d/forecastiq-deploy
visudo -cf /etc/sudoers.d/forecastiq-deploy
echo "  Wrappers + sudoers OK"

# ── 6. systemd + Caddy domain drop-in ───────────────────────────────
echo "[6/11] Configuring systemd + Caddy..."
# The repo Caddyfile references {$FIQ_DOMAIN}; expose it to the caddy unit.
mkdir -p /etc/systemd/system/caddy.service.d
cat > /etc/systemd/system/caddy.service.d/forecastiq.conf <<EOF
[Service]
Environment=FIQ_DOMAIN=${FIQ_DOMAIN}
EOF
systemctl daemon-reload
# Unit file + Caddyfile are installed by the first deploy (step 5 wrapper);
# enable them so they come up on boot once installed.
if [ -f /opt/forecastiq/current/deploy/systemd/forecastiq.service ]; then
  /usr/local/bin/forecastiq-install-configs
fi
systemctl enable caddy
systemctl enable forecastiq 2>/dev/null || true
echo "  systemd + Caddy OK (services start after first deploy)"

# ── 7. Firewall ─────────────────────────────────────────────────────
echo "[7/11] Configuring firewall (ufw)..."
# Rule additions are idempotent — never 'ufw reset', which would leave the
# host unfirewalled if a later step fails mid-run.
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp   comment "SSH"
ufw allow 80/tcp   comment "HTTP (Caddy redirect)"
ufw allow 443/tcp  comment "HTTPS (Caddy)"
# Metrics port localhost-only (grafana-agent scrapes same host)
ufw deny 9090/tcp  comment "Metrics (localhost only via bind)"
ufw --force enable
echo "  Firewall OK (22, 80, 443 open; 9090 denied external)"

# ── 8. fail2ban ─────────────────────────────────────────────────────
echo "[8/11] Configuring fail2ban..."
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

# ── 9. Cron entries (placeholders for WP-24) ────────────────────────
echo "[9/11] Setting up cron placeholders..."
CRON_FILE="/etc/cron.d/forecastiq"
if [ ! -f "$CRON_FILE" ]; then
  cat > "$CRON_FILE" <<'EOF'
# ForecastIQ scheduled tasks (WP-24 will populate these)
SHELL=/bin/bash
PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

# Nightly backup (WP-24): 02:30 UTC
# 30 2 * * * root /opt/forecastiq/current/deploy/scripts/backup.sh >> /var/log/forecastiq-backup.log 2>&1

# Monthly restore test (WP-24): 1st of month, 04:00 UTC
# 0 4 1 * * root /opt/forecastiq/current/deploy/scripts/restore-test.sh >> /var/log/forecastiq-restore-test.log 2>&1
EOF
fi
echo "  Cron placeholders OK (backup/restore commented until WP-24)"

# ── 10. Release placeholder ─────────────────────────────────────────
echo "[10/11] Checking release symlink..."
# No placeholder symlink: 'current' appears with the first deploy. A symlink
# pointing at the releases *container* would poison rollback target selection.
if [ -L /opt/forecastiq/current ] && [ "$(readlink -f /opt/forecastiq/current)" = "/opt/forecastiq/releases" ]; then
  rm /opt/forecastiq/current
  echo "  Removed legacy placeholder symlink"
fi
echo "  Release dir ready (current appears on first deploy)"

# ── 11. Verification ────────────────────────────────────────────────
echo ""
echo "=== Bootstrap Verification ==="
echo "  User forecastiq:  $(id forecastiq &>/dev/null && echo OK || echo MISSING)"
echo "  User deploy:      $(id deploy &>/dev/null && echo OK || echo MISSING)"
echo "  Sudo wrappers:    $([ -x /usr/local/bin/forecastiq-install-configs ] && echo OK || echo MISSING)"
echo "  Volume mount:     $(mountpoint -q /var/lib/forecastiq && echo MOUNTED || echo NOT_MOUNTED)"
echo "  Releases dir:     $([ -d /opt/forecastiq/releases ] && echo OK || echo MISSING)"
echo "  Secrets file:     $([ -f /etc/forecastiq/secrets.env ] && echo OK || echo MISSING)"
echo "  Caddy enabled:    $(systemctl is-enabled caddy 2>/dev/null || echo MISSING)"
echo "  Caddy domain:     ${FIQ_DOMAIN}"
echo "  FIQ unit:         $(systemctl is-enabled forecastiq 2>/dev/null || echo NOT_YET)"
echo "  UFW active:       $(ufw status | head -1)"
echo "  fail2ban:         $(systemctl is-active fail2ban 2>/dev/null || echo INACTIVE)"
echo ""
echo "=== Bootstrap Complete ==="
echo "Next: deploy the first release, then 'systemctl start forecastiq'."
