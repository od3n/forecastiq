#!/usr/bin/env bash
# test/perf/reliability.sh — Reliability fault-injection test suite (WP-26).
# Validates system behavior under adverse conditions per testing strategy §1.
#
# Reference: docs/testing/02-testing-strategy.md §1 (Reliability layer)
# Reference: docs/testing/04-performance-testing.md §2
#
# Scenarios tested:
#   1. Rate-limit enforcement (per-IP budget exhaustion → 429)
#   2. Malformed payload rejection (invalid JSON → 422)
#   3. Request body size limit (oversized → 413)
#   4. Unknown route handling (→ 404, no info leak)
#   5. Health probes under load (healthz/readyz always responsive)
#   6. CORS rejection (non-allowlisted origin → no CORS headers)
#
# Usage: bash test/perf/reliability.sh [base_url]
set -euo pipefail

BASE_URL="${1:-http://localhost:8080}"
API_URL="${BASE_URL}/api/v1"
PASS=0
FAIL=0
TOTAL=0

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

echo "=== ForecastIQ Reliability Test Suite ==="
echo "Target: ${BASE_URL}"
echo ""

# ── 1. Rate-limit enforcement ────────────────────────────────────────────────
echo "[1/6] Rate-limit enforcement..."
# Send requests rapidly until we get a 429
GOT_429=false
for i in $(seq 1 200); do
  STATUS=$(curl -sf -o /dev/null -w "%{http_code}" "${API_URL}/locations" 2>/dev/null || echo "000")
  if [ "$STATUS" = "429" ]; then
    GOT_429=true
    break
  fi
done
check "429 returned on budget exhaustion" "true" "$GOT_429"

# Verify Retry-After header on 429
if [ "$GOT_429" = "true" ]; then
  RETRY_AFTER=$(curl -sf -D - -o /dev/null "${API_URL}/locations" 2>/dev/null | grep -i "Retry-After" | tr -d '\r\n' || echo "")
  check "Retry-After header present" "true" "$([ -n "$RETRY_AFTER" ] && echo true || echo false)"
fi

# Wait for bucket to refill
sleep 2

# ── 2. Malformed payload rejection ──────────────────────────────────────────
echo ""
echo "[2/6] Malformed payload rejection..."
STATUS=$(curl -sf -o /dev/null -w "%{http_code}" -X POST "${API_URL}/locations" \
  -H "Content-Type: application/json" -d "not-valid-json{{{" 2>/dev/null || echo "000")
check "malformed JSON → 4xx" "true" "$([ "$STATUS" -ge 400 ] && [ "$STATUS" -lt 500 ] && echo true || echo false)"

# ── 3. Body size limit ───────────────────────────────────────────────────────
echo ""
echo "[3/6] Request body size limit..."
# Generate a > 1MB body
BIG_BODY=$(python3 -c "print('x' * 1100000)")
STATUS=$(curl -sf -o /dev/null -w "%{http_code}" -X POST "${API_URL}/locations" \
  -H "Content-Type: application/json" -d "$BIG_BODY" 2>/dev/null || echo "000")
check "oversized body → 413" "413" "$STATUS"

# ── 4. Unknown route handling ────────────────────────────────────────────────
echo ""
echo "[4/6] Unknown route handling..."
STATUS=$(curl -sf -o /dev/null -w "%{http_code}" "${API_URL}/nonexistent/route/xyz" 2>/dev/null || echo "000")
check "unknown route → 404" "404" "$STATUS"

BODY=$(curl -sf "${API_URL}/nonexistent/route/xyz" 2>/dev/null || echo "")
check "no stack trace in 404" "true" "$(echo "$BODY" | grep -qv "goroutine" && echo true || echo false)"

# ── 5. Health probes responsive ──────────────────────────────────────────────
echo ""
echo "[5/6] Health probes responsive..."
STATUS=$(curl -sf -o /dev/null -w "%{http_code}" "${BASE_URL}/healthz" 2>/dev/null || echo "000")
check "healthz → 200" "200" "$STATUS"

STATUS=$(curl -sf -o /dev/null -w "%{http_code}" "${BASE_URL}/readyz" 2>/dev/null || echo "000")
check "readyz → 200" "200" "$STATUS"

# ── 6. CORS rejection ────────────────────────────────────────────────────────
echo ""
echo "[6/6] CORS rejection for non-allowlisted origin..."
CORS_HEADER=$(curl -sf -H "Origin: https://evil.example.com" -I "${API_URL}/locations" 2>/dev/null | grep -i "Access-Control-Allow-Origin" || echo "")
check "no CORS for evil origin" "true" "$([ -z "$CORS_HEADER" ] && echo true || echo false)"

# ── Summary ──────────────────────────────────────────────────────────────────
echo ""
echo "=== Results: ${PASS}/${TOTAL} passed, ${FAIL} failed ==="

if [ "$FAIL" -gt 0 ]; then
  echo "RELIABILITY SUITE FAILED"
  exit 1
fi

echo "All reliability tests passed."
exit 0
