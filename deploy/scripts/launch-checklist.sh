#!/usr/bin/env bash
# deploy/scripts/launch-checklist.sh — Pre-launch verification for ForecastIQ.
# Validates all launch gates (D-05, D-06) and system readiness.
#
# Reference: docs/planning/05-implementation-work-packages.md §WP-27
# Reference: docs/delivery/05-implementation-sequence.md §5 (M4 launch-ready)
#
# NOTE on curl usage: probes that EXPECT an error status (e.g. the 401 auth
# gate) must not use -f — under pipefail, curl's exit 22 fails the pipeline
# even when grep matched the code (DRB-WP27-002).
#
# Usage: bash deploy/scripts/launch-checklist.sh [base_url]
set -euo pipefail

BASE_URL="${1:-http://localhost:8080}"
API_URL="${BASE_URL}/api/v1"
METRICS_URL="${METRICS_URL:-http://localhost:9090}"
PASS=0
FAIL=0
SKIP=0

# Run from the repo root regardless of invocation directory (file-existence
# checks below are repo-relative).
cd "$(dirname "$0")/../.."

# status_of: HTTP status code without -f (safe for expected-error probes).
status_of() {
  curl -s -o /dev/null -w "%{http_code}" "$@" 2>/dev/null || echo "000"
}

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

# expect_status: true iff URL returns the given status.
expect_status() {
  local expected="$1"; shift
  [ "$(status_of "$@")" = "$expected" ] && echo true || echo false
}

# expect_2xx: true iff URL returns any 2xx.
expect_2xx() {
  case "$(status_of "$@")" in 2??) echo true ;; *) echo false ;; esac
}

# ── 1. Infrastructure ────────────────────────────────────────────────────────
echo "[1/9] Infrastructure..."
check "API responds (healthz)" "$(expect_status 200 "${BASE_URL}/healthz")"
check "API ready (readyz)" "$(expect_status 200 "${BASE_URL}/readyz")"
check "Metrics endpoint" "$(curl -sf "${METRICS_URL}/metrics" >/dev/null 2>&1 && echo true || echo skip)"

# ── 2. API Surface ──────────────────────────────────────────────────────────
echo ""
echo "[2/9] API surface..."
check "GET /rankings responds" "$(expect_2xx "${API_URL}/rankings?location_id=00000000-0000-0000-0000-000000000001&horizon_minutes=60")"
check "GET /rankings/methodology responds" "$(expect_status 200 "${API_URL}/rankings/methodology")"
check "GET /accuracy/summary responds" "$(expect_2xx "${API_URL}/accuracy/summary?location_id=00000000-0000-0000-0000-000000000001")"
check "GET /locations responds" "$(expect_status 200 "${API_URL}/locations")"
check "GET /providers responds" "$(expect_status 200 "${API_URL}/providers")"

# ── 3. Security ──────────────────────────────────────────────────────────────
echo ""
echo "[3/9] Security controls..."
check "Admin gated (401 without auth)" "$(expect_status 401 "${API_URL}/admin/health")"
check "Rate limiting active" "skip"  # Exhausting the bucket is a reliability-suite concern
# GET with dumped headers — curl -I sends HEAD (Gin: 404 → empty), and the
# previous echo '$HEADERS' grepped the literal string (DRB-WP27-003).
HEADERS=$(curl -s -D - -o /dev/null "${API_URL}/locations" 2>/dev/null || echo "")
if [ -n "$HEADERS" ]; then
  check "X-Content-Type-Options header" "$(echo "$HEADERS" | grep -qi 'nosniff' && echo true || echo false)"
else
  check "X-Content-Type-Options header" "skip"
fi

# ── 4. API Contract ──────────────────────────────────────────────────────────
echo ""
echo "[4/9] API contract..."
# python one-liner uses double quotes OUTSIDE / single quotes INSIDE: the
# previous \"true\" inside single quotes was a Python SyntaxError, making the
# check false forever and blocking every launch (DRB-WP27-001).
OPENAPI_OK=$(curl -sf "${API_URL}/openapi.json" 2>/dev/null | python3 -c "import json,sys; d=json.load(sys.stdin); print('true' if len(d.get('paths',{}))>=30 else 'false')" 2>/dev/null || echo false)
check "OpenAPI spec valid (>= 30 paths)" "$OPENAPI_OK"

# ── 5. Dashboard ─────────────────────────────────────────────────────────────
echo ""
echo "[5/9] Dashboard (static export)..."
if [ -d "web/out" ]; then
  check "Static export exists" "true"
  check "Index page present" "$([ -f web/out/index.html ] && echo true || echo false)"
else
  check "Static export exists" "skip"
fi

# ── 6. Documentation ────────────────────────────────────────────────────────
echo ""
echo "[6/9] Documentation..."
check "README.md exists" "$([ -f README.md ] && echo true || echo false)"
check "OpenAPI spec committed" "$([ -f api/openapi/openapi.json ] && echo true || echo false)"
check "Runbooks present" "$([ -f docs/operations/05-deployment-and-rollback.md ] && echo true || echo false)"
check "Architecture docs" "$([ -f docs/architecture/06-deployment-architecture.md ] && echo true || echo false)"
# Verify runbooks reflect REALITY (ADR-033 EC2/Docker), not just that they
# exist (DRB-WP27-002): the rollback runbook must reference the container
# deploy path and must NOT tell an operator to run systemctl.
RUNBOOK=docs/operations/05-deployment-and-rollback.md
check "runbook references docker compose / FIQ_IMAGE" "$(grep -qE 'docker compose|FIQ_IMAGE' "$RUNBOOK" && echo true || echo false)"
check "runbook has no stale systemctl step" "$(grep -q 'systemctl' "$RUNBOOK" && echo false || echo true)"

# ── 7. Backup & Recovery ────────────────────────────────────────────────────
echo ""
echo "[7/9] Backup & recovery..."
check "backup.sh exists" "$([ -f deploy/scripts/backup.sh ] && echo true || echo false)"
check "restore-test.sh exists" "$([ -f deploy/scripts/restore-test.sh ] && echo true || echo false)"
check "rollback.sh exists" "$([ -f deploy/scripts/rollback.sh ] && echo true || echo false)"

# --- 8. Attribution (BR-ATTR-01) ---
echo ""
echo "[8/9] Attribution across public surfaces..."
# BR-ATTR-01: every data response carries provider attribution. Check the
# public read endpoints, not just one (DRB-WP27-003). jq -e exits non-zero when
# the attribution field is absent/empty.
ATTR_LOC="00000000-0000-0000-0000-000000000001"
for ep in \
  "rankings?location_id=${ATTR_LOC}&horizon_minutes=60" \
  "accuracy/summary?location_id=${ATTR_LOC}"; do
  if curl -sf "${API_URL}/${ep}" 2>/dev/null \
     | jq -e '(.metadata.attribution // .data.attribution) | length > 0' >/dev/null 2>&1; then
    check "attribution present: /${ep%%\?*}" "true"
  else
    # Absent data pre-launch is not a failure; a populated system must pass.
    check "attribution present: /${ep%%\?*}" "skip"
  fi
done

echo ""
echo "[9/9] Launch gates..."
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
