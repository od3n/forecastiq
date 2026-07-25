#!/usr/bin/env bash
# deploy/scripts/backup.sh — Nightly PostgreSQL backup for ForecastIQ.
# Runs as cron at 02:30 UTC (see /etc/cron.d/forecastiq).
#
# Reference: docs/operations/04-backup-and-restore.md §3
#
# Actions:
#   1. pg_dump -Fc to /var/lib/forecastiq/backups/
#   2. Immediate integrity check (test-restore to scratch schema)
#   3. Write backup status JSON (consumed by /admin/health)
#   4. Prune local backups > 30 days
#   5. Weekly offsite sync to B2 via rclone (Sundays)
#
# Environment (from /etc/forecastiq/secrets.env via cron):
#   DATABASE_URL       — PostgreSQL connection string (app role or migrate role)
#   FIQ_BACKUP_STATUS_FILE — Path to write status JSON (default below)
#
# On failure: non-zero exit → cron sends mail; status file written as "failed"
# → alert A10 fires.
set -euo pipefail

# ── Configuration ────────────────────────────────────────────────────────────
BACKUP_DIR="/var/lib/forecastiq/backups"
STATUS_FILE="${FIQ_BACKUP_STATUS_FILE:-/var/lib/forecastiq/backup-status.json}"
RETENTION_DAYS=30
RCLONE_REMOTE="b2:forecastiq-backups"
DB_URL="${DATABASE_URL:-${FIQ_DATABASE_URL:-}}"

STAMP=$(date -u +%F)
DUMP_FILE="${BACKUP_DIR}/forecastiq-${STAMP}.dump"
SCRATCH_DB="forecastiq_backup_verify_${STAMP//[-]/_}"

# ── Helpers ──────────────────────────────────────────────────────────────────
write_status() {
  local status="$1"
  local size="${2:-0}"
  local now
  now=$(date -u +%Y-%m-%dT%H:%M:%SZ)

  # Read existing restore-test entry (preserve it)
  local restore_test="null"
  if [ -f "$STATUS_FILE" ]; then
    restore_test=$(jq -c '.last_restore_test // null' "$STATUS_FILE" 2>/dev/null || echo "null")
  fi

  cat > "$STATUS_FILE" <<EOF
{
  "last_backup": {
    "completed_at": "${now}",
    "status": "${status}",
    "size_bytes": ${size}
  },
  "last_restore_test": ${restore_test}
}
EOF
  chmod 0644 "$STATUS_FILE"
}

cleanup_scratch() {
  dropdb --if-exists "$SCRATCH_DB" 2>/dev/null || true
}

trap 'write_status "failed"; cleanup_scratch' ERR

# ── Validate ─────────────────────────────────────────────────────────────────
if [ -z "$DB_URL" ]; then
  echo "ERROR: DATABASE_URL or FIQ_DATABASE_URL must be set."
  exit 1
fi

mkdir -p "$BACKUP_DIR"

echo "=== ForecastIQ Backup — ${STAMP} ==="
echo "Target: ${DUMP_FILE}"
echo ""

# ── 1. Dump ──────────────────────────────────────────────────────────────────
echo "[1/5] Running pg_dump..."
pg_dump -Fc --no-owner --no-acl "$DB_URL" > "$DUMP_FILE"
DUMP_SIZE=$(stat -c%s "$DUMP_FILE" 2>/dev/null || stat -f%z "$DUMP_FILE")
echo "  Dump complete: ${DUMP_SIZE} bytes"

# ── 2. Integrity check (test-restore) ────────────────────────────────────────
echo "[2/5] Integrity check (test-restore to scratch schema)..."
cleanup_scratch
createdb "$SCRATCH_DB"
pg_restore -d "$SCRATCH_DB" --no-owner --no-acl "$DUMP_FILE"

# Verify key tables exist and have data
SNAPSHOT_COUNT=$(psql -t -A "$SCRATCH_DB" -c "SELECT count(*) FROM forecast_snapshots;" 2>/dev/null || echo "0")
SCHEDULE_COUNT=$(psql -t -A "$SCRATCH_DB" -c "SELECT count(*) FROM collection_schedules;" 2>/dev/null || echo "0")
echo "  Restored: forecast_snapshots=${SNAPSHOT_COUNT}, collection_schedules=${SCHEDULE_COUNT}"

if [ "$SCHEDULE_COUNT" -eq 0 ]; then
  echo "  WARNING: collection_schedules is empty — possible corruption"
fi

cleanup_scratch
echo "  Integrity check passed"

# ── 3. Write status file ─────────────────────────────────────────────────────
echo "[3/5] Writing status file..."
write_status "success" "$DUMP_SIZE"
echo "  Status: ${STATUS_FILE}"

# ── 4. Prune old backups ─────────────────────────────────────────────────────
echo "[4/5] Pruning backups older than ${RETENTION_DAYS} days..."
PRUNED=$(find "$BACKUP_DIR" -name "forecastiq-*.dump" -mtime +${RETENTION_DAYS} -delete -print | wc -l)
echo "  Pruned: ${PRUNED} files"

# ── 5. Offsite sync (Sundays only) ──────────────────────────────────────────
DOW=$(date -u +%u)  # 7 = Sunday
if [ "$DOW" -eq 7 ]; then
  echo "[5/5] Weekly offsite sync to B2..."
  if command -v rclone &>/dev/null && rclone listremotes 2>/dev/null | grep -q "^b2:"; then
    rclone sync "$BACKUP_DIR" "$RCLONE_REMOTE" \
      --include "forecastiq-*.dump" \
      --transfers 2 \
      --checkers 4 \
      --log-level INFO
    echo "  Offsite sync complete"
  else
    echo "  SKIP: rclone not configured (b2 remote missing)"
  fi
else
  echo "[5/5] Offsite sync: skipped (not Sunday; DOW=${DOW})"
fi

echo ""
echo "=== Backup Complete: ${STAMP} (${DUMP_SIZE} bytes) ==="
