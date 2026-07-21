# ForecastIQ — MVP Scope Definition

**Version**: 1.0 (Phase 0 Amendment)
**Status**: Authoritative
**Resolves**: ARB recommendation #7 (cut to 2 providers + 1 observation source), provider/observation scope amendment

---

## 1. Scope Level

The MVP is **Level 2 — Portfolio MVP** (full level definitions in
`docs/planning/01-scope-levels.md`): publicly deployed, authenticated, tested,
documented, but single-operator and infrastructure-lean.

## 2. Forecast Provider Selection

### 2.1 Evaluation criteria (per amendment mandate)

| Criterion | Open-Meteo | OpenWeather | Tomorrow.io | Visual Crossing |
|-----------|-----------|-------------|-------------|-----------------|
| Developer access | No key needed; open API | Free key, instant | Free key, instant | Free key, instant |
| Free-tier limits | Generous (10K req/day non-commercial) | ~1,000 req/day | ~500–1,000 req/day | 1,000 req/day |
| Attribution required | Yes (link + notice) | Yes (attribution text) | Yes | Yes |
| Storage rights | Permissive (open data; verify current terms — D-05) | ToS review required (D-05) | ToS review required | ToS review required |
| Historical forecast availability | Via archive API (paid) — not needed; we store our own | N/A (we store our own) | N/A | N/A |
| Malaysian coverage | Global grid — excellent | Global — good | Global — good | Global — good |
| Redistribution restrictions | Low (open) | Moderate — validation required | Moderate | Moderate |
| Forecast array shape | Hourly array up to 16 days | 5-day/3h + OneCall hourly | Hourly timelines | Hourly/daily arrays |

### 2.2 Decision

- **Provider 1: Open-Meteo** — no key, permissive licensing, global coverage, clean
  hourly arrays. Lowest legal and operational risk.
- **Provider 2: OpenWeather** — largest ecosystem familiarity, distinct model lineage
  (own NWP), good tropical coverage. **Condition:** ToS validation (D-05) confirms
  storage + display with attribution for this non-commercial portfolio project.
- **Documented fallback:** if OpenWeather ToS review fails, swap to **Tomorrow.io**
  (adapter interface makes this a bounded change; estimate in
  `docs/planning/02-revised-mvp-estimate.md`).
- **Not selected for MVP:** Visual Crossing (deferred to Level 3; no disadvantage
  identified, simply not needed to prove the value with 2 providers).
- The review's suggestion to default "OpenWeather + Open-Meteo" is accepted **with the
  ToS condition made explicit** — OpenWeather is not assumed suitable without the check.

### 2.3 Collection cadence (rate-limit feasible)

| Horizon group | Cadence | Calls/day/location/provider |
|---------------|---------|------------------------------|
| +1h … +24h (from one hourly-array response) | Hourly | 24 |
| +3d, +7d (from same response — arrays include them) | Same hourly call covers them | 0 extra |

Key insight vs. Phase 0: because providers return **arrays**, one hourly call yields
snapshots for **all horizons** — the Phase 0 "every 15 min" cadence was unnecessary
overkill. MVP: hourly collection. OpenWeather: 24 calls/day/location → up to ~40
locations on the free key; MVP runs 5–10 locations with headroom.

## 3. Observation Source Strategy

### 3.1 Options evaluated (Johor Bahru + global expansion)

| Source | Type | Johor Bahru? | Legally usable? | MVP verdict |
|--------|------|--------------|-----------------|-------------|
| Open-Meteo Historical / past-weather API | Station blend + reanalysis (ERA5-family), per-variable provenance where exposed | Yes, global grid | Yes (verify terms — D-05) | **Selected (primary)** |
| METMalaysia | National station network | Best local ground truth | No public API confirmed | Not selectable; open question retained |
| NOAA/NWS | Direct station observations | No (US-only) | Yes (public domain) | US-expansion path, documented adapter slot |
| Provider "current weather" endpoints | Provider-estimated current conditions | Yes | Varies | Allowed only as `provider_estimated` type, weight 0.5, never primary |
| Other research datasets (e.g., ERA5 direct) | Reanalysis | Yes | Yes (Copernicus) | Post-MVP enhancement if quality spike justifies |

