# ForecastIQ — Architecture Review Response & Resolution Matrix

**Version**: 1.0 (Phase 0 Amendment)
**Status**: Authoritative record of how every critical/high finding was handled
**Input**: `docs/phase-0-business-analysis/11-architecture-review-board-report.md` (2026-07-22)

Findings were treated as evidence and recommendations, validated individually, and
either accepted, accepted-with-modification, or rejected with rationale. No critical
recommendation was silently accepted or rejected.

---

## 1. Blocker Resolutions

### Blocker 1 — Composite ranking score
- **Validation**: Concern fully valid. The core output had no algorithm.
- **Decision**: Weighted ratio-to-cohort-best methodology with 30/10 sample thresholds,
  coverage penalty + outranking rule, observation-quality weighting, horizon profiles,
  CI-based ties, ranking statuses, and full versioning. Worked example with 3
  providers included and turned into an integration test.
- **Documents**: `docs/domain/03-metric-methodology.md` (§6–8), BR-RANK-01..09,
  AC-3.6/3.7, ADR-010, API ranking shape.
- **Remaining risk**: cohort-relative normalization in weak cohorts (documented,
  mitigated by thresholds + always-visible raw metrics).

### Blocker 2 — Forecast acquisition and storage model
- **Validation**: Fully valid; structural flaw confirmed in 04/06 documents.
- **Decision**: ForecastCollection (one API call) → ForecastSnapshot (one prediction
  per target time). "Snapshot" terminology binding. Dedup uniqueness, idempotent
  inserts, partial-collection accounting, schema-drift detection, adapter versioning,
  replay, invalid-row handling, and full lineage specified.
- **Documents**: `docs/domain/01-domain-model.md` §4–5, `docs/domain/02-data-lineage.md`,
  FC-01..FC-15, AC-1.1..1.7, ADR-012, glossary.
- **Remaining risk**: provider push/webhook models would extend the pattern (noted in
  ADR-012 trigger).

### Blocker 3 — Accuracy formulas
- **Validation**: Partially valid. We re-derived AC-3.2: the published numbers
  (0.889/0.182/0.800/0.842) are numerically correct for TP=40/FP=10/FN=5/TN=45, but the
  review's underlying concern is real — the term "Hit Rate" is ambiguous across
  verification literature, the denominator labeling was confusing, and no zero-
  denominator/null/rounding/property rules existed anywhere.
- **Decision**: "Hit Rate" removed; Recall (POD) canonical. All formulas re-specified
  with confusion matrix, plain language, zero-denominator → null, quality weighting,
  rounding, test vectors TV-1..TV-5, and 11 property-based testing invariants.
  AC-3.2 rewritten with explicit formula checks.
- **Documents**: methodology §2, §4, §5, §10–11; AC-3.1..3.3; glossary; AC-7.4
  (verification suite).
- **Remaining risk**: formula-change discipline over time — mitigated by property
  tests + review checklist (R-07).

### Blocker 4 — MVP scope and infrastructure
- **Validation**: Fully valid; the DevOps/SRE dissent adopted in full.
- **Decision**: Modular monolith (Go) + PostgreSQL + Docker Compose + single VPS +
  managed DB + Caddy + CDN; $50–150/mo target. K8s/Temporal/NATS/Redis/TimescaleDB/S3
  deferred with per-technology problem/trigger/path/premature-adoption-risk entries.
  Hosting model and operational boundaries specified.
- **Documents**: `docs/architecture/00-phase-0-architecture-constraints.md`, NFR v2,
  ADR-001/004/005/006/007, scope levels, estimate.
- **Remaining risk**: single-VPS availability ceiling — answered by honest 99.5%
  target + rebuild runbook (R-12).

### Blocker 5 — Tenant and ownership model
- **Validation**: Fully valid; schema-impacting deferral confirmed.
- **Decision**: Hybrid A+B — single-operator behavior at MVP; workspace_id (NOT NULL,
  system default) on ownership-bearing mutable entities only; NOT on immutable child
  rows; denormalization tradeoff documented; RLS-ready; backfill path defined.
- **Documents**: domain model §8, ADR-009, FR AUTH section, risk R-11.
- **Remaining risk**: discipline to not ad-hoc denormalize workspace_id (ADR-009 is
  the gate).

