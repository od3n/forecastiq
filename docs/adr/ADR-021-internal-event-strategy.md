# ADR-021: Internal Event Strategy — Five Versioned In-Process Events

**Status**: Accepted (Phase 1) — implements ADR-006
**Date**: 2026-07-22

## Context
ADR-006 deferred NATS and mandated an in-process seam with stable names/payloads. Phase 1 fixes the exact event set, delivery semantics, and governance.

## Options considered
1. No events (direct calls between modules) — works but erodes the seam ADR-006 preserved; coupling grows silently.
2. **Five events (`forecast.collected`, `observation.collected`, `observation.corrected`, `accuracy.calculated`, `provider.health_changed`) via a versioned in-process bus; emission participates in the producing transaction; consumers are advisory (backlog gauges) not transactional triggers.**
3. DB outbox + poller — durability machinery for no MVP consumer need (ADR-006 rejected).
4. Database triggers as event sources — couples logic to DDL; untestable in unit scope; rejected.

## Decision
Option 2, per `docs/architecture/03-module-architecture.md` §5.

## Rationale
- Five events cover every cross-module signal the workflows need; each payload carries `schema_version` (frozen shape for the future JetStream swap).
- In-transaction emission gives exactly-once semantics trivially (no outbox needed — same process, same DB).
- Consumers treat events as hints (update gauges, mark scopes dirty); the batch schedule remains the execution authority — an event lost to a crash costs at most one cycle of latency, never correctness.

## Consequences
- (+) NATS promotion = publish at the same seam + move consumers (transport swap per ADR-006).
- (+) Event shapes documented and versioned now; changes require bump + test (compatibility rule).
- (−) No replayable history (lineage + payload replay cover the actual recovery need — ADR-006 accepted).

## Risks
Seam abuse into a multi-consumer in-process bus — ADR-006's review gate at seam changes applies; a second consumer for any event triggers the NATS evaluation instead.

## Migration trigger
Per ADR-006: engine lag > 15 min sustained, OR second consumer, OR module extraction.

## Review date
At any seam interface change; formally with ADR-006 (2027-01-22).
