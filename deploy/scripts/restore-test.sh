#!/usr/bin/env bash
# deploy/scripts/restore-test.sh — Monthly restore-test for ForecastIQ.
# Runs as the deploy user on the 1st of each month at 04:00 UTC.
#
# Reference: docs/operations/04-backup-and-restore.md §5 (as amended by ADR-033)
# Reference: docs/adr/ADR-033-personal-use-ec2-docker-deployment.md
#
# ADR-033 topology: restore into a THROWAWAY postgres:16-alpine container
# (never the production cluster — DRB-WP24-010), and read production row
# counts through the `db` compose service (no host port).
#
# Actions:
#   1. Find the latest offsite (or local) dump
#   2. Restore it into a scratch container
#   3. Integrity checks: schema present + restored row counts are not SHORT of
#      production beyond tolerance (append-only tables grow between the dump and
#      now, so restored may legitimately be LOWER than live — we only fail when
#      the dump is missing rows it should have, DRB-WP24-008)
#   4. Write last_restore_test (status + timestamp) to the backup status JSON;
#      a failed run sets status=failed → alert A11b fires (DRB-WP24-004)
#   5. Clean up scratch container + temp download
#
# Environment (overridable; defaults match the production host):
#   FIQ_COMPOSE_DIR / FIQ_BACKUP_DIR / FIQ_BACKUP_STATUS_FILE / FIQ_RCLONE_REMOTE
#   FIQ_DB_CONTAINER — co-tenant topology: name of an EXTERNAL postgres container
#                      holding the production DB (e.g. app-postgres-1). When
#                      unset, uses the compose `db` service (standalone default).
set -Eeuo pipefail

# ── Configuration ────────────────────────────────────────────────
COMPOSE_DIR="${FIQ_COMPOSE_DIR:-/opt/forecastiq}"
BACKUP_DIR="${FIQ_BACKUP_DIR:-/var/lib/forecastiq/backups}"
STATUS_FILE="${FIQ_BACKUP_STATUS_FILE:-/var/lib/forecastiq/backup-status.json}"
RCLONE_REMOTE="${FIQ_RCLONE_REMOTE:-b2:forecastiq-backups}"
DB_CONTAINER="${FIQ_DB_CONTAINER:-}"
PG_IMAGE="postgres:16-alpine"
TOLERANCE_PCT=2  # restored may be short of prod by at most this (dump age)
MIN_VERIFIED=3   # at least this many tables must be actually verified

STAMP=$(date -u +%F)
SCRATCH_NAME="fiq-restore-test-$$"
TMP_DIR=""

compose() { docker compose --project-directory "$COMPOSE_DIR" "$@"; }
prod_psql() {
  if [ -n "$DB_CONTAINER" ]; then
    docker exec -i "$DB_CONTAINER" psql -U forecastiq -d forecastiq -t -A "$@"
  else
    compose exec -T db psql -U forecastiq -d forecastiq -t -A "$@"
  fi
}
scratch_psql() { docker exec "$SCRATCH_NAME" psql -U postgres -d verify -t -A "$@"; }

# ── Helpers ──────────────────────────────────────────────────────
write_restore_status() {
  local status="$1"
  local now
  now=$(date -u +%Y-%m-%dT%H:%M:%SZ)

  local last_backup="null"
  if [ -s "$STATUS_FILE" ]; then
    last_backup=$(jq -c '.last_backup // null' "$STATUS_FILE" 2>/dev/null || echo "null")
  fi

  local tmp
  tmp=$(mktemp)
  cat > "$tmp" <<EOF
{
  "last_backup": ${last_backup},
  "last_restore_test": {
    "completed_at": "${now}",
    "status": "${status}"
  }
}
EOF
  cat "$tmp" > "$STATUS_FILE"
  rm -f "$tmp"
  chmod 0644 "$STATUS_FILE" 2>/dev/null || true
}

cleanup() {
  # -v: postgres:16-alpine declares an anonymous data volume; without -v every
  # scratch run leaks ~the DB size and eventually fills the shared disk.
  docker rm -fv "$SCRATCH_NAME" >/dev/null 2>&1 || true
  if [ -n "$TMP_DIR" ] && [ -d "$TMP_DIR" ]; then rm -rf "$TMP_DIR"; fi
}

on_error() {
  write_restore_status "failed" || true
  cleanup
}
trap on_error ERR

# ── Validate ─────────────────────────────────────────────────────
if ! command -v docker >/dev/null 2>&1; then
  echo "ERROR: docker not found."
  write_restore_status "failed"; exit 1
fi
if [ -n "$DB_CONTAINER" ]; then
  if ! docker ps --filter "name=^${DB_CONTAINER}$" --filter status=running --quiet | grep -q .; then
    echo "ERROR: co-tenant db container '${DB_CONTAINER}' is not running."
    write_restore_status "failed"; exit 1
  fi
elif ! compose ps --status running db --quiet 2>/dev/null | grep -q .; then
  echo "ERROR: db service is not running under ${COMPOSE_DIR}."
  write_restore_status "failed"; exit 1
fi

echo "=== ForecastIQ Restore Test — ${STAMP} ==="
echo ""

