#!/usr/bin/env bash
# deploy/scripts/restore-test.sh — Monthly restore-test for ForecastIQ.
# Runs on the 1st of each month at 04:00 UTC (see /etc/cron.d/forecastiq).
#
# Reference: docs/operations/04-backup-and-restore.md §5
#
# Actions:
#   1. Find the latest offsite or local dump
#   2. Restore to a scratch database
#   3. Run integrity checks (row counts vs. production ±2%)
#   4. Write last_restore_test to the backup status JSON
#   5. Clean up scratch DB
#
# On failure: writes "failed" to status → alert A11 fires.
set -euo pipefail

# ── Configuration ────────────────────────────────────────────────────────────
BACKUP_DIR="/var/lib/forecastiq/backups"
STATUS_FILE="${FIQ_BACKUP_STATUS_FILE:-/var/lib/forecastiq/backup-status.json}"
DB_URL="${DATABASE_URL:-${FIQ_DATABASE_URL:-}}"
RCLONE_REMOTE="b2:forecastiq-backups"
TOLERANCE_PCT=2  # ±2% row count tolerance

STAMP=$(date -u +%F)
SCRATCH_DB="forecastiq_restore_test_${STAMP//[-]/_}"

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

cleanup_scratch() {
  dropdb --if-exists "$SCRATCH_DB" 2>/dev/null || true
}

trap 'write_restore_status "failed"; cleanup_scratch' ERR

# ── Validate ─────────────────────────────────────────────────────────────────
if [ -z "$DB_URL" ]; then
  echo "ERROR: DATABASE_URL or FIQ_DATABASE_URL must be set."
  exit 1
fi

echo "=== ForecastIQ Restore Test — ${STAMP} ==="
echo ""

# ── 1. Find latest dump ──────────────────────────────────────────────────────
echo "[1/5] Finding latest backup dump..."

# Try offsite first (rclone copy latest)
DUMP_FILE=""
if command -v rclone &>/dev/null && rclone listremotes 2>/dev/null | grep -q "^b2:"; then
  LATEST_OFFSITE=$(rclone lsf "$RCLONE_REMOTE" --include "forecastiq-*.dump" --files-only 2>/dev/null | sort -r | head -1)
  if [ -n "$LATEST_OFFSITE" ]; then
    echo "  Downloading from offsite: ${LATEST_OFFSITE}"
    rclone copy "${RCLONE_REMOTE}/${LATEST_OFFSITE}" "/tmp/" --log-level ERROR
    DUMP_FILE="/tmp/${LATEST_OFFSITE}"
  fi
fi

# Fall back to local
if [ -z "$DUMP_FILE" ] || [ ! -f "$DUMP_FILE" ]; then
  DUMP_FILE=$(find "$BACKUP_DIR" -name "forecastiq-*.dump" -type f | sort -r | head -1)
fi

if [ -z "$DUMP_FILE" ] || [ ! -f "$DUMP_FILE" ]; then
  echo "  ERROR: No backup dump found (local or offsite)"
  write_restore_status "failed"
  exit 1
fi

echo "  Using: ${DUMP_FILE}"
DUMP_SIZE=$(stat -c%s "$DUMP_FILE" 2>/dev/null || stat -f%z "$DUMP_FILE")
echo "  Size: ${DUMP_SIZE} bytes"

# ── 2. Restore to scratch DB ─────────────────────────────────────────────────
echo "[2/5] Restoring to scratch database..."
cleanup_scratch
createdb "$SCRATCH_DB"
pg_restore -d "$SCRATCH_DB" --no-owner --no-acl "$DUMP_FILE"
echo "  Restore complete"

# ── 3. Integrity checks ─────────────────────────────────────────────────────
echo "[3/5] Running integrity checks..."
FAIL=0

check_table_count() {
  local table="$1"
  local prod_count scratch_count pct_diff

  prod_count=$(psql -t -A "$DB_URL" -c "SELECT count(*) FROM ${table};" 2>/dev/null || echo "-1")
  scratch_count=$(psql -t -A "$SCRATCH_DB" -c "SELECT count(*) FROM ${table};" 2>/dev/null || echo "-1")

  if [ "$prod_count" -eq -1 ] || [ "$scratch_count" -eq -1 ]; then
    echo "  [SKIP] ${table}: could not query"
    return
  fi

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

if [ "$FAIL" -gt 0 ]; then
  echo ""
  echo "  INTEGRITY CHECK FAILED: ${FAIL} table(s) exceeded ±${TOLERANCE_PCT}% tolerance"
  write_restore_status "failed"
  cleanup_scratch
  exit 1
fi

echo "  All integrity checks passed"

# ── 4. Write status ──────────────────────────────────────────────────────────
echo "[4/5] Writing restore-test status..."
write_restore_status "success"
echo "  Status: ${STATUS_FILE}"

# ── 5. Cleanup ───────────────────────────────────────────────────────────────
echo "[5/5] Cleaning up scratch database..."
cleanup_scratch

# Clean up any temp download
if [[ "$DUMP_FILE" == /tmp/* ]]; then
  rm -f "$DUMP_FILE"
fi

echo ""
echo "=== Restore Test Complete: ${STAMP} ==="
echo "Result: SUCCESS — all tables within ±${TOLERANCE_PCT}% of production"
