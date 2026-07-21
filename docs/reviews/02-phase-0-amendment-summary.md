# ForecastIQ — Phase 0 Amendment Summary

**Version**: 1.0
**Date**: 2026-07-22
**Status**: Authoritative — executive record of the amendment

---

## 1. Executive Summary

The Architecture Review Board issued a **Conditional GO** on Phase 0 with six blocking
findings. The Amendment Team validated every critical/high finding, resolved all six
blockers, and produced a revised, internally consistent, buildable MVP specification:

1. **Ranking methodology defined** — weighted composite score with published weights,
   30/10 sample thresholds, coverage penalty and outranking rule, observation-quality
   weighting, horizon profiles, CI-based ties, statuses, and versioning
   (`docs/domain/03-metric-methodology.md`).
2. **Data model fixed** — ForecastCollection (one API call) → ForecastSnapshot (one
   prediction per target time), with dedup, idempotency, partial collections, schema-
   drift handling, replay, and end-to-end lineage.
3. **Statistics made rigorous** — "hit rate" removed, Recall canonical, all formulas
   re-specified with null/zero-denominator rules, rounding, test vectors, and 11
   property-based invariants. (AC-3.2's numbers were verified correct; the real defect
   was terminology and missing rigor — fixed.)
4. **Infrastructure right-sized** — modular monolith + PostgreSQL + single VPS at
   ~$50/mo; K8s/Temporal/NATS/Redis/TimescaleDB/S3 deferred with measurable promotion
   criteria. The DevOps/SRE dissent was adopted in full.
5. **Ownership decided** — single-operator MVP on a workspace-ready schema
   (workspace_id on parents, not immutable children; tradeoff documented).
6. **Authentication decided** — Supabase Auth with full account lifecycle; no
   passwords in our database.

Additionally: provider scope cut to Open-Meteo + OpenWeather (ToS-gated, swap path
documented); observation strategy decided with provenance typing and a launch-gating
quality spike; condition taxonomy, matching rules, freshness model, timezone rules,
UX state matrix, API contract corrections, and database corrections all specified;
the unrelated auto-generated wiki page was removed with a tombstone; 12 ADRs created;
estimates converted to engineer-days (103–128 total; 6.5 months for one engineer,
3.5–4 for two).

## 2. Document Mapping (old → new authoritative set)

The repository's Phase 0 draft set is retained as a historical record, marked
**SUPERSEDED**. The authoritative baseline is now:

| Old document (superseded) | New authoritative document(s) |
|---------------------------|-------------------------------|
| `docs/phase-0-business-analysis/01-product-vision.md` | `docs/product/01-product-vision.md` |
| `docs/phase-0-business-analysis/02-business-requirements.md` | `docs/product/02-business-requirements.md`, `docs/product/05-business-rules.md`, `docs/risk/01-risk-register.md` |
| `docs/phase-0-business-analysis/03-software-requirements-spec.md` | `docs/requirements/01-functional-requirements.md`, `docs/architecture/00-phase-0-architecture-constraints.md` |
| `docs/phase-0-business-analysis/04-functional-requirements.md` | `docs/requirements/01-functional-requirements.md`, `docs/api/00-api-requirements.md` |
| `docs/phase-0-business-analysis/05-non-functional-requirements.md` | `docs/requirements/02-non-functional-requirements.md` |
| `docs/phase-0-business-analysis/06-domain-model.md` | `docs/domain/01-domain-model.md`, `docs/domain/02-data-lineage.md` |
| `docs/phase-0-business-analysis/07-use-case-diagram.md` | `docs/product/04-personas-and-user-journeys.md` (journeys replace use-case diagrams) |
| `docs/phase-0-business-analysis/08-user-stories.md` | `docs/requirements/03-user-stories.md`, `docs/planning/02-revised-mvp-estimate.md` |
| `docs/phase-0-business-analysis/09-acceptance-criteria.md` | `docs/requirements/04-acceptance-criteria.md` |
| `docs/phase-0-business-analysis/10-phase-summary.md` | `docs/reviews/02-phase-0-amendment-summary.md` (this document) |
| `docs/phase-0-business-analysis/11-architecture-review-board-report.md` | **Retained as-is** (review input; not superseded) |
| `.qoder/repowiki/.../System Architecture & Domain Model.md` | Removed; tombstone in place |

New documents with no Phase 0 predecessor: `docs/product/03-mvp-scope.md`,
`docs/product/06-product-contract.md`, `docs/product/07-glossary.md`,
`docs/domain/03-metric-methodology.md`, `docs/ui/00-screen-inventory.md`,
`docs/ui/01-ui-data-requirements.md`, `docs/planning/01-scope-levels.md`,
`docs/reviews/01-architecture-review-response.md`, and `docs/adr/ADR-001..012`.

## 3. What Changed at a Glance

| Area | Before | After |
|------|--------|-------|
| Providers (MVP) | 4 | 2 (ToS-gated; swap path) |
| Observation source | NOAA primary (unusable at launch) | Open-Meteo Historical, provenance-typed, spike-gated |
| Data model | 1 row per API response | Collection → snapshots with lineage |
| Ranking | Undefined composite | Versioned methodology, thresholds, penalties, CIs |
| "Hit rate" | Ambiguous metric | Removed; Recall canonical |
| Infrastructure | K8s+Temporal+NATS+Redis+TimescaleDB+S3 | Modular monolith + Postgres + VPS (~$50/mo) |
| Tenancy | Deferred, zero schema | Workspace-ready schema, single-operator behavior |
| Auth | Login only | Managed full lifecycle |
| Estimates | 144 uncalibrated points | 103–128 engineer-days with calendar plan |
| Availability promise | 99.9% on infra we won't run | 99.5% honestly, with rebuild runbook |
| UX states | Unspecified | Full state matrix per screen |
| API | total_count, no idempotency/request-IDs | Corrected contract per amendment |

## 4. Open Questions (genuinely blocking only)

| # | Question | Blocks | Resolution path |
|---|----------|--------|-----------------|
| Q1 | Does OpenWeather's free-tier ToS permit storage + display with attribution for this project? | Public launch (not Phase 1) | D-05 legal review; swap to Tomorrow.io if no |
| Q2 | Is Open-Meteo Historical adequate as JB ground truth within stated uncertainty? | Ranking publication | D-06 spike vs. METMalaysia bulletins/literature |

All other Phase 0 open questions were resolved by documented decisions (cloud →
Hetzner-class VPS; scheduling → ADR-005; dashboard → Next.js; monorepo → single repo;
multi-tenancy → ADR-009; raw payloads → ADR-011; alerts → Level 3; GDPR position →
NFR-D08). Non-blocking assumptions are recorded in BRD §7.

## 5. Final Decisions

| Gate | Decision | Conditions |
|------|----------|-----------|
| **Phase 1 Architecture** | **GO** | All six blockers resolved; architecture constraints document binding; ADRs 001–012 in force |
| **UI design exploration** | **GO** | Screen inventory + UI data requirements sufficient for a design agent; no business-rule invention needed |
| **Production implementation** | **CONDITIONAL GO** | Conditions: (a) D-05 ToS review cleared before public launch, (b) D-06 observation-quality spike accepted before rankings are published, (c) Level 1 exit criterion met (14 d uninterrupted collection) |
