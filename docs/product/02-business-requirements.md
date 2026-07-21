# ForecastIQ — Business Requirements Document (Revised)

**Version**: 2.0 (Phase 0 Amendment)
**Status**: Authoritative
**Supersedes**: `docs/phase-0-business-analysis/02-business-requirements.md`

---

## 1. Executive Summary

ForecastIQ measures weather forecast accuracy. It collects provider forecasts
immutably, collects observations with provenance, compares them with published
statistics, and ranks providers per location and horizon. This revision right-sizes
the business scope to a 1–2 engineer team and a USD 50–150/month operating envelope
while preserving the expansion path to commercial tiers.

## 2. Business Objectives (revised)

| ID | Objective | Priority | Success Criteria |
|----|-----------|----------|------------------|
| BO-01 | Enable data-driven provider selection | Critical | Users can rank providers by location/horizon with sample sizes and CIs shown |
| BO-02 | Provide historical accuracy trends | Critical | ≥ 90 days of comparable data for all MVP locations |
| BO-03 | Prove statistical rigor as differentiator | Critical | Methodology published; 100% of metrics reproducible from stored data |
| BO-04 | Validate willingness to use/pay (portfolio MVP) | High | ≥ 100 registered users; ≥ 5 recurring weekly users; qualitative interviews |
| BO-05 | Establish foundation for commercial tiers | Medium | Workspace-ready schema; provider expansion path documented |

Removed: BO-03-old (alert within 5 min — Level 3), "thought leadership" (vague; replaced
by BO-03's concrete rigor criterion).

## 3. Stakeholders (revised)

| Role | Interest | Influence | MVP relevance |
|------|----------|-----------|---------------|
| Platform operator (the team) | System health, cost, data quality | High | Primary user of admin views |
| Individual users | Accuracy data for personal decisions | Low | Primary external audience |
| Weather providers | ToS compliance, attribution | Medium | Compliance obligation |
| Future business subscribers | API access, reliability | Medium | Design-for, not build-for |
| Reviewers (ARB, portfolio viewers) | Engineering quality, honesty | High | Documentation & methodology quality |

## 4. Scope

### 4.1 In scope (MVP — Level 2 "Portfolio MVP")

Authoritative scope definition: `docs/product/03-mvp-scope.md`. Summary:

- Forecast collection from **2 providers**: Open-Meteo, OpenWeather (selection rationale
  and ToS validation task in §8 / ADR-012-supporting docs; Tomorrow.io is the
  documented fast-follow).
- Observation collection from **1 source family**: Open-Meteo Historical (global,
  provenance-typed). NOAA/NWS integration path documented for US expansion.
- Immutable storage with ForecastCollection → ForecastSnapshot lineage.
- Comparison engine: exact-hour matching, weighted metrics, CIs, ranking statuses.
- REST API v1 (OpenAPI 3.1), dashboard (Next.js), managed auth (Supabase).
- Admin capabilities inside the dashboard (providers, locations, schedules, health).
- Observability: structured logs, `/metrics`, health endpoints, request IDs.
- Tests (unit + property-based for all formulas + integration), CI/CD, backup/restore.

### 4.2 Deferred (Level 3 — commercial beta)

Alert engine, webhooks, billing, organization workspaces + RBAC, bulk/batch API
endpoints, additional providers (Tomorrow.io, Visual Crossing), NOAA/NWS station
observations, exports beyond CSV, SLAs, mobile-native experience, AI summaries.

**Conflict resolution vs. Phase 0:** the alert engine is now unambiguously post-MVP
(resolves Conflict #1); gRPC is removed entirely (Conflict #2); Kubernetes/NFR
conflicts resolved by the architecture constraints document (Conflict #3).

### 4.3 Out of scope (all levels)

Consumer weather display, real-time streaming (< 1 min), custom ML training, hardware
sensors, becoming a weather data reseller.

## 5. Business Rules

Moved to a dedicated authoritative document: `docs/product/05-business-rules.md`.
The BRD no longer duplicates rules; it references them. Core rules retained in summary:
immutability (BR-01), forecast/observation separation (BR-02), comparison requires both
sides (BR-03), attribution always displayed (BR-04), UTC everywhere (BR-08), plus new
ranking rules (BR-RANK-*), freshness rules (BR-FRESH-*), and matching rules (BR-MATCH-*).

## 6. Constraints (revised)

| ID | Constraint | Impact |
|----|-----------|--------|
| C-01 | Provider API rate limits (OpenWeather free: ~1,000 calls/day) | MVP collection cadence: hourly per location (24/day/location → ≤ 40 locations on one key); staggered schedules |
| C-02 | Provider ToS (attribution, storage, redistribution) | Legal validation task required before public launch (§8 D-08); attribution in UI + API metadata |
| C-03 | Budget: infrastructure **target USD 50–150/month** (hard ceiling $500) | Architecture constraints doc binding |
| C-04 | Team: 1–2 engineers | Modular monolith; no K8s/Temporal/NATS (promotion criteria documented) |
| C-05 | Observation availability varies by location | Provenance typing + coverage display; Open-Meteo global fallback is the MVP source |
| C-06 | Johor Bahru is the launch location | METMalaysia has no confirmed public API — not relied upon |

## 7. Assumptions (revised)

| ID | Assumption | Validation | Risk if wrong |
|----|-----------|------------|---------------|
| A-01 | Open-Meteo remains free/usable for this workload (non-commercial MVP usage) | Monitor licensing terms; commercial subscription available as fallback | Switch observation/provider mix |
| A-02 | OpenWeather free tier ToS permits storage + display with attribution for a non-commercial portfolio project | **Legal/ToS review before public launch (blocking task D-08)** | Swap second provider to Tomorrow.io (adapter pattern makes this a ~1-week change) |
| A-03 | 2 providers show meaningful accuracy differences | 30-day pilot data analysis | Add third provider earlier |
| A-04 | Open-Meteo Historical observations are adequate ground truth for tropical maritime climate | Spike: compare against METMalaysia published station bulletins where obtainable + literature | Add station source; display provenance caveats prominently |
| A-05 | Standard PostgreSQL meets time-series needs at MVP scale | Load test at 2× projected volume (Phase 4) | TimescaleDB promotion per criteria |
| A-06 | Single-region, single-VPS deployment meets MVP availability | Uptime monitoring; managed DB as separate failure domain | Vertical scaling / failover runbook |

## 8. Dependencies (revised)

| ID | Dependency | Owner | Status |
|----|-----------|-------|--------|
| D-01 | OpenWeather API key (free tier) | Engineering | Required |
| D-02 | Open-Meteo API (no key) | N/A | Available |
| D-03 | Supabase project (Auth + optional managed Postgres) | Engineering | Required |
| D-04 | Cloud infrastructure (VPS + managed DB + domain) | Engineering | Required |
| D-05 | Legal/ToS review: OpenWeather storage+display rights; Open-Meteo license for MVP usage | Operator | **Required before public launch** |
| D-06 | Observation quality spike (A-04) | Engineering | Required before ranking publication |

Removed: Tomorrow.io key, Visual Crossing key, NOAA feed (all deferred with scope).

## 9. Risks

Moved to the authoritative risk register: `docs/risk/01-risk-register.md`.
The BRD references it rather than duplicating.

## 10. Approval

| Role | Name | Date | Signature |
|------|------|------|-----------|
| Product Owner | | | |
| Technical Lead | | | |