### 3.2 Provenance distinctions (binding)

The system distinguishes and always displays:

| observation_type | Meaning | Quality weight |
|------------------|---------|----------------|
| `station_observation` | Direct measurement from a weather station | 1.0 |
| `interpolated` | Spatially interpolated from nearby stations | 0.8 |
| `reanalysis` | Model-assimilated historical reconstruction | 0.8 |
| `provider_estimated` | A forecast provider's "current conditions" estimate | 0.5 |

Open-Meteo Historical rows are typed per the API's exposed provenance; where the API
does not expose it per-variable, the source default (`reanalysis` blend) applies and is
documented. The UI and API always surface the type — rankings are never presented as
"ground truth" without provenance context.

### 3.3 Observation collection cadence

Hourly, at :05 past the hour (allows source publication delay), for all active
locations. Deduplicated by `(source, location_id, observed_at)`.

## 4. MVP Feature Set (included)

| Area | Included |
|------|----------|
| Collection | 2 providers, hourly, dedup, idempotent, checksummed raw payloads, circuit breaker, backoff |
| Observations | 1 source family, provenance typing, range validation, corrections |
| Analysis | Exact-hour matching, all §4 methodology metrics, CIs, coverage/reliability, ranking statuses, worked-methodology transparency |
| API | v1 REST: providers, locations, forecasts, observations, accuracy, rankings, health; auth endpoints via managed auth; API keys; idempotency; request IDs; ETags; cursor pagination; RFC 7807 errors; rate-limit headers; CORS |
| Dashboard | Overview/rankings, trends, forecast-vs-actual, location detail, provider detail, admin section, settings; all UX states per `docs/ui/00-screen-inventory.md` |
| Auth | Supabase Auth: self-registration + email verification, login, password reset, session management; app-side API keys |
| Ops | Docker Compose dev; single-VPS prod; managed DB; backups + tested restore; runbooks; `/metrics`; structured logs |
| Quality | Unit + property-based formula tests; adapter contract tests; integration tests; OpenAPI contract checks in CI |

## 5. Explicitly Deferred (with reason)

| Item | Level | Reason |
|------|-------|--------|
| Alert engine + notification preferences | 3 | Not needed to prove core value; event seam preserved |
| Webhooks | 3 | No MVP consumer |
| Billing | 3 | Free-tier validation first |
| Organization workspaces / RBAC | 3 | Schema-ready now (workspace_id on parents) |
| Providers 3–4 (Tomorrow.io, Visual Crossing) | 3 | 2 providers suffice for comparison value |
| NOAA/NWS adapter | 3 (US launch) | Johor Bahru focus |
| Bulk/batch API | 3 | No MVP journey requires it (per amendment) |
| Mobile app / PWA | 3 | Responsive web suffices |
| Heatmap (geo grid) | 3 | 5–10 locations don't justify a heatmap; location-comparison table instead |
| AI summaries | 3+ | Speculative |

## 6. Removed from prior scope

- 15-minute collection cadence (hourly suffices; arrays cover horizons).
- `total_count` in pagination (replaced by `has_more`; optional count endpoint later).
- gRPC internal interfaces (never specified; removed).
- S3 raw-payload storage (volume + promotion criteria instead).
- "Admin portal" as separate app (section of dashboard).
- 99.9% availability target for MVP (99.5% honest target; 99.9% is Level 3 with infra to match).

## 7. Cross-Reference

- Scope levels: `docs/planning/01-scope-levels.md`
- Estimate: `docs/planning/02-revised-mvp-estimate.md`
- Provider/observation ADRs: `docs/adr/ADR-002-provider-scope.md` (numbering per ADR index), observation strategy ADR-003
- ToS gate: BRD dependency D-05
