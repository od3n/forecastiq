#!/usr/bin/env bash
# deploy/bootstrap.sh — Idempotent EC2 provisioning for ForecastIQ.
# Run as root on Ubuntu 22.04+ (AWS EC2 t3.small; ADR-033 personal-use scale).
# Safe to re-run: every step is guarded.
#
# The instance itself is provisioned by an external Terraform project; this
# script only prepares the OS: Docker Engine + compose plugin, the deploy
# user, data directories, firewall, and fail2ban. The app runs entirely in
# containers (deploy/compose/docker-compose.prod.yml) — nothing app-specific
# is installed on the host.
#
# Reference: docs/adr/ADR-033-personal-use-ec2-docker-deployment.md
# Reference: docs/architecture/06-deployment-architecture.md §3 (amended)
#
# Usage: bash deploy/bootstrap.sh   (as root; review before piping from curl)
#
# Environment (optional):
#   DEPLOY_PUBKEY  SSH public key installed for the 'deploy' user
set -euo pipefail

if [ "${EUID:-$(id -u)}" -ne 0 ]; then
  echo "ERROR: bootstrap.sh must run as root." >&2
  exit 1
fi

DEPLOY_PUBKEY="${DEPLOY_PUBKEY:-}"

echo "=== ForecastIQ EC2 Bootstrap (Docker) ==="
echo "Host: $(hostname) | Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo ""

# ── 1. System packages + Docker Engine ──────────────────────────────
echo "[1/7] Installing packages + Docker..."
apt-get update -qq
apt-get install -y -qq fail2ban ufw jq curl ca-certificates gnupg rclone

if ! command -v docker &>/dev/null; then
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
  chmod a+r /etc/apt/keyrings/docker.asc
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] \
https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
    > /etc/apt/sources.list.d/docker.list
  apt-get update -qq
  apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-compose-plugin
fi
systemctl enable --now docker
echo "  Docker: $(docker --version)"

# ── 2. Deploy user ──────────────────────────────────────────────────
echo "[2/7] Creating deploy user..."
if ! id deploy &>/dev/null; then
  useradd --create-home --shell /bin/bash deploy
fi
# docker group membership is the entire privilege model (ADR-033: acceptable
# for a single-operator personal deployment; docker group ≈ root on this box).
usermod -aG docker deploy
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
echo "  User: deploy (docker group)"

# ── 3. Directories + secrets template ───────────────────────────────
echo "[3/7] Creating directories..."
mkdir -p /opt/forecastiq
mkdir -p /var/lib/forecastiq/pgdata
mkdir -p /var/lib/forecastiq/payloads
mkdir -p /var/lib/forecastiq/backups
mkdir -p /etc/forecastiq
touch /var/lib/forecastiq/backup-status.json
chown -R deploy:deploy /opt/forecastiq
# Backups + status file are written by the deploy user's cron jobs (WP-24).
# The app container (uid 65532) only READS the status file.
chown -R deploy:deploy /var/lib/forecastiq/backups
chown deploy:deploy /var/lib/forecastiq/backup-status.json
chmod 0644 /var/lib/forecastiq/backup-status.json
# The app container runs as distroless nonroot (uid 65532) and must write
# payloads. pgdata stays root-owned: the postgres image entrypoint fixes its
# own permissions at init.
chown -R 65532:65532 /var/lib/forecastiq/payloads

if [ ! -f /etc/forecastiq/secrets.env ]; then
  cat > /etc/forecastiq/secrets.env <<'EOF'
# ForecastIQ production secrets — managed by operator; mounted into the app
# container via compose env_file. POSTGRES_PASSWORD lives in /opt/forecastiq/.env
# (compose interpolation), everything else here.
# Reference: docs/security/04-secrets-management.md
# FIQ_PROVIDER_OPENWEATHER_API_KEY=...
# FIQ_SUPABASE_SERVICE_ROLE_KEY=...
# FIQ_AUTH_JWKS_URL=...
# FIQ_AUTH_ISSUER=...
# FIQ_AUTH_AUDIENCE=...
EOF
fi
chmod 0600 /etc/forecastiq/secrets.env
# The app container runs as a non-root distroless user but compose reads the
# env_file from the daemon side; deploy needs read access for `compose config`.
chown deploy:deploy /etc/forecastiq/secrets.env

if [ ! -f /opt/forecastiq/.env ]; then
  cat > /opt/forecastiq/.env <<'EOF'
# Compose interpolation values (operator-managed).
# POSTGRES_PASSWORD=<generate: openssl rand -hex 24>
# FIQ_IMAGE is written by deploy.sh on every release.
EOF
  chown deploy:deploy /opt/forecastiq/.env
  chmod 0600 /opt/forecastiq/.env
fi
echo "  Directories OK"

# ── 4. Firewall ─────────────────────────────────────────────────────
echo "[4/7] Configuring firewall (ufw)..."
# Rule additions are idempotent — never 'ufw reset', which would leave the
# host unfirewalled if a later step fails mid-run. TLS terminates at
# Cloudflare (proxied DNS); the origin serves HTTP :80 only (ADR-033).
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp comment "SSH"
ufw allow 80/tcp comment "HTTP (Cloudflare origin)"
ufw --force enable
echo "  Firewall OK (22, 80 open)"

# ── 5. fail2ban ─────────────────────────────────────────────────────
echo "[5/7] Configuring fail2ban..."
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

# ── 6. Cron entries (WP-24 backup + restore test) ───────────────────
echo "[6/7] Installing cron jobs..."
CRON_FILE="/etc/cron.d/forecastiq"
cat > "$CRON_FILE" <<'EOF'
# ForecastIQ scheduled tasks (WP-24). Run as the deploy user; scripts live in
# /opt/forecastiq (shipped by deploy.sh) and reach Postgres via the db
# container, so no DB URL is needed in the environment.
SHELL=/bin/bash
PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

# Nightly backup: 02:30 UTC
30 2 * * * deploy /opt/forecastiq/backup.sh >> /var/log/forecastiq-backup.log 2>&1

# Monthly restore test: 1st of month, 04:00 UTC
0 4 1 * * deploy /opt/forecastiq/restore-test.sh >> /var/log/forecastiq-restore-test.log 2>&1
EOF
touch /var/log/forecastiq-backup.log /var/log/forecastiq-restore-test.log
chown deploy:deploy /var/log/forecastiq-backup.log /var/log/forecastiq-restore-test.log
echo "  Cron active (nightly backup + monthly restore test as deploy user)"

# ── 7. Verification ─────────────────────────────────────────────────
echo ""
echo "=== Bootstrap Verification ==="
echo "  Docker:        $(docker --version 2>/dev/null || echo MISSING)"
echo "  Compose:       $(docker compose version 2>/dev/null || echo MISSING)"
echo "  User deploy:   $(id deploy &>/dev/null && echo OK || echo MISSING)"
echo "  Data dirs:     $([ -d /var/lib/forecastiq/pgdata ] && echo OK || echo MISSING)"
echo "  Secrets file:  $([ -f /etc/forecastiq/secrets.env ] && echo OK || echo MISSING)"
echo "  Compose env:   $([ -f /opt/forecastiq/.env ] && echo OK || echo MISSING)"
echo "  UFW active:    $(ufw status | head -1)"
echo "  fail2ban:      $(systemctl is-active fail2ban 2>/dev/null || echo INACTIVE)"
echo ""
echo "=== Bootstrap Complete ==="
echo "Next: set POSTGRES_PASSWORD in /opt/forecastiq/.env, fill"
echo "/etc/forecastiq/secrets.env, then run the first deploy."
