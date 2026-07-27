#!/usr/bin/env bash
# deploy/scripts/rollback.sh — Roll ForecastIQ back to the previous image.
# Target: < 5 minutes total (NFR-M07). Rehearsed monthly (scheduled.yml).
#
# Reference: docs/adr/ADR-033-personal-use-ec2-docker-deployment.md
# Reference: docs/operations/05-deployment-and-rollback.md §2 (amended)
#
# The previous image reference is recorded by deploy.sh in
# /opt/forecastiq/.previous-image. Rolling back swaps FIQ_IMAGE back and
# restarts the app container; the image is still present locally (pulled by
# the prior deploy), so no registry round-trip is needed.
#
# Usage (on host):  bash deploy/scripts/rollback.sh [image-ref]
# Usage (remote):   VPS_HOST=... bash deploy/scripts/rollback.sh [image-ref]
#   image-ref: explicit image to roll back to (default: recorded previous)
set -euo pipefail

START_TIME=$(date +%s)
COMPOSE_DIR="/opt/forecastiq"
BASE_URL="http://127.0.0.1"

# Determine if running locally on the host or remotely
if [ -d "$COMPOSE_DIR" ] && [ -f "$COMPOSE_DIR/docker-compose.yml" ]; then
  REMOTE=""
else
  VPS_HOST="${VPS_HOST:?Set VPS_HOST for remote rollback}"
  VPS_USER="${VPS_USER:-deploy}"
  SSH_KEY="${DEPLOY_SSH_KEY_PATH:-~/.ssh/deploy_key}"
  REMOTE="ssh -i ${SSH_KEY} -o StrictHostKeyChecking=accept-new ${VPS_USER}@${VPS_HOST}"
fi

run() {
  if [ -n "$REMOTE" ]; then
    $REMOTE "$@"
  else
    eval "$@"
  fi
}

echo "=== ForecastIQ Rollback (Docker) ==="
echo "Started: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo ""

# Step 1: determine current + target images
CURRENT=$(run "grep -E '^FIQ_IMAGE=' ${COMPOSE_DIR}/.env | cut -d= -f2-" || true)
if [ -n "${1:-}" ]; then
  TARGET="$1"
else
  TARGET=$(run "cat ${COMPOSE_DIR}/.previous-image 2>/dev/null" || true)
fi

if [ -z "$TARGET" ]; then
  echo "ERROR: no previous image recorded and none given — nothing to roll back to."
  exit 1
fi
if [ "$TARGET" = "$CURRENT" ]; then
  echo "ERROR: target equals the current image (${CURRENT})."
  exit 1
fi

echo "[1/4] Current: ${CURRENT:-<none>}"
echo "       Target:  ${TARGET}"

# Step 2: swap FIQ_IMAGE (and record the rolled-back-from image so a second
# rollback rolls forward again — this is what the monthly drill exercises)
echo "[2/4] Swapping image reference..."
run "cd ${COMPOSE_DIR} && grep -vE '^FIQ_IMAGE=' .env > .env.tmp || true; echo 'FIQ_IMAGE=${TARGET}' >> .env.tmp && mv .env.tmp .env && echo '${CURRENT}' > .previous-image"

# Step 3: restart the app on the target image
echo "[3/4] Restarting app container..."
run "cd ${COMPOSE_DIR} && docker compose up -d app"

# Wait for readyz
echo "  Waiting for /readyz..."
for i in $(seq 1 30); do
  if run "curl -sf ${BASE_URL}/readyz" >/dev/null 2>&1; then
    echo "  readyz OK after ${i}s"
    break
  fi
  if [ "$i" -eq 30 ]; then
    echo "  ERROR: /readyz not green after 30s!"
    exit 1
  fi
  sleep 1
done

# Step 4: smoke tests
echo "[4/4] Running smoke tests..."
run "bash ${COMPOSE_DIR}/smoke-test.sh ${BASE_URL}"

# Timing
END_TIME=$(date +%s)
ELAPSED=$((END_TIME - START_TIME))
echo ""
echo "=== Rollback Complete ==="
echo "  Rolled back: ${CURRENT:-<none>} → ${TARGET}"
echo "  Duration:    ${ELAPSED}s"
if [ "$ELAPSED" -lt 300 ]; then
  echo "  NFR-M07:     PASS (< 5 min)"
else
  echo "  NFR-M07:     FAIL (${ELAPSED}s > 300s) — investigate"
fi
echo ""
echo "Next: record incident note (image, reason, follow-up)."
