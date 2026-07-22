# ForecastIQ — API Response Conventions

**Version**: 1.0 (UI ↔ Backend Reconciliation Board)
**Status**: Authoritative — extends `docs/api/00-api-requirements.md` §3, §5, §6; binding for all v1 endpoints
**Companion**: `docs/api/03-error-and-partial-result-contracts.md`

---

## 1. Successful Response Envelope

All successful responses use a consistent envelope:

```json
{
  "data": { },
  "metadata": {
    "request_id": "0d1c…",
    "generated_at": "2026-07-22T10:00:00Z",
    "timezone": "Asia/Kuala_Lumpur",
    "units": {"temperature": "°C", "precipitation": "mm", "wind_speed": "m/s", "pressure": "hPa", "humidity": "%"},
    "methodology_version": "2026.1",
    "weights_version": "w-2026.1"
  },
  "freshness": {"state": "fresh", "last_updated": "…Z", "age_seconds": 720, "threshold_seconds": 4500},
  "provenance": {"observation_provenance_mix": {"reanalysis": 1.0}},
  "attribution": [{"provider": "Open-Meteo", "text": "…", "url": "…"}],
  "partial_result": false,
  "warnings": [],
  "pagination": {"next_cursor": "…", "has_more": true}
}
```

Field applicability (include only where meaningful — never emit null placeholders):

| Field | Applies to | Notes |
|-------|-----------|-------|
| `data` | all | Resource(s) |
| `metadata.request_id` | all | Echoed or generated UUIDv4 (API req §1) |
| `metadata.generated_at` | all | Server computation time |
| `metadata.timezone` | location-scoped payloads | IANA zone used for any bucketing/display context (BR-TZ echo) |
| `metadata.units` | payloads with weather values | Explicit per field family; field names also carry unit suffixes (`_c`, `_mm`, `_ms`, `_hpa`, `_pct`) |
| `metadata.methodology_version` / `weights_version` | derived payloads (rankings, accuracy, comparison) | PC-02, API-08 |
| `freshness` | all time-sensitive payloads | BR-FRESH-02; server-computed; four states only |
| `provenance` | derived payloads + observation-bearing payloads | BR-OBS-04 |
| `attribution` | provider-derived data | BR-ATTR-01; configured per provider, never hardcoded |
| `partial_result` | collection-aggregate payloads | `true` iff `warnings[]` non-empty (redundant by design — cheap client check) |
| `warnings` | multi-provider payloads | Per §3 |
| `pagination` | cursor-paginated lists | `has_more` only; **no total_count** (API-02) |

## 2. Freshness Block (binding shape)

```json
"freshness": {
  "state": "fresh | delayed | stale | unavailable",
  "last_updated": "2026-07-22T09:48:00Z",
  "age_seconds": 720,
  "threshold_seconds": 4500,
  "reason": "circuit_open"            // only when state = unavailable
}
```

- States and thresholds per BR-FRESH table (forecast collections / observations / rankings / operational health).
- `threshold_seconds` = the fresh→delayed boundary for the data type (enables honest UI tooltips).
- Freshness is **always server-computed**; clients must not derive state from `last_updated` alone (clock-skew protection, BR-FRESH-02).
- Every time-sensitive payload carries exactly one top-level freshness block; per-provider freshness differences are expressed via `warnings[]` (code `stale`), not nested blocks.

## 3. Partial-Result Convention (ratifies API req §6)

- Transport: **HTTP 200** for partial success. Decision rationale: HTTP 206 has poor browser/client/cache support and ambiguous semantics for JSON APIs; 207 (WebDAV) is unfamiliar and mishandled by intermediaries. A 200 with explicit `warnings[]` + `partial_result: true` works uniformly with browsers, CDNs, ETags, and standard API clients.
- `warnings[]` entry shape:
```json
{"provider_id": "…", "code": "provider_unavailable | stale", "message": "OpenWeather data temporarily unavailable", "since": "2026-07-22T08:00:00Z"}
```
- Affected providers are **omitted from data arrays and present in `warnings[]`** (fixed choice per API req §6 — no per-row error mixing).
- Retry guidance: `warnings[].code = provider_unavailable` → client may refetch later (no Retry-After; next collection cycle is the natural recovery); `stale` → data shown with staleness label.
- A response with all providers failed is **not** a partial result — it degrades to either (a) stale-cache serving with explicit staleness (NFR-A07) or (b) 503 with the standard error envelope if no cacheable prior state exists.

