# ForecastIQ — Query & Index Requirements

**Version**: 1.0 (UI ↔ Backend Reconciliation Board)
**Status**: Authoritative — binding for Phase 1 schema/migration design
**Inputs**: `docs/domain/01-domain-model.md` §11 (existing indexes); `docs/api/01-screen-api-contracts.md`; NFR-P01..P08
**Scale basis**: MVP = 5–10 locations × 2 providers × hourly × 168 target hours ≈ 30K snapshots/day (architecture §5 headroom). Design validated to 100M snapshot rows (NFR-S01) via partitioning + the access paths below.

---

## 1. High-Value Screen Queries

### Q-01: Latest rankings for one location (+ observation context) — S-01

| Attribute | Value |
|-----------|-------|
| Filter pattern | `provider_rankings WHERE location_id = ? AND horizon_minutes = ? AND period_end = (latest for cell)` |
| Sort | ranking_status priority (ranked → provisional → unranked), then composite DESC, tie groups by CI |
| Estimated rows | ≤ 10 (providers) |
| Result size | < 16 KB |
| Required index | `provider_rankings (location_id, horizon_minutes, period_end DESC)` — **verify exists**; domain model §11 lists `(provider_id, location_id, horizon_minutes, …)` — add location-first index if absent (one additive index) |
| Aggregation cost | None (pre-computed rows); latest-period selection via `DISTINCT ON` or window over ≤ 10×versions rows |
| Cache suitability | High — LRU + ETag 60s |
| Acceptable latency | p50 < 50ms, p95 < 200ms |
| Fallback | Stale-cache serving with staleness label (NFR-A07) |
| Observation context subquery | `observations WHERE location_id = ? ORDER BY observed_at DESC LIMIT 1` — index `(location_id, observed_at)` (exists, also uniqueness via source-prefixed index) |

### Q-02: Forecast evolution for one target time (Level 3 reserved — documented per C-13)

| Attribute | Value |
|-----------|-------|
| Query | All available predictions by one provider for one target_time, ordered by issued_at, joined to the matched observation |
| SQL shape | `SELECT issued_at, value FROM forecast_snapshots WHERE provider_id=? AND location_id=? AND target_time=? ORDER BY issued_at` + observation lookup at target hour |
| Required index | `(provider_id, location_id, target_time)` — **exists** (domain §11); issued_at ordering within ≤ ~720 rows (30d × hourly issuances) is a cheap sort |
| Expected volume | Low (analytical drill-down); not on MVP critical path |
| Verdict | **No new index required now.** Documented to prove Level 3 Forecast Evolution is additive. |

### Q-03: Latest observation for one location — S-01 context, S-10 collector status

| Attribute | Value |
|-----------|-------|
| Filter | `observations WHERE location_id = ? ORDER BY observed_at DESC LIMIT 1` (per location for S-10: GROUP BY location over active set) |
| Index | `(location_id, observed_at)` — exists |
| Rows | 1 per location |
| Latency | < 10ms |
| Suspect count (S-10) | `COUNT(*) WHERE location_id=? AND quality_flag='suspect' AND observed_at > now()-interval '24h'` — same index range scan, ≤ 24 rows |

### Q-04: Provider comparison grid for one period and horizon set — S-03

| Attribute | Value |
|-----------|-------|
| Filter | `provider_rankings WHERE provider_id = ? AND period_end = latest` across all locations × horizons |
| Index | `provider_rankings (provider_id, location_id, horizon_minutes, variable, period_end DESC)` — exists (domain §11 accuracy_metrics analog; verify ranking-table equivalent `(provider_id, …)` present) |
| Rows | ≤ 10 locations × 7 horizons = 70 |
| Result size | < 15 KB |
| Aggregation | None (pre-computed) |
| N+1 guard | Single query — the endpoint exists precisely to prevent per-location fan-out (C-08/DR-15) |

### Q-05: Metric trend across time — S-04

| Attribute | Value |
|-----------|-------|
| Filter | `accuracy_metrics WHERE provider_id=? AND location_id=? AND horizon_minutes=? AND variable=? AND metric_type=? AND period_start BETWEEN ? AND ?` |
| Sort | period_start |
| Index | `(provider_id, location_id, horizon_minutes, variable, period_end DESC)` — exists; period_start range within leading-prefix match |
| Rows | ≤ 365 (daily) — typically 30 |
| Bucketing | `date_trunc(?, period_start AT TIME ZONE ?)` — applied post-scan on ≤ 365 rows (trivial) |
| Aggregation cost | None (rows are the aggregation) |
| Latency | p95 < 100ms |

### Q-06: Ranking table full payload — S-01 (same as Q-01) + S-02 compact summary

Identical access path; S-02 reuses the S-01 response via client cache (same ETag) — no second server hit on navigation.

### Q-07: Collection-health summary — S-10

| Attribute | Value |
|-----------|-------|
| Per-cell query | `forecast_collections WHERE provider_id=? AND location_id=? ORDER BY requested_at DESC LIMIT 1` (+ status counts over 24h window) |
| Index | `(provider_id, location_id, requested_at DESC)` — exists |
| Cells | ≤ 20 (2 providers × 10 locations) |
| Assembly | Single grouped query over 24h window (≤ 480 rows) + circuit table (≤ 2 rows) + statfs + `MAX(calculated_at)` on accuracy_metrics (index tail) |
| Latency | p95 < 200ms under 60s polling by 2 operators |

### Q-08: Locations requiring attention — S-10 summary bar

Derived from Q-07 assembly (freshness state counts) — no separate query.

### Q-09: Forecast-vs-Actual day payload — S-05

