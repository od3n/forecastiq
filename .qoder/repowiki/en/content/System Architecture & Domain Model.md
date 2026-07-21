# System Architecture & Domain Model — REMOVED FROM ACTIVE DOCUMENTATION

**Status**: Removed (July 22, 2026)
**Removed by**: ForecastIQ Phase 0 Amendment Team
**Authority**: Architecture Review Board Report (2026-07-22), finding on conflicting documentation

---

## Reason for removal

This auto-generated wiki page described an **unrelated machine-learning model training
platform** (Forecasting Engine, Model Registry, Experiments, Feature Store, Scenario
Builder, model drift detection). It does not describe ForecastIQ, which is a **weather
forecast accuracy measurement platform** that compares external weather provider
forecasts against actual observations.

The Architecture Review Board identified this page as a source of dangerous confusion
for anyone onboarding, because its domain model (Organization, Tenant, Dataset, Feature,
Model, Experiment, Forecast, Scenario) conflicts with ForecastIQ's actual domain model
(Provider, Location, ForecastCollection, ForecastSnapshot, Observation, AccuracyMetric).

## Verification performed

- Content inspected: no ForecastIQ-specific concepts (forecast providers, observation
  collection, accuracy metrics, provider ranking) appear in this page.
- No ForecastIQ decision, requirement, or design depends on this page.
- No authoritative document links to this page as a source of truth.

## Authoritative replacements

The authoritative architecture and domain documentation is:

- `docs/domain/01-domain-model.md` — ForecastIQ domain model (authoritative)
- `docs/domain/02-data-lineage.md` — data lineage specification
- `docs/domain/03-metric-methodology.md` — metric and ranking methodology
- `docs/architecture/00-phase-0-architecture-constraints.md` — architecture constraints
- `docs/reviews/01-architecture-review-response.md` — review resolution record

This file is retained only as a tombstone explaining why the original content was
removed. Do not cite or regenerate the original content.
