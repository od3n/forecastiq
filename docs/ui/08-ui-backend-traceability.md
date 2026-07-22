# ForecastIQ — UI ↔ Backend Traceability Matrices

**Version**: 1.0 (UI ↔ Backend Reconciliation Board)
**Status**: Authoritative — complete traceability per board mandate
**Companions**: `docs/ui/05-screen-specifications.md` (narrative specs), `docs/api/01-screen-api-contracts.md` (endpoint detail)

Seven matrices. "Supported today" = capability exists in the approved Phase 0 API/domain model; "Action required" = amendment from this reconciliation.

---

## Matrix 1: UI Element → Data Field

| Screen | UI element | Field/metric | Source entity | Source attribute | Derived/Direct | Freshness | Provenance | Sample-size req | Supported today | Action required |
|--------|-----------|--------------|---------------|------------------|----------------|-----------|------------|-----------------|-----------------|-----------------|
| S-01 | Ranking row: score | composite_score | ProviderRanking | composite_score, ci_lower/upper | Derived (meth §6) | rankings (BR-FRESH) | methodology_version, weights_version | ≥30 ranked / 10 provisional | Yes | None |
| S-01 | Ranking row: status | ranking_status | ProviderRanking | ranking_status, reason | Derived (meth §7) | rankings | — | threshold echoed | Yes | None |
| S-01 | Ranking row: samples | sample_count | ProviderRanking | sample_count | Derived | rankings | — | self | Yes | None |
| S-01 | Ranking row: coverage | coverage | ProviderRanking | coverage | Derived (meth §4.4) | rankings | — | — | Yes | None |
| S-01 | Breakdown panel | component scores | ProviderRanking + AccuracyMetric | component_scores JSONB | Derived | rankings | observation_provenance_mix | per-component n | Yes | None |
| S-01 | Tie annotation | significant_vs_next | ProviderRanking (pair) | CI overlap | Derived (BR-RANK-05) | rankings | — | — | Yes | None |
| S-01 | Observation context line | temperature_c, precipitation_mm, observed_at | Observation | direct fields | Direct | observations (BR-FRESH) | source, observation_type | n/a | **Partially** — data exists; not in rankings payload | **Add `observation_context` to rankings response (C-01/C-04)** |
| S-01 | Location context | name, timezone, country_code | Location | direct | Direct | n/a | — | n/a | Yes | None |
| S-01 | Freshness indicator | freshness.state | server-computed | freshness block | Derived | self | — | n/a | Yes | None |
| S-01 | Attribution footer | attribution[] | Provider | attribution_text/url | Direct | n/a | BR-ATTR-01 | n/a | Yes | None |
| S-02 | Variable metric tables | value, ci, sample_count | AccuracyMetric | value, ci_lower/upper, sample_count | Derived (meth §4) | rankings/metrics | methodology_version | per-metric n | Yes | None |
| S-02 | Occurrence agreement warning | occurrence_agreement | AccuracyMetric | value + flag | Derived | metrics | imbalance warning text | n | Yes | None |
| S-02 | Collection window | first/last snapshot, coverage | ForecastSnapshot + AccuracyMetric | MIN/MAX(created_at), coverage | Derived | collections | — | — | **No** | **Add `collection_window` to `/accuracy/summary` (C-08)** |
| S-02 | Observation provenance badge | observation_provenance_mix | Observation (aggregate) | mix over period | Derived | metrics | self | — | Yes (API-08) | None |
| S-03 | Cross-location grid | composite per cell | ProviderRanking | composite_score, ranking_status | Derived | rankings | methodology_version | per-cell | **Partially** — one request per location today | **Extend `/accuracy/summary?provider_id=` for full grid (C-08/N+1 guard)** |
| S-03 | Reliability vs coverage | reliability, coverage | ProviderRanking | components | Derived (meth §4.4) | rankings | error_code classification (FC-13) | — | Yes | None |
| S-03 | Provider header: adapter ver | adapter_version | ForecastCollection (latest) | adapter_version | Direct | collections | lineage | n/a | **Partially** — admin endpoint only | **Expose in `/providers/{id}` public payload (non-sensitive)** |
| S-04 | Trend series | value, ci, sample_count per bucket | AccuracyMetric | value, ci, sample_count, period_start | Derived | metrics | methodology_version, tz bucketing (BR-TZ-05) | per-bucket | Yes | None |
| S-04 | Hollow points | sample_count < threshold | AccuracyMetric | sample_count | Derived | metrics | — | self | Yes | None |
| S-04 | Summary table (avg/latest/Δ) | series aggregate | AccuracyMetric | from series | Derived (client) | metrics | — | — | Yes (client-side) | None |
| S-05 | Forecast lines | value per target_time, issued_at | ForecastSnapshot | variable fields, issued_at | Direct | collections | provider, collection lineage | n/a | **No** — raw endpoint is user+ | **New public `GET /forecast-comparison` (C-19)** |
| S-05 | Observation line | value, provenance, quality | Observation | direct fields | Direct | observations | source, observation_type, quality_flag | n/a | **No** (same) | Same as above |
| S-05 | Error band ±MAE | MAE 30d | AccuracyMetric | value (mae) | Derived | metrics | methodology_version | ≥30 | **No** (same) | Included in `/forecast-comparison` |
| S-05 | Day summary table | MAE/Bias/RMSE (day) | MatchedEvaluation (day subset) | computed | Derived | metrics | methodology_version | day n shown | **No** (same) | Included in `/forecast-comparison` |
| S-06 | Formulas/weights/thresholds | methodology registry | MethodologyVersion (config) | registry payload | Direct | n/a (versioned) | version | n/a | Yes | None |
| S-09 | API keys table | prefix, scopes, last_used | APIKey | direct (no secret fields) | Direct | n/a | — | n/a | Yes | None |
| S-10 | Health grid | last_success, circuit, errors, freshness | ForecastCollection + circuit state | aggregates | Derived | operational (BR-FRESH) | error_code | n/a | Yes (`/admin/health`) | **Extend: next_scheduled_at (C-12)** |
| S-10 | Observation collector status | last success/loc, suspect 24h | Observation | aggregates | Derived | operational | source | n/a | **Partially** | **Add observation-collector section to `/admin/health` (C-11)** |
| S-10 | System health panel | volume, engine lag, backup | filesystem + metrics table + status file | — | Derived | operational | — | n/a | **No** | **Add `system` section to `/admin/health` (C-11)** |
| S-13 | Run history | status, counts, latency, error | ForecastCollection | direct fields | Direct | n/a | adapter/schema version | n/a | Yes | None |
| S-13 | Payload availability | raw_payload exists + checksum prefix | ForecastCollection | raw_payload_object_key, checksum | Direct | retention-bounded (90d) | checksum | n/a | Yes | None |
| S-14 | User list | email, role, status, last_login | User | direct | Direct | n/a | — | n/a | **No** | **New `GET /admin/users` (C-09)** |
| S-14 | Audit log | action, resource, user, ip | AuditEvent | direct | Direct | n/a | — | n/a | Yes | None |

