# ForecastIQ — Screen ↔ API Contracts

**Version**: 1.0 (UI ↔ Backend Reconciliation Board)
**Status**: Authoritative — amends `docs/api/00-api-requirements.md` where marked; all changes additive within v1 governance (API-10)
**Companions**: `docs/api/02-response-conventions.md` (envelopes), `docs/api/03-error-and-partial-result-contracts.md` (errors/partials)

API composition strategy decision (board mandate): **reusable domain endpoints** as the backbone (rankings, accuracy, locations, providers), with **two purpose-built screen endpoints** justified below (`/forecast-comparison`, extended `/accuracy/summary`), and **no BFF aggregation layer** (a BFF would couple backend releases to dashboard layout, defeat per-endpoint ETag caching, and add a code layer a 1–2 engineer team should not maintain). The dashboard composes at most 2 requests per screen load — within the "no excessive orchestration" bound.

Amendments to `docs/api/00-api-requirements.md` are marked **[AMENDS]** and must be folded into that document at the next revision.

---

## 1. `GET /rankings` — S-01 Overview (primary)

| Attribute | Value |
|-----------|-------|
| Purpose | Composite provider rankings for one location + horizon with full transparency |
| Auth | Public (AUTH-08) |
| Permissions | None |
| Parameters | `location_id` (required, UUID), `horizon_minutes` (optional; default: all via `horizon_profile=uniform`) or `horizon_profile` (uniform\|short_term\|daily_planning), `period_days` (default 30; 90 auto for +3d/+7d per BR-RANK-03), `min_sample_count` (default 30, echoed), `weights` (optional custom JSON; echoed as `weights_version: custom:<hash>`) |
| Filters | — |
| Pagination | None (rows = provider count, bounded ≤ ~10) |
| Response shape | Per API req §5 canonical example, **plus** the additions below |
| Error behaviour | 422 invalid location_id/horizon; standard envelope |
| Partial-data behaviour | `warnings[]` per API req §6; affected providers omitted from `rankings[]`, present in warnings |
| Freshness fields | `freshness{state, last_updated, age_seconds, threshold_seconds}` (rankings thresholds, BR-FRESH) |
| Provenance fields | `methodology_version`, `weights_version`, `evaluation_period`, `observation_provenance_mix`, `attribution[]` |
| Methodology fields | `min_sample_count` echoed; thresholds; `coverage_penalty_applied` per row |
| Request ID | `X-Request-Id` standard |
| Cache | `ETag` + in-process LRU (TTL 60s); `Cache-Control: public, max-age=60` |
| Rate-limit headers | Standard (per-IP bucket for public) |
| Latency target | p50 < 50ms, p95 < 200ms (NFR-P01/P02) — single indexed scan over pre-computed rows |
| Max response size | < 16 KB (10 providers × full components) |

**[AMENDS §5]** — additive block in the response:

```json
"observation_context": {
  "temperature_c": 31.4,
  "precipitation_mm": 0.0,
  "observed_at": "2026-07-21T10:05:00Z",
  "source": "openmeteo_historical",
  "observation_type": "reanalysis",
  "freshness": {"state": "fresh", "last_updated": "…", "age_seconds": 720, "threshold_seconds": 5400}
}
```

Rationale (C-01/C-04/DR-09/DR-11): serves the S-01 observation context line without a third request; single indexed query (`observations (location_id, observed_at DESC)` LIMIT 1); null when no observations exist (drives the "Ground truth unavailable" context state). This is an observation **record** (provenance-labeled evidence), not a weather product — NP-01 preserved.

## 2. `GET /locations` — S-01 selector, S-12 admin

| Attribute | Value |
|-----------|-------|
| Auth | Public (list/read); POST/PUT/PATCH admin |
| Parameters | `active` (bool filter), `bbox` (reserved, MVP ignores with documented note), `cursor`/`limit` |
| Response | `locations[{id, name, country_code, latitude, longitude, timezone, status, created_at}]` |
| Cache | ETag; changes rare |
| Latency | p95 < 50ms (≤ 10 rows) |

