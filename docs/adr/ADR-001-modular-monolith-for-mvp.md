# ADR-001: Modular Monolith for MVP

**Status**: Accepted (Phase 0 Amendment)
**Deciders**: Amendment Team
**Date**: 2026-07-22

## Context
Phase 0 specified 6+ communicating services (collectors, comparison, API, dashboard,
admin, alerts) over NATS. The ARB flagged resume-driven architecture and distributed-
monolith risk for a 1–2 engineer team (Smells #1–2, Risk R-12).

## Options considered
1. Microservices over NATS (Phase 0 baseline) — full operational surface from day one.
2. **Modular monolith** — one Go binary, package-level module boundaries, in-process
   calls, single DB.
3. Monolith with background workers as separate processes — partial split.

## Decision
Option 2: modular monolith with explicit module boundaries (identity, catalog,
collection, analysis, api, scheduler) owning their tables; cross-module access only via
module interfaces.

## Rationale
- One deployable, one DB: deploys, debugging, and transactions stay trivial.
- Module boundaries preserve the Phase 0 bounded contexts, so extraction later is a
  seam change, not a redesign.
- Matches the approved hosting model (~$50/mo) and bus-factor mitigation.

## Consequences
- (+) Orders-of-magnitude less ops; faster iteration; simple local dev (Docker Compose).
- (+) ACID across collection→snapshot→event in one transaction.
- (−) No independent scaling of modules until extraction.
- (−) Discipline required to keep module boundaries clean (lint rule: no cross-module
  repository imports).

## Migration trigger
Extract a module when: it needs independent scaling (analysis batch > 10 min starving
API), or a second consumer of its events appears, or team ≥ 4 with ownership split.
See promotion criteria (Redis/NATS/workers/K8s) in the architecture constraints doc.

## Review date
2027-01-22 (or at Level 3 gate, whichever first).
