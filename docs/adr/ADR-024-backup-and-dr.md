# ADR-024: Backup and Disaster Recovery — PITR Primary, Nightly Dumps, No Payload Backup

**Status**: Accepted (Phase 1)
**Date**: 2026-07-22

## Context
NFR-A04/A05 set RPO < 1 h, RTO < 4 h. Phase 1 fixes the backup stack and the explicit decision not to back up the payload volume.

## Options considered
1. **Managed PITR (continuous WAL) as primary + nightly pg_dump to volume + weekly offsite (B2) + monthly automated restore test; payload volume deliberately not backed up.**
2. Self-managed PostgreSQL with WAL-G to S3 — full control but operates the largest failure domain ourselves; rejected (managed DB is the amendment's foundation).
3. Back up payloads too — doubles backup volume for data that is 90-day-ephemeral by design and regenerable from providers in the recent window; rejected (ADR-011 logic).

## Decision
Option 1, per `docs/operations/04-backup-and-restore.md`.

## Rationale
- PITR gives RPO < 1 h for the system of record (everything that matters permanently).
- Nightly logical dumps are the vendor-independent second copy (portable restore path; also feeds the restore test).
- Payloads: normalized snapshots + checksums preserve every claim users see; payload loss degrades only replay/debugging for the recent window (documented acceptance).
- Monthly restore testing converts "we have backups" into "we have verified recovery" (NFR-D06) — visible in admin health so it cannot silently lapse.

## Consequences
- (+) RTO < 4 h is a rehearsed runbook, not a hope (VPS rebuild drilled).
- (+) Restore test result is an observable (alert A11 if overdue).
- (−) Up-to-24 h logical gap between PITR and dump is covered by PITR anyway (dump is the secondary path).
- (−) Payload volume loss = 90 d of replay capability lost (accepted; checksums remain).

## Risks
Vendor PITR retention tier limits (7–30 d) — beyond that, dumps (90 d offsite) are the floor; adequate for MVP.

## Migration trigger
Customer SLAs requiring RPO < 15 min or multi-region DR (Level 3) → synchronous replica + cross-region WAL shipping.

## Review date
Quarterly with restore-test results; at any vendor tier change.
