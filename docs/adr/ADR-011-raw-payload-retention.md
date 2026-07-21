# ADR-011: Raw Payload Retention — 90 Days on Volume, Checksums Forever

**Status**: Accepted (Phase 0 Amendment)
**Date**: 2026-07-22

## Context
Phase 0 stored raw API responses in S3 indefinitely ("for audit") without a retention
model, while the ARB asked whether payloads should even be queryable (Q8) and flagged
storage-cost risk. The amendment required explicit raw-payload retention and lineage
decisions.

## Options considered
1. S3 indefinite retention (Phase 0 implicit) — cost and lifecycle undefined; S3 is a
   deferred technology.
2. **Gzip-compressed payloads on the app's block volume, 90-day retention, SHA-256
   checksums retained permanently on the collection row; normalized snapshots retained
   2 years as the queryable record.**
3. No raw payload storage at all — cheapest, but kills replay/debugging/audit value.
4. Store raw payloads in PostgreSQL (JSONB/BYTEA) — bloats the DB and backups.

## Decision
Option 2. Payload key scheme `payloads/{provider}/{yyyy}/{mm}/{dd}/{collection_id}.json.gz`
with scheme-prefix support so `s3://…` keys work unchanged after promotion.

## Rationale
- Payloads are debugging/replay inputs, not the system of record: normalized snapshots
  (2y) + metrics (indefinite) + checksums (forever) preserve every claim users see.
- 90 days covers: adapter-bug discovery windows, a full season of schema-drift
  investigation, and replay of recent data — at ~tens of MB/day the volume stays small.
- Keeps MVP off S3 (one fewer dependency/credential) while the key design makes the
  promotion mechanical.

## Consequences
- (+) Bounded, predictable storage; simple retention job (delete by path date).
- (+) Replay (FC-14) works for the recent window — the window that matters for fixes.
- (−) Payloads older than 90 d are unrecoverable — accepted: checksums still prove
  integrity of what was processed, and normalized rows remain.
- (−) Volume loss = payload loss (mitigated: normalized data unaffected; DR table).

## Migration trigger
Move to S3-compatible storage when: payload volume > 50 GB, OR retention must exceed
90 days (compliance/customer need), OR backup size is dominated by payloads.

## Review date
Quarterly with volume metrics; formally 2027-01-22.