## Matrix 2: UI Element → API

| Screen | UI element | User interaction | Endpoint/capability | Method | Auth | Permission | Request params | Response fields | Cacheable | Expected latency | Partial behaviour | Status |
|--------|-----------|------------------|---------------------|--------|------|------------|----------------|-----------------|-----------|------------------|-------------------|--------|
| S-01 | Ranking table | Load / filter change | `/rankings` | GET | public | none | location_id (req), horizon_minutes, min_sample_count, weights | rankings[], methodology, freshness, warnings[], observation_context | ETag + LRU | p50 <50ms / p95 <200ms | warnings[] per provider | Exists + amend (obs context) |
| S-01 | Location selector | Load | `/locations?active=true` | GET | public | none | active | locations[] | ETag | <50ms | n/a | Exists |
| S-01 | Breakdown expand | Click row | (same payload) | — | — | — | — | components{} | client | instant | — | Exists |
| S-01 | Export CSV | Click | client-side | — | — | — | — | — | — | instant | — | Ratified (DR-05) |
| S-02 | Metric tables | Load | `/accuracy/summary` | GET | public | none | location_id, horizon_minutes, period_days | metrics[], collection_window, provenance, freshness | ETag | p95 <200ms | warnings[] | Exists + amend (window) |
| S-02 | Ranking summary | Load | `/rankings` | GET | public | none | location_id | rankings[] | ETag | <200ms | warnings[] | Exists |
| S-03 | Composite grid | Load | `/accuracy/summary?provider_id=` | GET | public | none | provider_id | cells[location×horizon], collection_window | ETag | p95 <200ms | warnings[] | **New (extension)** |
| S-03 | Per-horizon detail | Location select | `/accuracy` | GET | public | none | provider_id, location_id, horizon_minutes | metrics[] | ETag | <200ms | warnings[] | Exists |
| S-04 | Trend chart | Load / control change | `/accuracy` | GET | public | none | location_id, horizon, variable, metric_type, period_start/end, aggregation, tz | series[] with buckets | ETag | p95 <200ms | warnings[] | Exists |
| S-04 | Point → S-05 | Click point | navigate | — | — | — | date param | — | — | — | — | Frontend |
| S-05 | Chart + tables | Load | `/forecast-comparison` | GET | public | none | location_id, date, variable, providers, horizon_minutes | series[], observations[], day_metrics[], error_band_mae, provenance, attribution, freshness | ETag (per date) | p95 <200ms | warnings[] | **New (C-19)** |
| S-06 | Methodology content | Load | `/rankings/methodology` | GET | public | none | — | registry | ETag (long) | <50ms | n/a | Exists |
| S-07 | Default location | Select | `PATCH /me` | PATCH | bearer | self | default_location_id | profile | no | <100ms | n/a | Exists + amend (field) |
| S-09 | Keys CRUD | Create/revoke | `/api-keys` | POST/DELETE | bearer | self (owner) | name, scopes, rate_limit | key (once), list | no | <200ms | n/a | Exists |
| S-09 | GDPR export | Request | `/me/export` | POST | bearer | self | — | job{download_url, expires_at} | no | <200ms (job async) | n/a | Exists |
| S-09 | Delete account | Confirm | `/me` | DELETE | bearer | self | — | 204 | no | <500ms | n/a | Exists |
| S-10 | Health grid | Load + 60s poll | `/admin/health` | GET | bearer | admin | provider_id, status, location_id | cells[], observation_collector, system, freshness | ETag | p95 <200ms | per-cell states | Exists + amend (C-11/12) |
| S-10 | Re-collect | Click | `/admin/collections/trigger` | POST | bearer | admin | provider_id, location_id | collection preview / 409 | no | <2s (provider call) | 409 circuit-open | **New (C-10)** |
| S-11 | Provider toggle | Click | `/admin/providers/{id}/status` | PATCH | bearer | admin | status | provider | no | <200ms | n/a | Exists |
| S-11 | Config edit | Save | `/admin/provider-configurations/{id}` | PUT | bearer | admin | schedule, credential_ref, attribution | configuration | no | <200ms | n/a | Exists |
| S-12 | Add location | Submit | `/locations` | POST | bearer | admin | name, lat, lon, timezone, country_code, allow_near_duplicate | location / 409 duplicate | no | <200ms | 409 + reference | Exists |
| S-12 | Disable | Confirm | `/locations/{id}/status` | PATCH | bearer | admin | status | location | no | <200ms | n/a | Exists |
| S-13 | Run history | Load/filter | `/forecast-collections` | GET | bearer | admin | provider_id, location_id, status, time range, cursor | collections[], pagination | ETag (short) | <200ms | n/a | Exists |
| S-13 | Replay | Confirm | `/admin/collections/{id}/replay` | POST | bearer | admin | — | new collection | no | <5s | 422 payload-missing | Exists |
| S-13 | Recompute | Confirm scope | `/admin/rankings/recompute` | POST | bearer | admin | location_id, horizon, provider_id | job acknowledgement | no | <1s (batch async) | n/a | Exists |
| S-14 | User list | Load | `/admin/users` | GET | bearer | admin | status, cursor | users[], pagination | no | <200ms | n/a | **New (C-09)** |
| S-14 | Disable/delete user | Confirm | `/admin/users/{id}/status`, `/admin/users/{id}` | PATCH/DELETE | bearer | admin | status / — | user / 204; 409 self | no | <500ms (Supabase call) | n/a | **New (C-09)** |
| S-14 | Audit log | Load/filter | `/admin/audit-events` | GET | bearer | admin | action, user_id, resource_type, cursor | events[], pagination | no | <200ms | n/a | Exists |

