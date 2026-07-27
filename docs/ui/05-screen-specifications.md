# ForecastIQ — Reconciled Screen Specifications

**Version**: 1.0 (UI ↔ Backend Reconciliation Board)
**Status**: Authoritative — binding for Phase 1 Architecture and implementation
**Supersedes**: conflicting elements of `docs/ui/03-operational-dashboard-design.md`; amends `docs/ui/02-ui-design-specification.md` where noted
**Companion documents**: `docs/api/01-screen-api-contracts.md` (endpoint detail), `docs/ui/06-ui-state-contracts.md` (state contracts), `docs/ui/08-ui-backend-traceability.md` (matrices)

Screen IDs follow the authoritative inventory (`docs/ui/00-screen-inventory.md`). Each screen is reconciled against domain, API, authorization, testing, and operational requirements. Decorative elements are omitted per the board mandate.

---

## S-01 Overview

### Screen identity

| Field | Value |
|-------|-------|
| Route | `/` |
| MVP status | Required (core value front door) |
| Primary persona | Daniel (weather-curious individual); Aisyah (quick health check via freshness only) |
| Primary user goal | Know which forecast provider is most accurate for my location and horizon |
| Primary decision supported | Which provider to trust for the selected location + time horizon |
| Primary actions | Select location/horizon → read ranking → expand breakdown → drill to provider/location detail |
| Required permissions | Public (no auth) |

### UI elements → data requirements

| UI element | Data fields | Source entity | Direct/derived | Aggregation | Freshness threshold | Provenance shown | Authorization |
|------------|-------------|---------------|----------------|-------------|---------------------|------------------|---------------|
| Location context bar | name, country_code, timezone | Location | direct | — | n/a (mutable config) | — | public |
| Observation context line (C-01/C-04) | temperature_c, precipitation_mm, observed_at, source, observation_type | Observation (latest per location) | direct | latest single record | observations: fresh <90m / delayed 90–240m / stale >240m (BR-FRESH) | source + observation_type badge (BR-OBS-01) | public |
| Quick stats (ranked/provisional/unranked counts) | derived client-side from `rankings[].ranking_status` | ProviderRanking | derived (count) | per location+horizon | inherits rankings freshness | — | public |
| Ranking table: rank | `rank` (null → "—") | ProviderRanking | derived (ordering + tie groups per BR-RANK-05) | per cell | rankings: fresh <2h / delayed 2–6h / stale >6h | — | public |
| Ranking table: composite score | composite_score (3dp), ci_lower/upper | ProviderRanking | derived (methodology §6) | per cell | as above | methodology_version, weights_version | public |
| Ranking table: status badge | ranking_status + reason | ProviderRanking | derived (methodology §7) | per cell | as above | — | public |
| Ranking table: samples | sample_count | ProviderRanking | direct | per cell | as above | — | public |
| Ranking table: coverage | coverage (0dp %) | ProviderRanking | derived (methodology §4.4) | per cell | as above | — | public |
| Breakdown panel (expanded) | components{}: value, normalized, weight, sample_count, ci per component; coverage_penalty_applied | ProviderRanking.component_scores + AccuracyMetric | derived | per component | as above | observation_provenance_mix | public |
| Tie annotation | `significant_vs_next` | ProviderRanking pair comparison | derived (CI overlap, BR-RANK-05) | per adjacent pair | as above | — | public |
| Methodology footer | methodology_version, weights_version | MethodologyVersion (config) | direct | — | n/a | version link → S-06 | public |
| Attribution footer | attribution[]{provider, text, url} | Provider | direct | — | n/a | BR-ATTR-01 | public |
| Freshness indicator | freshness{state, last_updated} | server-computed | derived | per payload | BR-FRESH | — | public |

**Units/timezone**: scores dimensionless (3dp); observation context line °C / mm with unit labels; timestamps location-tz default (BR-TZ-02) with zone label + relative time.

**Expected cardinality**: ranking rows = provider count (MVP 2, max ~4); breakdown 7 components; response < 8 KB.

### Backend capability

| Capability | Specification |
|------------|---------------|
| `GET /rankings?location_id=&horizon_minutes=` | Cached projection (ProviderRanking table rows, batch-computed every 30 min per CE-09); in-process LRU + ETag |
| `GET /locations?active=true` | DB read (catalog module); ETag |
| Observation context line | Served **within the rankings response** as `observation_context{temperature_c, precipitation_mm, observed_at, source, observation_type, freshness}` — one indexed query (`observations (location_id, observed_at DESC)` limit 1). Avoids a third request. |

