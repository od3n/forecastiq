#!/usr/bin/env bash
# deploy/scripts/rotation-drill.sh — Secret rotation drill for ForecastIQ.
# Run monthly to validate rotation procedures per docs/security/04-secrets-management.md.
#
# Topology: ADR-033 (EC2 + Docker Compose). Secrets live in
# /etc/forecastiq/secrets.env, read client-side by the `deploy` user via the
# compose `env_file`; POSTGRES_PASSWORD lives in /opt/forecastiq/.env. env_file
# changes require CONTAINER RECREATION (not just restart) to take effect.
#
# Modes:
#   (default) dry-run — non-destructive checks: file perms, required vars,
#             health, OWASP subset, security headers.
#   --live    performs a real no-op rotation cycle: recreate the app container
#             and verify it comes back healthy with a working collection. This
#             exercises the exact restart path a real rotation uses WITHOUT
#             changing any secret value, so it is safe to run monthly.
set -euo pipefail

COMPOSE_DIR="${FIQ_COMPOSE_DIR:-/opt/forecastiq}"
BASE_URL="${BASE_URL:-http://127.0.0.1}"
SECRETS_FILE="${FIQ_SECRETS_FILE:-/etc/forecastiq/secrets.env}"
MODE="${1:-dry-run}"
PASS=0
FAIL=0

compose() { docker compose --project-directory "$COMPOSE_DIR" "$@"; }

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

wait_ready() {
  for _ in $(seq 1 30); do
    if curl -sf "${BASE_URL}/readyz" >/dev/null 2>&1; then return 0; fi
    sleep 1
  done
  return 1
}

# 1. Secrets file permissions. Under ADR-033 the file is read by the `deploy`
#    user running docker compose (not root via systemd EnvironmentFile), so the
#    expected owner is `deploy` with mode 600 (DRB-WP25-002).
echo "[1/5] Checking secrets file permissions..."
if [ -f "$SECRETS_FILE" ]; then
  PERMS=$(stat -c "%a" "$SECRETS_FILE" 2>/dev/null || stat -f "%Lp" "$SECRETS_FILE")
  OWNER=$(stat -c "%U" "$SECRETS_FILE" 2>/dev/null || stat -f "%Su" "$SECRETS_FILE")
  check "secrets file exists" "true"
  check "permissions = 600" "$([ "$PERMS" = "600" ] && echo true || echo false)"
  check "owner = deploy" "$([ "$OWNER" = "deploy" ] && echo true || echo false)"
else
  check "secrets file exists" "false"
  echo "  WARNING: Secrets file not found at ${SECRETS_FILE}"
fi

# 2. Required environment variables present in the secrets file.
echo ""
echo "[2/5] Checking required variables are set..."
REQUIRED_VARS="FIQ_AUTH_JWKS_URL FIQ_AUTH_ISSUER FIQ_AUTH_AUDIENCE"
for var in $REQUIRED_VARS; do
  if grep -q "^${var}=" "$SECRETS_FILE" 2>/dev/null || [ -n "${!var:-}" ]; then
    check "$var set" "true"
  else
    check "$var set" "false"
  fi
done

# 3. Pre-rotation health baseline.
echo ""
echo "[3/5] Pre-rotation health check..."
check "healthz responsive" "$(curl -sf "${BASE_URL}/healthz" >/dev/null 2>&1 && echo true || echo false)"
check "readyz responsive" "$(curl -sf "${BASE_URL}/readyz" >/dev/null 2>&1 && echo true || echo false)"

# 4. OWASP checklist verification (automated subset).
echo ""
echo "[4/5] OWASP checklist (automated checks)..."
if command -v git &>/dev/null; then
  SECRET_FILES=$(git -C "$(dirname "$0")/../.." ls-files 2>/dev/null | grep -iE "(secret|password|token|api.key)" | grep -vE "example|test|fixture|\.md|\.go" || true)
  check "no secret files in repo" "$([ -z "$SECRET_FILES" ] && echo true || echo false)"
fi
if [ -f "$(dirname "$0")/../../.gitignore" ]; then
  check ".env in gitignore" "$(grep -q '\.env' "$(dirname "$0")/../../.gitignore" && echo true || echo false)"
fi
# Security headers (GET with header dump — curl -I sends HEAD, which Gin routes
# as 404, silently voiding the check; DRB-WP25 finding).
HEADERS=$(curl -s -D - -o /dev/null "${BASE_URL}/healthz" 2>/dev/null || echo "")
if [ -n "$HEADERS" ]; then
  check "X-Content-Type-Options header" "$(echo "$HEADERS" | grep -qi 'X-Content-Type-Options' && echo true || echo false)"
  check "X-Frame-Options header" "$(echo "$HEADERS" | grep -qi 'X-Frame-Options' && echo true || echo false)"
  check "Content-Security-Policy header" "$(echo "$HEADERS" | grep -qi 'Content-Security-Policy' && echo true || echo false)"
else
  check "security headers reachable" "false"
fi

# 5. Rotation cycle.
echo ""
if [ "$MODE" = "--live" ]; then
  echo "[5/5] Live rotation cycle (no-op recreation)..."
  echo "  Operator: update credentials in ${SECRETS_FILE} + /opt/forecastiq/.env now,"
  echo "  then this drill recreates the app container to load them."
  # env_file changes are only picked up by a fresh container, not `restart`.
  compose up -d --force-recreate app
  if wait_ready; then
    check "app healthy after recreation" "true"
  else
    check "app healthy after recreation" "false"
    echo "  ERROR: /readyz not green after 30s — rotation would have failed."
    exit 1
  fi
  # Verify a gated write path works with the (re)loaded credentials.
  check "smoke checks pass post-rotation" "$(bash "$(dirname "$0")/smoke-test.sh" "$BASE_URL" >/dev/null 2>&1 && echo true || echo false)"
  echo "  Reminder: revoke old credentials at the provider/vendor and record in the ops log."
else
  echo "[5/5] Dry-run: skipping container recreation (use --live to exercise it)."
fi

echo ""
echo "=== Rotation Drill Results: ${PASS} passed, ${FAIL} failed ==="

if [ "$FAIL" -gt 0 ]; then
  echo "DRILL FAILED — resolve issues before production rotation."
  exit 1
fi

echo "Drill passed. Rotation procedures validated."
exit 0
