#!/usr/bin/env bash
# deploy/scripts/backup.sh — Nightly PostgreSQL backup for ForecastIQ.
# Runs as the deploy user via cron at 02:30 UTC (see /etc/cron.d/forecastiq).
#
# Reference: docs/operations/04-backup-and-restore.md §3 (as amended by ADR-033)
# Reference: docs/adr/ADR-033-personal-use-ec2-docker-deployment.md
#
# ADR-033 topology: PostgreSQL runs as the compose `db` service with NO host
# port; all DB access goes through `docker compose exec db` (local socket,
# trust auth inside the official image — no password material needed here).
# The integrity test-restore runs in a THROWAWAY postgres:16-alpine container,
# never in the production cluster (DRB-WP24-010: a same-cluster restore can
# exhaust the shared EBS volume and take production down).
#
# Actions:
#   1. pg_dump -Fc via the db container (atomic: .tmp then mv)
#   2. Integrity check: test-restore into a scratch container
#   3. Write backup status JSON (consumed by /admin/health + A10 metrics)
#   4. Prune local backups > 30 days
#   5. Weekly offsite copy to B2 via rclone (Sundays) + offsite prune > 90 days
#      — a missing rclone remote on Sunday is a FAILURE, not a skip: offsite
#      is the only durability that survives instance/EBS loss (DRB-WP24-007).
#
# Environment (overridable; defaults match the production host):
#   FIQ_COMPOSE_DIR         — compose project dir (default /opt/forecastiq)
#   FIQ_BACKUP_DIR          — dump directory (default /var/lib/forecastiq/backups)
#   FIQ_BACKUP_STATUS_FILE  — status JSON path (default /var/lib/forecastiq/backup-status.json)
#   FIQ_RCLONE_REMOTE       — offsite remote (default b2:forecastiq-backups)
#   FIQ_DB_CONTAINER        — co-tenant topology: name of an EXTERNAL postgres
#                             container to dump through (e.g. app-postgres-1).
#                             When unset, uses the compose `db` service (ADR-033
#                             standalone default).
#   FIQ_RETENTION_DAYS      — local dump retention (default 30; co-tenant hosts
#                             with tight disks may shorten this — offsite keeps 90d)
#
# On failure: non-zero exit + status file written as "failed" → alert A10
# fires (forecastiq_backup_status != 1). -E ensures the ERR trap fires inside
# functions too. A verified dump is NEVER deleted by a later failure
# (DRB-WP24-003).
set -Eeuo pipefail

# ── Configuration ────────────────────────────────────────────────
COMPOSE_DIR="${FIQ_COMPOSE_DIR:-/opt/forecastiq}"
BACKUP_DIR="${FIQ_BACKUP_DIR:-/var/lib/forecastiq/backups}"
STATUS_FILE="${FIQ_BACKUP_STATUS_FILE:-/var/lib/forecastiq/backup-status.json}"
RCLONE_REMOTE="${FIQ_RCLONE_REMOTE:-b2:forecastiq-backups}"
DB_CONTAINER="${FIQ_DB_CONTAINER:-}"
RETENTION_DAYS="${FIQ_RETENTION_DAYS:-30}"
OFFSITE_RETENTION_DAYS=90
PG_IMAGE="postgres:16-alpine"

STAMP=$(date -u +%F)
DUMP_FILE="${BACKUP_DIR}/forecastiq-${STAMP}.dump"
DUMP_TMP="${DUMP_FILE}.tmp"
SCRATCH_NAME="fiq-backup-verify-$$"
DUMP_OK=0

compose() { docker compose --project-directory "$COMPOSE_DIR" "$@"; }

# DB access is topology-dependent: compose `db` service (standalone) or an
# external co-tenant container named by FIQ_DB_CONTAINER.
db_exec() {
  if [ -n "$DB_CONTAINER" ]; then
    docker exec -i "$DB_CONTAINER" "$@"
  else
    compose exec -T db "$@"
  fi
}

# ── Helpers ──────────────────────────────────────────────────────
write_status() {
  local status="$1"
  local size="${2:-0}"
  local now
  now=$(date -u +%Y-%m-%dT%H:%M:%SZ)

  # Read existing restore-test entry (preserve it)
  local restore_test="null"
  if [ -s "$STATUS_FILE" ]; then
    restore_test=$(jq -c '.last_restore_test // null' "$STATUS_FILE" 2>/dev/null || echo "null")
  fi

  # Stage the full document, then write in one pass. The status file is a
  # single-inode bind mount into the app container, so it must be truncated
  # in place (mv would orphan the container's inode) — staging minimizes the
  # partial-read window (DRB-WP24-011).
  local tmp
  tmp=$(mktemp)
  cat > "$tmp" <<EOF
{
  "last_backup": {
    "completed_at": "${now}",
    "status": "${status}",
    "size_bytes": ${size}
  },
  "last_restore_test": ${restore_test}
}
EOF
  cat "$tmp" > "$STATUS_FILE"
  rm -f "$tmp"
  chmod 0644 "$STATUS_FILE" 2>/dev/null || true
}

cleanup_scratch() {
  # -v: postgres:16-alpine declares an anonymous data volume; without -v every
  # scratch run leaks ~the DB size and eventually fills the shared disk.
  docker rm -fv "$SCRATCH_NAME" >/dev/null 2>&1 || true
}

on_error() {
  write_status "failed" || true
  # Never delete a dump that already passed its integrity check: a transient
  # late failure (e.g. Sunday rclone hiccup) must not erase the night's good
  # backup (DRB-WP24-003).
  if [ "$DUMP_OK" -ne 1 ]; then
    rm -f "$DUMP_TMP" "$DUMP_FILE" || true
  else
    rm -f "$DUMP_TMP" || true
  fi
  cleanup_scratch
}
trap on_error ERR

