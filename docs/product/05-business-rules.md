# ForecastIQ — Business Rules

**Version**: 1.0 (Phase 0 Amendment)
**Status**: Authoritative — single source of truth for all business rules
**Resolves**: ARB §14 Business Rule Gaps (all 8), plus rules carried over from Phase 0 BRD §5

---

## 1. Data Integrity Rules (carried over, renumbered)

| ID | Rule | Rationale |
|----|------|-----------|
| BR-01 | Forecasts and observations are immutable once stored. Corrections create new records. | Audit trail, reproducibility |
| BR-02 | Observations are stored separately from forecasts; linked only via matching. | Clear lineage, independent lifecycles |
| BR-03 | Accuracy is calculated only when both forecast and observation exist for the same location/target time with non-null values for the variable. | Data integrity |
| BR-04 | Provider attribution (name + required link/text) is displayed in the UI wherever provider data appears, and included in API response metadata (`attribution` field). | Legal compliance (ToS), transparency |
| BR-05 | API keys are scoped; rate limits enforced per key. | Least privilege, fair usage |
| BR-06 | All timestamps stored in UTC; display timezone per §7. | Consistency |
| BR-07 | Collection failures retried with exponential backoff (1s→16s, max 5), then circuit breaker. | Resilience |
| BR-08 | Provider credentials encrypted at rest; never logged; never returned by API. | Security |
| BR-09 | Retention: snapshots 2y, observations 5y, metrics indefinite, raw payloads 90d, audit 1y. | Storage cost vs. value |

## 2. Ranking Rules (new — Blocker 1)

| ID | Rule |
|----|------|
| BR-RANK-01 | No ranking is published without its methodology version, weights version, sample count per component, coverage, reliability, and ranking status. |
| BR-RANK-02 | Minimum sample: 30 matched pairs per (provider, location, variable, horizon, evaluation period) for `ranked` status; 10–29 → `provisionally_ranked`; < 10 → `unranked` ("insufficient data"). |
| BR-RANK-03 | For slow horizons (+3d, +7d), the evaluation period extends (30d → 90d) to reach the threshold; the threshold count is never lowered. |
| BR-RANK-04 | A provider with coverage < 0.5 can never outrank a provider with coverage ≥ 0.8. Coverage in [0.5, 0.8) incurs linear score penalty (×coverage/0.8). |
| BR-RANK-05 | Providers whose composite 95% CIs overlap are displayed as a tied group, not false-precision ordered. |
| BR-RANK-06 | The composite score is never shown without its per-metric breakdown being one interaction away (UI) or in the same payload (API). |
| BR-RANK-07 | Methodology or weight changes create new ranking rows; historical rows are never rewritten. |
| BR-RANK-08 | Null metrics (zero denominators) are excluded with weight redistribution — never treated as 0 or as failures. |
| BR-RANK-09 | Minimum data age before first publishing any ranking: a provider-location-horizon cell must span ≥ 7 calendar days of data even if pair counts are met (prevents "ranked #1 on 2 hours of data"). |

## 3. Matching Rules (new — observation matching amendment)

| ID | Rule |
|----|------|
| BR-MATCH-01 | Matching is exact-hour in UTC: `floor(target_time, 1h) = observed_at` for hourly sources. The universal ±30 min rule is abolished. |
| BR-MATCH-02 | Sub-hourly observation sources: ±15 min tolerance; values aggregated to the hour (precipitation summed; instantaneous variables take the reading nearest the top of hour). |
| BR-MATCH-03 | Multiple observations for one target hour: prefer by provenance rank (station > interpolated = reanalysis > provider_estimated), then nearest to top of hour; the chosen ID is recorded on the matched pair. |
| BR-MATCH-04 | Corrected observations supersede originals for matching; affected metrics recomputed. |
| BR-MATCH-05 | `suspect` observations never enter metrics. |
| BR-MATCH-06 | DST/timezone: irrelevant to matching (UTC only); relevant only to display (§7). |

## 4. Observation Rules (new)

| ID | Rule |
|----|------|
| BR-OBS-01 | Every observation carries `source`, `observation_type` (station/interpolated/reanalysis/provider_estimated), and `quality_flag`. Provenance is always exposed in API and UI. |
| BR-OBS-02 | Observation source priority for the same location/time is the provenance rank in BR-MATCH-03; there is exactly one chosen observation per matched pair, with lineage recorded. |
| BR-OBS-03 | Out-of-range values are stored with `quality_flag = suspect` and excluded from metrics. |
| BR-OBS-04 | Derived metrics inherit provenance disclosure: any metric computed with weighted observation types discloses the mix in its metadata. |

