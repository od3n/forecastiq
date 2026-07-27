# ForecastIQ — WP-24 Backup and Recovery: Delivery Review Board

**Review date**: 2026-07-27
**Work package**: WP-24 — Backup and Recovery (PR #30, `feature/wp24-backup-recovery`)
**Reviewed SHA**: `92cd8be` (post ADR-033 merge-up)
**Decision**: **REJECTED — 3 Critical + 4 High findings; scripts incompatible with the ADR-033 topology**

---

## 1. Context

ADR-033 (accepted with WP-23) moved production to EC2 + Docker Compose with a
**containerized postgres:16-alpine** and removed vendor PITR entirely — making
WP-24 backups **the only durability net**. The branch merged `main` (bringing
ADR-033 in) but did not adapt scripts, cron, bootstrap, or the backup runbook
to the containerized environment. Shell quality is good; topology fit is not.

## 2. Findings

### Critical

**DRB-WP24-001 (C)** — Scripts cannot reach the database. Host-side
`pg_dump`/`psql`/`pg_restore` against `$DATABASE_URL`: the `db` service
publishes **no host port**, bootstrap installs **no postgresql-client**, and
Ubuntu 22.04's client (v14) refuses to dump a v16 server anyway. Rewrite DB
access through the container (`docker compose exec -T db pg_dump ...` /
scratch `postgres:16-alpine` containers). `deploy/scripts/backup.sh`,
`deploy/scripts/restore-test.sh`

**DRB-WP24-002 (C)** — No delivery or activation path: cron placeholders stay
commented, and the referenced path `/opt/forecastiq/deploy/scripts/backup.sh`
never exists on the host (image-based deploy ships only compose + smoke-test).
Backups never run; A10 metrics are absent-by-design until first success →
total silent durability gap. Populate cron in bootstrap + ship the scripts in
`deploy.sh`.

**DRB-WP24-003 (C)** — The ERR trap deletes a **verified** dump when a late
step fails (e.g. Sunday `rclone copy` hiccup): `rm -f "$DUMP_TMP" "$DUMP_FILE"`
runs unconditionally. Guard removal behind a `DUMP_OK` flag set after the
integrity check.

### High

**DRB-WP24-004 (H)** — A failed restore test **suppresses** A11 instead of
firing it: the collector exports `restore_test_last_timestamp_seconds` from
`completed_at` regardless of status, so writing `failed` + fresh timestamp
silences the staleness alert for 35 days. Add a restore-status metric + rule,
or keep the prior timestamp on failure.

**DRB-WP24-005 (H)** — Permission model: cron runs as `deploy`, but
`/var/lib/forecastiq/backups` is root-owned and the status file is owned by
uid 65532 → both write paths EACCES; combined with 003 the trap then deletes
the good dump. Reconcile ownership in bootstrap.

**DRB-WP24-006 (H)** — `DATABASE_URL` never exists in the cron environment
(cron.d doesn't source env files; secrets.env has no DB URL — compose
synthesizes it) and the misconfig path `exit 1`s **without** writing a failed
status (plain exit doesn't fire the ERR trap) → silent forever.

**DRB-WP24-007 (H)** — Offsite sync silently optional: missing rclone/remote
prints SKIP and reports **success**; bootstrap never installs rclone. Offsite
is the only durability that survives instance/EBS loss under ADR-033 —
a missing remote must be a failure (or distinct degraded status), and
provisioning belongs in bootstrap.

### Medium

**DRB-WP24-008 (M)** — Restore test compares a 1–6-day-stale offsite dump
against live row counts at ±2%: append-only tables early in system life grow
faster than that → guaranteed false failures (which 004 then swallows).
Make the check staleness-aware or drop the live comparison.

**DRB-WP24-009 (M)** — "PITR validated with vendor" is dead under ADR-033 and
the branch is silent: `docs/operations/04-backup-and-restore.md` still
promises vendor PITR / RPO < 1 h. Amend the doc (real RPO: 24 h nightly, up
to 7 d for instance loss) and record the scope change.

**DRB-WP24-010 (M)** — Nightly integrity check and monthly restore test both
restore full production copies **into the production cluster** (same
container, same EBS volume): 2× disk nightly, restore can exhaust the volume
and take prod down. Use a throwaway scratch container.

### Low

**DRB-WP24-011 (L)** — Status-file write truncates in place; the container can
read partial JSON mid-write (all three metrics vanish for that scrape). Stage
to a temp file, then `cat tmp > status` (single-inode bind mount forbids mv).
**DRB-WP24-012 (L)** — `admin_url` sed breaks on `/` in passwords (latent).

## 3. Verified correct

Status-file schema matches `adapters/backupstatus` exactly; host path matches
the compose bind mount; A10 fires correctly on `status != success`; retention
design (copy-not-sync + separate 90 d offsite prune, 30 d local prune) is
right; `set -Eeuo pipefail`, no secret leakage; checked tables all exist.

## 4. Decision

**REJECTED.** 001–007 must be fixed (containerized DB access, delivery/cron
activation, trap/permission/env repair, offsite mandatory, A11 semantics);
008–010 fixed or explicitly tracked; the backup runbook amended per ADR-033.
Re-review requires green CI **plus a live rehearsal**: run backup.sh and
restore-test.sh end-to-end against a compose stack (the WP-23 rehearsal
pattern), showing dump → verify → status file → collector metrics.
