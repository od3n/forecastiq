#!/usr/bin/env bash
# test/perf/reliability-faults.sh — Reliability fault-injection suite (WP-26b,
# slice 2 of the WP-26 reliability layer; completes DRB-WP26-001).
#
# Injects real faults into the docker-compose stack and asserts recovery:
#   1. Provider timeout   — Open-Meteo api_base_url → hanging fake provider;
#                           collection classified 'timeout'; health stays up.
#   2. Duplicate job      — same-hour double trigger; second run stores 0 new
#                           snapshots (dedup boundary, domain §4.3).
#   3. Late observation   — correcting row for an already-matched hour →
#                           recompute → NEW pair added, original pair retained.
#   4. Worker restart     — docker compose restart app; readyz recovers.
#   5. DB reconnect       — docker compose stop/start postgres; readyz drops,
#                           then recovers; reads work again.
#
# Reference: docs/testing/02-testing-strategy.md §1 (Reliability layer)
# Reference: docs/reviews/work-packages/WP-26-delivery-review.md (DRB-WP26-001)
#
# REQUIREMENTS:
#   - the ISOLATED perf stack up (make perf-up; compose project fiqperf) with
#     FIQ_ENV=development — defaults below target it, never the dev stack:
#     the suite mutates providers.api_base_url, inserts a correcting
#     observation (scenario 3, intentionally persistent), and restarts/stops
#     stack services
#   - perf dataset seeded (go run ./test/perf/seeder --preset=base|analysis):
#     provides matched pairs (scenario 3) and the perf admin user
#   - ADMIN_TOKEN (default perf-admin-token, mapped by the dev verifier)
#
# The request-path reliability checks (malformed payload, body limit, 404,
# health, CORS, rate limit) live in reliability.sh (WP-26 slice 1); run both
# for the full reliability layer. Like slice 1, expected-error probes use curl
# WITHOUT -f (DRB-WP26-001 note).
#
# Usage: bash test/perf/reliability-faults.sh [base_url]
set -euo pipefail

BASE_URL="${1:-http://localhost:28080}"
API_URL="${BASE_URL}/api/v1"
ADMIN_TOKEN="${ADMIN_TOKEN:-perf-admin-token}"
# Defaults address the ISOLATED fiqperf project (DRB-WP26b-002): a bare
# invocation must never rewrite provider URLs or stop the database of a
# developer stack.
COMPOSE="${COMPOSE:-docker compose -p fiqperf -f docker-compose.yml -f test/perf/compose.perf.yml}"
NETWORK="${NETWORK:-fiqperf_default}"
FAKE_NAME="fiq-fakeprovider"
FAKE_URL="http://${FAKE_NAME}:8080"
OPEN_METEO_ID="00000000-0000-0000-0000-000000000010"
JB_LOCATION_ID="00000000-0000-0000-0000-000000000030"
# 5 transport attempts x FIQ_PROVIDER_TIMEOUT (10s default) + backoff ~15s.
TIMEOUT_SCENARIO_MAX="${TIMEOUT_SCENARIO_MAX:-240}"

PASS=0
FAIL=0
TOTAL=0

check() {
  local name="$1" expected="$2" actual="$3"
  TOTAL=$((TOTAL + 1))
  if [ "$actual" = "$expected" ]; then
    echo "  [PASS] $name"
    PASS=$((PASS + 1))
  else
    echo "  [FAIL] $name (expected=$expected, got=$actual)"
    FAIL=$((FAIL + 1))
  fi
}

status_of() {
  # -w already emits 000 when the request fails; no fallback echo (it would
  # concatenate to "000000").
  curl -s -o /dev/null -w "%{http_code}" --max-time 10 "$@" 2>/dev/null || true
}

psqlc() {
  $COMPOSE exec -T postgres psql -U forecastiq -d forecastiq -tAc "$1"
}