# ── Validate ─────────────────────────────────────────────────────
# Misconfiguration must fire A10, not exit silently (DRB-WP24-006):
# plain `exit 1` does not trigger the ERR trap.
if ! command -v docker >/dev/null 2>&1; then
  echo "ERROR: docker not found."
  write_status "failed"
  exit 1
fi
if [ -n "$DB_CONTAINER" ]; then
  if ! docker ps --filter "name=^${DB_CONTAINER}$" --filter status=running --quiet | grep -q .; then
    echo "ERROR: co-tenant db container '${DB_CONTAINER}' is not running."
    write_status "failed"
    exit 1
  fi
elif ! compose ps --status running db --quiet 2>/dev/null | grep -q .; then
  echo "ERROR: db service is not running under ${COMPOSE_DIR}."
  write_status "failed"
  exit 1
fi

mkdir -p "$BACKUP_DIR"

echo "=== ForecastIQ Backup — ${STAMP} ==="
echo "Target: ${DUMP_FILE}"
echo ""

# ── 1. Dump via the db container (atomic: .tmp, mv on success) ───
echo "[1/5] Running pg_dump (${DB_CONTAINER:-db service})..."
db_exec pg_dump -U forecastiq -Fc --no-owner --no-acl forecastiq > "$DUMP_TMP"
mv "$DUMP_TMP" "$DUMP_FILE"
DUMP_SIZE=$(stat -c%s "$DUMP_FILE" 2>/dev/null || stat -f%z "$DUMP_FILE")
echo "  Dump complete: ${DUMP_SIZE} bytes"

# ── 2. Integrity check (test-restore into a scratch container) ───
echo "[2/5] Integrity check (scratch ${PG_IMAGE} container)..."
cleanup_scratch
docker run -d --name "$SCRATCH_NAME" -e POSTGRES_PASSWORD=scratch "$PG_IMAGE" >/dev/null
for i in $(seq 1 30); do
  if docker exec "$SCRATCH_NAME" pg_isready -U postgres >/dev/null 2>&1; then break; fi
  if [ "$i" -eq 30 ]; then echo "  ERROR: scratch postgres not ready after 30s"; false; fi
  sleep 1
done
docker exec "$SCRATCH_NAME" createdb -U postgres verify
docker exec -i "$SCRATCH_NAME" pg_restore -U postgres -d verify --no-owner --no-acl < "$DUMP_FILE"

# Verify the dump restored with data intact. `providers` is seeded at
# bootstrap and is never empty in a real deployment, so it is a stable
# usability signal; collection_schedules/forecast_snapshots are populated
# later by the scheduler and may legitimately be 0 on a fresh system.
PROVIDER_COUNT=$(docker exec "$SCRATCH_NAME" psql -U postgres -d verify -t -A -c "SELECT count(*) FROM providers;")
SNAPSHOT_COUNT=$(docker exec "$SCRATCH_NAME" psql -U postgres -d verify -t -A -c "SELECT count(*) FROM forecast_snapshots;")
echo "  Restored: providers=${PROVIDER_COUNT}, forecast_snapshots=${SNAPSHOT_COUNT}"

if [ "$PROVIDER_COUNT" -eq 0 ]; then
  echo "  ERROR: providers is empty in the restored dump — backup unusable"
  false # triggers ERR trap → failed status + dump removed (not yet DUMP_OK)
fi

cleanup_scratch
DUMP_OK=1
echo "  Integrity check passed"

# ── 3. Write status file ─────────────────────────────────────────
echo "[3/5] Writing status file..."
write_status "success" "$DUMP_SIZE"
echo "  Status: ${STATUS_FILE}"

# ── 4. Prune old local backups ───────────────────────────────────
echo "[4/5] Pruning local backups older than ${RETENTION_DAYS} days..."
PRUNED=$(find "$BACKUP_DIR" -name "forecastiq-*.dump" -mtime +${RETENTION_DAYS} -delete -print | wc -l)
echo "  Pruned: ${PRUNED} files"

# ── 5. Offsite copy (Sundays only) ───────────────────────────────
# copy (NOT sync): sync would mirror the 30d-pruned local dir and destroy the
# 90d offsite retention (backup doc §1). Offsite prune is done separately at
# its own retention window.
DOW=$(date -u +%u) # 7 = Sunday
if [ "$DOW" -eq 7 ]; then
  echo "[5/5] Weekly offsite copy to B2..."
  REMOTE_NAME="${RCLONE_REMOTE%%:*}:"
  if ! command -v rclone >/dev/null 2>&1 || ! rclone listremotes 2>/dev/null | grep -q "^${REMOTE_NAME}$"; then
    echo "  ERROR: rclone remote '${REMOTE_NAME}' not configured — offsite is the only"
    echo "  durability surviving instance loss under ADR-033; this is a failure."
    false # ERR trap → failed status; local dump kept (DUMP_OK=1)
  fi
  rclone copy "$BACKUP_DIR" "$RCLONE_REMOTE" \
    --include "forecastiq-*.dump" \
    --transfers 2 \
    --checkers 4 \
    --log-level INFO
  # Prune offsite copies past the 90-day offsite retention window.
  rclone delete "$RCLONE_REMOTE" \
    --min-age "${OFFSITE_RETENTION_DAYS}d" \
    --include "forecastiq-*.dump" \
    --log-level INFO || true
  echo "  Offsite copy + ${OFFSITE_RETENTION_DAYS}d prune complete"
else
  echo "[5/5] Offsite copy: skipped (not Sunday; DOW=${DOW})"
fi

echo ""
echo "=== Backup Complete: ${STAMP} (${DUMP_SIZE} bytes) ==="
