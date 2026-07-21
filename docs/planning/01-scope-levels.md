# ForecastIQ — Scope Levels

**Version**: 1.0 (Phase 0 Amendment)
**Status**: Authoritative
**Resolves**: Scope reduction exercise (Demo / Portfolio MVP / Commercial beta)

Three scope levels define what is built when. The MVP commitment is **Level 2 only**.
Level 1 is a stepping stone inside Level 2 delivery; Level 3 requires its own approval
gate after MVP validation.

---

## Level 1 — Demo (prove the concept locally)

**Purpose**: end-to-end vertical slice proving collection → storage → matching →
metrics → simple display works with real provider data.

| Capability | Specification |
|------------|---------------|
| Locations | 1 (Johor Bahru) |
| Forecast providers | 1–2 (Open-Meteo minimum; OpenWeather when key arrives) |
| Observation source | Open-Meteo Historical |
| Collection | Scheduled (hourly), dedup, raw payload storage, checksums |
| Storage | Full domain schema (ForecastCollection + ForecastSnapshot from day one — no throwaway model) |
| Evaluation | Temperature MAE/RMSE/Bias + precipitation occurrence (Recall/Precision/F1) at +24h |
| Display | Minimal local dashboard or API-only + script-rendered summary |
| Auth | None (localhost only) |
| Infra | Docker Compose, local Postgres |

**Exit criterion → Level 2**: 14 consecutive days of uninterrupted collection for JB
with ≥ 95% slot success and at least one reproducible metric computation.

## Level 2 — Portfolio MVP (public deployment; THE MVP)

**Purpose**: public, authenticated, trustworthy demonstration of the core value.

Everything in Level 1, plus:

| Area | Capability |
|------|-----------|
| Locations | Multiple (5–10: JB + global reference cities) |
| Providers | 2 (Open-Meteo + OpenWeather, ToS-gated) |
| Observations | Provenance typing, quality flags, corrections |
| Comparison | All methodology metrics, all 7 horizons, CIs, matching rules |
| Ranking | Composite score, weights, statuses, coverage penalty, ties, methodology transparency |
| Auth | Supabase Auth (registration, verification, reset), API keys |
| API | Full v1 per API requirements doc, OpenAPI published |
| Dashboard | All screens + all states per screen inventory |
| Ops | Single-VPS production, managed DB, backups + tested restore, monitoring, runbooks |
| Quality | Unit + property + integration + contract tests; CI/CD; < 5 min rollback |
| Docs | OpenAPI, ADRs, methodology page, runbooks, onboarding |

**Exit criterion → Level 3**: ≥ 90 days live; ≥ 100 registered users; ≥ 5 weekly
actives; positive signal from 5 user interviews; ToS gates cleared; infra headroom
confirmed.

## Level 3 — Commercial Beta (initial external customers)

**Purpose**: first paying usage. Requires separate approval; nothing here is committed.

May add (each with its own business case):

- Organization workspaces + RBAC (Option C ownership model; RLS enforcement)
- Billing (external payment service)
- Alert engine + notification preferences + webhooks (event bus promotion likely here)
- Additional providers (Tomorrow.io, Visual Crossing) and NOAA/NWS station observations
- Higher retention tiers; exports beyond CSV (API subscriptions)
- Bulk/batch API endpoints
- Service-level targets (with the infrastructure promotions to back them: Redis,
  read replicas, second instance, possibly K8s per criteria)
- Mobile experience; heatmap; advanced charting

## Feature Placement Summary

| Feature | L1 | L2 | L3 |
|---------|----|----|----|
| Core pipeline (collect/store/match/metrics) | ✓ | ✓ | ✓ |
| ForecastCollection lineage | ✓ | ✓ | ✓ |
| 2nd provider | optional | ✓ | +2 more |
| Provenance + quality weighting | basic | ✓ | enhanced |
| Composite ranking + transparency | — | ✓ | ✓ |
| Auth + API keys | — | ✓ | orgs/RBAC |
| Public dashboard + all states | — | ✓ | +mobile |
| Alerts/webhooks | — | — | ✓ |
| Billing | — | — | ✓ |
| Bulk API | — | — | ✓ |
| SLA-backed availability | — | — | ✓ |