## Matrix 3: Screen → Domain Model

| Screen | Domain entity | Aggregate/relationship | Operation | R/W | Missing model capability | Proposed change |
|--------|---------------|------------------------|-----------|-----|--------------------------|-----------------|
| S-01 | ProviderRanking, Location, Observation, Provider | Ranking aggregate; Observation→Location | read rankings + latest observation | R | Latest-observation exposure in rankings payload | None (query-level; no entity change) |
| S-02 | AccuracyMetric, ProviderRanking, ForecastSnapshot, Observation | Metric aggregate; Snapshot→Collection→Location | read metrics + window | R | collection_window fields | None (derived in query; additive API fields only) |
| S-03 | ProviderRanking, AccuracyMetric, Provider, ForecastCollection | cross-location ranking cells | read | R | provider grid in one payload; adapter_version public exposure | None (API-level) |
| S-04 | AccuracyMetric | metric aggregate over periods | read bucketed | R | none | None |
| S-05 | ForecastSnapshot, Observation, MatchedEvaluation, AccuracyMetric | snapshot+observation join via matching rules | read + day-metric derivation | R | bounded public comparison payload | None (new endpoint over existing entities) |
| S-06 | MethodologyVersion (config) | — | read | R | none | None |
| S-07 | User | User aggregate | write default_location_id | W | `users.default_location_id` + `users.preferences JSONB` | **Add columns** (see `docs/domain/04-ui-domain-model-reconciliation.md`) |
| S-08 | User, AuditEvent | User aggregate creation on first login | W | user upsert, audit insert | none (designed in ADR-008) | None |
| S-09 | User, APIKey, AuditEvent | User aggregate + children | CRUD | both | none | None |
| S-10 | ForecastCollection, Observation, circuit state | Collection aggregate status | read + trigger write | both | circuit state persistence (in-memory today per FC-09) | **Persist circuit state** (table or DB-backed) for `/admin/health` + multi-restart correctness |
| S-11 | Provider, ProviderConfiguration | Catalog aggregate | CRUD | both | none | None |
| S-12 | Location | Catalog aggregate | CRUD | both | none | None |
| S-13 | ForecastCollection, ProviderRanking | Collection aggregate; ranking recompute | read + admin write | both | none | None |
| S-14 | User, AuditEvent | User aggregate lifecycle | admin write | W | admin user-management service (Supabase propagation) | Service-level; no entity change |

