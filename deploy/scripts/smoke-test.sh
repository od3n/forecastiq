#!/usr/bin/env bash
# deploy/scripts/smoke-test.sh — Post-deploy verification for ForecastIQ.
# Exits 0 if all checks pass; exits 1 on any failure.
#
# Reference: docs/operations/05-deployment-and-rollback.md §1 step 4f
# Reference: docs/delivery/02-ci-cd.md §2 (smoke tests)
#
# Usage: bash deploy/scripts/smoke-test.sh [base_url]
#   base_url: defaults to http://127.0.0.1:8080
set -euo pipefail

BASE_URL="${1:-http://127.0.0.1:8080}"
PASS=0
FAIL=0

check() {
  local name="$1"
  local url="$2"
  local expected_status="${3:-200}"

  # No curl -f here: with -f curl exits non-zero on 4xx while still emitting
  # the -w write-out, so the fallback would append "000" and corrupt the
  # captured status (DRB-WP23-005). -w alone reports the code; only transport
  # failures take the 000 fallback.
  status=$(curl -s -o /dev/null -w "%{http_code}" "$url" 2>/dev/null || echo "000")
  if [ "$status" = "$expected_status" ]; then
    echo "  [PASS] $name (HTTP $status)"
    PASS=$((PASS + 1))
  else
    echo "  [FAIL] $name — expected $expected_status, got $status"
    FAIL=$((FAIL + 1))
  fi
}

echo "=== ForecastIQ Smoke Tests ==="
echo "Target: $BASE_URL"
echo ""

# 1. Liveness probe
check "GET /healthz" "${BASE_URL}/healthz"

# 2. Readiness probe
check "GET /readyz" "${BASE_URL}/readyz"

# 3. Public endpoint (rankings — returns 200 even with no data). Routes are
# mounted under /api/v1 (internal/api/router.go); unversioned paths 404.
check "GET /api/v1/rankings" "${BASE_URL}/api/v1/rankings?location_id=00000000-0000-0000-0000-000000000001&horizon_minutes=60"

# 4. Admin health (requires auth in production; expect 401 without token)
check "GET /api/v1/admin/health (auth gate)" "${BASE_URL}/api/v1/admin/health" "401"

echo ""
echo "=== Results: ${PASS} passed, ${FAIL} failed ==="

if [ "$FAIL" -gt 0 ]; then
  echo ""
  echo "SMOKE TESTS FAILED — consider rollback:"
  echo "  bash deploy/scripts/rollback.sh"
  exit 1
fi

echo "All smoke tests passed."
exit 0
