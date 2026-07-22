# ADR-027: Transaction Architecture — Short Bounded Transactions, No Outbox

**Status**: Accepted (Phase 1)
**Date**: 2026-07-22

## Context
The prompt requires explicit transaction boundaries per workflow and a verdict on the outbox pattern.

## Options considered
1. **Short bounded transactions per unit of work; batch writes chunked; in-process event emission inside the tx; no outbox, no saga, no 2PC.**
2. Transactional outbox for events — durable event delivery machinery; ADR-006/021 make it unnecessary (single process; events are advisory hints; batch schedule is the authority).
3. One giant transaction per batch — minutes-long txs → lock/visibility problems, WAL pressure; rejected.

## Decision
Option 1, per `docs/architecture/03-module-architecture.md` §3 and workflows 01–04.

## Boundary table (normative)

| Operation | Boundary | Isolation | Locking |
|-----------|----------|-----------|---------|
| Location creation | One tx (dedup check + insert + audit) | READ COMMITTED | None (dedup race → 409 via unique violation) |
| Slot claim | One short tx (SELECT FOR UPDATE SKIP LOCKED + UPDATE) | READ COMMITTED | Row-level pessimistic (seconds) |
| Forecast collection completion | One tx (collection row + snapshot batch ≤ ~400 + circuit upsert + event) | READ COMMITTED | None (uniqueness dedup) |
| Observation import | One tx per location-call (≤ 3 rows typical) | READ COMMITTED | None |
| Matching | Chunked txs (1,000 pairs) | READ COMMITTED | None (uniqueness) |
| Metric aggregation | Chunked txs (500 rows) + supersede link in same tx | READ COMMITTED | None (append + link) |
| Ranking publication | One tx per batch scope (≤ 700 rows) — atomic visibility | READ COMMITTED | None |
| Config updates | One tx (update + audit) | READ COMMITTED | Optimistic via updated_at where concurrent edits plausible |
| Manual retry/replay | Same as collection completion | — | — |

- **Isolation:** READ COMMITTED everywhere (default); no serializable needed (append-only pipeline + uniqueness constraints).
- **Optimistic locking:** `updated_at` check on mutable config rows where two admins could race (single operator makes this theoretical; cheap insurance).
- **Advisory locks:** not used (slots are the coordination mechanism).
- **Retry behaviour:** tx failure → rollback; job-level retry per backoff; idempotent re-execution by constraints.
- **Batch sizes:** snapshots ≤ ~400/tx (one collection); pairs 1,000/tx; metrics 500/tx; purges 10,000/tx with 100 ms throttle.
- **Statement timeouts:** 30 s API context; 5 min batch context; 30 min maintenance.

## Rationale
Single process + single DB + append-only pipeline = ACID covers every consistency need; an outbox would add failure modes for zero consumers.

## Consequences
- (+) No partial batch visibility for rankings (atomic publication) — the consistency property the partial-result contract depends on.
- (+) Crash anywhere → rollback + idempotent re-run (no compensation logic exists to get wrong).
- (−) Event consumers miss events on rollback — by design (events are hints; schedule re-covers).

## Risks
Long-running aggregation tx growth with cell count — chunking bounds it; PT-4 monitors.

## Migration trigger
Multi-process extraction (ADR-001 trigger) → then evaluate outbox/JetStream per ADR-006.

## Review date
At any module extraction evaluation.
