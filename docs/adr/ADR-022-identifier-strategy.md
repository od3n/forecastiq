# ADR-022: Identifier Strategy — UUIDv7 Everywhere

**Status**: Accepted (Phase 1)
**Date**: 2026-07-22

## Context
The domain model mandates UUIDv7 PKs; Phase 1 records the tradeoff analysis and implementation rules (the prompt requires the decision documented).

## Options considered
1. **UUIDv7** (time-ordered 128-bit) for all entities.
2. ULID — equivalent ordering; non-native PG type (stored as uuid anyway); separate ecosystem tooling. No advantage.
3. BIGINT identity — compact and fast, but: publicly exposes exact cardinality and creation order (business + enumeration leak), requires DB round trip before insert (complicates idempotent batch writes and future multi-writer), painful across any future DB split.
4. UUIDv4 — random ordering → B-tree page splits and index bloat on the insert-heavy snapshot table (~50K/day); no benefit over v7.

## Decision
Option 1: UUIDv7 generated in Go (`github.com/google/uuid`), stored as `uuid`, serialized as canonical hyphenated strings.

## Rationale
- Time-ordering gives bigint-like index behavior on the hot insert path while keeping global uniqueness (client-side generation → batch inserts need no returned IDs for child rows).
- Public IDs reveal nothing actionable (128-bit; sub-millisecond timestamp component is not sensitive).
- Composite PKs on partitioned tables `(id, target_time)` keep the logical identity stable across partitioning.

## Consequences
- (+) Idempotent writes: snapshot IDs can be precomputed in the adapter before the tx.
- (+) Cursor pagination keys naturally unique and stable.
- (−) 16 bytes vs 8: ~2× index key size on FK columns — negligible at MVP storage (~19 GB steady state total).
- (−) All generators must use a v7 implementation (pinned dependency; test asserts version bits).

## Risks
Clock regression on VPS producing non-monotonic v7 within the same millisecond — uuid libraries handle with random tail; ordering guarantee needed is per-insert-batch only (satisfied).

## Migration trigger
None — identifier choice is permanent by nature; no trigger anticipated.

## Review date
Not scheduled (foundational, stable).
