#!/usr/bin/env bash
# test/perf/pt4-analysis-batch.sh — PT-4: analysis batch at volume (WP-26b).
# Times the REAL analysis pipeline (match → aggregate → rank) over a seeded
# 100K-pair dataset via POST /admin/recompute. Gate: batch < 10 min (NFR-P06).
#
# Reference: docs/testing/04-performance-testing.md §2 (PT-4), §5 (miss action)
#
# Setup (one-time, against the perf stack):
#   go run ./test/perf/seeder --preset=analysis --reset   # ≈100K in-window pairs
#
# Usage: bash test/perf/pt4-analysis-batch.sh [base_url]
#   ADMIN_TOKEN — bearer accepted by the admin group (dev stack maps
#                 "perf-admin-token" to the seeded perf admin; default).
#   PT4_BUDGET_SECONDS — gate override (default 600, NFR-P06).
set -euo pipefail

BASE_URL="${1:-http://localhost:8080}"
API_URL="${BASE_URL}/api/v1"
ADMIN_TOKEN="${ADMIN_TOKEN:-perf-admin-token}"
BUDGET="${PT4_BUDGET_SECONDS:-600}"

echo "=== PT-4 Analysis Batch at Volume ==="
echo "Target: ${API_URL}/admin/recompute  (gate: < ${BUDGET}s, NFR-P06)"
echo ""

START=$(date +%s)
BODY_FILE=$(mktemp)
# curl -w already prints 000 when the request itself fails; do NOT add a
# fallback echo or the two concatenate ("000000").
STATUS=$(curl -s -o "$BODY_FILE" -w "%{http_code}" -X POST \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  --max-time "$((BUDGET + 60))" \
  "${API_URL}/admin/recompute" || true)
STATUS="${STATUS:-000}"
END=$(date +%s)
ELAPSED=$((END - START))

echo "HTTP status:      ${STATUS}"
echo "Batch duration:   ${ELAPSED}s"
# records_affected without a jq dependency.
AFFECTED=$(grep -o '"records_affected":[0-9]*' "$BODY_FILE" | head -1 | cut -d: -f2 || true)
echo "Records affected: ${AFFECTED:-unknown}"
rm -f "$BODY_FILE"
echo ""

FAIL=0
if [ "$STATUS" != "200" ]; then
  echo "[FAIL] recompute returned ${STATUS} (expected 200 — is ADMIN_TOKEN valid and the stack in dev mode?)"
  FAIL=1
else
  echo "[PASS] recompute 200"
fi
if [ "$ELAPSED" -ge "$BUDGET" ]; then
  echo "[FAIL] batch ${ELAPSED}s >= ${BUDGET}s (NFR-P06) — see performance doc §5: chunking/parallelism review"
  FAIL=1
else
  echo "[PASS] batch ${ELAPSED}s < ${BUDGET}s (NFR-P06)"
fi

exit "$FAIL"
