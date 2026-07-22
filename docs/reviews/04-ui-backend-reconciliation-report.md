# ForecastIQ — UI ↔ Backend Reconciliation Report

**Board**: Principal Product Designer · Principal UX Architect · Principal Software Architect · Principal Backend Engineer · Principal Frontend Engineer · Principal Data Engineer · Principal SRE · Database Architect · API Architect · Security Architect · Accessibility Lead · QA Lead · Product Manager · Business Analyst
**Version**: 1.0
**Date**: 2026-07-22
**Inputs**: All Phase 0 Amendment authoritative documents; ADR-001..012; UI design outputs (`docs/ui/00..03`)
**Outputs**: 16 reconciliation documents (listed in §16)

---

## 1. Executive Summary

**The approved UI is technically supportable — conditional on the amendments in this report.**

The Phase 0 Amendment baseline (`docs/ui/00-screen-inventory.md`, `01-ui-data-requirements.md`) and the production UI specification (`docs/ui/02-ui-design-specification.md`) reconcile cleanly with the domain model, methodology, API requirements, and architecture constraints. Every screen, state, and metric in that baseline maps to existing or additively-amended backend capabilities within the approved modular-monolith architecture.

The operational dashboard exploration (`docs/ui/03-operational-dashboard-design.md`) required substantive reconciliation. It introduced a competing product framing (current-conditions dashboard vs. rankings-first comparison platform), four undefined metrics (consensus, confidence, disagreement, change impact), and navigation for capabilities that do not exist in MVP (Alerts, Reports, Live Weather). The Board resolved these by reaffirming the Phase 0 product truth — *transparent, evidence-based comparison of forecasts against observed reality* — absorbing the small set of exploration elements that map to existing data (observation context line, provider grid styling), and deferring the rest with zero architectural cost.