Composition strategy decision (board mandate): **reusable domain endpoints** (rankings + locations), with the observation context embedded in the rankings payload. Rationale: rankings payload is ETag-cacheable and already carries freshness/provenance; a BFF aggregate endpoint would couple backend releases to dashboard layout and defeat per-endpoint caching. Two requests total on load (rankings, locations) — within the "no excessive orchestration" bound.

### Behavioural states

Per `docs/ui/06-ui-state-contracts.md` — this screen implements: loading (skeleton 3 stat cards + 2 rows), empty-no-locations, empty-no-data ("Collecting since {date}"), insufficient-data (per-row n/30), partial (warnings[] → per-provider badge), stale (banner + per-ranking badge), full error (retry + cached dim), observation-unavailable (context line shows "Ground truth unavailable" note; rankings unaffected if metrics exist from prior observations).

### Actions and mutations

None (read-only screen). Export CSV (client-side, DR-05 format) is the only action — see `docs/api/02-response-conventions.md` §5.

---

## S-02 Location Detail

### Screen identity

| Field | Value |
|-------|-------|
| Route | `/locations/:id` |
| MVP status | Required |
| Primary persona | Daniel (deep comparison), Mei (data exploration) |
| Primary user goal | Which provider is best for which variable at this location? |
| Primary decision supported | Per-variable provider selection for this location |
| Primary actions | Select horizon → read per-variable tables → export → drill to provider/trends |
| Required permissions | Public |

### UI elements → data requirements

| UI element | Data fields | Source entity | Direct/derived | Notes |
|------------|-------------|---------------|----------------|-------|
| Location header | name, country, lat/lon, timezone, status, created_at | Location | direct | "Collecting since" = created_at |
| Observation source badge | source, observation_type mix | Observation (distinct sources in period) | derived (mix over evaluation period) | BR-OBS-01 |
| Ranking summary (compact) | rank, provider, composite per provider | ProviderRanking | derived | Link → S-01 |
| Per-variable metric tables (temp, precip occurrence, precip amount, wind, humidity, pressure) | metric_type, value, ci_lower/upper, sample_count per provider | AccuracyMetric | derived (methodology §4) | Null → "—" + tooltip (never 0); occurrence_agreement always with imbalance warning; best value bold |
| Collection window per provider | first_snapshot_at, last_snapshot_at, coverage | ForecastSnapshot MIN/MAX + AccuracyMetric(coverage) | derived | **New fields** in `/accuracy/summary` (C-08) |
| Export CSV button | current view data | client-side assembly | — | DR-05 metadata headers; disabled with tooltip when no data |

**Evaluation window**: trailing 30 days default (90d for +3d/+7d horizons per BR-RANK-03); horizon from URL param; period adjustable (7/30/90d).
**Units**: °C, mm, m/s, %, hPa per methodology §5 rounding (2dp errors; 1dp ratios as %).
**Expected response size**: `/accuracy/summary` for one location ≈ providers × variables × metrics ≈ 2×6×8 rows ≈ < 20 KB.

### Backend capability

| Capability | Specification |
|------------|---------------|
| `GET /accuracy/summary?location_id=&horizon_minutes=&period_days=` | All metrics per provider in one payload + `collection_window` (C-08) + provenance block; ETag; batch-computed AccuracyMetric rows (not on-demand) |
| `GET /rankings?location_id=` | Compact ranking summary (same payload as S-01) |
| Observation provenance | `observation_provenance_mix` within `/accuracy/summary` (API-08) — no separate observations request needed |

Metric computation strategy decision (board mandate): **stored immutable evaluation results** (AccuracyMetric rows, batch-computed every 30 min). Not on-demand calculation (p95 200ms budget), not materialized views (no TimescaleDB per ADR-004), not cached projections over live queries. This is the simplest strategy meeting NFR-P02: reads are single-table indexed scans over pre-computed rows.

### Behavioural states

Loading, empty-location-just-added ("first metrics appear after ≥7 days"), insufficient-data banner, observation-unavailable ("Ground truth unavailable — metrics not computed" + provenance note), partial (per-provider badge on affected rows), stale, error. All per `docs/ui/06-ui-state-contracts.md`.

### Actions and mutations

Export CSV only (client-side). No server mutations.

---