## 3. `GET /accuracy/summary` — S-02 Location Detail, S-03 Provider Detail grid

| Attribute | Value |
|-----------|-------|
| Purpose | All metrics per provider for a location+horizon (S-02), or all location×horizon ranking cells for a provider (S-03) |
| Auth | Public |
| Parameters | `location_id` OR `provider_id` (exactly one required — **[AMENDS §4.3]**: adds `provider_id` mode per C-08/DR-15), `horizon_minutes` (optional with location_id; ignored with provider_id), `period_days` (default 30) |
| Response (location mode) | `providers[{provider, ranking_status, metrics[{variable, metric_type, value, ci_lower, ci_upper, sample_count}], collection_window{first_snapshot_at, last_snapshot_at, coverage, reliability}}]`, provenance block, freshness, warnings[] |
| Response (provider mode) | `cells[{location_id, location_name, horizon_minutes, composite_score, ranking_status, sample_count, coverage}]`, `collection_window` per location, provenance, freshness, warnings[] |
| Partial behaviour | warnings[] per provider/location |
| Cache | ETag + LRU 60s |
| Latency | p95 < 200ms; provider mode = one scan over `provider_rankings (provider_id)` (indexed) — **no N+1** |
| Max response size | < 40 KB (provider mode: 10 locations × 7 horizons × full cells) |

**[AMENDS §4.3]** — `collection_window` block (C-08): MIN/MAX derived from `forecast_snapshots` per provider-location (indexed `(provider_id, location_id, target_time)`); coverage/reliability from ranking components. Replaces the screen inventory's reference to a "public `/forecast-collections` subset" — that endpoint remains admin-only.

**[AMENDS §4.1]** — `GET /providers/{id}` public payload gains `adapter_version` (from latest successful collection; non-sensitive lineage exposure per S-03 header need) and `collecting_since` (min collection requested_at).

## 4. `GET /accuracy` — S-04 Trends, S-03 per-horizon detail

| Attribute | Value |
|-----------|-------|
| Auth | Public |
| Parameters | `provider_id`, `location_id`, `horizon_minutes`, `variable`, `metric_type`, `period_start`/`period_end` (max 365d), `aggregation` (daily\|weekly\|monthly), `tz` (IANA; bucketing zone, echoed per BR-TZ-05), `cursor`/`limit` |
| Response | `series[{provider_id, buckets[{period_start, period_end, value, ci_lower, ci_upper, sample_count}]}]`, `tz` echoed, methodology_version, freshness, warnings[] |
| Buckets | SQL `date_trunc` over metric `period_start` in requested tz; every bucket carries sample_count (hollow-point support) |
| Cache | ETag + LRU 60s |
| Latency | p95 < 200ms (indexed scan; ≤ 365 buckets × providers) |
| Max response size | < 80 KB worst case (daily 365d × 2 providers); typical 30d < 10 KB |

## 5. `GET /forecast-comparison` — S-05 Forecast vs. Actual **[NEW, C-19]**

| Attribute | Value |
|-----------|-------|
| Purpose | Bounded public payload for the FvA screen: one day, one variable, selected providers |
| Auth | **Public** (portfolio demonstrability; raw `/forecasts`+`/observations` remain user+ per AUTH-08) |
| Permissions | None; per-IP rate limiting (public bucket) |
| Parameters | `location_id` (required), `date` (required, ISO date interpreted in location tz), `variable` (required), `providers` (optional CSV UUIDs; default all active), `horizon_minutes` (required; selects issuance per DR-02) |
| Response shape | See below |
| Error behaviour | 422 invalid params; 404 unknown location |
| Partial behaviour | warnings[] per provider (absent provider → line absent + warning) |
| Freshness | per-series collection freshness + observation freshness |
| Provenance | per-observation `source`/`observation_type`/`quality_flag`; per-series `issued_at`, `forecast_collection_id`; attribution[]; methodology_version for day_metrics |
| Request ID | Standard |
| Cache | `ETag`; past dates effectively immutable (new observations/corrections change ETag); today's date `Cache-Control: max-age=60` |
| Latency | p95 < 200ms — two indexed queries (snapshots by `(location_id, target_time, horizon)`; observations by `(location_id, observed_at)`) + day-metric computation over ≤ 24×providers pairs |
| Max response size | < 20 KB (24h × 3 series + metrics) |

