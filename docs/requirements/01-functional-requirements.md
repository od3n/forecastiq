# ForecastIQ — Functional Requirements (Revised)

**Version**: 2.0 (Phase 0 Amendment)
**Status**: Authoritative
**Supersedes**: `docs/phase-0-business-analysis/04-functional-requirements.md` and SRS §3 (`03-software-requirements-spec.md`)

Requirements use RFC 2119 keywords. IDs are stable for traceability.

---

## 1. Forecast Collection (FC)

| ID | Requirement | Priority |
|----|-------------|----------|
| FC-01 | System SHALL collect forecasts from Open-Meteo and OpenWeather (MVP providers). | Critical |
| FC-02 | Each provider API call SHALL create one **ForecastCollection** record; the response array SHALL be decomposed into one **ForecastSnapshot** per target period (0..n per collection). | Critical |
| FC-03 | System SHALL store snapshots immutably with `UNIQUE (provider_id, location_id, issued_at, target_time)` deduplication via conflict-safe inserts. | Critical |
| FC-04 | System SHALL record per collection: requested_at, completed_at, status, provider_request_id, model_run_time (when available), raw payload key + SHA-256 checksum, HTTP status, latency, records_received, stored/deduplicated/invalid counts, schema_version, adapter_version, error_code/message. | Critical |
| FC-05 | System SHALL store the raw response gzip-compressed on the payload volume; retention 90 days (BR-09). | High |
| FC-06 | System SHALL collect hourly per active provider-location (one call yields all horizons). | Critical |
| FC-07 | Collection schedules SHALL be configurable per provider-configuration (cron-like interval stored in DB); changes apply on the next cycle. | High |
| FC-08 | Failed collections SHALL be retried with exponential backoff (1, 2, 4, 8, 16 s; max 5). | High |
| FC-09 | Circuit breaker SHALL open after 5 consecutive failures per provider, half-open after 60 s. | High |
| FC-10 | System SHALL respect provider rate limits (token bucket per provider; OpenWeather free-tier budget enforced). | Critical |
| FC-11 | Invalid rows (range/temporal/required-field failures) SHALL be rejected individually, counted, and reported in the collection record; > 50% invalid → collection `failed` with `schema_drift` alert. | Critical |
| FC-12 | System SHALL emit collection metrics (success/failure/dedup/partial per provider) at `/metrics`. | High |
| FC-13 | System SHALL classify failures as provider-side vs. system-side (error_code taxonomy) so reliability ≠ coverage. | High |
| FC-14 | Admin SHALL be able to replay a stored raw payload through the current adapter, producing a new collection (idempotent w.r.t. snapshots). | Medium |
| FC-15 | System SHALL map provider condition codes to the canonical taxonomy (v1); unmapped codes → `unknown` + WARN + metric; > 1%/day unmapped → alert. | High |

## 2. Observation Collection (OC)

| ID | Requirement | Priority |
|----|-------------|----------|
| OC-01 | System SHALL collect hourly observations from Open-Meteo Historical for all active locations (at :05 past the hour). | Critical |
| OC-02 | Each observation SHALL carry `source`, `observation_type` (station/interpolated/reanalysis/provider_estimated), and `quality_flag`. | Critical |
| OC-03 | Observations SHALL be stored in a separate table from forecasts, deduplicated by `UNIQUE (source, location_id, observed_at)`. | Critical |
| OC-04 | Range validation (temp −90…60 °C, humidity 0…100 %, wind 0…120 m/s, pressure 870…1084 hPa, precip 0…500 mm/h); violations stored as `suspect`, excluded from metrics. | High |
| OC-05 | Corrected observations SHALL be stored as new records with `quality_flag = corrected` referencing the superseded record. | High |
| OC-06 | System SHALL emit observation collection metrics. | High |

## 3. Comparison Engine (CE)

| ID | Requirement | Priority |
|----|-------------|----------|
| CE-01 | System SHALL match forecasts to observations per BR-MATCH-01..06 (exact-hour UTC; ±15 min only for sub-hourly sources). | Critical |
| CE-02 | System SHALL support horizons +1h, +3h, +6h, +12h, +24h, +3d, +7d (`forecast_horizon_minutes`). | Critical |
| CE-03 | System SHALL compute continuous metrics (MAE, RMSE, Bias) for temperature, wind speed, humidity, pressure; rain amount MAE (all-pairs and wet-only). | Critical |
| CE-04 | System SHALL compute occurrence metrics (Recall/POD, Precision, F1, FAR, Threat Score, occurrence_agreement) per methodology §4.2, and Brier Score per §4.3. | Critical |
| CE-05 | All formulas SHALL follow `docs/domain/03-metric-methodology.md` exactly, including zero-denominator → null, quality weighting, and rounding rules. | Critical |
| CE-06 | System SHALL compute coverage and reliability per provider-location-period. | Critical |
| CE-07 | System SHALL aggregate metrics per (provider, location, horizon, variable, period) with sample counts and 95% CIs, stored as immutable AccuracyMetric rows with methodology_version. | Critical |
| CE-08 | System SHALL compute ProviderRanking rows per methodology §6–7 (normalization, weights, coverage penalty, statuses, ties). | Critical |
| CE-09 | The engine SHALL run as a batch every 30 minutes and on-demand (admin). | High |
| CE-10 | Late/corrected observations SHALL trigger re-matching and recomputation as new rows (BR-INV-01..03). | High |
| CE-11 | Missing data SHALL be skipped per-variable without failing the batch. | High |

## 4. REST API (API)

Full specification: `docs/api/00-api-requirements.md`. Summary requirements:

