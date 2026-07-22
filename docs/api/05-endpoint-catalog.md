# ForecastIQ — Endpoint Catalog (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: `docs/api/00-api-requirements.md` §4; `docs/api/01-screen-api-contracts.md` (amendments folded per board mandate)

Complete catalog of all MVP endpoints. Response envelope per `docs/api/02-response-conventions.md`; errors per `docs/api/03-error-and-partial-result-contracts.md`. Latency targets: p50/p95.

---

## 1. Catalog

### 1.1 Public analysis endpoints

| Method | Route | Purpose | Auth | Key parameters | Response (data) | Errors | Partial | Cache | p50/p95 |
|--------|-------|---------|------|----------------|-----------------|--------|---------|-------|---------|
| GET | `/rankings` | Composite rankings + transparency (S-01/S-02) | public | location_id (req), horizon_minutes \| horizon_profile, period_days (30; 90 auto for +3d/+7d), min_sample_count (30, echoed), weights (optional custom) | rankings[] (rank, provider+attribution, status, composite, ci, sample_count, coverage, reliability, components{}, penalty flag, significant_vs_next), observation_context{}, evaluation_period, methodology/weights versions | 422 invalid location/horizon | warnings[] per provider; affected omitted | ETag + LRU 60 s; public max-age=60 | 50/200 ms |
| GET | `/rankings/methodology` | Machine-readable methodology (S-06) | public | — | formulas[], default_weights, thresholds, coverage_penalty, statuses, tie_rule, change_history[] | — | — | ETag; long max-age | 10/50 ms |
| GET | `/accuracy/summary` | All metrics per provider for location+horizon (S-02) OR all cells for provider (S-03) | public | location_id XOR provider_id (req), horizon_minutes (location mode), period_days | location mode: providers[{ranking_status, metrics[], collection_window{}}]; provider mode: cells[{location, horizon, composite, status, sample, coverage}] | 422 both/neither id | warnings[] per provider/location | ETag + LRU 60 s | 50/200 ms |
| GET | `/accuracy` | Metric trend series (S-04, S-03) | public | provider_id, location_id, horizon_minutes, variable, metric_type, period_start/end (≤ 365 d), aggregation (daily\|weekly\|monthly), tz (IANA, echoed) | series[{provider_id, buckets[{period_start, period_end, value, ci, sample_count}]}] | 422 range > 365 d | warnings[] | ETag + LRU 60 s | 50/200 ms |
| GET | `/forecast-comparison` | FvA day payload (S-05) | **public (new, C-19)** | location_id (req), date (req, location tz), variable (req), horizon_minutes (req), providers (optional CSV) | series[] (per provider, points), observations[] (provenance per point), day_metrics[] (mae/bias/rmse + methodology_version), error_band_mae, provenance, attribution | 422 params; 404 location | warnings[] per provider; absent line + warning | ETag; past 300 s / today 60 s | 80/200 ms |

### 1.2 Public catalog endpoints

| Method | Route | Purpose | Auth | Key parameters | Response | Errors | Cache | p50/p95 |
|--------|-------|---------|------|----------------|----------|--------|-------|---------|
| GET | `/providers` | Provider list + attribution + status | public | — | providers[{id, name, slug, status, attribution, adapter_version (latest success), collecting_since}] | — | ETag 300 s | 10/50 ms |
| GET | `/providers/{id}` | Provider detail | public | — | single provider + adapter_version + collecting_since | 404 | ETag 300 s | 10/50 ms |
| GET | `/locations` | Location list (S-01 selector, S-12) | public | active (bool), cursor/limit | locations[{id, name, country_code, lat, lon, timezone, status, created_at}] | — | ETag 300 s | 10/50 ms |
| GET | `/locations/{id}` | Location detail | public | — | single location | 404 | ETag 300 s | 10/50 ms |
| POST | `/locations` | Create location | **admin** | Idempotency-Key; body {name, latitude, longitude, country_code, timezone, allow_near_duplicate?} | created location | 422 validation; 409 duplicate (existing ref + distance) | no-store | 100/500 ms |
| PUT | `/locations/{id}` | Update mutable fields | admin | {name?, timezone?} | updated | 422; 404 | no-store | 100/300 ms |
| PATCH | `/locations/{id}/status` | Enable/disable | admin | {status} | updated | 422 enum; 404 | no-store | 100/300 ms |

### 1.3 User (authenticated) endpoints

| Method | Route | Purpose | Auth | Key parameters | Response | Errors | Cache | p50/p95 |
|--------|-------|---------|------|----------------|----------|--------|-------|---------|
| GET | `/forecasts` | Raw snapshot query | user+ | location_id (req), provider_id, issued_after/before, target_after/before (range ≤ 31 d req), horizon_minutes, cursor/limit ≤ 200 | snapshots[] + forecast_collection_id lineage | 422 missing filters | no-store | 50/200 ms |
| GET | `/forecasts/{id}` | Snapshot detail + collection provenance | user+ | — | snapshot + collection block | 404 | no-store | 20/100 ms |
| GET | `/observations` | Raw observation query | user+ | location_id (req), source, observation_type, quality_flag, observed_after/before (≤ 31 d), cursor/limit | observations[] + provenance | 422 | no-store | 50/200 ms |
| GET | `/observations/{id}` | Observation detail + supersession | user+ | — | observation + links | 404 | no-store | 20/100 ms |
| GET | `/me` | Profile (S-07/S-09) | bearer | — | auth_subject, email, role, status, workspace, default_location_id, preferences, timestamps | 401 | no-store | 20/100 ms |
| PATCH | `/me` | Preferences/default location | bearer | {default_location_id?, preferences?} | updated | 422 (location not active) | no-store | 50/200 ms |
| POST | `/api-keys` | Create key (shown once) | bearer | Idempotency-Key; {name, scopes, rate_limit_per_min?, expires_at?} | {key (plaintext, once), id, prefix, scopes, ...} | 422 | no-store | 100/300 ms |
| GET | `/api-keys` | Own keys | bearer | cursor/limit | keys[] (prefix, scopes, status, created, last_used — never hash) | — | no-store | 20/100 ms |
| DELETE | `/api-keys/{id}` | Revoke | bearer | — | 204 | 404 (not owner) | no-store | 50/200 ms |
| POST | `/me/export` | GDPR export | bearer | — | {job_id, status} | 409 active job | no-store | 100/300 ms |
| DELETE | `/me` | Account deletion | bearer | — | 202 + audit | 409 (sole admin) | no-store | 500/2000 ms |