```json
{
  "data": {
    "location": {"id": "…", "name": "…", "timezone": "Asia/Kuala_Lumpur"},
    "date": "2026-07-21", "variable": "temperature", "horizon_minutes": 1440,
    "series": [
      {"provider": {"id": "…", "name": "Open-Meteo"},
       "issued_at": "2026-07-20T10:00:00Z",
       "points": [{"target_time": "…Z", "value": 31.2}]}
    ],
    "observations": [
      {"observed_at": "…Z", "value": 31.4, "source": "openmeteo_historical",
       "observation_type": "reanalysis", "quality_flag": "valid"}
    ],
    "day_metrics": [
      {"provider_id": "…", "mae": 1.24, "bias": 0.31, "rmse": 1.62, "sample_count": 24,
       "methodology_version": "2026.1"}
    ],
    "error_band_mae": 1.38,
    "observations_available": true,
    "provenance": {"observation_provenance_mix": {"reanalysis": 1.0}},
    "attribution": [{"provider": "…", "text": "…", "url": "…"}],
    "freshness": {"state": "fresh", "last_updated": "…"}
  },
  "warnings": []
}
```

Issuance selection (DR-02 binding): for each provider, the snapshot set whose `forecast_horizon_minutes` equals the requested horizon; if a provider has no issuance at exactly that horizon for a target hour, the nearest shorter horizon is used and `issued_at` reflects the actual issuance (subtitle honesty). Gaps render as line breaks.

Observation gaps: hours with no observation are **absent** from `observations[]` (never interpolated) — the UI renders breaks (PC-10, methodology integrity).

## 6. `GET /rankings/methodology` — S-06

| Attribute | Value |
|-----------|-------|
| Auth | Public |
| Response | `methodology_version`, `weights_version`, `formulas[{metric_type, formula, plain_language, direction, zero_denominator_behaviour, anchor}]`, `default_weights{}`, `thresholds{ranked: 30, provisional: 10}`, `coverage_penalty{}`, `statuses{}`, `tie_rule`, `change_history[]` |
| Cache | ETag; long max-age (changes on version bump only) |
| Latency | < 50ms (static config) |

## 7. `GET /me`, `PATCH /me` — S-07 onboarding, S-09 Settings

| Attribute | Value |
|-----------|-------|
| Auth | Bearer (self) |
| GET response | `{auth_subject, email, role, status, workspace, default_location_id, preferences{tz_display}, created_at, last_login_at}` |
| PATCH body | `{default_location_id?, preferences?}` — **[AMENDS §4.4]**: adds `default_location_id` + `preferences` (DR-04 support; onboarding default-location persistence) |
| Validation | default_location_id must reference an active location (422 otherwise) |
| Idempotency | PATCH natural |
| Audit | yes (profile changes) |
| UI behaviour | Optimistic (only approved optimistic mutation, state contract §8) |

## 8. Admin user management — S-14 **[NEW, C-09; AMENDS §4.5]**

| Method | Path | Purpose | Validation | Guard | Audit |
|--------|------|---------|------------|-------|-------|
| GET | `/admin/users` | List users (`?status=&cursor=&limit=`) | — | admin | no |
| PATCH | `/admin/users/{id}/status` | Disable/enable (`{status}`) | enum | 409 on self-disable | yes |
| DELETE | `/admin/users/{id}` | Delete account (AUTH-09 scope) | — | 409 on self-delete | yes (pre-deletion) |
| POST | `/admin/users/{id}/export` | Admin-triggered GDPR export | one active job → 409 | — | yes |