## 5. Provider & Location Rules (new — gap closures)

| ID | Rule | Closes gap |
|----|------|-----------|
| BR-PROV-01 | Provider timezone differences are normalized in the adapter (all `issued_at`/`target_time` UTC at ingest); the conversion is covered by adapter contract tests. | Gap #2 |
| BR-LOC-01 | Location deduplication: a new location within **0.05° (~5 km)** of an existing active location is rejected with a "possible duplicate of {name}" error unless `allow_near_duplicate` is explicitly set by the operator. | Gap #3 |
| BR-LOC-02 | MVP has no per-user location limits (single operator). Tier limits (3/25/unlimited) are Level 3 enforcement requirements, documented but not built. | Gap #4 |
| BR-LOC-03 | Disabling a location/provider stops future collection only; all historical data remains queryable and ranked (with coverage reflecting the stop). | Soft-delete behavior |
| BR-ATTR-01 | API responses containing provider-derived data include `attribution: {provider, text, url}`; the dashboard renders it in the footer of every data-bearing view. | Gap #7 |
| BR-LIC-01 | Derived accuracy metrics are ForecastIQ's own computations over lawfully stored inputs; ToS review (D-05) confirms whether publishing derived metrics requires additional notices per provider. Publication is gated on that review. | Gap #8 |

## 6. Freshness Rules (new — data freshness amendment)

State model: `fresh` → `delayed` → `stale` → `unavailable`.

| Data type | fresh | delayed | stale | unavailable |
|-----------|-------|---------|-------|-------------|
| Forecast collection (per provider-location) | < 75 min since last success | 75–180 min | > 180 min | no successful collection in 24 h, or circuit open |
| Observations (per location) | < 90 min | 90–240 min | > 240 min | none in 24 h |
| Rankings (per cell) | recomputed within 2 h of latest input | 2–6 h | > 6 h | inputs unavailable |
| Operational health | < 5 min | 5–15 min | > 15 min | health endpoint down |

| State | API representation | UI behavior | Alert behavior |
|-------|--------------------|-------------|----------------|
| fresh | `freshness: "fresh"`, `last_updated` | normal | none |
| delayed | `freshness: "delayed"` | amber badge "data delayed" | none (logged) |
| stale | `freshness: "stale"` | orange banner "data may be out of date (last update Xh ago)"; rankings shown with stale badge | operator alert (log + metrics threshold) |
| unavailable | `freshness: "unavailable"` | explicit empty state "no data — {reason if known}" | operator alert |

BR-FRESH-01: stale data is always shown **with** its staleness — never silently served
as current. BR-FRESH-02: freshness is computed server-side and included in every
time-sensitive payload.

## 7. Timezone Display Rules (new — closes Missing Req #14)

| ID | Rule |
|----|------|
| BR-TZ-01 | Storage and all API values: UTC (ISO 8601, `Z` suffix). |
| BR-TZ-02 | Dashboard default display: the **location's** timezone (from `locations.timezone`), because users compare against local daily life. |
| BR-TZ-03 | User override: a global "show times in browser timezone" toggle in Settings (default off). |
| BR-TZ-04 | Every displayed timestamp carries an explicit zone label (e.g., "18:00 MYT") — never bare numbers. |
| BR-TZ-05 | Chart axes for daily aggregations bucket by the location's local day (documented; API `tz` parameter echoes the bucketing zone). |

## 8. Metric Invalidation Rules (new — closes gap #6)

| ID | Rule |
|----|------|
| BR-INV-01 | When a corrected observation arrives, all MatchedEvaluations referencing the superseded observation are re-matched to the correction; affected AccuracyMetric and ProviderRanking rows are recomputed as **new rows** with `superseded_by` links on the old rows. |
| BR-INV-02 | Recomputation completes within the next two batch cycles (≤ 1 h) and is logged as an audit event. |
| BR-INV-03 | Historical (superseded) metric rows remain queryable with their original methodology version for reproducibility. |

## 9. Rule Change Governance

Business rules change only via the amendment process: proposal → review against
methodology/architecture constraints → version bump of this document → update of
affected acceptance criteria → ADR if material. Rules are versioned with this
document's version and referenced by ID in acceptance criteria.
