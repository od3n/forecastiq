#!/usr/bin/env bash
# deploy/scripts/restore-test.sh — Monthly restore-test for ForecastIQ.
# Runs on the 1st of each month at 04:00 UTC (see /etc/cron.d/forecastiq).
#
# Reference: docs/operations/04-backup-and-restore.md §5
#
# Actions:
#   1. Find the latest offsite or local dump
#   2. Restore to a scratch database (same cluster as DATABASE_URL)
#   3. Run integrity checks (row counts vs. production ±2%)
#   4. Write last_restore_test to the backup status JSON
#   5. Clean up scratch DB + temp download
#
# On failure: writes "failed" to status → alert A11 fires.
# -E ensures the ERR trap fires inside functions too.
set -Eeuo pipefail

# ── Configuration ────────────────────────────────────────────────────────────
BACKUP_DIR="/var/lib/forecastiq/backups"
STATUS_FILE="${FIQ_BACKUP_STATUS_FILE:-/var/lib/forecastiq/backup-status.json}"
DB_URL="${DATABASE_URL:-${FIQ_DATABASE_URL:-}}"
RCLONE_REMOTE="b2:forecastiq-backups"
TOLERANCE_PCT=2   # ±2% row count tolerance
MIN_VERIFIED=3    # at least this many tables must be actually verified (not skipped)

STAMP=$(date -u +%F)
SCRATCH_DB="forecastiq_restore_test_${STAMP//[-]/_}"
TMP_DIR=""

# ── Helpers ──────────────────────────────────────────────────────────────────
write_restore_status() {
  local status="$1"
  local now
  now=$(date -u +%Y-%m-%dT%H:%M:%SZ)

  # Read existing backup entry (preserve it)
  local last_backup="null"
  if [ -f "$STATUS_FILE" ]; then
    last_backup=$(jq -c '.last_backup // null' "$STATUS_FILE" 2>/dev/null || echo "null")
  fi

  cat > "$STATUS_FILE" <<EOF
{
  "last_backup": ${last_backup},
  "last_restore_test": {
    "completed_at": "${now}",
    "status": "${status}"
  }
}
EOF
  chmod 0644 "$STATUS_FILE"
}

# admin_url: maintenance connection to the given DB on the SAME cluster as
# DB_URL (createdb/psql target the cluster we verify against, not libpq defaults).
admin_url() {
  local dbname="$1"
  echo "$DB_URL" | sed -E "s|(postgres(ql)?://[^/]+)/[^?]*|\1/${dbname}|"
}

cleanup() {
  psql "$(admin_url postgres)" -c "DROP DATABASE IF EXISTS ${SCRATCH_DB};" >/dev/null 2>&1 || true
  if [ -n "$TMP_DIR" ] && [ -d "$TMP_DIR" ]; then
    rm -rf "$TMP_DIR"
  fi
}

on_error() {
  write_restore_status "failed" || true
  cleanup
}
trap on_error ERR

# ── Validate ─────────────────────────────────────────────────────────────────
if [ -z "$DB_URL" ]; then
  echo "ERROR: DATABASE_URL or FIQ_DATABASE_URL must be set."
  exit 1
fi

echo "=== ForecastIQ Restore Test — ${STAMP} ==="
echo ""

# ── 1. Find latest dump ──────────────────────────────────────────────────────
echo "[1/5] Finding latest backup dump..."

# Try offsite first (download into a private temp dir, cleaned on any exit path)
DUMP_FILE=""
if command -v rclone &>/dev/null && rclone listremotes 2>/dev/null | grep -q "^b2:"; then
  LATEST_OFFSITE=$(rclone lsf "$RCLONE_REMOTE" --include "forecastiq-*.dump" --files-only 2>/dev/null | sort -r | head -1)
  if [ -n "$LATEST_OFFSITE" ]; then
    TMP_DIR=$(mktemp -d)
    echo "  Downloading from offsite: ${LATEST_OFFSITE}"
    rclone copy "${RCLONE_REMOTE}/${LATEST_OFFSITE}" "$TMP_DIR/" --log-level ERROR
    DUMP_FILE="${TMP_DIR}/${LATEST_OFFSITE}"
  fi
