# ForecastIQ — WP-24 Backup and Recovery: DRB Confirmatory Re-Review

**Review date**: 2026-07-27
**Work package**: WP-24 — Backup and Recovery (PR #30, `feature/wp24-backup-recovery`)
**Prior review**: WP-24-delivery-review.md — REJECTED on `92cd8be` (DRB-WP24-001…012)
**Reviewed SHA**: `233d20c`
**Decision**: **ACCEPTED**

---

## 1. Verification of evidence

| Check | Result |
|-------|--------|
| Commit identity: local branch == remote == CI head | ✅ all `233d20c` |
| Six PR jobs green (incl. promtool: 18 rules with A11b) | ✅ first run |
| `bash -n` on backup/restore/bootstrap/deploy | ✅ |
| promexport unit tests (success + failed-restore status) | ✅ |
| **Live rehearsal against a compose stack** | ✅ see §3 |

## 2. Finding closure (DRB-WP24-001…012)

| Finding | Status | Resolution |
|---------|--------|-----------|
| 001 (C) can't reach containerized DB | ✅ | `docker compose exec -T db pg_dump/psql`; scratch `postgres:16-alpine` for restore — no host port, no client-version wall |
| 002 (C) no delivery/activation | ✅ | `deploy.sh` scps backup + restore scripts to `/opt/forecastiq`; bootstrap writes an **active** `/etc/cron.d/forecastiq` (deploy user) |
| 003 (C) trap deletes verified dump | ✅ | `DUMP_OK` flag — a post-verify failure keeps the good dump |
| 004 (H) failed restore suppresses A11 | ✅ | New `forecastiq_restore_test_status` metric + **A11b RestoreTestFailed** rule; unit-tested |
| 005 (H) deploy-user write EACCES | ✅ | bootstrap chowns `/var/lib/forecastiq/backups` + status file to `deploy` |
| 006 (H) DB URL absent in cron → silent | ✅ | No DB URL needed (container access); misconfig now writes `failed` status before exit |
| 007 (H) offsite silently optional | ✅ | Missing rclone remote on Sunday is a failure; bootstrap installs rclone |
| 008 (M) ±2% vs stale dump false-fails | ✅ | Staleness-aware: restored may lag prod; only SHORT-beyond-tolerance or EXCESS fails |
| 009 (M) PITR doc fiction | ✅ | Runbook amended (no PITR; RPO 24 h / 7 d instance loss; A11b) |
| 010 (M) restore into prod cluster | ✅ | Throwaway scratch container, separate from prod |
| 011 (L) non-atomic status write | ✅ | Staged temp then single `cat >` (single-inode bind mount preserved) |
| 012 (L) admin_url sed fragility | ✅ | Eliminated — no URL surgery (container access) |

## 3. Live rehearsal (the evidence the first review demanded)

Against a local compose stack (app image + `postgres:16-alpine`, seeded):

- **backup.sh**: `pg_dump` via `db` container → 113 KB dump → integrity restore
  into scratch container → `providers=2` verified → **status=success** written.
- **Failure path also exercised**: an over-strict `collection_schedules>0`
  assertion (that table is populated by the scheduler, not seed) correctly
  drove status=failed and removed the dump — then corrected to assert on
  `providers` (always seeded).
- **restore-test.sh**: restored latest dump into scratch → 8 tables verified
  within tolerance → **status=success**; status file retained the backup entry.
- **End-to-end metric flow**: brought up the app container against the same
  status file → `/metrics` exposed `forecastiq_backup_status 1`,
  `forecastiq_backup_last_success_timestamp_seconds`,
  `forecastiq_restore_test_status 1`,
  `forecastiq_restore_test_last_timestamp_seconds`.

## 4. Scope coverage

**Delivered**: backup.sh + restore-test.sh (containerized) · active cron ·
rclone B2 offsite (mandatory) · status file → /admin/health + A10/A11/A11b ·
restore-test green (rehearsed) · A10/A11b firing paths verified · runbook
amended per ADR-033.
**Descoped**: vendor PITR (impossible under ADR-033, documented).

## 5. Decision

**ACCEPTED.** All 12 findings closed, six jobs green on `233d20c`, and both
scripts executed end-to-end against a containerized stack with the full
status-file→collector→/metrics chain verified. PR #30 ready to merge.
**WP-25 (Security Hardening) becomes eligible.**
