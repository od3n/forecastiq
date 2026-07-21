# ForecastIQ — API Requirements

**Version**: 1.0 (Phase 0 Amendment)
**Status**: Authoritative
**Resolves**: API corrections amendment (idempotency, correlation IDs, conditional GET,
pagination, rate-limit headers, CORS, error envelope, provenance fields, methodology
and score versions, freshness, partial results)

---

## 1. General Conventions

| Convention | Requirement |
|------------|-------------|
| Base URL | `https://api.forecastiq.example/api/v1` (URL-path versioning) |
| Format | JSON UTF-8; timestamps ISO 8601 UTC (`Z`) |
| Spec | OpenAPI 3.1 generated from code; served at `/api/v1/openapi.json`; contract check in CI |
| Request ID | `X-Request-Id` honored if supplied (validated format), else generated (UUIDv4); echoed in response and logs |
| Idempotency | `Idempotency-Key` header on all mutable POSTs; stored 24 h; replays return the original response (same status + body); key scoped per authenticated principal |
| Conditional GET | `ETag` on rankings, accuracy summaries, providers, locations, and single-resource GETs; `If-None-Match` → 304 |
| Pagination | Cursor-based: `?limit=1..200&cursor=…`; response `pagination: {next_cursor, has_more}` — **no total_count** (a separate `GET …/count` may be added post-MVP only with a measured need) |
| Sorting | `sort=field` / `sort=-field` on whitelisted fields |
| Rate limits | Per API key (default 60 req/min; per-key configurable); headers `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`; 429 + `Retry-After` |
| CORS | Allowlist: production dashboard origin + `http://localhost:*` in dev; `Access-Control-Allow-Headers` includes `Authorization`, `X-API-Key`, `Idempotency-Key`, `X-Request-Id`; preflight cached 1 h |
| Auth | `Authorization: Bearer <supabase-jwt>` for user sessions; `X-API-Key` for programmatic access; public read on rankings/accuracy/providers/locations (AUTH-08) |
| Deprecation | `Sunset` + `Deprecation` headers on deprecated endpoints; ≥ 6-month window; changelog in OpenAPI description |
| Bulk endpoints | **Not in MVP.** Recorded as post-MVP; no MVP user journey requires them (amendment mandate). |

## 2. Standard Error Envelope (RFC 7807 + extensions)

```json
{
  "type": "https://forecastiq.example/errors/validation",
  "title": "Validation Error",
  "status": 422,
  "detail": "Field 'latitude' must be between -90 and 90",
  "instance": "/api/v1/locations",
  "request_id": "0d1c…",
  "errors": [{"field": "latitude", "message": "must be between -90 and 90"}]
}
```

Error type registry (stable URIs): `validation`, `unauthorized`, `forbidden`,
`not_found`, `conflict`, `duplicate` (idempotency collision), `rate_limited`,
`provider_unavailable` (partial-result context), `internal`.

## 3. Provenance & Freshness Metadata (binding on derived payloads)

Every ranking/accuracy payload includes:

```json
{
  "methodology_version": "2026.1",
  "weights_version": "w-2026.1",
  "evaluation_period": {"start": "…", "end": "…"},
  "sample_count": 720,
  "coverage": 0.98,
  "reliability": 0.99,
  "observation_provenance_mix": {"reanalysis": 0.9, "interpolated": 0.1},
  "freshness": {"state": "fresh", "last_updated": "…"},
  "attribution": [{"provider": "Open-Meteo", "text": "…", "url": "…"}]
}
```

## 4. Endpoint Inventory (MVP)

### 4.1 Catalog

| Method | Path | Auth | Notes |
|--------|------|------|-------|
| GET | `/providers` | public | list with status, attribution |
| GET | `/providers/{id}` | public | ETag |
| GET | `/locations` | public | filters: `bbox`, `active` |
| POST | `/locations` | admin | Idempotency-Key; dedup check BR-LOC-01 → 409 `duplicate` with existing location reference |
| GET | `/locations/{id}` | public | ETag |
| PUT | `/locations/{id}` | admin | mutable fields only (name, timezone, status) |
| PATCH | `/locations/{id}/status` | admin | enable/disable (soft) |

### 4.2 Data

| Method | Path | Auth | Notes |
|--------|------|------|-------|
| GET | `/forecasts` | user+ | filters: provider_id, location_id, issued_after/before, horizon_minutes, target_after/before; includes `forecast_collection_id` lineage |
| GET | `/forecasts/{id}` | user+ | includes collection provenance block |
| GET | `/forecast-collections` | admin | health/lineage queries: status, provider, location, time range |
| GET | `/observations` | user+ | filters: location_id, source, observation_type, observed_after/before, quality_flag |
| GET | `/observations/{id}` | user+ | provenance + supersession links |

### 4.3 Analysis

