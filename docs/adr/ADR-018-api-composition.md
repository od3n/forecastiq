# ADR-018: API Composition Strategy — Domain Endpoints + Two Screen Endpoints, No BFF

**Status**: Accepted (Phase 1) — ratifies the Reconciliation Board mandate
**Date**: 2026-07-22

## Context
The UI↔Backend board mandated a composition strategy (doc 01 preamble): reusable domain endpoints as backbone, `/forecast-comparison` + extended `/accuracy/summary` as purpose-built, no BFF. Phase 1 ratifies this as an architecture decision and fixes the guardrails.

## Options considered
1. BFF aggregation layer (dashboard-specific backend) — couples backend releases to layout; defeats per-endpoint ETag caching; extra layer for 1–2 engineers.
2. Endpoint-per-card — proliferation (30+ endpoints), cache granularity chaos.
3. One mega-endpoint per screen — over-fetching; single cache key; partial-failure granularity lost.
4. **Domain endpoints + exactly two purpose-built screen endpoints; dashboard composes ≤ 2 requests per screen.**

## Decision
Option 4 (board mandate ratified), per `docs/api/04-api-architecture.md` §1 and `docs/api/05-endpoint-catalog.md`.

## Rationale
- Public S-05 genuinely cannot use raw endpoints (user+ gated) — the bounded public `/forecast-comparison` is the minimal surface that resolves C-19.
- Provider-mode `/accuracy/summary` eliminates the only real N+1 (S-03 grid) — one indexed scan instead of per-location fan-out.
- ≤ 2 requests per screen keeps frontend orchestration trivial without a BFF.

## Consequences
- (+) 34 total endpoints; each cacheable/testable independently.
- (+) Screen changes rarely require backend changes (composition is frontend work).
- (−) Two endpoints carry screen-shaped payloads — governed as first-class contracts (versioned, tested), not ad hoc.

## Risks
Future screens requesting a third purpose-built endpoint — gate: board-style justification required (bounded payload + why domain endpoints fail), else extend a domain endpoint.

## Migration trigger
A BFF becomes justified only if ≥ 4 screens need > 3 composed requests each with client-side join logic — not anticipated at MVP scale.

## Review date
At each new screen addition (Level 3 IA changes).