## 4. Provenance Block (binding shape)

```json
"provenance": {
  "observation_provenance_mix": {"reanalysis": 0.9, "interpolated": 0.1},
  "observation_sources": ["openmeteo_historical"],
  "quality_weighting_applied": true
}
```

Per-entity provenance fields ride with the entity (per data-lineage doc §3): snapshots carry `provider_id` + `forecast_collection_id` + `issued_at`; observations carry `source`, `observation_type`, `quality_flag`; metrics carry `sample_count`, `methodology_version`, `period_start/end`, `ci_lower/upper`; rankings carry the full component set.

Provenance display tiers (board decision):

| Tier | Content | Where |
|------|---------|-------|
| Shown directly | observation_type badge, sample count, freshness state, methodology version | Data views, context lines, footers |
| Tooltip | CI values, coverage %, reliability %, quality weighting note | Hover/focus on metrics |
| Details view | component breakdown, evaluation period, weights, tie rule | S-01 breakdown panel, S-02 tables, S-06 |
| API metadata / exports only | forecast_collection_id, adapter_version, schema_version, checksums, observation weights | Raw endpoints, CSV headers, OpenAPI |

## 5. CSV Export Convention (ratifies DR-05; PC-09)

Client-generated from the current view. Binding file structure:

```
# ForecastIQ Export
# Generated: 2026-07-22T10:00:00Z
# Screen: Trends (S-04)
# Methodology: 2026.1
# Weights: w-2026.1
# Period: 2026-06-22 to 2026-07-21
# Location: Johor Bahru (1.4927, 103.7414)
# Horizon: +24h (1440 minutes)
# Variable: temperature
# Observation provenance: reanalysis 100%
# Attribution: Open-Meteo — https://open-meteo.com; OpenWeather — https://openweathermap.org
# Disclaimer: ForecastIQ measures forecast accuracy. We don't deliver weather forecasts.

provider,period_start,sample_count,mae_c,ci_lower,ci_upper
Open-Meteo,2026-06-22,24,1.22,1.13,1.31
…
```

Rules:
- `#`-prefixed comment block first; blank line; column headers; data rows.
- Column names snake_case with unit suffixes matching API field names.
- Nulls as empty cells (never "0", never "null").
- Timestamps ISO 8601 UTC.
- Row bounds: trends ≤ 365 rows; FvA ≤ 24 × providers; rankings = provider count; location detail = providers × metrics. All bounded and predictable → synchronous client generation approved for MVP (no server export infrastructure).
- Exports reflect **current filters** (specified behaviour, doc 00 §3).
- Server-side async export remains only for GDPR (`POST /me/export`, AUTH-09): job → download link valid 24h → deleted; failure → job status `failed` + retry allowed.

## 6. Cache Headers Convention

| Payload class | Cache-Control | ETag |
|---------------|---------------|------|
| Rankings / accuracy summaries / methodology | `public, max-age=60` | strong, content-based |
| Forecast-comparison (past dates) | `public, max-age=300` | strong (immutable inputs; corrections change content) |
| Forecast-comparison (today) | `public, max-age=60` | strong |
| Locations / providers | `public, max-age=300` | strong |
| Admin endpoints | `no-store` | none |
| Mutations | `no-store` | none |

Conditional GET (`If-None-Match` → 304) on all cacheable GETs (API-05). The 60s polling on `/admin/health` uses `no-store` + cheap assembly (ETag optional; assembly < 200ms makes 304 optimization unnecessary — implementation choice).

## 7. Rounding (binding, methodology §5)

| Kind | API rounding |
|------|--------------|
| Ratios and scores | 4 dp |
| Temperature °C, rain mm | 2 dp |
| Wind m/s | 1 dp |
| Pressure hPa | 2 dp |

Storage retains full precision; rounding is presentation-layer (API). UI applies its own display formatting per doc 02 §1.8 (3dp composite, percentages, etc.) on top of API values.
