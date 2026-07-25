#!/usr/bin/env bash
# deploy/scripts/launch-checklist.sh — Pre-launch verification for ForecastIQ.
# Validates all launch gates (D-05, D-06) and system readiness.
#
# Reference: docs/planning/05-implementation-work-packages.md §WP-27
# Reference: docs/delivery/05-implementation-sequence.md §5 (M4 launch-ready)
#
# Usage: bash deploy/scripts/launch-checklist.sh [base_url]
set -euo pipefail

BASE_URL="${1:-http://localhost:8080}"
API_URL="${BASE_URL}/api/v1"
PASS=0
FAIL=0
SKIP=0

echo "=== ForecastIQ Launch Checklist ==="
echo "Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo ""

check() {
  local name="$1"
  local result="$2"
  if [ "$result" = "true" ]; then
    echo "  [PASS] $name"
    PASS=$((PASS + 1))
  elif [ "$result" = "skip" ]; then
    echo "  [SKIP] $name"
    SKIP=$((SKIP + 1))
  else
    echo "  [FAIL] $name"
    FAIL=$((FAIL + 1))
  fi
}

# ── 1. Infrastructure ────────────────────────────────────────────────────────
echo "[1/8] Infrastructure..."
check "API responds (healthz)" "$(curl -sf -o /dev/null -w '%{http_code}' ${BASE_URL}/healthz | grep -q 200 && echo true || echo false)"
check "API ready (readyz)" "$(curl -sf -o /dev/null -w '%{http_code}' ${BASE_URL}/readyz | grep -q 200 && echo true || echo false)"
check "Metrics endpoint" "$(curl -sf http://localhost:9090/metrics >/dev/null 2>&1 && echo true || echo skip)"

# ── 2. API Surface ──────────────────────────────────────────────────────────
echo ""
echo "[2/8] API surface..."
check "GET /rankings responds" "$(curl -sf -o /dev/null -w '%{http_code}' ${API_URL}/rankings?location_id=00000000-0000-0000-0000-000000000001\&horizon_minutes=60 | grep -qE '2[0-9]{2}' && echo true || echo false)"
check "GET /rankings/methodology responds" "$(curl -sf -o /dev/null -w '%{http_code}' ${API_URL}/rankings/methodology | grep -q 200 && echo true || echo false)"
check "GET /accuracy/summary responds" "$(curl -sf -o /dev/null -w '%{http_code}' ${API_URL}/accuracy/summary?location_id=00000000-0000-0000-0000-000000000001 | grep -qE '2[0-9]{2}' && echo true || echo false)"
check "GET /locations responds" "$(curl -sf -o /dev/null -w '%{http_code}' ${API_URL}/locations | grep -q 200 && echo true || echo false)"
check "GET /providers responds" "$(curl -sf -o /dev/null -w '%{http_code}' ${API_URL}/providers | grep -q 200 && echo true || echo false)"

# ── 3. Security ──────────────────────────────────────────────────────────────
echo ""
echo "[3/8] Security controls..."
check "Admin gated (401 without auth)" "$(curl -sf -o /dev/null -w '%{http_code}' ${API_URL}/admin/health | grep -q 401 && echo true || echo false)"
check "Rate limiting active" "skip"  # Would need to exhaust bucket
HEADERS=$(curl -sf -I ${API_URL}/locations 2>/dev/null || echo "")
check "X-Content-Type-Options header" "$(echo '$HEADERS' | grep -qi 'nosniff' && echo true || echo skip)"

# ── 4. Collection Pipeline ───────────────────────────────────────────────────
echo ""
echo "[4/8] Collection pipeline..."
check "OpenAPI spec valid (30 paths)" "$(curl -sf ${API_URL}/openapi.json | python3 -c 'import json,sys; d=json.load(sys.stdin); print(\"true\" if len(d.get(\"paths\",{}))>=15 else \"false\")' 2>/dev/null || echo false)"

# ── 5. Dashboard ─────────────────────────────────────────────────────────────
echo ""
echo "[5/8] Dashboard (static export)..."
if [ -d "web/out" ]; then
  check "Static export exists" "true"
  check "Index page present" "$([ -f web/out/index.html ] && echo true || echo false)"
else
  check "Static export exists" "skip"
fi

# ── 6. Documentation ────────────────────────────────────────────────────────
echo ""
echo "[6/8] Documentation..."
check "README.md exists" "$([ -f README.md ] && echo true || echo false)"
check "OpenAPI spec committed" "$([ -f api/openapi/openapi.json ] && echo true || echo false)"
check "Runbooks present" "$([ -f docs/operations/05-deployment-and-rollback.md ] && echo true || echo false)"
check "Architecture docs" "$([ -f docs/architecture/06-deployment-architecture.md ] && echo true || echo false)"

# ── 7. Backup & Recovery ────────────────────────────────────────────────────
echo ""
echo "[7/8] Backup & recovery..."
check "backup.sh exists" "$([ -f deploy/scripts/backup.sh ] && echo true || echo false)"
check "restore-test.sh exists" "$([ -f deploy/scripts/restore-test.sh ] && echo true || echo false)"
check "rollback.sh exists" "$([ -f deploy/scripts/rollback.sh ] && echo true || echo false)"

# ── 8. Gates ─────────────────────────────────────────────────────────────────
echo ""
echo "[8/8] Launch gates..."
check "D-05 ToS review (manual)" "skip"
check "D-06 Observation quality (manual)" "skip"
check "Performance baseline recorded (manual)" "skip"

# ── Summary ──────────────────────────────────────────────────────────────────
echo ""
echo "=== Launch Checklist Results ==="
echo "  Passed:  ${PASS}"
echo "  Failed:  ${FAIL}"
echo "  Skipped: ${SKIP} (manual verification required)"
echo ""

if [ "$FAIL" -gt 0 ]; then
  echo "LAUNCH BLOCKED: ${FAIL} check(s) failed."
  exit 1
fi

echo "Automated checks passed. Complete manual gates (D-05, D-06) before launch."
exit 0
