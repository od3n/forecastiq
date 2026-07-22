# ADR-028: Caching and Stale-Serving Strategy

**Status**: Accepted (Phase 1) — see ADR-020 for the Redis-deferral decision; this ADR covers the stale-serving contract
**Date**: 2026-07-22

## Context
NFR-A07 requires graceful degradation with explicit staleness. Phase 1 must define when serving stale data is permitted and how it is labeled.

## Options considered
1. Fail hard on DB errors (503 always) — honest but unnecessarily unavailable; violates NFR-A07's intent.
2. Silent stale serving — violates BR-FRESH-01 (never serve stale as current).
3. **Stale-serving from expired LRU entries ONLY during DB unavailability, with freshness.state forced to "stale" + warning entry; mutations always fail hard (503).**

## Decision
Option 3, per `docs/api/08-caching-and-partial-results.md` §3.

## Rationale
- Reads of derived data tolerate minutes of staleness by product design (rankings are batch products); availability during a 2-minute DB blip is pure gain.
- Forced staleness labeling keeps the honesty contract machine-checkable (UI renders the banner from the same block).
- Mutations have no stale mode — a write against stale state is corruption; 503 + Retry-After is the only correct behavior.

## Consequences
- (+) Availability SLO protected against sub-TTL DB incidents without a cache tier.
- (+) One code path (expired-entry serving) — no separate "degradation mode" state machine.
- (−) Stale bodies can be up to one TTL old plus the outage duration — bounded and labeled.

## Risks
Operator misreads stale-for-fresh during incidents — mitigated: staleness is in the payload, the UI banner, and incident dashboards (cache-hit metrics spike pattern).

## Migration trigger
Redis promotion (ADR-020) extends stale-serving across restarts — same contract, larger buffer.

## Review date
With ADR-020.