| Method | Path | Auth | Notes |
|--------|------|------|-------|
| GET | `/accuracy` | public | filters: provider_id, location_id, horizon_minutes, variable, metric_type, period_start/end, aggregation (daily/weekly/monthly); every row: value, sample_count, ci_lower/upper, methodology_version |
| GET | `/accuracy/summary` | public | per (provider, location, horizon): all metrics in one payload; ETag |
| GET | `/rankings` | public | filters: location_id (required), horizon_minutes or horizon_profile, period, min_sample_count (default 30), weights (optional custom → echoed); response per §5 |
| GET | `/rankings/methodology` | public | machine-readable methodology: formulas registry, default weights, thresholds, version |

### 4.4 Auth & Keys

| Method | Path | Notes |
|--------|------|-------|
| — | Session lifecycle (register/login/refresh/reset/logout) handled by **Supabase Auth endpoints**; the app does not reimplement them. Dashboard integrates the managed flow; backend verifies JWTs. |
| POST | `/api-keys` | Idempotency-Key; key shown once; scopes + rate limit |
| GET | `/api-keys` | caller's keys (prefix, scopes, status, created, last_used) |
| DELETE | `/api-keys/{id}` | revoke (immediate) |
| GET | `/me` | profile: subject, email, role, workspace |
| POST | `/me/export` | GDPR export job → download link (AUTH-09) |
| DELETE | `/me` | account deletion request |

### 4.5 Operations

| Method | Path | Auth | Notes |
|--------|------|------|-------|
| GET | `/healthz`, `/readyz` | public | liveness/readiness |
| GET | `/admin/health` | admin | per provider-location: last success, circuit state, error counts, freshness |
| POST | `/admin/collections/{id}/replay` | admin | idempotent replay (FC-14) |
| POST | `/admin/rankings/recompute` | admin | body: scope filters |
| GET | `/admin/audit-events` | admin | cursor pagination |
| PATCH | `/admin/providers/{id}/status` | admin | enable/disable |
| PUT | `/admin/provider-configurations/{id}` | admin | schedule + credential_ref |

## 5. Ranking Response Shape (canonical example)

```json
{
  "data": {
    "location_id": "…", "horizon_minutes": 1440, "horizon_profile": "uniform",
    "evaluation_period": {"start": "…", "end": "…"},
    "methodology_version": "2026.1", "weights_version": "w-2026.1",
    "min_sample_count": 30,
    "rankings": [
      {
        "rank": 1,
        "provider": {"id": "…", "name": "Open-Meteo", "attribution": {"text": "…", "url": "…"}},
        "ranking_status": "ranked",
        "composite_score": 0.940, "ci_lower": 0.91, "ci_upper": 0.96,
        "sample_count": 720, "coverage": 0.98, "reliability": 0.99,
        "components": {
          "temp_mae":        {"value": 1.20, "normalized": 0.917, "weight": 0.30, "sample_count": 720, "ci": [1.11, 1.29]},
          "precip_f1":       {"value": 0.769, "normalized": 1.000, "weight": 0.25, "sample_count": 720},
          "rain_mae_all":    {"value": 0.90, "normalized": 0.944, "weight": 0.15, "sample_count": 720},
          "wind_mae":        {"value": 1.10, "normalized": 1.000, "weight": 0.15, "sample_count": 720},
          "temp_abs_bias":   {"value": 0.30, "normalized": 0.833, "weight": 0.05, "sample_count": 720},
          "coverage":        {"value": 0.98, "weight": 0.05},
          "reliability":     {"value": 0.99, "weight": 0.05}
        },
        "coverage_penalty_applied": false,
        "significant_vs_next": true
      },
      {
        "rank": null,
        "provider": {"id": "…", "name": "ProviderX"},
        "ranking_status": "unranked",
        "reason": "coverage 0.45 below 0.5 threshold",
        "sample_count": 380, "coverage": 0.45
      }
    ],
    "provenance": {"observation_provenance_mix": {"reanalysis": 1.0}},
    "freshness": {"state": "fresh", "last_updated": "…"}
  }
}
```

## 6. Partial-Result Representation

When some providers in a requested set are unavailable/stale but others are servable:

- HTTP 200 with the available providers included;
- top-level `warnings: [{provider_id, code: "provider_unavailable"|"stale", message}]`;
- affected provider entries carry `ranking_status: "unranked"` with the reason, or are
  omitted from the list but present in `warnings` (choice fixed: present in warnings).

This satisfies "partial-result representation" without inventing a 207-style mechanism.

## 7. API Change Governance

1. OpenAPI diff check in CI: breaking changes to v1 fail the build.
2. Additive changes (new optional fields/endpoints) allowed within v1.
3. Deprecation: `Deprecation: true` + `Sunset: <date ≥ now+6 months>` headers +
   changelog entry; monitored via usage metrics per endpoint.
4. Response schema versioning within v1: `schema_version` field on envelope if a
   payload shape evolves additively and clients must detect (rare; default: none).