## S-03 Provider Detail

### Screen identity

| Field | Value |
|-------|-------|
| Route | `/providers/:id` |
| MVP status | Required |
| Primary persona | Daniel (comparing providers), Aisyah (provider health overview) |
| Primary user goal | Is this provider reliable and accurate across locations and horizons? |
| Primary decision supported | Provider trust assessment beyond a single location |
| Primary actions | Scan cross-location grid → select location → read per-horizon detail |
| Required permissions | Public |

### UI elements → data requirements

| UI element | Data fields | Source entity | Direct/derived | Notes |
|------------|-------------|---------------|----------------|-------|
| Provider header | name, status, attribution, adapter_version (via latest collection), collecting-since | Provider + ForecastCollection(min requested_at) | direct + derived | adapter version from latest successful collection (lineage exposure, non-sensitive) |
| Cross-location composite grid | composite_score or "—" per location × horizon | ProviderRanking (all cells for provider) | derived | Cell click → S-02 for that location/horizon; insufficient → "—" + tooltip with n/30 |
| Collection reliability vs. coverage panel | reliability, coverage + inline explainer | ProviderRanking components | derived (methodology §4.4) | PC-06 distinction; explainer text fixed (doc 02 §7.3) |
| Per-horizon metric detail | metric values per horizon for selected location | AccuracyMetric | derived | Same null/sample rules as S-02 |
| Circuit status banner (partial state) | provider circuit state | `/admin/health` is NOT used (admin-only) | — | Public banner derives from `warnings[]` in rankings/accuracy responses only |

**Expected cardinality**: grid = locations (5–10) × horizons (7) = ≤ 70 cells; response < 15 KB.

### Backend capability

| Capability | Specification |
|------------|---------------|
| `GET /accuracy?provider_id=&location_id=&horizon_minutes=` | Metric rows (existing endpoint, public) |
| `GET /rankings?location_id=` per location OR `GET /accuracy/summary?provider_id=` (extended) | Grid data. Decision: extend `/accuracy/summary` to accept `provider_id` filter returning all location×horizon ranking cells for the provider in one payload (avoids N+1 requests per location — board mandate on unbounded orchestration). Additive within v1. |
| Collection window | `collection_window` block in summary (C-08) |

**N+1 guard**: the grid must NOT issue one `/rankings` request per location. The extended `/accuracy/summary?provider_id=` returns the full grid in one response.

### Behavioural states

Loading, empty-provider-no-data, empty-cell ("—" + tooltip), insufficient (cell "—" + n/30 tooltip), partial (banner "Provider temporarily unavailable — showing last known data ({time})" when warnings[] present), stale, error.

### Actions and mutations

Export CSV only.

---

## S-04 Trends (Accuracy Analytics)

### Screen identity

| Field | Value |
|-------|-------|
| Route | `/trends` |
| MVP status | Required |
| Primary persona | Daniel (long-term trust), Mei (analysis), Aisyah (quality monitoring) |
| Primary user goal | Is a provider's accuracy stable, improving, or degrading? |
| Primary decision supported | Trend-based trust adjustment |
| Primary actions | Select variable/metric/aggregation/period → compare trend lines → click point → S-05 |
| Required permissions | Public |

### UI elements → data requirements

| UI element | Data fields | Source entity | Direct/derived | Notes |
|------------|-------------|---------------|----------------|-------|
| Trend chart series per provider | period bucket, value, ci_lower/upper, sample_count | AccuracyMetric (aggregated rows) | derived | Bucketing per BR-TZ-05 (location local day); API `tz` param echoes bucketing zone |
| Hollow points | sample_count < threshold (30) | AccuracyMetric | derived | "provisional" legend entry |
| CI band | ci_lower/upper (10% opacity) | AccuracyMetric | derived | Wilson for ratios; normal approx for continuous |
| Summary table | period avg, latest, trend Δ, samples, CI per provider | AccuracyMetric (client-computed from series OR server `summary` param) | derived | Decision: client computes from the series (≤365 points) — no extra endpoint |
| Y-axis direction label | "lower is better" / "higher is better" | metric_type registry (static) | direct | From `/rankings/methodology` formulas registry |
| Metric selector options | per-variable metric list | methodology registry | direct | Static mapping (doc 02 §6.4) |

**Cardinality**: max 365 daily buckets × 2 providers = 730 points; weekly/monthly far fewer. Response < 60 KB worst case (daily, 365d).