## Matrix 4: UI State → Backend Representation

See `docs/ui/06-ui-state-contracts.md` §1 (full matrix with all board-mandated columns: triggering condition, API status, response code, response field, retryable, user message, monitoring signal). Summary of coverage: loading ✓, no data ✓, insufficient sample ✓, delayed ✓, stale ✓, partial ✓, unavailable ✓, unauthorized ✓, forbidden ✓, rate limited ✓, failed ✓ — all eleven board-mandated states plus timeout, validation, conflict, offline.

## Matrix 5: Permission Matrix

See `docs/security/01-ui-authorization-matrix.md` (full matrix with board-mandated columns per persona × screen × action, with rationale). Summary: public read on S-01..S-06 (AUTH-08); user self-service on S-07/S-09; admin-only on S-10..S-14 with server-side enforcement on every mutation; self-lockout guards; credential invisibility (BR-08).

## Matrix 6: Metric Traceability

See `docs/domain/05-metric-ui-contract.md` (full matrix: every UI metric → definition, formula, inputs, aggregation, horizon, period, min samples, confidence representation, methodology version, API field, database source, test requirement). All 15 published metrics trace to methodology §4; composite traces to §6–7; zero unexplained metrics after deferrals (C-03).

## Matrix 7: UI-Discovered Requirements

