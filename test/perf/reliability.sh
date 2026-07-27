#!/usr/bin/env bash
# test/perf/reliability.sh — Reliability test suite (WP-26, SLICE 1 OF N).
# Validates system behavior under adverse conditions per testing strategy §1.
#
# SCOPE (DRB-WP26-001): this slice covers the request-path reliability checks
# below. The fault-injection scenarios in the WP-26 scope — provider timeout,
# duplicate job, late observation, worker restart, DB reconnect — are NOT here;
# they require compose-level fault injection and are tracked in WP-26b
# (docs/reviews/work-packages/WP-26-delivery-review.md).
#
# Reference: docs/testing/02-testing-strategy.md §1 (Reliability layer)
# Reference: docs/testing/04-performance-testing.md §2
#
# Scenarios tested (this slice):
#   1. Malformed payload rejection (invalid JSON → 4xx; needs ADMIN_TOKEN to
#      reach validation, otherwise asserts the 401 auth gate)
#   2. Request body size limit (oversized → 413)
#   3. Unknown route handling (→ 404, no info leak)
#   4. Health probes under load (healthz/readyz always responsive)
#   5. CORS rejection (non-allowlisted origin → no CORS headers)
#   6. Rate-limit enforcement (per-IP budget exhaustion → 429) — LAST, because
#      it drains the shared token bucket (DRB-WP26-005).
#
# ENVIRONMENT: run against the DEFAULT rate limiter. This is mutually exclusive
# with the k6 perf env (PT-1/PT-2 raise FIQ_RATE_LIMIT_PER_IP_PER_MIN to a huge
# value, under which scenario 6 can never see a 429).
#
# NOTE on curl usage: probes that EXPECT an error status must not use -f —
# curl -f exits 22 after printing the -w output, so `$(curl -sf -w ... || echo 000)`
# captures "429000"/"413000" and the checks can never pass (DRB-WP26-001).
#
# Usage: bash test/perf/reliability.sh [base_url]
#   ADMIN_TOKEN: optional bearer token to exercise payload validation in test 1
set -euo pipefail

BASE_URL="${1:-http://localhost:8080}"
API_URL="${BASE_URL}/api/v1"
ADMIN_TOKEN="${ADMIN_TOKEN:-}"
PASS=0
FAIL=0
TOTAL=0

# status_of: GET status code without -f (expected-error probes).
status_of() {
  curl -s -o /dev/null -w "%{http_code}" "$@" 2>/dev/null || echo "000"
}

check() {
  local name="$1"
  local expected="$2"
  local actual="$3"
  TOTAL=$((TOTAL + 1))
  if [ "$actual" = "$expected" ]; then
    echo "  [PASS] $name"
    PASS=$((PASS + 1))
  else
    echo "  [FAIL] $name (expected=$expected, got=$actual)"
    FAIL=$((FAIL + 1))
  fi
}

echo "=== ForecastIQ Reliability Test Suite (WP-26 slice 1) ==="
echo "Target: ${BASE_URL}"
echo ""

# ── 1. Malformed payload rejection ──────────────────────────────────────────
echo "[1/6] Malformed payload rejection..."
if [ -n "$ADMIN_TOKEN" ]; then
  # Authenticated: the malformed body reaches JSON binding → 400/422.
  STATUS=$(status_of -X POST "${API_URL}/locations" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    -H "Content-Type: application/json" -d "not-valid-json{{{")
  check "malformed JSON → 400/422" "true" "$([ "$STATUS" = "400" ] || [ "$STATUS" = "422" ] && echo true || echo false)"
else
  # Unauthenticated: auth middleware runs first — assert the 401 gate and
  # note that payload validation needs ADMIN_TOKEN to be exercised.
  STATUS=$(status_of -X POST "${API_URL}/locations" \
    -H "Content-Type: application/json" -d "not-valid-json{{{")
  check "unauthenticated mutation → 401 (set ADMIN_TOKEN to test validation)" "401" "$STATUS"
fi

# ── 2. Body size limit ───────────────────────────────────────────────────────
echo ""
echo "[2/6] Request body size limit..."
# > 1 MB body must go through a file: a 1.1 MB CLI argument exceeds the kernel
# per-arg limit (Linux MAX_ARG_STRLEN 128 KiB) and curl never executes.
BIG_FILE=$(mktemp)
python3 -c "import sys; sys.stdout.write('x' * 1100000)" > "$BIG_FILE"
STATUS=$(status_of -X POST "${API_URL}/locations" \
  -H "Content-Type: application/json" --data-binary "@${BIG_FILE}")
rm -f "$BIG_FILE"
check "oversized body → 413" "413" "$STATUS"

# ── 3. Unknown route handling ────────────────────────────────────────────────
echo ""
echo "[3/6] Unknown route handling..."
STATUS=$(status_of "${API_URL}/nonexistent/route/xyz")
check "unknown route → 404" "404" "$STATUS"

BODY=$(curl -s "${API_URL}/nonexistent/route/xyz" 2>/dev/null || echo "")
# grep -q (not -qv): assert the trace marker is ABSENT. grep -qv succeeds if
# ANY line lacks the marker — a tautology on multi-line bodies (DRB-WP26-003).
check "no stack trace in 404" "true" "$(echo "$BODY" | grep -q "goroutine" && echo false || echo true)"

# ── 4. Health probes responsive ──────────────────────────────────────────────
echo ""
echo "[4/6] Health probes responsive..."
STATUS=$(status_of "${BASE_URL}/healthz")
check "healthz → 200" "200" "$STATUS"

STATUS=$(status_of "${BASE_URL}/readyz")
check "readyz → 200" "200" "$STATUS"

# ── 5. CORS rejection ────────────────────────────────────────────────────────
echo ""
echo "[5/6] CORS rejection for non-allowlisted origin..."
# GET with dumped headers (curl -I sends HEAD → Gin 404 → vacuous pass).
CORS_HEADER=$(curl -s -D - -o /dev/null -H "Origin: https://evil.example.com" "${API_URL}/locations" 2>/dev/null | grep -i "Access-Control-Allow-Origin" || echo "")
check "no CORS for evil origin" "true" "$([ -z "$CORS_HEADER" ] && echo true || echo false)"

# ── 6. Rate-limit enforcement (LAST — drains the shared bucket) ──────────────
echo ""
echo "[6/6] Rate-limit enforcement..."
GOT_429=false
for i in $(seq 1 200); do
  STATUS=$(status_of "${API_URL}/locations")
  if [ "$STATUS" = "429" ]; then
    GOT_429=true
    break
  fi
done
check "429 returned on budget exhaustion" "true" "$GOT_429"

if [ "$GOT_429" = "true" ]; then
  RETRY_AFTER=$(curl -s -D - -o /dev/null "${API_URL}/locations" 2>/dev/null | grep -i "Retry-After" | tr -d '\r\n' || echo "")
  check "Retry-After header present" "true" "$([ -n "$RETRY_AFTER" ] && echo true || echo false)"
fi

# ── Summary ──────────────────────────────────────────────────────────────────
echo ""
echo "=== Results: ${PASS}/${TOTAL} passed, ${FAIL} failed ==="

if [ "$FAIL" -gt 0 ]; then
  echo "RELIABILITY SUITE FAILED"
  exit 1
fi

echo "All reliability tests passed (slice 1; fault-injection scenarios tracked in WP-26b)."
exit 0
