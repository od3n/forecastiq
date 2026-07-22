# ADR-019: Object Storage Use — Volume Only at MVP, Scheme-Prefixed Keys

**Status**: Accepted (Phase 1) — extends ADR-011
**Date**: 2026-07-22

## Context
ADR-011 decided 90-day gzip payloads on a block volume. Phase 1 fixes the implementation: key scheme, access model, and the exact promotion interface.

## Options considered
1. S3-compatible storage at MVP — rejected by constraints §3 (deferred technology; extra credential/dependency).
2. **Filesystem PayloadStore behind an interface; keys scheme-prefixed (`payloads/{provider}/{yyyy}/{mm}/{dd}/{id}.json.gz`); `s3://` prefix support reserved; no file-serving HTTP route.**
3. Payloads in PostgreSQL (BYTEA/JSONB) — rejected (ADR-011: DB bloat, backup size).

## Decision
Option 2, per `docs/architecture/05-data-flow-architecture.md` §2 and workflows 01/06.

## Rationale
- The interface (Store/Load/Exists/Delete by key) makes the S3 promotion a second implementation, not a refactor (constraints §4 trigger: > 50 GB or > 90 d retention need).
- Scheme prefix in `raw_payload_object_key` means historical keys remain valid across the promotion (mixed `payloads/…` and `s3://…` coexist).
- Absence of any file-serving route is a security property, not just simplicity (threat model §5).

## Consequences
- (+) Zero new dependencies; retention is path-based deletion (no object lifecycle config).
- (+) Replay reads through the same interface (checksum verify before parse).
- (−) Volume loss = payload loss (accepted: normalized data + checksums survive; DR table).
- (−) No cross-instance payload access until S3 (irrelevant at one instance; worker-split promotion would pair with S3 if replay is needed on the worker).

## Risks
Volume growth (R-20): ~7 MB/day → 630 MB/90 d — alert at 80% of 50 GB is far above trajectory.

## Migration trigger
Per ADR-011/constraints §4: volume > 50 GB, retention > 90 d required, or backup size dominated by payloads.

## Review date
Quarterly with volume metrics.
