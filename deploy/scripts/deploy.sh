#!/usr/bin/env bash
# deploy/scripts/deploy.sh — Deploy a ForecastIQ image to the EC2 host.
# Can be run manually or called by CI. Requires SSH access as the deploy user.
#
# Reference: docs/adr/ADR-033-personal-use-ec2-docker-deployment.md
# Reference: docs/operations/05-deployment-and-rollback.md §1 (amended)
#
# Usage: bash deploy/scripts/deploy.sh <image-ref> [vps_host] [vps_user]
#   image-ref: full image reference — prefer the immutable digest form
#              (ghcr.io/od3n/forecastiq@sha256:...); tags work for manual runs
#   vps_host:  EC2 IP or hostname (default: $VPS_HOST env)
#   vps_user:  SSH user (default: deploy)
#
# Optional env:
#   GHCR_TOKEN — read:packages token; when set, the host logs in to ghcr.io
#                before pulling (required while the package is private)
set -euo pipefail

IMAGE="${1:?Usage: deploy.sh <image-ref> [vps_host] [vps_user]}"
VPS_HOST="${2:-${VPS_HOST:-}}"
VPS_USER="${3:-${VPS_USER:-deploy}}"
SSH_KEY="${DEPLOY_SSH_KEY_PATH:-~/.ssh/deploy_key}"

if [ -z "$VPS_HOST" ]; then
  echo "ERROR: VPS_HOST not set. Pass as argument or export VPS_HOST."
  exit 1
fi

# accept-new (TOFU) rather than 'no': first contact records the key, later
# mismatches hard-fail instead of being silently accepted (DRB-WP23-010).
SSH_OPTS="-i ${SSH_KEY} -o StrictHostKeyChecking=accept-new"
SSH_CMD="ssh ${SSH_OPTS} ${VPS_USER}@${VPS_HOST}"

echo "=== ForecastIQ Deploy (Docker) ==="
echo "Image:   $IMAGE"
echo "Target:  ${VPS_USER}@${VPS_HOST}"
echo ""

# Step 1: ship the compose file + smoke test (versioned with the repo)
echo "[1/6] Uploading compose file + operational scripts..."
scp $SSH_OPTS deploy/compose/docker-compose.prod.yml \
  "${VPS_USER}@${VPS_HOST}:/opt/forecastiq/docker-compose.yml"
scp $SSH_OPTS deploy/scripts/smoke-test.sh deploy/scripts/backup.sh deploy/scripts/restore-test.sh \
  "${VPS_USER}@${VPS_HOST}:/opt/forecastiq/"
$SSH_CMD "chmod +x /opt/forecastiq/smoke-test.sh /opt/forecastiq/backup.sh /opt/forecastiq/restore-test.sh"

# Step 2: record rollback target + select the new image
echo "[2/6] Selecting image..."
$SSH_CMD bash <<REMOTE
  set -euo pipefail
  cd /opt/forecastiq
  # Remember the currently-running image for rollback (only when one exists).
  CURRENT=\$(grep -E '^FIQ_IMAGE=' .env | cut -d= -f2- || true)
  if [ -n "\$CURRENT" ] && [ "\$CURRENT" != "${IMAGE}" ]; then
    echo "\$CURRENT" > .previous-image
  fi
  # Upsert FIQ_IMAGE in the compose interpolation env.
  grep -vE '^FIQ_IMAGE=' .env > .env.tmp || true
  echo "FIQ_IMAGE=${IMAGE}" >> .env.tmp
  mv .env.tmp .env
REMOTE

# Step 3: pull + start
echo "[3/6] Pulling and starting containers..."
$SSH_CMD bash <<REMOTE
  set -euo pipefail
  cd /opt/forecastiq
  if [ -n "${GHCR_TOKEN:-}" ]; then
    echo "${GHCR_TOKEN:-}" | docker login ghcr.io -u token --password-stdin
  fi
  docker compose pull app db
  docker compose up -d db
REMOTE

# Step 4: run migrations (embedded in the binary; one-shot container)
echo "[4/6] Running migrations..."
$SSH_CMD "cd /opt/forecastiq && docker compose run --rm app migrate up"

# Step 5: start the app + wait for readyz
echo "[5/6] Starting app..."
$SSH_CMD "cd /opt/forecastiq && docker compose up -d app"
echo "  Waiting for /readyz..."
for i in $(seq 1 30); do
  if $SSH_CMD "curl -sf http://127.0.0.1/readyz" >/dev/null 2>&1; then
    echo "  readyz OK after ${i}s"
    break
  fi
  if [ "$i" -eq 30 ]; then
    echo "  ERROR: /readyz not green after 30s — consider rollback!"
    exit 1
  fi
  sleep 1
done

# Step 6: smoke tests + prune old images
echo "[6/6] Running smoke tests..."
$SSH_CMD "bash /opt/forecastiq/smoke-test.sh http://127.0.0.1"
$SSH_CMD "docker image prune -f --filter 'until=720h'" || true

echo ""
echo "=== Deploy Complete: ${IMAGE} ==="
echo "Post-deploy: monitor dashboards for 10 minutes."
echo "Rollback:    bash deploy/scripts/rollback.sh"