trigger() { # trigger [max-time-seconds] — POST /admin/collections/trigger; prints HTTP status
  curl -s -o /dev/null -w "%{http_code}" --max-time "${1:-120}" -X POST \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" -H "Content-Type: application/json" \
    -d "{\"provider_id\":\"${OPEN_METEO_ID}\",\"location_id\":\"${JB_LOCATION_ID}\"}" \
    "${API_URL}/admin/collections/trigger" 2>/dev/null || true
}

wait_status() { # wait_status <url> <expected> <seconds>
  local url="$1" expected="$2" deadline=$(($(date +%s) + $3))
  while [ "$(date +%s)" -lt "$deadline" ]; do
    if [ "$(status_of "$url")" = "$expected" ]; then
      return 0
    fi
    sleep 2
  done
  return 1
}

start_fake() { # start_fake <mode>
  docker rm -f "$FAKE_NAME" >/dev/null 2>&1 || true
  docker run -d --rm --name "$FAKE_NAME" --network "$NETWORK" \
    -v "$PWD/test/perf/fakeprovider.py:/srv/fakeprovider.py:ro" \
    -e MODE="$1" python:3.12-alpine python /srv/fakeprovider.py >/dev/null
}

ORIG_PROVIDER_URL=""
cleanup() {
  if [ -n "$ORIG_PROVIDER_URL" ]; then
    psqlc "UPDATE providers SET api_base_url = '${ORIG_PROVIDER_URL}' WHERE id = '${OPEN_METEO_ID}'" >/dev/null 2>&1 || true
    # Close the circuit the timeout scenario opened so later runs (and the
    # stack's own scheduler) start clean.
    psqlc "UPDATE provider_circuits SET state='closed', consecutive_failures=0, opened_at=NULL, next_probe_at=NULL
      WHERE provider_id = '${OPEN_METEO_ID}'" >/dev/null 2>&1 || true
  fi
  docker rm -f "$FAKE_NAME" >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "=== ForecastIQ Reliability Fault-Injection Suite (WP-26b) ==="
echo "Target: ${BASE_URL}"
echo ""

if [ "$(status_of "${BASE_URL}/readyz")" != "200" ]; then
  echo "ERROR: stack not ready at ${BASE_URL} — run 'make perf-up' (and seed) first."
  exit 1
fi
ORIG_PROVIDER_URL=$(psqlc "SELECT api_base_url FROM providers WHERE id = '${OPEN_METEO_ID}'")
# Poisoning guard: if a previous run died between the URL swap and its trap,
# the "original" we just captured IS the fake URL — restoring it on exit would
# leave the provider permanently broken. Fall back to the canonical upstream.
if [ "$ORIG_PROVIDER_URL" = "$FAKE_URL" ]; then
  ORIG_PROVIDER_URL="https://api.open-meteo.com"
  echo "WARN: provider URL was left pointing at the fake by a previous run; will restore ${ORIG_PROVIDER_URL}"
fi

# ── 1. Provider timeout ─────────────────────────────────────────────────────
echo "[1/5] Provider timeout (hanging upstream)..."
# Close the circuit first: repeated suite runs open it via FC-09 (consecutive
# timeout failures), and an open circuit short-circuits the trigger with 409
# before any collection row is written.
psqlc "UPDATE provider_circuits SET state='closed', consecutive_failures=0, opened_at=NULL, next_probe_at=NULL
  WHERE provider_id = '${OPEN_METEO_ID}'" >/dev/null
MARKER=$(psqlc "SELECT now()")
start_fake hang
psqlc "UPDATE providers SET api_base_url = '${FAKE_URL}' WHERE id = '${OPEN_METEO_ID}'" >/dev/null

STATUS_FILE=$(mktemp)
trigger "$TIMEOUT_SCENARIO_MAX" > "$STATUS_FILE" &
TRIGGER_PID=$!
# Health must stay responsive while the collection stalls in retry/backoff.
HEALTH_OK=true
for _ in 1 2 3 4 5; do
  sleep 3
  [ "$(status_of "${BASE_URL}/healthz")" = "200" ] || HEALTH_OK=false
done
wait "$TRIGGER_PID" || true
check "healthz responsive during provider stall" "true" "$HEALTH_OK"

# Back-to-back suite runs can drain the 6/min provider token bucket, in which
# case the trigger is refused (429) before any row is written — wait for the
# refill and re-trigger once.
if [ "$(cat "$STATUS_FILE")" = "429" ]; then
  echo "  (provider budget drained by a prior run — waiting 65s and re-triggering)"
  sleep 65
  trigger "$TIMEOUT_SCENARIO_MAX" > "$STATUS_FILE" || true
fi
rm -f "$STATUS_FILE"

# Assert on the collection created by THIS trigger. NOTE: requested_at is the
# HOUR-TRUNCATED issuance (domain §4.3), so the marker must compare against
# created_at (the actual insert instant).
LAST_STATUS=$(psqlc "SELECT collection_status FROM forecast_collections
  WHERE provider_id = '${OPEN_METEO_ID}' AND location_id = '${JB_LOCATION_ID}'
    AND created_at >= '${MARKER}'
  ORDER BY created_at DESC LIMIT 1")
check "collection classified 'timeout'" "timeout" "${LAST_STATUS:-no-row}"

# Refill the provider token bucket (6/min) drained by the retry attempts.
echo "  (waiting 65s for the provider rate budget to refill)"
sleep 65

# ── 2. Duplicate job ────────────────────────────────────────────────────────
echo ""
echo "[2/5] Duplicate job (same-hour double trigger)..."
start_fake ok
STATUS1=$(trigger 60)
STATUS2=$(trigger 60)
check "first trigger 200" "200" "$STATUS1"
check "second trigger 200" "200" "$STATUS2"

# Both runs share the hour-truncated dedup key, so the SECOND run must be
# recorded as a 'deduplicated' collection owning zero new snapshots (domain
# §4.3; WP-08 DRB regression: replay/dedup rows never shadow LatestSuccessful).
read -r STATUS2ROW STORED2 DEDUP2 RECEIVED2 <<< "$(psqlc "SELECT collection_status, snapshots_stored, snapshots_deduplicated, records_received
  FROM forecast_collections
  WHERE provider_id = '${OPEN_METEO_ID}' AND location_id = '${JB_LOCATION_ID}'
    AND collection_status IN ('success','deduplicated')
  ORDER BY created_at DESC LIMIT 1" | tr '|' ' ')"
check "second run recorded as 'deduplicated'" "deduplicated" "${STATUS2ROW:-missing}"
check "duplicate run stores 0 new snapshots" "0" "${STORED2:-missing}"
check "duplicate run dedups all received rows" "${RECEIVED2:-x}" "${DEDUP2:-y}"

psqlc "UPDATE providers SET api_base_url = '${ORIG_PROVIDER_URL}' WHERE id = '${OPEN_METEO_ID}'" >/dev/null
docker rm -f "$FAKE_NAME" >/dev/null 2>&1 || true

# ── 3. Late observation (correction → rematch adds, never edits) ───────────
echo ""
echo "[3/5] Late observation correction → rematch..."
PAIR=$(psqlc "SELECT m.observation_id, o.location_id, o.source, o.observed_at, o.temperature_c
  FROM matched_evaluations m
  JOIN observations o ON o.id = m.observation_id AND o.observed_at = m.target_time
  WHERE o.superseded_observation_id IS NULL
  ORDER BY m.target_time DESC LIMIT 1")
if [ -z "$PAIR" ]; then
  check "matched pair available (seed the perf dataset first)" "present" "absent"
else
  OLD_OBS=$(echo "$PAIR" | cut -d'|' -f1)
  NEW_OBS=$(psqlc "SELECT gen_random_uuid()")
  # Supersede-then-insert, the WP-10 correction order: the partial dedup index
  # allows only ONE live row per (source, location, hour), so the old row must
  # leave the live set before the correcting row lands. Single psql command =
  # single transaction.
  psqlc "UPDATE observations SET superseded_observation_id = '${NEW_OBS}' WHERE id = '${OLD_OBS}';
    INSERT INTO observations (id, location_id, source, observation_type, observed_at, temperature_c, quality_flag)
    SELECT '${NEW_OBS}', location_id, source, observation_type, observed_at,
           COALESCE(temperature_c, 30) + 0.5, 'corrected'
    FROM observations WHERE id = '${OLD_OBS}'" >/dev/null
  OLD_PAIRS=$(psqlc "SELECT count(*) FROM matched_evaluations WHERE observation_id = '${OLD_OBS}'")

  RECOMPUTE=$(curl -s -o /dev/null -w "%{http_code}" --max-time 600 -X POST \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" "${API_URL}/admin/recompute" 2>/dev/null || true)
  check "recompute 200" "200" "$RECOMPUTE"

  NEW_PAIRS=$(psqlc "SELECT count(*) FROM matched_evaluations WHERE observation_id = '${NEW_OBS}'")
  OLD_PAIRS_AFTER=$(psqlc "SELECT count(*) FROM matched_evaluations WHERE observation_id = '${OLD_OBS}'")
  check "rematch created pair(s) for the correcting row" "true" "$([ "${NEW_PAIRS:-0}" -ge 1 ] && echo true || echo false)"
  check "original pair(s) retained (append-only)" "$OLD_PAIRS" "$OLD_PAIRS_AFTER"
fi

# ── 4. Worker restart ───────────────────────────────────────────────────────
echo ""
echo "[4/5] Worker restart (docker compose restart app)..."
$COMPOSE restart app >/dev/null 2>&1
if wait_status "${BASE_URL}/readyz" 200 90; then
  check "readyz recovered after app restart" "200" "200"
else
  check "readyz recovered after app restart" "200" "$(status_of "${BASE_URL}/readyz")"
fi
check "public read serves after restart" "200" "$(status_of "${API_URL}/locations")"

# ── 5. DB reconnect (stop/start postgres) ───────────────────────────────────
echo ""
echo "[5/5] DB reconnect (docker compose stop/start postgres)..."
$COMPOSE stop postgres >/dev/null 2>&1
DOWN=false
DEADLINE=$(($(date +%s) + 30))
while [ "$(date +%s)" -lt "$DEADLINE" ]; do
  if [ "$(status_of "${BASE_URL}/readyz")" != "200" ]; then
    DOWN=true
    break
  fi
  sleep 2
done
check "readyz reports not-ready while DB is down" "true" "$DOWN"

$COMPOSE start postgres >/dev/null 2>&1
if wait_status "${BASE_URL}/readyz" 200 120; then
  check "readyz recovered after DB restart" "200" "200"
else
  check "readyz recovered after DB restart" "200" "$(status_of "${BASE_URL}/readyz")"
fi
check "public read serves after DB reconnect" "200" "$(status_of "${API_URL}/locations")"

# ── Summary ──────────────────────────────────────────────────────────────────
echo ""
echo "=== Results: ${PASS}/${TOTAL} passed, ${FAIL} failed ==="
if [ "$FAIL" -gt 0 ]; then
  echo "RELIABILITY FAULT-INJECTION SUITE FAILED"
  exit 1
fi
echo "All fault-injection scenarios passed."
exit 0