| ID | Requirement | Priority |
|----|-------------|----------|
| API-01 | Endpoints: providers, locations (CRUD), forecasts, observations, accuracy, rankings, health/ready, API keys, auth session endpoints (managed-auth integration). | Critical |
| API-02 | Cursor pagination with `has_more` (no `total_count`); limit param; stable cursors. | Critical |
| API-03 | RFC 7807 error envelope with `request_id`; `X-Request-Id` on all responses (generated if absent). | Critical |
| API-04 | `Idempotency-Key` support on mutable POSTs (locations, api-keys): same key within 24 h returns the original result. | High |
| API-05 | Conditional GET (`ETag`/`If-None-Match`) on rankings/accuracy/summary endpoints. | High |
| API-06 | Rate limiting per API key with `X-RateLimit-Limit/Remaining/Reset` headers; 429 + `Retry-After`. | High |
| API-07 | CORS: explicit origin allowlist (dashboard origin + localhost dev). | Critical |
| API-08 | Every derived payload SHALL include provenance fields: methodology_version, weights_version, sample_count, coverage, freshness, attribution. | Critical |
| API-09 | Ranking responses SHALL include component breakdowns and `ranking_status`; unranked cells return status + reason, never fabricated scores. | Critical |
| API-10 | URL versioning `/api/v1/`; deprecation via `Sunset`/`Deprecation` headers, ≥ 6-month notice. | High |
| API-11 | OpenAPI 3.1 document generated from code, published at `/api/v1/openapi.json`. | High |
| API-12 | Bulk/batch endpoints: NOT in MVP (recorded as post-MVP per amendment). | — |

## 5. Authentication & Authorization (AUTH) — Blocker 6 decisions

| ID | Requirement | Priority |
|----|-------------|----------|
| AUTH-01 | Authentication SHALL use **Supabase Auth** (managed): email+password, self-registration with mandatory email verification, password reset via email, password update, account disable/delete via admin. | Critical |
| AUTH-02 | The Go backend SHALL verify Supabase JWTs via JWKS; no password hashes stored in ForecastIQ's database. | Critical |
| AUTH-03 | Session handling: Supabase access token (≤ 1 h) + refresh token with rotation; concurrent sessions allowed (managed-service policy); logout revokes refresh token. | High |
| AUTH-04 | Brute-force protection and login rate limiting provided by the managed service; additionally app-level rate limiting on auth-adjacent endpoints via API gateway limiter. | High |
| AUTH-05 | API keys (app-issued): created with scopes + per-key rate limit, shown once, stored hashed, prefix-indexed, revocable, audited. | High |
| AUTH-06 | Roles MVP: `admin` (operator) and `user`. RBAC extension point retained for Level 3. | High |
| AUTH-07 | All auth events (login, failed login, registration, verification, reset, key create/revoke) SHALL be audit-logged. | High |
| AUTH-08 | Public read access: rankings/accuracy/providers/locations readable without auth (portfolio visibility); forecasts/observations raw queries and admin require auth. | High |
| AUTH-09 | Account deletion: personal data (user row, keys) deleted; weather data (non-personal) retained per BR-09. GDPR export = account data JSON + user's created resources list. | Medium |

Rationale and alternatives (Auth0, Clerk, app-managed): ADR-008.

## 6. Dashboard (DB)

| ID | Requirement | Priority |
|----|-------------|----------|
| DB-01 | Screens per `docs/ui/00-screen-inventory.md`: Overview, Location Detail, Provider Detail, Trends, Forecast-vs-Actual, Methodology, Admin (Health, Providers, Locations, Schedules), Settings, Auth pages. | Critical |
| DB-02 | Every screen SHALL implement the states in the screen inventory: loading, empty, no-location, insufficient-data, partial-failure, stale, error, permission-denied. | Critical |
| DB-03 | Rankings display: rank, provider, composite score, status badge, sample size, coverage, freshness, provenance, CI/tie annotation; breakdown one click away. | Critical |
| DB-04 | Data freshness displayed per BR-FRESH (state + last-updated, local-time labeled per BR-TZ). | Critical |
| DB-05 | Date range, provider, horizon, variable selectors; URL-synced state (shareable). | High |
| DB-06 | CSV export of the current view with metadata header (methodology, period, provenance). | High |
| DB-07 | First-use onboarding for signed-in users: explains what the platform measures, links methodology, suggests default location. | High |
| DB-08 | Responsive (desktop + tablet); WCAG 2.1 AA target (contrast, keyboard nav, labels). | Medium |
| DB-09 | Initial meaningful paint < 2 s on a standard connection. | High |

## 7. Administration (ADMIN) — section of dashboard

| ID | Requirement | Priority |
|----|-------------|----------|
| ADMIN-01 | Enable/disable providers and edit provider configurations (credentials encrypted, schedules). | Critical |
| ADMIN-02 | Manage locations (add/edit/disable) with dedup check (BR-LOC-01) and coordinate/timezone validation. | Critical |
| ADMIN-03 | View collector health: last success per provider-location, circuit state, error counts/messages, freshness states; retry failed slots (idempotent). | High |
| ADMIN-04 | View schedule run history; edit schedules. | High |
| ADMIN-05 | Manage users (disable/delete) and view audit log. | High |
| ADMIN-06 | Trigger raw-payload replay (FC-14); trigger ranking recomputation. | Medium |

## 8. Removed requirements (vs. Phase 0)

- Alert engine (ALERT-01..05) → Level 3.
- gRPC internal interfaces → removed (never specified; phantom architecture).
- NATS event publishing → in-process event seam (names/payloads preserved).
- S3 storage requirement → payload volume + promotion criteria.
- `total_count` pagination field → removed.
- 15-minute collection cadence → hourly.
- NOAA/NWS collection (OC-01-old) → Level 3 US expansion path.
