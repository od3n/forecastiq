# ADR-012: Forecast Collection Lineage — Collection/Snapshot Decomposition

**Status**: Accepted (Phase 0 Amendment)
**Date**: 2026-07-22

## Context
Blocker 2: the Phase 0 `ForecastSnapshot` modeled one API call as one row while
weather APIs return multi-period arrays — the acquisition pattern and storage model
were unreconciled (ARB Critical #3). The amendment mandated introducing
`ForecastCollection`, defining snapshot semantics, dedup/idempotency, partial
collections, schema-change handling, adapter versioning, replay, and lineage.

## Options considered
1. Keep one-row-per-response with a JSONB payload column — simple, but destroys
   per-target-time querying, horizon computation, and metric matching.
2. **ForecastCollection (1 API call) → ForecastSnapshot (1 prediction per target
   time)**, snapshots immutable and uniquely keyed by
   `(provider_id, location_id, issued_at, target_time)`; collection carries raw
   payload key + checksum + status accounting; replay creates new collections.
3. Snapshot-per-response plus a separate "periods" child table — same shape as option
   2 with an extra indirection and no benefit.

## Decision
Option 2, fully specified in `docs/domain/01-domain-model.md` §4 and
`docs/domain/02-data-lineage.md`. "Snapshot" is binding terminology for one
prediction at one target time (glossary entry).

## Rationale
- Matches how providers actually respond (arrays), making adapters natural mappers
  instead of shape-fighters.
- Per-target-time rows make horizon computation, exact-hour matching, and dedup
  trivial and indexable.
- The collection parent preserves what Phase 0 lost: "one API call produced N rows"
  accounting, partial-failure visibility, raw-payload lineage, and replay capability.
- Idempotent by construction: uniqueness constraint + `ON CONFLICT DO NOTHING` means
  scheduler double-fires and replays are safe by design, not by convention.

## Consequences
- (+) Full lineage chain raw→collection→snapshot→match→metric→ranking.
- (+) Schema drift is detected per row and accounted per collection (FC-11).
- (+) Historical data is never rewritten; adapter fixes recover via replay.
- (−) Row volume = periods per response × collections (as intended; partitioning
  handles it — ADR-004).
- (−) Every consumer must understand the two-level model (API exposes both; docs and
  glossary enforce terminology).

## Migration trigger
Structural change only if a provider introduces a non-array or push-based model
(webhook delivery) — the collection entity absorbs that too (one delivery = one
collection); no trigger currently anticipated.

## Review date
When adding provider 3 (adapter pattern stress check).
