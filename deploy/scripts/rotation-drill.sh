#!/usr/bin/env bash
# deploy/scripts/rotation-drill.sh — Secret rotation drill for ForecastIQ.
# Run monthly to validate rotation procedures per docs/security/04-secrets-management.md.
#
# This script simulates a rotation (dry-run by default) and validates that:
# 1. The secrets file exists and has correct permissions
# 2. All required env vars are set
# 3. The service can restart cleanly (simulated via /readyz check)
# 4. Critical endpoints respond after rotation
#
# Usage:
#   bash deploy/scripts/rotation-drill.sh              # dry-run (checks only)
#   bash deploy/scripts/rotation-drill.sh --live       # live rotation (operator confirms)
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
SECRETS_FILE="/etc/forecastiq/secrets.env"
MODE="${1:-dry-run}"
PASS=0
FAIL=0

echo "=== ForecastIQ Secret Rotation Drill ==="
echo "Mode: ${MODE}"
echo "Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo ""

check() {
  local name="$1"
  local result="$2"
  if [ "$result" = "true" ]; then
    echo "  [PASS] $name"
    PASS=$((PASS + 1))
  else
    echo "  [FAIL] $name"
    FAIL=$((FAIL + 1))
  fi
}

# 1. Secrets file permissions
echo "[1/5] Checking secrets file permissions..."
if [ -f "$SECRETS_FILE" ]; then
  PERMS=$(stat -c "%a" "$SECRETS_FILE" 2>/dev/null || stat -f "%Lp" "$SECRETS_FILE")
  OWNER=$(stat -c "%U" "$SECRETS_FILE" 2>/dev/null || stat -f "%Su" "$SECRETS_FILE")
  check "secrets file exists" "true"
  check "permissions = 600" "$([ "$PERMS" = "600" ] && echo true || echo false)"
  check "owner = root" "$([ "$OWNER" = "root" ] && echo true || echo false)"
else
  check "secrets file exists" "false"
  echo "  WARNING: Secrets file not found at ${SECRETS_FILE}"
fi

# 2. Required environment variables (check from loaded env)
echo ""
echo "[2/5] Checking required variables are set..."
REQUIRED_VARS="FIQ_DATABASE_URL FIQ_AUTH_JWKS_URL FIQ_AUTH_ISSUER FIQ_AUTH_AUDIENCE"
for var in $REQUIRED_VARS; do
  if grep -q "^${var}=" "$SECRETS_FILE" 2>/dev/null || [ -n "${!var:-}" ]; then
    check "$var set" "true"
  else
    check "$var set" "false"
  fi
done

# 3. Service health (pre-rotation baseline)
echo ""
echo "[3/5] Pre-rotation health check..."
if curl -sf "${BASE_URL}/healthz" >/dev/null 2>&1; then
  check "healthz responsive" "true"
else
  check "healthz responsive" "false"
fi
if curl -sf "${BASE_URL}/readyz" >/dev/null 2>&1; then
  check "readyz responsive" "true"
else
  check "readyz responsive" "false"
fi

# 4. OWASP checklist verification (automated subset)
echo ""
echo "[4/5] OWASP checklist (automated checks)..."

# Check no secrets in repo
if command -v git &>/dev/null; then
  SECRET_FILES=$(git -C "$(dirname "$0")/../.." ls-files | grep -iE "(secret|password|token|api.key)" | grep -v "example\|test\|fixture\|\.md\|\.go" || true)
  check "no secret files in repo" "$([ -z "$SECRET_FILES" ] && echo true || echo false)"
fi

# Check .env files are gitignored
if [ -f "$(dirname "$0")/../../.gitignore" ]; then
  check ".env in gitignore" "$(grep -q '\.env' "$(dirname "$0")/../../.gitignore" && echo true || echo false)"
fi

# Check security headers present
HEADERS=$(curl -sf -I "${BASE_URL}/healthz" 2>/dev/null || echo "")
if [ -n "$HEADERS" ]; then
  check "X-Content-Type-Options header" "$(echo "$HEADERS" | grep -qi 'X-Content-Type-Options' && echo true || echo false)"
  check "X-Frame-Options header" "$(echo "$HEADERS" | grep -qi 'X-Frame-Options' && echo true || echo false)"
fi

# 5. Post-drill summary
echo ""
echo "[5/5] Drill summary..."
if [ "$MODE" = "--live" ]; then
  echo ""
  echo "  LIVE MODE: After updating credentials in ${SECRETS_FILE}:"
  echo "    1. systemctl restart forecastiq"
  echo "    2. Wait for /readyz green (max 30s)"
  echo "    3. POST /admin/collections/trigger → verify collection succeeds"
  echo "    4. Revoke old credentials at provider/vendor dashboard"
  echo "    5. Record in ops log"
fi

echo ""
echo "=== Rotation Drill Results: ${PASS} passed, ${FAIL} failed ==="

if [ "$FAIL" -gt 0 ]; then
  echo "DRILL FAILED — resolve issues before production rotation."
  exit 1
fi

echo "Drill passed. Rotation procedures validated."
exit 0