### Backend capability

`GET /accuracy?location_id=&horizon_minutes=&variable=&metric_type=&period_start=&period_end=&aggregation=daily|weekly|monthly&tz=` — existing endpoint. Aggregation produces bucketed AccuracyMetric sets. Buckets computed by SQL date_trunc over `period_start` in the requested tz (echoed in response). All buckets carry sample_count; no bucket is served without it.

### Behavioural states

Loading, empty-no-data-in-period, all-provisional banner, partial (missing provider line + legend note), stale, error. Point click → S-05 with date param.

### Actions and mutations

Export CSV (client-side; ≤365 rows bounded). No server mutations.

---

## S-05 Forecast vs. Actual

### Screen identity

| Field | Value |
|-------|-------|
| Route | `/forecast-vs-actual` |
| MVP status | Required (public — C-19) |
| Primary persona | Daniel ("was yesterday's rain forecast right?"), Mei |
| Primary user goal | Compare provider predictions against what actually happened for one day |
| Primary decision supported | Which provider's track matched reality for the selected day |
| Primary actions | Select date/variable/providers → read chart → read day-metric table → export |
| Required permissions | Public (via new bounded endpoint; raw `/forecasts`+`/observations` remain user+) |

### UI elements → data requirements

| UI element | Data fields | Source entity | Direct/derived | Notes |
|------------|-------------|---------------|----------------|-------|
| Forecast lines per provider | target_time, value per variable, issued_at, provider | ForecastSnapshot | direct | Issuance selection per DR-02: snapshot whose `forecast_horizon_minutes` matches the selected horizon; subtitle "Forecasts issued {issued_at} · Horizon +{N}h" |
| Observation line | observed_at, value, source, observation_type, quality_flag | Observation | direct | Gray-900 dashed; provenance badge in legend; gaps = line breaks (never interpolated) |
| Error band (±MAE) | MAE for selected variable/horizon over 30d | AccuracyMetric | derived | 10% opacity around observation |
| Day summary table | MAE, Bias, RMSE, samples per provider (that day) | computed server-side over that day's matched pairs | derived | Returned in the comparison payload |
| Hover tooltip | time, per-provider forecast, observed, per-provider error | from chart data | derived (client) | — |

**Timezone**: x-axis in location timezone (BR-TZ-02) with explicit zone label. **Units**: per variable. **Cardinality**: ≤ 24 hourly points × (2 providers + 1 observation) + 24×2 day metrics — response < 15 KB.

### Backend capability (C-19)

**New public endpoint**: `GET /forecast-comparison?location_id=&date=&variable=&providers=&horizon_minutes=`

Response: `{location, date, variable, horizon_minutes, series: [{provider_id, points: [{target_time, value, issued_at}]}], observations: [{observed_at, value, source, observation_type, quality_flag}], day_metrics: [{provider_id, mae, bias, rmse, sample_count}], error_band_mae, provenance, attribution, freshness, warnings[]}`.

- Bounded: one day, one variable, selected providers (default all active).
- Public read justified: portfolio demonstrability; bulk-scrape surface limited vs. raw endpoints (rate-limited per IP); attribution embedded (BR-ATTR-01, BR-LIC-01 gate applies before launch).
- Raw `GET /forecasts` / `GET /observations` remain user+ per AUTH-08.

### Behavioural states

Loading, empty-no-forecasts-for-date ("Collection started {date}"), observation-unavailable (forecast lines shown; observation line + band absent; banner "Ground truth unavailable — metrics not computed"), partial (absent provider + legend note), stale, error.

### Actions and mutations

Export CSV (≤ 24×providers rows). No server mutations.

---

## S-06 Methodology

### Screen identity

| Field | Value |
|-------|-------|
| Route | `/methodology` |
| MVP status | Required |
| Primary persona | All (trust verification) |
| Primary user goal | Understand exactly how every number is computed |
| Required permissions | Public |

### UI elements → data requirements

Rendered from `GET /rankings/methodology` (machine-readable: formulas registry, default weights table, thresholds 30/10, coverage penalty rule, statuses, tie handling, version numbers) + static prose (worked example from methodology §8, plain-language explanations, change history). Every metric label across the app deep-links to a `#section-anchors` here.

### Backend capability

`GET /rankings/methodology` — static configuration payload, aggressively cached (ETag; changes only on methodology version bump).

### Behavioural states

