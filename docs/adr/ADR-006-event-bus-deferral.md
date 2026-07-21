# ADR-006: Event Bus Deferral — In-Process Seam, NATS Later

**Status**: Accepted (Phase 0 Amendment)
**Date**: 2026-07-22

## Context
Phase 0 routed all module communication through NATS JetStream with events defined by
name only. The ARB flagged event-driven-without-governance (Smell #3) and operational
overload. The amendment directed: no NATS at MVP, but do not remove the expansion path.

## Options considered
1. NATS JetStream at MVP (Phase 0 baseline) — broker to operate, schema governance
   needed immediately, delivery semantics to get right.
2. **In-process event seam**: same event names and payload shapes
   (`forecast.collected`, `observation.collected`, `accuracy.calculated`,
   `provider.health_changed`), delivered as synchronous in-process calls through a
   versioned Go interface; payloads carry `schema_version`.
3. DB outbox + poller — durable but adds polling machinery for no MVP consumer need.

## Decision
Option 2. Event contracts are documented and versioned now; the transport is the only
thing that changes later.

## Rationale
- MVP has exactly one producer and one consumer per event, in one process — a broker
  buys nothing but failure modes.
- Keeping names/payloads stable means introducing JetStream later is: publish at the
  existing seam, move consumers to JetStream consumers (ack `explicit`, streams
  `FORECASTS`/`OBSERVATIONS`, 7d retention) — a transport swap, not a redesign.
- Removes the entire class of event-schema-evolution/consumer-breakage risk (ARB R-13)
  until multiple consumers exist.

## Consequences
- (+) No broker ops; no network serialization failures; transactional consistency
  (event emission participates in the DB transaction via the seam interface).
- (+) Future migration is bounded and well-defined.
- (−) No replayable event history at MVP (lineage + replay of raw payloads covers the
  actual recovery need).
- (−) If a future consumer is added without promoting NATS, the seam must not be
  abused into a multi-consumer in-process bus (review gate at seam changes).

## Migration trigger
Introduce JetStream when: comparison lag > 15 min behind collection at sustained
volume, OR a second consumer of any event appears (webhooks, analytics), OR modules
are extracted to separate processes (ADR-001 trigger).

## Review date
At any seam-interface change; formally 2027-01-22.