| Attribute | Value |
|-----------|-------|
| Snapshot query | `forecast_snapshots WHERE location_id=? AND target_time BETWEEN day_start AND day_end AND forecast_horizon_minutes=? AND provider_id = ANY(?)` |
| Index | `(location_id, target_time, forecast_horizon_minutes)` — exists; partition pruning on monthly `target_time` partition (day query touches exactly 1–2 partitions) |
| Observation query | `observations WHERE location_id=? AND observed_at BETWEEN day bounds` — index `(location_id, observed_at)` |
| Day metrics | Computed over ≤ 24 × 2 pairs in-memory (methodology formulas) — no aggregation query |
| Rows | ≤ 24×2 snapshots + 24 observations |
| Result size | < 20 KB |
| Latency | p95 < 200ms |

### Q-10: Collection window (S-02/S-03)

`SELECT MIN(created_at), MAX(created_at) FROM forecast_snapshots WHERE provider_id=? AND location_id=?` — index `(provider_id, location_id, target_time)` provides MIN/MAX via index-only scan ends. Rows: 1 per provider-location. Cost: O(log n) each.

### Q-11: Run history — S-13

`forecast_collections WHERE [provider/location/status filters] ORDER BY requested_at DESC LIMIT ? CURSOR ?` — index `(provider_id, location_id, requested_at DESC)`; status-filtered variants acceptable at MVP volume (≤ 480 rows/day) via filtered scan + sort; add `(status, requested_at DESC)` only if measured (promotion-style discipline).

## 2. Index Verdicts

| Index | Status | Justification |
|-------|--------|---------------|
| `forecast_snapshots (provider_id, location_id, target_time)` | Exists (domain §11) | Q-02, Q-10, evolution reserved |
| `forecast_snapshots (location_id, target_time, forecast_horizon_minutes)` | Exists | Q-09 |
| `observations (location_id, observed_at)` | Exists | Q-01 context, Q-03, Q-09 |
| `observations (source, location_id, observed_at)` | Exists (uniqueness) | Dedup + source-filtered queries |
| `forecast_collections (provider_id, location_id, requested_at DESC)` | Exists | Q-07, Q-11 |
| `matched_evaluations (provider_id, location_id, forecast_horizon_minutes, target_time)` | Exists | Batch matching + recomputation |
| `accuracy_metrics (provider_id, location_id, horizon_minutes, variable, period_end DESC)` | Exists | Q-05 |
| `provider_rankings (location_id, horizon_minutes, period_end DESC)` | **Add if absent** | Q-01 location-first access (domain §11 lists provider-first; verify at migration design; one additive B-tree) |
| `provider_rankings (provider_id, location_id, horizon_minutes, period_end DESC)` | Verify covers Q-04 | Provider-first grid scan |
| `provider_circuits (provider_id PK)` | New table (§2.3 domain reconciliation) | Q-07 circuit join |
| `export_jobs (target_user_id) WHERE status='pending'` | New partial unique | 409 guard |

**No other new indexes.** Discipline: indexes added only with a measured query path (NFR-P08 pg_stat_statements review at Level 1 exit).

## 3. N+1 and Unbounded-Query Risk Audit

| Risk | Location | Verdict |
|------|----------|---------|
| Per-location rankings fan-out (S-03 grid) | Frontend composition | **Eliminated by design**: `/accuracy/summary?provider_id=` single payload (C-08) |
| Per-provider metric fetch (S-02 tables) | Frontend composition | Eliminated: `/accuracy/summary` returns all providers in one payload |
| Unbounded `/forecasts` scans | Raw endpoint (user+) | Bounded by required filters: `location_id` required + time range required (max 31d) + limit ≤ 200 (API-02 cursor). OpenAPI validation enforces. |
| Unbounded `/observations` scans | Raw endpoint (user+) | Same filter requirements as forecasts |
| Audit log growth scan | S-14 | Cursor pagination + `(created_at DESC)` scan; retention 1y caps size (~100K rows) |
| Trend bucket fan-out | S-04 | Single query; bucketing post-scan on ≤ 365 rows |
| Health polling cost | S-10 | Bounded assembly (≤ 480-row window scan); 60s × ≤ 2 operators = negligible |
| Export row explosion | CSV | Client-side over already-bounded view data (§5 response-conventions) |

## 4. Partitioning Verification

Monthly declarative partitions on `forecast_snapshots.target_time` and `observations.observed_at` (domain §11):
- Q-09 day queries prune to 1–2 partitions ✓
- Retention drops (2y snapshots / 5y observations) are partition drops ✓
- Q-01/Q-04/Q-05 hit pre-computed tables (unpartitioned, small: rankings ≤ 10 providers × 10 locations × 7 horizons × versions ≈ 10⁴ rows; metrics ≈ 10⁵ rows at 2y daily granularity) — no partitioning needed ✓

## 5. Latency Budget Summary

| Query class | p50 | p95 | Mechanism |
|-------------|-----|-----|-----------|
| Rankings/summary reads (Q-01, Q-04, Q-06) | < 50ms | < 200ms | Pre-computed rows + LRU + ETag |
| Trend reads (Q-05) | < 50ms | < 100ms | Indexed scan ≤ 365 rows |
| Comparison payload (Q-09) | < 80ms | < 200ms | Two indexed queries + in-memory day metrics |
| Health assembly (Q-07) | < 80ms | < 200ms | Bounded window scan |
| Admin lists (Q-11) | < 50ms | < 200ms | Indexed + cursor |
| Mutations | < 100ms | < 500ms | Single-row writes (trigger/replay bounded by provider call time, separate budget < 2s) |

All within NFR-P01/P02/P08. **No TimescaleDB, Redis, read replica, or materialized view required** — promotion criteria (architecture §4) remain the only scale path, triggered by measurement.