| Req ID | Originating screen | User need | Missing capability | Proposed resolution | Backend impact | Frontend impact | Data impact | Security impact | MVP/Deferred | Priority | Owner |
|--------|-------------------|-----------|--------------------|--------------------|----------------|-----------------|-------------|-----------------|--------------|----------|-------|
| DR-01 | S-01 | Gauge data maturity via status counts | none | Counts derived client-side | None | Render counts | None | None | MVP (no action) | Low | Frontend |
| DR-02 | S-05 | See forecast at selected horizon issuance | Issuance selection rule | Horizon-matching snapshot; subtitle shows issued_at | None (filter exists) | Subtitle + selection logic | None | None | MVP | Medium | Frontend |
| DR-03 | S-10 | Current data during incident | Refresh interval | 60s polling + ETag + manual refresh | Endpoint must stay cheap (<200ms) | Polling logic | None | Admin-only | MVP | Medium | Backend + Frontend |
| DR-04 | S-07 | Onboarding shown once | Dismissal persistence | localStorage keyed by user ID | None | Storage + re-open link | None | None (cosmetic) | MVP | Low | Frontend |
| DR-05 | S-02/S-04/S-05 | Provenance in exports | CSV metadata format | `#`-prefixed header block (ratified) | None (client assembles) | Header generation | None | None | MVP | High | Frontend + QA |
| DR-06 | Overview (doc 03) | Know when forecasts shifted | Change detection between snapshots | Deferred L3; LAG() query documented | None now | None now | None | None | Deferred | High (L3) | Architect |
| DR-07 | Overview (doc 03) | Single consensus value | Consensus aggregation | Deferred L3; methodology gate | None now | None now | None | None | Deferred | High (L3) | Data Eng |
| DR-08 | Overview (doc 03) | Human-readable confidence | Confidence formula | Deferred L3; methodology gate | None now | None now | None | None | Deferred | Medium (L3) | Data Eng |
| DR-09 | Overview (doc 03) | Current observation at a glance | Latest-observation endpoint | Simplified: context line in rankings payload; feels-like excluded | Add `observation_context` block | Render context line | None (existing data) | Public (bounded, single record) | MVP (simplified) | Medium | Backend |
| DR-10 | Overview (doc 03) | Disagreement over time | Std-dev across providers | Deferred L3 | None now | None now | None | None | Deferred | Medium (L3) | Data Eng |
| DR-11 (new) | S-01/S-02 | Trust ground-truth currency | Observation freshness on public screens | `observation_context.freshness` per BR-FRESH | Included in DR-09 block | Freshness dot on context line | None | Public | MVP | Medium | Backend |
| DR-12 (new) | S-10 | Recovery from failed collection | Immediate re-collection | `POST /admin/collections/trigger` (C-10) | New endpoint + circuit guard | Button + 409 handling | None | Admin; audited; rate-budget | MVP | High | Backend |
| DR-13 (new) | S-14 | Admin account management | User management endpoints | Four endpoints (C-09) | New endpoints + Supabase admin propagation | Tables + dialogs | None | Admin; self-lockout guard; audited | MVP | Critical | Backend |
| DR-14 (new) | S-05 | Public access to comparison data | Public bounded comparison payload | `GET /forecast-comparison` (C-19) | New endpoint | Chart wiring | None | Public; rate-limited per IP; attribution embedded | MVP | Critical | Backend |
| DR-15 (new) | S-03 | Grid without N+1 requests | Provider-scoped summary | `/accuracy/summary?provider_id=` extension (C-08) | Additive param | Single fetch | None | Public | MVP | High | Backend |