### 1.4 Admin endpoints

| Method | Route | Purpose | Auth | Key parameters | Response | Errors | Cache | p50/p95 |
|--------|-------|---------|------|----------------|----------|--------|-------|---------|
| GET | `/admin/health` | Collection health (S-10) | admin | provider_id, location_id, status filters | cells[] (last success, circuit, errors, next_scheduled_at), observation_collector{}, system{payload_volume, engine_lag, last_backup, last_restore_test} | — | no-store | 80/200 ms |
| POST | `/admin/collections/trigger` | Immediate collection (S-10/S-13) | admin | {provider_id, location_id}; Idempotency-Key | collection result | 409 circuit open; 429 budget; 422 inactive | no-store | 500/2000 ms |
| GET | `/forecast-collections` | Collection lineage/health query | admin | provider_id, location_id, status, time range, cursor | collections[] (full accounting, payload key + checksum prefix) | 422 | no-store | 50/200 ms |
| POST | `/admin/collections/{id}/replay` | Payload replay (FC-14) | admin | — | new collection | 422 payload_unavailable; 404 | no-store | 500/2000 ms |
| POST | `/admin/rankings/recompute` | Scoped recompute | admin | {provider_id?, location_id?, horizon?, period?} | {batch_id, scope} | 422 | no-store | 200/500 ms (async work follows) |
| PATCH | `/admin/providers/{id}/status` | Enable/disable provider | admin | {status} | updated | 422 | no-store | 100/300 ms |
| PUT | `/admin/provider-configurations/{id}` | Schedule/credential update | admin | {collection_schedule?, credential_ref?} | updated (never echoes credential) | 422 schedule invalid | no-store | 100/300 ms |
| GET | `/admin/users` | User list (S-14) | admin | status, cursor/limit | users[] (no auth_subject) | — | no-store | 50/200 ms |
| PATCH | `/admin/users/{id}/status` | Disable/enable user | admin | {status} | updated | 409 self-disable | no-store | 200/500 ms |
| DELETE | `/admin/users/{id}` | Delete account | admin | — | 202 | 409 self-delete | no-store | 500/2000 ms |
| POST | `/admin/users/{id}/export` | Admin-triggered export | admin | — | {job_id} | 409 active job | no-store | 100/300 ms |
| GET | `/admin/audit-events` | Audit log (S-14) | admin | action, resource_type, cursor/limit | events[] | — | no-store | 50/200 ms |

### 1.5 Operational endpoints

| Method | Route | Purpose | Auth | Response | p50/p95 |
|--------|-------|---------|------|----------|---------|
| GET | `/healthz` | Liveness | public | 200 {} | < 5 ms |
| GET | `/readyz` | Readiness (DB + volume + JWKS) | public | 200/503 with checks | < 100 ms |
| GET | `/metrics` | Prometheus exposition | local-only (bind 127.0.0.1; agent scrapes) | text | < 50 ms |
| GET | `/api/v1/openapi.json` | Generated spec | public | OpenAPI 3.1 | < 50 ms |

**Total: 34 endpoints** (27 documented in reconciliation + health/metrics/openapi/forecasts-detail set). No bulk endpoints (API-12). No endpoint-per-card proliferation.

## 2. Screen ↔ Endpoint Mapping (verified ≤ 2 requests per screen)

| Screen | Primary | Secondary |
|--------|---------|-----------|
| S-01 Overview | GET /rankings | (locations cached from prior nav or 1 GET /locations) |
| S-02 Location Detail | GET /accuracy/summary?location_id | GET /rankings (client ETag cache reuse) |
| S-03 Provider Detail | GET /accuracy/summary?provider_id | GET /providers/{id} |
| S-04 Trends | GET /accuracy | — |
| S-05 FvA | GET /forecast-comparison | — |
| S-06 Methodology | GET /rankings/methodology | — |
| S-07/S-09 Settings | GET/PATCH /me | GET /api-keys |
| S-10 Health | GET /admin/health | POST trigger (action) |
| S-11 Providers | GET /providers | PUT config (action) |
| S-12 Locations | GET /locations | POST/PUT/PATCH (actions) |
| S-13 Schedules | GET /forecast-collections | POST trigger/replay (actions) |
| S-14 Users | GET /admin/users + GET /admin/audit-events | — |

## 3. Cross-Reference

- Full per-endpoint contracts: `docs/api/01-screen-api-contracts.md`
- OpenAPI outline: `docs/api/06-openapi-outline.yaml`
- Auth model: `docs/api/07-authentication-and-authorization.md`
