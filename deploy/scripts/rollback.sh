#!/usr/bin/env bash
# deploy/scripts/rollback.sh — Rollback ForecastIQ to the previous release.
# Target: < 5 minutes total (NFR-M07). Rehearsed monthly.
#
# Reference: docs/operations/05-deployment-and-rollback.md §2
#
# Usage (on VPS):  bash deploy/scripts/rollback.sh [version]
#   version: specific release to roll back to (default: previous)
#
# Usage (remote): VPS_HOST=... bash deploy/scripts/rollback.sh [version]
set -euo pipefail

START_TIME=$(date +%s)
RELEASES_DIR="/opt/forecastiq/releases"
BASE_URL="http://127.0.0.1:8080"

# Determine if running locally on VPS or remotely
if [ -d "$RELEASES_DIR" ]; then
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

echo "=== ForecastIQ Rollback ==="
echo "Started: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo ""

# Step 1: List available releases
echo "[1/5] Available releases:"
run "ls -1dt ${RELEASES_DIR}/*/ 2>/dev/null" | head -5

# Determine target version. Exact-name exclusion, not substring grep: a
# substring filter mis-selects on version-prefix collisions (main-4 vs
# main-42) and filters everything when current points at the releases
# container (DRB-WP23-015). The guarded substitution keeps pipefail from
# killing the script before the friendly error below.
CURRENT=$(run "readlink -f /opt/forecastiq/current" | xargs basename)
if [ -n "${1:-}" ]; then
  TARGET="$1"
else
  # Default: the newest release that is not the current one
  TARGET=$(run "ls -1t ${RELEASES_DIR}" | awk -v cur="$CURRENT" '$0 != cur' | head -1 || true)
fi

if [ -z "$TARGET" ]; then
  echo "ERROR: No previous release found to roll back to."
  exit 1
fi

echo ""
echo "  Current: $CURRENT"
echo "  Target:  $TARGET"
echo ""

# Step 2: Symlink swap
echo "[2/5] Swapping symlink to ${TARGET}..."
run "ln -sfn ${RELEASES_DIR}/${TARGET} /opt/forecastiq/current"

# Step 3: Restart service
echo "[3/5] Restarting forecastiq..."
run "sudo /usr/bin/systemctl restart forecastiq"

# Step 4: Wait for readyz
echo "[4/5] Waiting for /readyz..."
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

# Step 5: Smoke tests
echo "[5/5] Running smoke tests..."
run "bash ${RELEASES_DIR}/${TARGET}/deploy/scripts/smoke-test.sh"

# Timing
END_TIME=$(date +%s)
ELAPSED=$((END_TIME - START_TIME))
echo ""
echo "=== Rollback Complete ==="
echo "  Rolled back: ${CURRENT} → ${TARGET}"
echo "  Duration:    ${ELAPSED}s"
if [ "$ELAPSED" -lt 300 ]; then
  echo "  NFR-M07:     PASS (< 5 min)"
else
  echo "  NFR-M07:     FAIL (${ELAPSED}s > 300s) — investigate"
fi
echo ""
echo "Next: record incident note (version, reason, follow-up)."