# ── 1. Find latest dump ──────────────────────────────────────────
echo "[1/5] Finding latest backup dump..."
DUMP_FILE=""
REMOTE_NAME="${RCLONE_REMOTE%%:*}:"
if command -v rclone >/dev/null 2>&1 && rclone listremotes 2>/dev/null | grep -q "^${REMOTE_NAME}$"; then
  LATEST_OFFSITE=$(rclone lsf "$RCLONE_REMOTE" --include "forecastiq-*.dump" --files-only 2>/dev/null | sort -r | head -1)
  if [ -n "$LATEST_OFFSITE" ]; then
    TMP_DIR=$(mktemp -d)
    echo "  Downloading from offsite: ${LATEST_OFFSITE}"
    rclone copy "${RCLONE_REMOTE}/${LATEST_OFFSITE}" "$TMP_DIR/" --log-level ERROR
    DUMP_FILE="${TMP_DIR}/${LATEST_OFFSITE}"
  fi
fi
if [ -z "$DUMP_FILE" ] || [ ! -f "$DUMP_FILE" ]; then
  DUMP_FILE=$(find "$BACKUP_DIR" -name "forecastiq-*.dump" -type f | sort -r | head -1)
fi
if [ -z "$DUMP_FILE" ] || [ ! -f "$DUMP_FILE" ]; then
  echo "  ERROR: No backup dump found (local or offsite)"
  write_restore_status "failed"; cleanup; exit 1
fi
echo "  Using: ${DUMP_FILE}"

# ── 2. Restore into a scratch container ──────────────────────────
echo "[2/5] Restoring into scratch ${PG_IMAGE} container..."
docker rm -fv "$SCRATCH_NAME" >/dev/null 2>&1 || true
docker run -d --name "$SCRATCH_NAME" -e POSTGRES_PASSWORD=scratch "$PG_IMAGE" >/dev/null
for i in $(seq 1 30); do
  if docker exec "$SCRATCH_NAME" pg_isready -U postgres >/dev/null 2>&1; then break; fi
  if [ "$i" -eq 30 ]; then echo "  ERROR: scratch postgres not ready after 30s"; false; fi
  sleep 1
done
docker exec "$SCRATCH_NAME" createdb -U postgres verify
docker exec -i "$SCRATCH_NAME" pg_restore -U postgres -d verify --no-owner --no-acl < "$DUMP_FILE"
echo "  Restore complete"

# ── 3. Integrity checks ──────────────────────────────────────────
echo "[3/5] Running integrity checks..."
FAIL=0
VERIFIED=0

check_table_count() {
  local table="$1"
  local prod_count scratch_count

  # A SCRATCH-side query failure is a FAILURE (restored schema broken); only a
  # PROD-side failure (table absent in prod) is a skip.
  if ! prod_count=$(prod_psql -c "SELECT count(*) FROM ${table};" 2>/dev/null); then
    echo "  [SKIP] ${table}: not queryable in production"
    return
  fi
  if ! scratch_count=$(scratch_psql -c "SELECT count(*) FROM ${table};" 2>/dev/null); then
    echo "  [FAIL] ${table}: exists in production but missing/broken in restored dump"
    FAIL=$((FAIL + 1)); return
  fi
  VERIFIED=$((VERIFIED + 1))

  if [ "$prod_count" -eq 0 ]; then
    echo "  [OK]   ${table}: prod=0, restored=${scratch_count}"; return
  fi

  # The dump is older than "now", and these tables are append-only, so
  # restored <= prod is expected. Only fail when the dump is SHORT beyond
  # tolerance (missing rows it should already have contained), or when
  # restored EXCEEDS prod (impossible for a past dump → data corruption).
  local short_pct=$(( (prod_count - scratch_count) * 100 / prod_count ))
  if [ "$scratch_count" -gt "$prod_count" ]; then
    echo "  [FAIL] ${table}: restored=${scratch_count} > prod=${prod_count} (impossible for a past dump)"
    FAIL=$((FAIL + 1))
  elif [ "$short_pct" -gt "$TOLERANCE_PCT" ]; then
    echo "  [FAIL] ${table}: prod=${prod_count}, restored=${scratch_count} (short ${short_pct}%)"
    FAIL=$((FAIL + 1))
  else
    echo "  [OK]   ${table}: prod=${prod_count}, restored=${scratch_count} (short ${short_pct}%)"
  fi
}

check_table_count "providers"
check_table_count "locations"
check_table_count "forecast_snapshots"
check_table_count "collection_schedules"
check_table_count "observations"
check_table_count "matched_evaluations"
check_table_count "accuracy_metrics"
check_table_count "users"

if [ "$VERIFIED" -lt "$MIN_VERIFIED" ]; then
  echo ""
  echo "  INTEGRITY CHECK FAILED: only ${VERIFIED} table(s) verified (need >= ${MIN_VERIFIED})."
  echo "  An all-SKIP run is a false green — treat as failure."
  write_restore_status "failed"; cleanup; exit 1
fi
if [ "$FAIL" -gt 0 ]; then
  echo ""
  echo "  INTEGRITY CHECK FAILED: ${FAIL} table(s) failed (tolerance ±${TOLERANCE_PCT}%)"
  write_restore_status "failed"; cleanup; exit 1
fi
echo "  All integrity checks passed (${VERIFIED} tables verified)"

# ── 4. Write status ──────────────────────────────────────────────
echo "[4/5] Writing restore-test status..."
write_restore_status "success"
echo "  Status: ${STATUS_FILE}"

# ── 5. Cleanup ───────────────────────────────────────────────────
echo "[5/5] Cleaning up..."
cleanup

echo ""
echo "=== Restore Test Complete: ${STAMP} ==="
echo "Result: SUCCESS — ${VERIFIED} tables verified within ±${TOLERANCE_PCT}%"