Loading (skeleton text), error (retry). No data-dependent empty/stale states (static content; version display is always current by construction).

---

## S-07 Onboarding (first-use)

### Screen identity

| Field | Value |
|-------|-------|
| Route | Overlay on `/` (no dedicated route) |
| MVP status | Required |
| Primary persona | New signed-in user |
| Primary user goal | Understand what ForecastIQ measures and doesn't; pick default location |
| Required permissions | Signed-in, first visit |

### UI elements → data requirements

Content: what we measure / don't measure (NP-01..07 in plain language), link to S-06, default-location picker (from `GET /locations`), "how rankings work" summary (thresholds, provisional, ties).

### Backend capability

None beyond existing endpoints. Dismissal: `localStorage` keyed by user ID (DR-04 approved). Re-openable from Settings. Default-location selection persists via `PATCH /me` (profile preferences — see `docs/api/01-screen-api-contracts.md` §7; additive field `default_location_id`).

### Behavioural states

Shown once per account; dismissible (X / "Get started"); never shown to public visitors; never blocks page content (overlay after first data paint).

### Actions and mutations

| Action | Endpoint | Notes |
|--------|----------|-------|
| Set default location | `PATCH /me {default_location_id}` | Idempotent; audited; validation: location must exist + be active |

---

## S-08 Auth pages

### Screen identity

| Field | Value |
|-------|-------|
| Routes | `/auth/signin`, `/auth/signup`, `/auth/verify`, `/auth/reset` |
| MVP status | Required |
| Required permissions | Public |

### Reconciliation (C-17)

