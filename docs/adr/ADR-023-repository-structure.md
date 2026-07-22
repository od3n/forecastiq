# ADR-023: Repository Structure — Monorepo with Idiomatic Go Module Boundaries

**Status**: Accepted (Phase 1)
**Date**: 2026-07-22

## Context
Phase 1 must fix the repository shape: monorepo vs polyrepo, and a Go layout that enforces the ADR-001 module boundaries.

## Options considered
1. Polyrepo (backend / dashboard / infra separate) — atomic API+dashboard contract changes become cross-repo coordination; CI triplicated; rejected for a 1–2 engineer team.
2. **Monorepo: `cmd/` + `internal/` (modules) + `adapters/` + `migrations/` + `web/` + `deploy/` + `terraform/` + `docs/`; depguard-enforced rule that modules never import another module's persistence package.**
3. Monorepo with a shared `pkg/` library — public API surface implies SDK obligations MVP doesn't have; rejected (`internal/` only).

## Decision
Option 2, per `docs/delivery/01-repository-structure.md`.

## Rationale
- One PR can change schema + migration + use case + handler + dashboard client + tests — the atomicity the contract-first workflow needs.
- `internal/` visibility + depguard turns the ADR-001 boundary from convention into a CI-enforced compile-time property.
- Single commit history = single audit trail for a portfolio project.

## Consequences
- (+) OpenAPI client regeneration couples dashboard to backend contract automatically.
- (+) Docs live with code (drift visible in review).
- (−) Repo size grows (fixtures, dashboard) — irrelevant at this scale; git handles it.
- (−) CI must be path-aware (frontend jobs only when `web/` touched) to keep feedback < 10 min.

## Risks
Boundary erosion via "temporary" cross-module imports — depguard blocks at lint; no override mechanism without ADR note.

## Migration trigger
Extraction of a module to its own service (constraints §4 triggers) moves its packages to a repo-local `services/` or a new repo — interface-only coupling makes either cheap.

## Review date
At any module extraction evaluation.
