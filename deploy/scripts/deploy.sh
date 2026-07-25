#!/usr/bin/env bash
# deploy/scripts/deploy.sh — Deploy a release to the ForecastIQ VPS.
# Can be run manually or called by CI. Requires SSH access to the VPS.
#
# Reference: docs/operations/05-deployment-and-rollback.md §1
# Reference: docs/architecture/06-deployment-architecture.md §4
#
# Usage: bash deploy/scripts/deploy.sh <version> [vps_host] [vps_user]
#   version:  release identifier (e.g. main-42, v2026.07.25-1)
#   vps_host: VPS IP or hostname (default: $VPS_HOST env)
#   vps_user: SSH user (default: deploy)
set -euo pipefail

VERSION="${1:?Usage: deploy.sh <version> [vps_host] [vps_user]}"
VPS_HOST="${2:-${VPS_HOST:-}}"
VPS_USER="${3:-${VPS_USER:-deploy}}"
SSH_KEY="${DEPLOY_SSH_KEY_PATH:-~/.ssh/deploy_key}"

if [ -z "$VPS_HOST" ]; then
  echo "ERROR: VPS_HOST not set. Pass as argument or export VPS_HOST."
  exit 1
fi

RELEASE_DIR="/opt/forecastiq/releases/${VERSION}"
SSH_CMD="ssh -i ${SSH_KEY} -o StrictHostKeyChecking=no ${VPS_USER}@${VPS_HOST}"

echo "=== ForecastIQ Deploy ==="
echo "Version:  $VERSION"
echo "Target:   ${VPS_USER}@${VPS_HOST}"
echo "Release:  $RELEASE_DIR"
echo ""

# Step 1: rsync artifact to VPS
echo "[1/7] Uploading artifact..."
rsync -avz -e "ssh -i ${SSH_KEY} -o StrictHostKeyChecking=no" \
  bin/forecastiq checksums.txt migrations/ deploy/ \
  "${VPS_USER}@${VPS_HOST}:${RELEASE_DIR}/"

# Step 2: Verify checksums on VPS
echo "[2/7] Verifying checksums..."
$SSH_CMD "cd ${RELEASE_DIR} && sha256sum -c checksums.txt"

# Step 3: Symlink swap
echo "[3/7] Swapping symlink..."
$SSH_CMD "ln -sfn ${RELEASE_DIR} /opt/forecastiq/current"

# Step 4: Install configs
echo "[4/7] Installing configs..."
$SSH_CMD bash <<REMOTE
  cp ${RELEASE_DIR}/deploy/systemd/forecastiq.service /etc/systemd/system/
  cp ${RELEASE_DIR}/deploy/caddy/Caddyfile /etc/caddy/Caddyfile
  systemctl daemon-reload
REMOTE

# Step 5: Run migrations
echo "[5/7] Running migrations..."
$SSH_CMD "/opt/forecastiq/current/forecastiq migrate --confirm"

# Step 6: Restart service
echo "[6/7] Restarting forecastiq..."
$SSH_CMD "systemctl restart forecastiq"

# Wait for readyz (max 30s)
echo "  Waiting for /readyz..."
for i in $(seq 1 30); do
  if $SSH_CMD "curl -sf http://127.0.0.1:8080/readyz" >/dev/null 2>&1; then
    echo "  readyz OK after ${i}s"
    break
  fi
  if [ "$i" -eq 30 ]; then
    echo "  ERROR: /readyz not green after 30s — consider rollback!"
    exit 1
  fi
  sleep 1
done

# Step 7: Smoke tests
echo "[7/7] Running smoke tests..."
$SSH_CMD "bash ${RELEASE_DIR}/deploy/scripts/smoke-test.sh"

# Cleanup: keep only last 5 releases
echo ""
echo "Cleaning old releases (keeping last 5)..."
$SSH_CMD 'ls -1dt /opt/forecastiq/releases/*/ 2>/dev/null | tail -n +6 | xargs rm -rf' || true

echo ""
echo "=== Deploy Complete: ${VERSION} ==="
echo "Post-deploy: monitor Grafana dashboards for 10 minutes."
echo "Rollback:    bash deploy/scripts/rollback.sh"