### Blocker 6 — Authentication and account lifecycle
- **Validation**: Fully valid; registration/reset/policy were absent.
- **Decision**: Supabase Auth (managed) after comparing app-managed/Auth0/Clerk:
  self-registration + email verification, reset, update, rotation with theft
  detection, disable/delete, brute-force protection, audit events, concurrent-session
  policy, password policy (min 12). No passwords in our DB.
- **Documents**: FR AUTH-01..09, NFR-SEC15/16, ADR-008, AC-5.1..5.3, personas J2.
- **Remaining risk**: vendor dependency (R-14) with documented portability.

## 2. Resolution Matrix — Critical and High Findings

Columns per amendment mandate. "A" = accepted, "M" = accepted with modification,
"R" = rejected (with rationale).

| Finding ID | Review finding | Disposition | Decision | Affected documents | Affected requirements | Implementation impact | Remaining risk | Owner |
|------------|---------------|-------------|----------|--------------------|-----------------------|----------------------|----------------|-------|
| RISK-1 (Critical) | MVP scope exceeds team capacity | A | Level 2 scope cut; 90–110 engineer-day estimate; promotion criteria | mvp-scope, scope-levels, estimate, stories | all epics re-estimated | ~40% feature reduction; infra drastically simpler | Scope creep (R-04, governed) | Product |
| RISK-2 (Critical) | Composite ranking score undefined | A | Full methodology per Blocker 1 | methodology, domain, AC, API | CE-07/08, BR-RANK-* | New RankingService + transparency fields | Cohort relativity (documented) | Eng |
| RISK-3 (Critical) | ForecastSnapshot model mismatch | A | ForecastCollection/Snapshot decomposition | domain, lineage, FR, AC | FC-01..FC-15 | Schema redesign pre-implementation (cheap now) | Provider push models (watch) | Eng |
| RISK-4 (Critical) | Math error / ambiguity in AC-3.2 | M | Values verified correct; terminology + rigor fixed; "hit rate" removed | methodology, AC, glossary | CE-04/05, AC-3.x | Property-test suite mandatory | Formula drift (R-07) | Eng |
| RISK-5 (High) | Multi-tenancy deferred but schema-impacting | A | Hybrid A+B model now | domain §8, ADR-009 | schema, AUTH-06 | workspace_id columns at creation | Backfill discipline | Architect |
| RISK-6 (High) | Non-US observation quality unvalidated | A | Provenance typing + weighting + pre-launch spike gate | mvp-scope §3, ADR-003, BR-OBS-* | OC-02, spike D-06 | Spike before ranking publication | Tropical reanalysis error | Eng |
| RISK-7 (High) | Provider ToS may prohibit redistribution | A | ToS review gate before public launch; attribution everywhere; swap path | BRD D-05, ADR-002, BR-ATTR-01 | NFR-CMP02 | Launch gated on review | Review outcome unknown | Operator |
| RISK-8 (High) | K8s+Temporal+NATS+Redis+TimescaleDB overload | A | All deferred with triggers; modular monolith | arch-constraints, ADRs 001/004–007 | NFR v2 wholesale | Single binary + managed DB | None material | DevOps |
| RISK-9 (High) | No validation for provider schema changes | A | Versioned schema validation, drift alerting, invalid-row accounting, replay | domain §4.6–4.9, FC-11, FC-14 | AC-1.5, AC-1.7 | Adapter validation layer | Arms race with providers (R-01) | Eng |
| CONFLICT-1 | Alert engine in SRS but post-MVP in BRD | A | Alerts unambiguously Level 3 | FR §8, mvp-scope | ALERT-* removed | None (removed) | None | Product |
| CONFLICT-2 | gRPC mentioned, never specified | A | gRPC removed from all documents | arch-constraints, FR | — | None | None | Architect |
| CONFLICT-3 | K8s vs 1–2 engineers vs $500 | A | Resolved by arch-constraints + ADR-007 | NFR v2 | NFR-A01/A08 revised | Hosting model change | Single-VPS ceiling | DevOps |
| CONFLICT-4 | AC-3.2 denominator confusion | M | See RISK-4 | AC | AC-3.2 rewritten | Test vectors | None | Eng |
| CONFLICT-5 | US-4.2 promises confidence intervals, unspecified | A | CIs fully specified (methodology §7.4) and implemented | methodology, AC-3.1 | US-3.2, CE-07 | CI computation in engine | Approximation for composite (documented) | Eng |
| CONFLICT-6 | Billing "external service" w/o integration req | A | Billing deferred to Level 3; no phantom integration remains | BRD §4.2, mvp-scope | — | None | None | Product |
| CONFLICT-7 | Domain relationship direction wrong (snapshot→metric 1:many) | A | MatchedEvaluation many↔many aggregation model | domain §3 | CE-07 | Schema corrected pre-build | None | Architect |
| CONFLICT-8 | NOAA vs Open-Meteo priority undefined | A | Source priority = provenance rank; NOAA deferred; JB source decided | ADR-003, BR-MATCH-03, BR-OBS-02 | OC-01 | One source family in MVP | Spike outcome | Eng |
| MISSING-1 | Ranking algorithm | A | = Blocker 1 | methodology | — | — | — | Eng |
| MISSING-2 | Registration/password reset | A | Managed auth flows | FR AUTH, ADR-008 | AUTH-01 | Supabase integration | Vendor (R-14) | Eng |
| MISSING-3 | Response→snapshot decomposition | A | = Blocker 2 | domain, lineage | FC-02 | — | — | Eng |
| MISSING-4 | Condition code normalization | A | Canonical taxonomy v1 + mapping rules + unmapped behavior | domain §6, FC-15 | AC (implicit in FC-15) | Mapping tables per adapter | Taxonomy gaps (alerting) | Eng |
| MISSING-5 | Location ownership model | A | = Blocker 5 | domain §8 | — | — | — | Architect |
| MISSING-6 | Minimum sample threshold | A | 30/10 thresholds + statuses | methodology §7, BR-RANK-02 | CE-08, AC-3.6 | Status logic | Threshold calibration (90-day review) | Eng |
| MISSING-7 | Confidence intervals | A | = CONFLICT-5 | methodology §7.4 | — | — | — | Eng |
| MISSING-8 | Data freshness indicator | A | Freshness state model + thresholds + API/UI behavior | BR-FRESH-*, API §3, screen inventory | DB-04, AC-6.1 | Freshness computation service | Threshold tuning | Eng |
| MISSING-9 | Attribution display requirements | A | BR-ATTR-01: UI footer + API attribution field | business-rules, API | NFR-CMP01 | Attribution config per provider | ToS text accuracy | Operator |
| MISSING-10 | Soft-delete behavior for locations | A | Status-based deactivation; history preserved | domain §11, BR-LOC-03 | ADMIN-02 | Status fields + triggers | None | Eng |
| MISSING-14 | Timezone display rules | A | BR-TZ-01..05 | business-rules, UI data reqs | DB views | tz toggle + labels | User confusion (watchlist) | Eng |
| MISSING-15 | Empty state behavior | A | Full state matrix per screen | screen inventory §3 | DB-02, AC-6.1 | State implementation | None | Eng/Design |
| MISSING-16 | Concurrent session limits | A | Policy defined (allowed + rotation theft detection) | NFR-SEC16, ADR-008 | AUTH-03 | Managed behavior | None | Eng |
| MISSING-17 | Password policy | A | Min 12 chars; managed-auth enforcement | NFR-SEC15 | AUTH-01 | Config | None | Eng |
| MISSING-19 | Observation conflict resolution | A | Provenance-rank priority + lineage on pair | BR-MATCH-03, methodology §3.1 | CE-01 | Match selection logic | None | Eng |
| MISSING-20 | Graceful degradation per component | A | Freshness-labeled degradation; circuit breakers; public-mode fallback | NFR-A07, BR-FRESH | AC-7.x | Degradation paths | None | Eng |
| SUGG-1..20 | Top 20 suggestions | Mixed | MUST 1–6 all actioned; SHOULD 7–12 all actioned (7 via ADR-002, 8 via taxonomy, 9 via matching rules, 10 via screen inventory, 11 via API §1, 12 via ADR-004 deferral with criteria); NICE 13–18: contract checks in CI (13 done as OpenAPI diff), NATS topology documented in ADR-006 trigger path (14), Day-2 runbooks required (15 done), idempotency (16 done), pooling spec'd (17: pgxpool; PgBouncer per criteria), provider status as feature flag (18 done) | various | various | various | various | various |
| UI-GAPS 1–12 | UX gaps | A (1–11 in MVP), partial (12: admin IA defined as dashboard section) | Screen inventory + UI data requirements | ui docs | DB-01..09 | State-complete UI | Design phase work | Design |
| API-GAPS 1–12 | API gaps | A for 1, 4, 5, 6, 8, 10 (in MVP); 3/9/13 deferred as post-MVP with rationale; 7 (sparse fieldsets) rejected for MVP (YAGNI; payload sizes small); 11/12 handled by versioning governance | api reqs | API-* | OpenAPI work | None material | Eng |
| DB-GAPS 1–12 | DB gaps | A: partitioning (1, via ADR-004), indexes (2), compression (3: rejected at MVP per ADR-004 rationale), continuous aggregates (4: rejected per mandate — not justified), ForecastCollection (5), observation dedup (6), soft delete (7), audit partitioning (8: 1y retention + partitioning), FK strategy (9: no deletes ever), updated_at (10), materialized rankings (11: ProviderRanking table), pooling (12: pgxpool + criteria) | domain §11, ADR-004 | schema | Migrations | None | Eng |
| OPS-GAPS 1–12 | Ops gaps | A for MVP-applicable: key rotation runbook (1), disk-full alerting (2), NATS recovery N/A (3: deferred), failover procedure (4: managed DB + runbook), rollback (5: pipeline), log volume (6: bounded by request-ID model, hosted logs), certs (7: Caddy auto), chaos testing (8: M to post-MVP; degradation tested via integration instead), capacity model (9: NFR-S01 + load test), incident response (10: runbook), restore tests (11: monthly automated), feature flags (12: DB status fields) | NFR v2, arch-constraints §6 | runbooks | Runbook authoring | Operational diligence | DevOps |
| SMELLS 1–7 | Architecture smells | A: 1–4 resolved by monolith + deferrals; 5 (missing gateway) M: Caddy as TLS/ingress, no API gateway needed for one service; 6 (single DB) A: accepted deliberately with module-owned tables + promotion path; 7 (no CQRS) A: precomputed metric/ranking rows ARE the read model | arch-constraints | — | — | None | Architect |
| BIZ-GAPS 1–8 | Business rule gaps | A: all 8 closed in business-rules doc (min data age BR-RANK-09; tz BR-PROV-01; dedup BR-LOC-01; tier limits BR-LOC-02; source priority BR-OBS-02; invalidation BR-INV-*; API attribution BR-ATTR-01; derived-metric licensing BR-LIC-01) | business-rules | BR-* | Rule enforcement points | ToS review (D-05) | Operator |
| WIKI | Auto-generated wiki describes ML platform | A | Content removed; tombstone with rationale placed; no dependencies found | repowiki tombstone | — | None | None | Eng |
| DISSENT | DevOps/SRE NO-GO dissent on infra | A | Adopted in full as the basis of Blocker 4 resolution | arch-constraints, ADR-007 | — | — | None | DevOps |

## 3. Rejected Items (with rationale — none critical)

| Item | Rationale |
|------|-----------|
| API gap #7 (sparse fieldsets) | MVP payloads are small; adds API surface without a user journey. Revisit with real consumer feedback. |
| Suggestion #16 full NATS topology now | NATS deferred; topology documented in ADR-006 migration path instead (right-sized). |
| Chaos testing as MVP gate (OPS-8) | Single-binary degradation is integration-testable; chaos program belongs to Level 3 ops maturity. |
| "Add tenant_id to every table now" (review rec. 5 literal wording) | Modified per amendment mandate: ownership columns on parents only; the mandate explicitly warned against duplicating ownership on immutable child rows without benefit. Retrofit cost concern is still fully addressed (ADR-009). |

## 4. Verification of Consistency

All six blockers resolved and reflected consistently across: vision, BRD, MVP scope,
business rules, FR, NFR, AC, domain model, methodology, API, UI, risk register, scope
levels, estimate, and 12 ADRs. Terminology audit performed against the glossary
("snapshot", "hit rate" absent except as deprecated entry, "accuracy" never bare).