- **Custom forms calling the Supabase JS SDK** (approved). No application-managed password handling (ADR-008 binding: no password hashes in ForecastIQ's DB; no app-designed password screens beyond SDK-backed forms).
- Flows managed by Supabase: credential verification, email verification delivery, password reset email, brute-force protection, refresh rotation.
- Handled by ForecastIQ: user-row creation on first authenticated API call (`auth_subject` mapping), role assignment (default `user`; first account seeded as `admin` at bootstrap), audit events (AUTH-07), disabled-account refusal (generic message per AC-5.3).
- Session expiry: Supabase access token ≤1h; SDK handles silent refresh; on refresh failure → redirect to `/auth/signin` with return URL.
- Local development (no Supabase project configured): `/auth/signin` renders a dev-token form backed by the ADR-008 dev-mode verifier; the token is validated against `GET /me` before being stored client-side. Dev-only — the dev verifier is compiled out of release builds, and no token is ever injected from build-time environment.

### Behavioural states

Validation failure (inline field errors + `role="alert"` summary), disabled account (generic refusal), rate-limited (Supabase 429 → "Too many attempts, try later"), verified (redirect to `/` + onboarding trigger), reset-sent (confirmation regardless of email existence — no account enumeration).

### Actions and mutations

All mutations are Supabase SDK calls (register, signIn, resetPassword, updatePassword, signOut). ForecastIQ backend mutations triggered indirectly: user-row upsert on first JWT-presenting request; `audit_events` rows for login/failed-login/registration/verification/reset.

---

## S-09 Settings

### Screen identity

| Field | Value |
|-------|-------|
| Route | `/settings?tab=profile\|keys\|preferences\|danger` |
| MVP status | Required |
| Required permissions | Signed-in (self-service only) |

### UI elements → data requirements

| UI element | Data fields | Source | Notes |
|------------|-------------|--------|-------|
| Profile | email, role, created_at, last_login_at | `GET /me` | email editable only via Supabase (verification required) |
| Default location + timezone toggle | default_location_id, tz_display_pref | `GET /me` / `PATCH /me` | BR-TZ-03 |
| API keys table | id, name, key_prefix, scopes, rate_limit_per_min, created_at, last_used_at, status | `GET /api-keys` | Full key never shown after creation (AUTH-05) |
| Create key dialog | name, scopes, rate limit | `POST /api-keys` + Idempotency-Key | Key shown once with copy + warning |
| Revoke | — | `DELETE /api-keys/{id}` | Immediate; confirmation dialog; audited |
| GDPR export | — | `POST /me/export` | Async job → download link (email + Settings poll); AUTH-09 |
| Delete account | type-to-confirm | `DELETE /me` | States deleted vs. retained (AUTH-09); Supabase admin API server-side |

### Behavioural states

Loading, key-created (one-time display state), export-pending ("Export preparing — link valid 24h"), export-ready (download link), error states per contract.

### Actions and mutations

| Action | Endpoint | Validation | Idempotency | Audit | UI behaviour |
|--------|----------|------------|-------------|-------|--------------|
| Update preferences | `PATCH /me` | enum checks | natural | yes | optimistic |
| Create key | `POST /api-keys` | name length, scopes whitelist | Idempotency-Key | yes | pessimistic (need server key) |
| Revoke key | `DELETE /api-keys/{id}` | ownership check | natural | yes | pessimistic + confirm |
| Request export | `POST /me/export` | one active job per user (409) | Idempotency-Key | yes | pessimistic |
| Delete account | `DELETE /me` | type-to-confirm | natural | yes (pre-deletion) | pessimistic |

---

## S-10 Admin Health

### Screen identity

| Field | Value |
|-------|-------|
| Route | `/admin/health` |
| MVP status | Required |
| Primary persona | Aisyah (operator, daily loop J4) |
| Primary user goal | Is the collection pipeline healthy? Which provider-location is failing and why? |
| Primary actions | Scan grid → expand anomaly → re-collect or consult runbook → verify recovery |
| Required permissions | Admin (server-enforced; 403 state per contract) |

### UI elements → data requirements

| UI element | Data fields | Source | Notes |
|------------|-------------|--------|-------|
| Summary bar | counts per freshness state; payload volume % | `/admin/health` | — |
| Health grid row (per provider×location) | last_success_at, freshness state, circuit_state, consecutive_failures, next_scheduled_at (C-12) | ForecastCollection aggregation + scheduler state | Freshness per BR-FRESH forecast thresholds |
| Expanded row | error_code, error_message, circuit opened_at, half-open probe countdown | ForecastCollection + circuit state | error_message truncated (no stack traces) |
| Observation collector status | last success per location, suspect count (24h), locations covered | Observation aggregation | — |
| System health | payload_volume{used,total,pct}, engine_lag_seconds, last_backup{at,status}, last_restore_test{at,status} | statfs + max(calculated_at) + backup status file (C-11) | — |
| Filters | provider, status, location | query params | — |

### Backend capability

`GET /admin/health` (admin) — assembled from: DB aggregates over `forecast_collections` (last success, error counts), `observations` (collector status), `accuracy_metrics` (engine lag), filesystem statfs (payload volume), backup status file (C-11). ETag for 60s polling (DR-03). All inputs are application-table reads or local filesystem — **no log/metric-system queries** (board principle).

### Actions and mutations

| Action | Endpoint | Validation | Idempotency | Error recovery | UI behaviour |
|--------|----------|------------|-------------|----------------|--------------|
| Re-collect now (C-10) | `POST /admin/collections/trigger {provider_id, location_id}` | 409 while circuit open ("retry available after half-open probe"); rate-limit budget check | natural (dedup on snapshots) | 429 → "Rate limit budget exhausted — next slot {time}" | pessimistic; success → row refresh |
| View runbook | external link | — | — | — | — |
| View collections | navigate to S-13 filtered | — | — | — | — |

Auto-refresh: 60s polling + manual refresh button + last-refresh timestamp; `aria-live="polite"` announces state changes.

### Behavioural states

Loading, empty-no-providers-configured (+ link to S-11), all-healthy, partial (row highlight + summary counts), error-loading-health, permission-denied (403 state).

---

## S-11 Admin Providers

### Screen identity

| Field | Value |
|-------|-------|
| Route | `/admin/providers` |
| MVP status | Required |
| Required permissions | Admin |

### UI elements

Provider list (name, status, adapter_version, schedule summary, credential status indicator — **never the credential**, BR-08), edit dialog (status, collection interval 15–1440 min, attribution text/URL).

### Actions and mutations

| Action | Endpoint | Validation | Idempotency | Audit | UI behaviour |
|--------|----------|------------|-------------|-------|--------------|
| Enable/disable | `PATCH /admin/providers/{id}/status` | status enum; disable warning (BR-LOC-03 pattern) | natural | yes | pessimistic + confirm on disable |
| Update configuration | `PUT /admin/provider-configurations/{id}` | interval 15–1440; credential_ref format | natural (PUT) | yes | pessimistic; "applies next cycle" note (FC-07) |

Credential status is "Configured / Not set" only. Credential rotation via env/secret store per runbook (NFR-SEC13) — no credential entry in UI for MVP (credential_ref points to secret-store key).

---

## S-12 Admin Locations

### Screen identity

| Field | Value |
|-------|-------|
| Route | `/admin/locations` |
| MVP status | Required |
| Required permissions | Admin |

### UI elements

Location list (name, coords, timezone, status, collecting-since), add/edit form (name, lat, lon, IANA timezone picker, country), dedup warning (BR-LOC-01: within 0.05° of active location → warning + link to existing + explicit override).

### Actions and mutations

| Action | Endpoint | Validation | Idempotency | Error recovery | UI behaviour |
|--------|----------|------------|-------------|----------------|--------------|
| Add location | `POST /locations` | lat/lon ranges; IANA tz valid; tz-coords soft warning | Idempotency-Key | 409 duplicate → inline warning + "Add anyway" sets `allow_near_duplicate` | pessimistic |
| Edit | `PUT /locations/{id}` | same (mutable: name, timezone, status) | natural | — | pessimistic |
| Disable/enable | `PATCH /locations/{id}/status` | confirmation text (historical data remains) | natural | — | pessimistic + confirm |

Geocoding ambiguity: **no geocoding service in MVP** — operator enters coordinates directly (removes an external dependency; consistent with MVP simplicity). Timezone validated against coordinate-derived solar offset (soft warning only).

---

## S-13 Admin Schedules & Runs

### Screen identity

| Field | Value |
|-------|-------|
| Route | `/admin/schedules` |
| MVP status | Required |
| Required permissions | Admin |

### UI elements

Filters (provider, location, status, time range), run history table (requested_at, provider, location, collection_status, stored/dedup/invalid counts, latency_ms, error_code), row expansion (collection ID, error detail, schema/adapter versions, payload availability + checksum prefix), replay button, recompute dialog (scope: location/horizon/provider + immutability explanation), cursor pagination (`has_more`, no total_count).

### Actions and mutations

| Action | Endpoint | Validation | Idempotency | Error recovery | UI behaviour |
|--------|----------|------------|-------------|----------------|--------------|
| Replay payload | `POST /admin/collections/{id}/replay` | payload must exist (≤90d retention) — else button disabled + tooltip "Payload expired" | idempotent snapshots (FC-14) | 422 payload-missing/corrupt → explicit error | pessimistic + confirm |
| Trigger recompute | `POST /admin/rankings/recompute` | scope filters validated | natural (new rows) | — | pessimistic + confirm dialog (immutability text) |
| Re-collect (same as S-10) | `POST /admin/collections/trigger` | per S-10 | per S-10 | per S-10 | per S-10 |

---

## S-14 Admin Users & Audit

### Screen identity

| Field | Value |
|-------|-------|
| Route | `/admin/users?tab=users\|audit` |
| MVP status | Required |
| Required permissions | Admin |

### UI elements

Users tab: list (email, role, status, created, last_login), actions menu (disable, delete, GDPR export). Audit tab: filters (action, user, resource type) + table (timestamp, user, action, resource, IP) with cursor pagination.

### Actions and mutations (C-09 — new endpoints)

| Action | Endpoint | Validation | Guard | Audit |
|--------|----------|------------|-------|-------|
| List users | `GET /admin/users?status=&cursor=` | — | admin | no |
| Disable user | `PATCH /admin/users/{id}/status {status: disabled}` | status enum | 409 self-disable (lockout guard) | yes |
| Delete user | `DELETE /admin/users/{id}` | type-email-to-confirm (UI) | 409 self-delete | yes (pre-deletion) |
| Admin-triggered export | `POST /admin/users/{id}/export` | one active job (409) | — | yes |

Disable/delete propagate to Supabase Auth via server-side admin API (service-role key, backend-only — never in browser). Disabled login → generic refusal (AC-5.3). Deletion scope per AUTH-09 (personal data deleted; weather data retained).

---

## S-15 Error pages

| Route | Trigger | Content |
|-------|---------|---------|
| 404 | Unknown route / missing resource | "Page not found" + link to Overview |
| 403 | Forbidden on admin section | "Administrator access required" + sign-in switch hint |
| 500 | Server error (API passthrough page) | "Something went wrong" + request_id display (support correlation) + retry |
| Offline | `navigator.onLine=false` / network failure | Cached data + banner per `docs/ui/06-ui-state-contracts.md` §7 |

The 500 page displays the `request_id` from the error envelope — operational correlation without leaking internals (security requirement: no stack traces, no provider error detail on public errors).
