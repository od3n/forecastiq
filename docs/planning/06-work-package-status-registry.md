# ForecastIQ — Work-Package Status Registry

**Version**: 1.0
**Status**: Living document — updated at each work-package completion
**Authority**: `docs/planning/05-implementation-work-packages.md` (definitions); delivery reports (evidence)

State model: Not Started → Prototype Exists → Partially Implemented → Implementation Complete → Review Findings Open → Accepted. Side states: Blocked, Deferred.

---

## Registry

| WP | Name | State | Last updated | Evidence / notes |
|----|------|-------|--------------|------------------|
| 01 | Repository + dev env | Accepted (bootstrap) | 2026-07-22 | Repository Bootstrap final report; `make dev-up`, CI green |
| 02 | DB foundation | Accepted (bootstrap) | 2026-07-22 | Migrations 20260801000001..05; integration suite |
| 03 | Identity + workspace | Not Started | 2026-07-22 | Audit recorder seam exists (used by WP-04); JWKS/API keys pending |
| 04 | Location management | Implementation Complete | 2026-07-22 | This execution. BR-LOC-01..03 proven by unit + integration tests; PUT/PATCH endpoints live; awaiting Delivery Review Board |
| 05 | Adapter framework | Prototype Exists | 2026-07-22 | First-slice collection pipeline + Open-Meteo adapter; hardening pending |
| 06 | First provider (Open-Meteo) | Prototype Exists | 2026-07-22 | Adapter + fixtures exist; full contract matrix pending |
| 07 | Second provider (OpenWeather) | Not Started | 2026-07-22 | |
| 08 | Scheduler + collection ops | Prototype Exists | 2026-07-22 | Scheduler loop, slots, runs, trigger endpoint exist; hardening pending |
| 09–27 | (remaining) | Not Started | 2026-07-22 | |

---

## Recorded discrepancies

Documentation-vs-documentation conflicts discovered during implementation, with resolutions (master prompt: "record the discrepancy and resolve it before materially extending the affected behaviour").

| # | Discrepancy | Resolution | Affected docs | Packages impacted |
|---|-------------|------------|---------------|-------------------|
| DR-01 | Location timezone mutability: domain architecture §2.3 lists timezone as **immutable** (Mutable: name, status, updated_at); `docs/api/00-api-requirements.md` §4.1 listed PUT as "(name, timezone, status)"; UI design spec edit form includes timezone; security matrix lists "name/timezone/status" | Domain architecture authoritative (Phase 1 aggregate design; timezone derives from immutable coordinates; mutation would re-bucket historical display data). Implementation: PUT accepts name only. API requirements doc corrected 2026-07-22. | `docs/api/00-api-requirements.md` (corrected); `docs/ui/02-ui-design-specification.md` §S-12 (edit form should render timezone read-only — flagged for WP-21); `docs/security/01-ui-authorization-matrix.md` S-12 row (flagged for WP-19 review) | WP-21 (UI form), WP-19 (matrix) |

---

## Deferred items recorded during WP-04

| Item | Rationale | Revisit |
|------|-----------|---------|
| Idempotency-Key infrastructure for POST /locations | No `idempotency_keys` table in the approved 18-table schema; cross-cutting concern spanning all mutable POSTs; WP-04 acceptance (BR-LOC-01..03) does not require it; snapshot/collection dedup makes re-execution harmless for the pipeline | WP-15 or a dedicated cross-cutting package |
| Optimistic concurrency on location update | Master prompt conditions it on "if approved"; no ADR or domain-architecture section approves it; single-operator MVP with DB unique constraints suffices | If multi-operator admin emerges |