fi

# Fall back to local
if [ -z "$DUMP_FILE" ] || [ ! -f "$DUMP_FILE" ]; then
  DUMP_FILE=$(find "$BACKUP_DIR" -name "forecastiq-*.dump" -type f | sort -r | head -1)
fi

if [ -z "$DUMP_FILE" ] || [ ! -f "$DUMP_FILE" ]; then
  echo "  ERROR: No backup dump found (local or offsite)"
  write_restore_status "failed"
  cleanup
  exit 1
fi

echo "  Using: ${DUMP_FILE}"
DUMP_SIZE=$(stat -c%s "$DUMP_FILE" 2>/dev/null || stat -f%z "$DUMP_FILE")
echo "  Size: ${DUMP_SIZE} bytes"

# ── 2. Restore to scratch DB ─────────────────────────────────────────────────
echo "[2/5] Restoring to scratch database..."
psql "$(admin_url postgres)" -c "DROP DATABASE IF EXISTS ${SCRATCH_DB};" >/dev/null 2>&1 || true
psql "$(admin_url postgres)" -c "CREATE DATABASE ${SCRATCH_DB};" >/dev/null
pg_restore -d "$(admin_url "$SCRATCH_DB")" --no-owner --no-acl "$DUMP_FILE"
echo "  Restore complete"

# ── 3. Integrity checks ─────────────────────────────────────────────────────
echo "[3/5] Running integrity checks..."
FAIL=0
VERIFIED=0

check_table_count() {
  local table="$1"
  local prod_count scratch_count pct_diff

  # A SCRATCH-side query failure is a FAILURE: the restored schema is broken.
  # Only a PROD-side failure (e.g. table doesn't exist yet in prod) is a skip.
  if ! prod_count=$(psql -t -A "$DB_URL" -c "SELECT count(*) FROM ${table};" 2>/dev/null); then
    echo "  [SKIP] ${table}: not queryable in production"
    return
  fi
  if ! scratch_count=$(psql -t -A "$(admin_url "$SCRATCH_DB")" -c "SELECT count(*) FROM ${table};" 2>/dev/null); then
    echo "  [FAIL] ${table}: exists in production but missing/broken in restored dump"
    FAIL=$((FAIL + 1))
    return
  fi

  VERIFIED=$((VERIFIED + 1))

  if [ "$prod_count" -eq 0 ]; then
    echo "  [OK]   ${table}: prod=0, restored=${scratch_count}"
    return
  fi

  pct_diff=$(( (prod_count - scratch_count) * 100 / prod_count ))
  if [ "${pct_diff#-}" -gt "$TOLERANCE_PCT" ]; then
    echo "  [FAIL] ${table}: prod=${prod_count}, restored=${scratch_count} (diff ${pct_diff}%)"
    FAIL=$((FAIL + 1))
  else
    echo "  [OK]   ${table}: prod=${prod_count}, restored=${scratch_count} (diff ${pct_diff}%)"
  fi
}

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
  write_restore_status "failed"
  cleanup
  exit 1
fi

if [ "$FAIL" -gt 0 ]; then
  echo ""
  echo "  INTEGRITY CHECK FAILED: ${FAIL} table(s) failed (tolerance ±${TOLERANCE_PCT}%)"
  write_restore_status "failed"
  cleanup
  exit 1
fi

echo "  All integrity checks passed (${VERIFIED} tables verified)"

# ── 4. Write status ──────────────────────────────────────────────────────────
echo "[4/5] Writing restore-test status..."
write_restore_status "success"
echo "  Status: ${STATUS_FILE}"

# ── 5. Cleanup ───────────────────────────────────────────────────────────────
echo "[5/5] Cleaning up..."
cleanup

echo ""
echo "=== Restore Test Complete: ${STAMP} ==="
echo "Result: SUCCESS — ${VERIFIED} tables within ±${TOLERANCE_PCT}% of production"