Most important changes:
1. **Three new endpoints** (all justified by board mandate): public `GET /forecast-comparison` (S-05 is public but raw data endpoints are user+ — C-19), `POST /admin/collections/trigger` (the health screen's "retry" targeted the wrong operation — C-10), and four admin user-management endpoints (S-14 had no API surface — C-09).
2. **Five additive amendments** to existing endpoints: `observation_context` in rankings, `collection_window` + provider-mode in `/accuracy/summary`, `next_scheduled_at` + system section in `/admin/health`, `default_location_id`/`preferences` on `/me`, public `adapter_version` on `/providers/{id}`.
3. **Three small schema additions**: `users` preference fields, `export_jobs` (GDPR-scoped), `provider_circuits` (persistent circuit state — required for health UI and restart safety).
4. **Removal of all non-functional MVP navigation** (Alerts, Reports, notification bell) per the board mandate.
5. **Binding contracts** for states, errors, partial results, freshness, provenance, CSV exports, authorization, testing, and operational signals — eliminating every ambiguity the design phase surfaced.

No distributed systems, databases, queues, or services were introduced. The architecture remains appropriate for 1–2 engineers.

## 2. Reconciliation Status

| Category | Count |
|----------|-------|
| Total UI capabilities reviewed (screens + distinct elements/states/behaviors) | 96 |
| Fully supported (existing backend, no change) | 68 |
| Supported after changes (additive amendments per §5–6) | 14 |
| Simplified for MVP (reduced functionality, approved) | 4 |
| Deferred to Commercial Beta (documented, no MVP surface) | 17 |
| Removed (product-truth or mandate violations) | 7 |
| Still blocked | **0** |

Simplified: S-01 observation context line (vs. conditions panel), DR-09 without feels-like, version indicator (optional), mobile layouts (responsive-only, per scope).

## 3. Critical Conflicts

All 20 conflicts resolved (`docs/reviews/03-ui-backend-conflicts.md`). Critical and high severity:

| ID | Conflict | Resolution |
|----|----------|------------|
| C-01 (Critical) | Two competing Overview designs — operational dashboard vs. rankings-first | Rankings-first retained (product truth, NP-01); observation context line absorbed; conditions/consensus/changes deferred |
| C-09 (Critical) | S-14 Admin Users had no API endpoints despite ADMIN-05/AUTH-09 | Four endpoints added with lockout guards + Supabase propagation |
| C-19 (Critical) | Public S-05 depends on user+-gated `/forecasts`+`/observations` | New bounded public `GET /forecast-comparison`; raw endpoints stay gated |
| C-02 (High) | Forecast changes feed — no backend capability, undefined impact taxonomy | Deferred L3; snapshot schema already supports the query |
| C-03 (High) | Consensus + confidence rating — no documented formulas (PC-02 violation risk) | Deferred L3 behind methodology extension |
| C-04 (High) | Live Weather screen vs. NP-01 "we don't deliver weather" | Removed; provenance-labeled context lines only |
| C-05 (High) | Alerts/Reports nav items for non-existent capabilities | Removed from MVP navigation (board mandate) |
| C-08 (High) | S-03 mapped to a "public subset" of an admin-only endpoint | `collection_window` added to `/accuracy/summary`; admin endpoint stays admin-only |
| C-10 (High) | Health "retry" conflated replay (needs payload) with re-collection (failed collections have none) | `POST /admin/collections/trigger` with circuit/rate-budget guards |

## 4. Approved MVP Screens

15 screens, all Required (`docs/ui/05-screen-specifications.md`):

| Screen | Route | Auth | Key amendment |
|--------|-------|------|---------------|
| S-01 Overview | `/` | public | + observation context line |
| S-02 Location Detail | `/locations/:id` | public | + collection_window |
| S-03 Provider Detail | `/providers/:id` | public | provider-mode summary (N+1 guard) |
| S-04 Trends | `/trends` | public | — |
| S-05 Forecast vs. Actual | `/forecast-vs-actual` | public | new bounded endpoint |
| S-06 Methodology | `/methodology` | public | — |
| S-07 Onboarding | overlay | signed-in | localStorage dismissal |
| S-08 Auth | `/auth/*` | public | custom forms + Supabase SDK |
| S-09 Settings | `/settings` | signed-in | + default_location_id/preferences |
| S-10 Admin Health | `/admin/health` | admin | + system section, trigger action, next_scheduled_at |
| S-11 Admin Providers | `/admin/providers` | admin | — |
| S-12 Admin Locations | `/admin/locations` | admin | — |
| S-13 Admin Schedules & Runs | `/admin/schedules` | admin | + trigger action |
| S-14 Admin Users & Audit | `/admin/users` | admin | + four user endpoints |
| S-15 Error pages | — | all | request_id display on 500 |

No post-MVP capability appears as functional MVP navigation (quality gate verified).

## 5. Approved API Capabilities

Composition: **reusable domain endpoints; no BFF; no endpoint-per-component** (max 2 requests per screen load). Full contracts in `docs/api/01-screen-api-contracts.md`.

**New (3)**: `GET /forecast-comparison` (public, bounded, attributed), `POST /admin/collections/trigger`, `GET/PATCH/DELETE /admin/users` + `POST /admin/users/{id}/export`.

**Amended (5, all additive within v1)**: `/rankings` + `observation_context`; `/accuracy/summary` + `collection_window` + `provider_id` mode; `/admin/health` + `system` section + `next_scheduled_at`; `/me` + preferences; `/providers/{id}` + `adapter_version`/`collecting_since`.

**Unchanged**: `/locations` CRUD, `/accuracy`, `/rankings/methodology`, `/api-keys`, `/me/export`, `DELETE /me`, `/forecast-collections` (admin), replay, recompute, `/admin/audit-events`, healthz/readyz.

Response conventions (`docs/api/02-response-conventions.md`): unified envelope (data, metadata, freshness, provenance, attribution, warnings, pagination); server-computed freshness with thresholds; partial = HTTP 200 + `warnings[]` (206/207 rejected); binding CSV export format (DR-05); cache-header classes; methodology §5 rounding.

Error contracts (`docs/api/03-error-and-partial-result-contracts.md`): 11-class RFC 7807 taxonomy with `retryable` + `request_id`; anti-enumeration and no-provider-body-forwarding rules; partial-result rules including ranking stability during provider outages.

## 6. Domain and Data-Model Changes

**Additions (3, all small)**: `users.default_location_id` + `users.preferences` (S-07/S-09); `export_jobs` (GDPR-scoped, partial-unique active-job guard); `provider_circuits` (persistent circuit state — required for S-10 display and restart safety).

**Removals**: none. **Relationship corrections**: none required (Phase 0 Amendment ERD verified against all screens). **Index requirements**: one additive index to verify/add (`provider_rankings (location_id, horizon_minutes, period_end DESC)`); all other access paths covered by existing indexes (`docs/data/01-query-and-index-requirements.md`).

**Explicitly not added**: ObservationCollection, AggregatedMetric, MethodologyVersion table, ExportJob-as-report-engine, WorkspaceMembership, Alert entities — each evaluated and either represented by existing entities, derived, or deferred (§ domain reconciliation).

## 7. Metric and Ranking Contract

**Every displayed metric is traceable to a defined formula and dataset** (`docs/domain/05-metric-ui-contract.md`): 24 metric rows covering all methodology §4 formulas, composite (§6–7), coverage/reliability, day-scope metrics, and the error band — each with definition, formula, inputs, aggregation, horizon, period, minimum samples, confidence representation, methodology version (2026.1), API field, database source, and test requirement.

Computation strategy: **stored immutable evaluation results** (batch every 30 min) + on-demand day metrics over ≤ 48 pairs. Not on-demand aggregation, not continuous aggregates, not TimescaleDB.

Undefined metrics (consensus, confidence rating, disagreement index, observation-derived feels-like) are **excluded from all MVP surfaces** — publishing them would violate PC-02. Each has a documented introduction gate.

## 8. State-Handling Contract

`docs/ui/06-ui-state-contracts.md`: 19 states with exactly one backend trigger and one UI treatment each; binding copy for user messages; state priority ordering (offline > permission > error > stale > partial > empty > loading) with composition rules; freshness contract (server-computed, four states, BR-FRESH thresholds); partial contract (200 + warnings, unaffected providers always render); cached-data labeling (PC-10); loading (skeleton debounce, no layout shift); offline recovery; mutation retry semantics (optimistic only for `PATCH /me`). All 11 board-mandated states covered, plus timeout, validation, conflict, offline.

## 9. Permission and Security Contract

`docs/security/01-ui-authorization-matrix.md`: public read S-01..S-06 (AUTH-08); user self-service S-07/S-09 with object-level checks; admin S-10..S-14 with server-side role enforcement on every action; credential invisibility at serializer level (BR-08); raw payload keys admin-visible but never served as files; self-lockout guards; anti-enumeration on auth flows; every UI mutation audit-logged (12 action classes). Frontend hiding is UX only — no action relies on it. Workspace isolation join present for additive Level 3 RLS.

## 10. Performance and Caching Decisions

Latency targets: API p50 < 50ms / p95 < 200ms / p99 < 500ms (NFR-P01..P03); DB query p95 < 100ms; meaningful paint < 2s; CLS < 0.1. Query strategy: pre-computed metric/ranking rows for all aggregate reads; indexed scans ≤ 365 rows for trends; two indexed queries + in-memory day metrics for FvA; bounded health assembly (< 200ms under 60s operator polling). Caching: in-process LRU + ETag (no Redis); response max sizes bounded per endpoint (16–80 KB); cursor pagination without total_count; exports client-side over bounded view data. N+1 risks eliminated by design (provider-mode summary). Scale path unchanged: promotion criteria (architecture §4) triggered by measurement only.

## 11. Accessibility Readiness

**The design can meet WCAG 2.1 AA** (`docs/ui/07-accessibility-requirements.md`). Ten backend-dependent accessibility requirements are binding on the API (structured series for chart tables, machine-readable statuses, freshness/CI/sample fields, methodology anchors, IANA timezones, enumerable filters, warning/error text for live regions). Six amendments adopted (emoji as decoration only, dot+text freshness, keyboard chart navigation, multi-channel provisional markers, zoom reflow, polite refresh announcements). Automated axe-core in CI + manual keyboard/screen-reader passes on chart screens before launch.

## 12. Testing Readiness

`docs/testing/01-requirement-test-traceability.md`: every critical business rule (BR-01..BR-INV-03, AUTH, FC, OC, CE, API, DB, ADMIN) and every mandated UI state maps to at least one of 14 test categories. Methodology: all 5 test vectors + all 11 property invariants (100% formula coverage, NFR-M01). Adapter contract tests against recorded fixtures. Partial/stale/provider-failure fixtures for every data-bearing screen. Performance scenarios tied to NFR targets. **No untested critical surface.**

## 13. Operational Readiness

`docs/operations/01-ui-operational-signals.md`: all 12 board-mandated failure modes have metric + log + alert threshold + dashboard view + runbook reference within the approved stack (Prometheus `/metrics`, structured logs, hosted Grafana, uptime checks). Binding rule: admin UI never queries logs/metrics systems — triage data served from application tables via `/admin/health`. SLO tracking monthly. No new observability infrastructure.

## 14. Deferred Capabilities

To Commercial Beta (Level 3), each with rationale and migration path (`docs/planning/03-ui-mvp-classification.md`): forecast changes feed (no impact taxonomy), consensus/agreement/confidence/disagreement (no methodology — gated on methodology extension), Live Weather screen + map (NP-01, tile dependency), raw Forecasts screen (S-05 covers the need), issuance-axis Forecast Evolution (post-freeze scope; query documented as additive), Alerts/Reports/Integrations (no MVP backend), sidebar nav + command palette + drawer (chrome polish), chart zoom/brush, dark mode, mobile-native, heatmap, additional providers. **Architectural impact now: zero** — verified per item; existing reservations (workspace_id, event seam, adapter slots, provider colors) are sufficient.

## 15. Open Questions

**Blocking Phase 1 Architecture: none.**

Non-blocking ambiguities resolved as explicit assumptions:
- A1: Chart library — implementation choice constrained by budget (200KB) + capability list (CI bands, gaps, hollow points, keyboard nav, SR table); decided during frontend setup, not architecture.
- A2: `POST /admin/collections/trigger` sync (202 vs 200) — Phase 1 implementation choice; contract fields identical.
- A3: Backup status file format — JSON convention specified in operations doc; script integration is a runbook task.
- A4: OpenWeather ToS review (D-05/BR-LIC-01) — **pre-existing product dependency, unchanged by this reconciliation**; gates public launch of provider-attributed displays, not architecture.
- A5: METMalaysia API availability — pre-existing open question (ADR-003); no MVP dependency.

## 16. Document Change Summary

| Document | Status | Key changes |
|----------|--------|-------------|
| `docs/reviews/03-ui-backend-conflicts.md` | **Created** | 20 conflicts, all resolved + owned |
| `docs/planning/03-ui-mvp-classification.md` | **Created** | Full capability classification + post-MVP impact verdicts |
| `docs/ui/04-approved-information-architecture.md` | **Created** | Approved IA; supersedes doc 03 IA; amends doc 02 §2 |
| `docs/ui/05-screen-specifications.md` | **Created** | Screen-by-screen reconciliation (identity, elements, data, capability, states, mutations) |
| `docs/ui/06-ui-state-contracts.md` | **Created** | 19-state backend contract; partial/freshness/cached/offline conventions |
| `docs/ui/07-accessibility-requirements.md` | **Created** | Backend-dependent a11y requirements; 6 amendments |
| `docs/ui/08-ui-backend-traceability.md` | **Created** | All 7 matrices |
| `docs/api/01-screen-api-contracts.md` | **Created** | Endpoint contracts; 3 new + 5 amendments marked [AMENDS] |
| `docs/api/02-response-conventions.md` | **Created** | Envelope, freshness, partial, provenance, CSV, caching, rounding |
| `docs/api/03-error-and-partial-result-contracts.md` | **Created** | 11-class error taxonomy; partial transport decision |
| `docs/domain/04-ui-domain-model-reconciliation.md` | **Created** | Entity verdicts; 3 schema additions; 8 corrections |
| `docs/domain/05-metric-ui-contract.md` | **Created** | 24-row metric traceability; computation strategy; exclusions |
| `docs/data/01-query-and-index-requirements.md` | **Created** | 11 query specs; index verdicts; N+1 audit; latency budgets |
| `docs/security/01-ui-authorization-matrix.md` | **Created** | Screen + action matrices; object-level rules; 10 findings |
| `docs/testing/01-requirement-test-traceability.md` | **Created** | Rule/state/screen → test matrices |
| `docs/operations/01-ui-operational-signals.md` | **Created** | 17 failure-mode signals; placement rules; SLOs |
| `docs/ui/00-screen-inventory.md` | **Amended** | Reconciliation note + corrected API mappings (S-01, S-03, S-05) |
| `docs/ui/02-ui-design-specification.md` | Superseded in part | §2.1 hierarchy and §14.8 open items resolved by new docs (noted in 04-IA §10) |
| `docs/ui/03-operational-dashboard-design.md` | Reclassified | Design exploration; superseded where conflicting (noted in 04-IA §10) |

## 17. Final Decisions

### UI Specification: **CONDITIONALLY APPROVED**

Conditions (all specific and owned):
1. Doc 02 stands as the approved design **as amended** by `docs/ui/04..08` (context line on S-01; removed elements per classification; state contracts binding).
2. Doc 03 is reclassified as design exploration; its Level-3 elements may not leak into MVP implementation or acceptance criteria.
3. Accessibility amendments A-01..A-06 incorporated before frontend code review.

### Phase 1 Architecture: **GO**

The reconciled UI requires no architecture changes beyond the approved modular monolith: three small tables/columns, three endpoints, five additive amendments. All promotion criteria and operational boundaries (architecture §4, §6) remain intact. Zero blocking open questions.

### Frontend Implementation: **CONDITIONAL GO**

Conditions: (1) Phase 1 Architecture completed first (production implementation remains blocked until then, per board rules); (2) contracts in `docs/ui/06..08` + `docs/api/01..03` treated as binding acceptance criteria; (3) axe-core CI gate from first screen.

### Backend Implementation: **CONDITIONAL GO**

Conditions: (1) Phase 1 Architecture first; (2) [AMENDS] markers folded into `docs/api/00-api-requirements.md` and the domain model at Phase 1 schema design; (3) circuit-state persistence and the three new endpoints included in Level 1/2 sequencing; (4) OpenWeather ToS gate (D-05) cleared before public launch of attributed provider displays (product dependency, not engineering blocker).

---

## Quality Gate Verification

| Gate criterion | Verdict |
|----------------|---------|
| Every MVP UI element maps to real data or an approved derived value | ✓ (traceability matrix 1; undefined metrics excluded) |
| Every user action maps to a defined backend command | ✓ (screen specs § actions; matrix 2) |
| Every metric has a formula, sample size, scope, and version | ✓ (metric contract; methodology 2026.1) |
| Every screen has partial, stale, empty, and error behaviour | ✓ (state contracts §9 applicability matrix) |
| Every protected action has server-side authorization | ✓ (authorization matrix §3; no frontend-only gates) |
| Every high-value query has an index or query strategy | ✓ (query doc §1–2; one additive index identified) |
| Every critical workflow has test coverage requirements | ✓ (test traceability §1–3) |
| Every serious failure mode has an operational signal | ✓ (operations doc §1; 12/12 covered) |
| No post-MVP capability appears as functional MVP navigation | ✓ (C-05; classification doc §1) |
| No unresolved critical conflicts | ✓ (20/20 resolved, owned) |
| Architecture remains appropriate for 1–2 engineers | ✓ (no new infrastructure; promotion criteria unchanged) |

**The reconciliation passes.**
