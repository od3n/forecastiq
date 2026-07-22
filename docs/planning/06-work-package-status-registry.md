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
| 04 | Location management | Ready for Re-Review | 2026-07-22 | DRB-WP04-001..005 remediated: advisory-lock dedup serialization, fp-tolerant boundary, mandatory override reason, restricted status lifecycle, doc corrections. Regression tests added. Awaiting DRB re-review. |
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

## WP-04 Delivery Review Board outcome (2026-07-22)

Decision: **CHANGES REQUIRED** (report: `docs/reviews/work-packages/WP-04-delivery-review.md`).

| Finding | Severity | Summary | Remediation target |
|---------|----------|---------|--------------------|
| DRB-WP04-001 | High | BR-LOC-01 dedup race: concurrent creates bypass proximity check (reproduced: 6 parallel POSTs → 2 rows); no concurrency test | Serialize create tx (advisory lock or SERIALIZABLE+retry) + concurrency integration test |
| DRB-WP04-002 | Medium | Exact-0.05° boundary fp-fragile (live pair rejected at 0.04999999999999716°) | Epsilon/precision-tolerant comparison + multi-coordinate boundary tests |
| DRB-WP04-003 | Medium | `override_reason` not mandatory when `allow_near_duplicate` set | 422 on empty reason + tests |
| DRB-WP04-004 | Medium | Reserved `archived` settable via PATCH; no transition validation | Restrict to active\|disabled (or ADR) + transition tests |
| DRB-WP04-005 | Medium | `docs/api/05-endpoint-catalog.md` still lists PUT timezone; README endpoint list stale | Doc corrections |
| DRB-WP04-006 | Low | Repository UPDATE writes immutable timezone column | Remove column from statement |
| DRB-WP04-007 | Low | Malformed `active` query param silently coerced | 422 on invalid boolean |

Tracked conditions: TC-01 Idempotency-Key (→ WP-15, this registry's deferral accepted); TC-02 optimistic concurrency (deferred); TC-03 dev-token seam replacement (WP-03/19) + DR-01 UI/matrix follow-ups (WP-21/19); TC-04 CI green on pushed branch + testcontainers run (review env had no Docker).

## Deferred items recorded during WP-04

| Item | Rationale | Revisit |
|------|-----------|---------|
| Idempotency-Key infrastructure for POST /locations | No `idempotency_keys` table in the approved 18-table schema; cross-cutting concern spanning all mutable POSTs; WP-04 acceptance (BR-LOC-01..03) does not require it; snapshot/collection dedup makes re-execution harmless for the pipeline | WP-15 or a dedicated cross-cutting package |
| Optimistic concurrency on location update | Master prompt conditions it on "if approved"; no ADR or domain-architecture section approves it; single-operator MVP with DB unique constraints suffices | If multi-operator admin emerges |
