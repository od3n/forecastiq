# ADR-020: Redis Deferral — In-Process LRU + ETag for MVP Caching

**Status**: Accepted (Phase 1) — ratifies constraints §3/§4
**Date**: 2026-07-22

## Context
Caching and rate limiting need an implementation. Constraints exclude Redis at MVP with a measured trigger. Phase 1 specifies what replaces it.

## Options considered
1. Redis at MVP — excluded (constraints §3); premature at < 20 effective DB qps.
2. **In-process LRU (256 entries, per-class TTL) + strong ETags/304 + stale-serving on DB blips; in-process token-bucket rate limiting.**
3. No cache (DB direct) — works at MVP volume but wastes DB on 60 s polling; ETag-only would still hit DB for body generation.
4. CDN-side caching of API responses — Cloudflare can cache public GETs, but per-parameter cache keys + partial results + freshness blocks make origin-side control preferable; CDN reserved for static dashboard.

## Decision
Option 2, per `docs/api/08-caching-and-partial-results.md`.

## Rationale
- Pre-computed rows + LRU absorb polling: effective DB load < 20 qps at 100 concurrent users — Redis buys nothing measurable.
- Stale-serving with explicit staleness (NFR-A07) is a reliability feature the LRU provides for free.
- The `Cache` port makes Redis a swap, not a redesign (scaling doc §2 seam).

## Consequences
- (+) Zero infrastructure; cache consistency trivial (TTL + single writer batch).
- (+) Rate limiting needs no shared state at one instance.
- (−) Cold cache after restart (seconds to repopulate; acceptable).
- (−) Per-instance rate limits undercount if a second instance appears — the promotion trigger itself.

## Risks
Memory: 256 × ≤ 80 KB ≈ 20 MB worst case — bounded and monitored.

## Migration trigger
Constraints §4: p95 > 150 ms dominated by repeated ranking/metric reads, OR second instance needing shared rate limits.

## Review date
At any instance-count change; quarterly latency review.