All four: bearer + admin role enforced server-side; disable/delete propagate to Supabase Auth via server-side admin API (service-role key backend-only, never in browser); responses exclude `auth_subject` except in audit context; cursor pagination on list. Latency: list < 200ms; disable/delete < 500ms (external call).

## 9. `GET /admin/health` — S-10 **[AMENDS §4.5, C-11/C-12]**

| Attribute | Value |
|-----------|-------|
| Auth | Bearer + admin |
| Parameters | `provider_id`, `location_id`, `status` (filters) |
| Response additions | Per cell: `next_scheduled_at` (C-12). New sections: `observation_collector{per-location last success, suspect_count_24h, locations_covered}`, `system{payload_volume{used_bytes,total_bytes,used_pct}, engine_lag_seconds, last_backup{completed_at,status}, last_restore_test{completed_at,status}}` |
| Sources | DB aggregates (collections, observations, metrics) + `statfs` on payload volume + backup status file (written by backup scripts per operations doc) — **no log/metrics-system queries** |
| Cache | ETag for 60s polling (DR-03); must remain < 200ms p95 under 1–2 operator polling |
| Audit | no (read) |

## 10. `POST /admin/collections/trigger` — S-10/S-13 **[NEW, C-10; AMENDS §4.5]**

| Attribute | Value |
|-----------|-------|
| Purpose | Immediate out-of-band collection for one provider-location (recovery action for failed collections — distinct from replay of stored payloads) |
| Auth | Bearer + admin |
| Body | `{provider_id, location_id}` |
| Validation | provider + location active (422); **409 while circuit open** (`detail: "Circuit open — retry available after half-open probe at {time}"`); 429 if provider rate-limit budget exhausted (`Retry-After` + budget reset time) |
| Idempotency | `Idempotency-Key` supported; snapshot dedup makes re-execution harmless regardless |
| Resulting state | New ForecastCollection row (normal status taxonomy); scheduler slot unaffected |
| Audit | yes (`collection.triggered`) |
| UI behaviour | Pessimistic; button disabled with explanatory tooltip while circuit open |
| Latency | < 2s (synchronous provider call with timeout); 202 alternative if Phase 1 prefers async (implementation choice; contract fields identical) |

## 11. Endpoint ↔ Screen Summary

| Endpoint | Screens | Auth | Status |
|----------|---------|------|--------|
| `GET /rankings` (+ observation_context) | S-01, S-02 | public | exists, amended |
| `GET /locations` (+CRUD) | S-01, S-12 | public/admin | exists |
| `GET /accuracy/summary` (location + provider modes, collection_window) | S-02, S-03 | public | exists, amended |
| `GET /accuracy` | S-03, S-04 | public | exists |
| `GET /forecast-comparison` | S-05 | public | **new** |
| `GET /rankings/methodology` | S-06 | public | exists |
| `GET/PATCH /me` | S-07, S-09 | user | exists, amended |
| `/api-keys` CRUD, `/me/export`, `DELETE /me` | S-09 | user | exists |
| `GET /admin/health` (extended) | S-10 | admin | exists, amended |
| `POST /admin/collections/trigger` | S-10, S-13 | admin | **new** |
| `/admin/providers/{id}/status`, `/admin/provider-configurations/{id}` | S-11 | admin | exists |
| `GET /forecast-collections`, replay, recompute | S-13 | admin | exists |
| `GET /admin/users` (+ status/delete/export) | S-14 | admin | **new** |
| `GET /admin/audit-events` | S-14 | admin | exists |
| `GET /providers` (+ adapter_version, collecting_since) | S-03, S-11 | public | exists, amended |

**No endpoint-per-component proliferation**: 3 new endpoints + 5 additive amendments total. No BFF. No bulk endpoints (API-12 preserved).
