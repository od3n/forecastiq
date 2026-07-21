# ForecastIQ — Product Vision (Revised)

**Version**: 2.0 (Phase 0 Amendment)
**Status**: Authoritative
**Supersedes**: `docs/phase-0-business-analysis/01-product-vision.md`

---

## Tagline

> Measure. Compare. Know which forecast to trust.

## Vision Statement

ForecastIQ is a **weather forecast verification platform** that continuously collects
forecasts from multiple providers, stores immutable forecast records with full data
lineage, collects actual observations with explicit provenance, compares forecasts
against reality using published statistical methodology, and shows **which provider
performs best for a location and forecast horizon**.

ForecastIQ is **not** a weather app. It does not tell users what the weather will be.
It tells users **which forecast provider to trust** — and proves it with transparent,
reproducible statistics.

## Core Value (MVP focus)

> Collect forecasts, collect observations, compare predictions against reality, and
> show which provider performs best for a location and forecast horizon.

Every MVP feature must trace to this sentence. Anything that does not is out of scope
until the MVP is validated.

## Problem Statement

| Question | Impact |
|----------|--------|
| Which forecast provider is most accurate for my location? | Operational efficiency |
| Which forecast horizon performs best? | Planning confidence |
| How accurate are rain predictions specifically? | Event preparation |
| Is a provider's "90% score" based on 5 samples or 500? | Statistical honesty |
| Has forecast quality improved or degraded over time? | Vendor accountability |

No accessible tool answers these with published methodology and reproducible data.

## Solution Overview (revised)

1. **Multi-provider forecast collection** — immutable snapshots with collection lineage
   (MVP: Open-Meteo + OpenWeather; see `docs/product/03-mvp-scope.md`).
2. **Observation collection with provenance** — station/interpolated/reanalysis sources
   explicitly typed and quality-weighted.
3. **Comparison engine** — exact-hour matching, weighted metrics, confidence intervals.
4. **Transparent ranking** — composite score with published weights, minimum samples,
   coverage penalties, and always-visible per-metric breakdowns
   (`docs/domain/03-metric-methodology.md`).
5. **REST API** — programmatic access with provenance, freshness, and methodology
   version in every response.
6. **Dashboard** — rankings, trends, forecast-vs-actual, with complete empty/loading/
   error/stale states.

**Removed from the solution statement:** alert engine (Level 3), admin "portal" as a
separate product surface (admin is a section of the dashboard), AI summaries,
community tier.

## Target Users (MVP)

Primary MVP user: **the operator** (1–2 engineer team running the platform) and
**weather-curious individuals** in the initial coverage area (Johor Bahru, expanding
globally). Business/enterprise tiers remain the long-term direction but are not MVP
design drivers (see personas document).

## Success Metrics (revised, honest for MVP)

| Metric | MVP target (6 months post-launch) |
|--------|-----------------------------------|
| Providers integrated | 2 (path to 4 documented) |
| Locations monitored | ≥ 5 (Johor Bahru + 4 global reference cities) |
| Days of continuous collection | ≥ 90 without data-loss incident |
| Ranked provider–location–horizon cells | ≥ 60% of cells at `ranked` status |
| API uptime | 99.5% (single-VPS reality; 99.9% is Level 3) |
| API p95 latency | < 200 ms |
| Metric reproducibility | 100% (any published number recomputable from stored data) |
| Registered users | ≥ 100 |

The prior "10M snapshots / 1,000 users / 10 paying customers in Year 1" targets are
deferred to Level 3 planning; they assumed infrastructure the MVP no longer has.

## Guiding Principles (unchanged, reinforced)

1. **Immutability** — forecasts and observations are never overwritten.
2. **Transparency** — every metric is reproducible; methodology, weights, and sample
   sizes are always published alongside results.
3. **Provider-agnostic** — no preferential treatment; our own collection failures are
   distinguished from provider failures.
4. **API-first** — every feature available via API before UI.
5. **Statistical honesty** — insufficient data is displayed as such, never papered over.
6. **Simplicity as a feature** — the simplest architecture that proves the value
   (modular monolith; see architecture constraints).

## Revenue Model (future, unchanged direction)

Free / Pro / Enterprise tiers remain the long-term model. The MVP validates the free
tier only. Billing integration is explicitly out of scope until Level 3.
