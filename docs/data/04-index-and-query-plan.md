# ForecastIQ — Index and Query Plan (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: `docs/data/01-query-and-index-requirements.md` (reconciliation, binding Q-01..Q-11); domain model §11; NFR-P01..P08

This document consolidates the reconciliation query requirements with the Phase 1 physical index set and adds the remaining operational queries. All latency targets per NFR-P01/P02/P08.

---

## 1. Physical Index Set

### 1.1 forecast_snapshots (partitioned monthly on target_time)

| Index | Columns | Serves |
|-------|---------|--------|
| PK (per partition) | (id, target_time) | Identity |
| `snapshots_dedup` (UNIQUE) | (provider_id, location_id, issued_at, target_time) | FC-03 dedup, ON CONFLICT |
| `snapshots_provider_loc_target` | (provider_id, location_id, target_time) | Q-02 evolution, Q-10 collection window, matching source scan |
| `snapshots_loc_target_horizon` | (location_id, target_time, forecast_horizon_minutes) | Q-09 FvA day payload |
| `snapshots_collection` | (forecast_collection_id) | Lineage drill-down (admin) |

### 1.2 observations (partitioned monthly on observed_at)

| Index | Columns | Serves |
|-------|---------|--------|
| PK (per partition) | (id, observed_at) | Identity |
| `observations_dedup` (UNIQUE) | (source, location_id, observed_at) | OC-03 dedup + source-filtered reads |
| `observations_loc_observed` | (location_id, observed_at DESC) | Q-01 context (latest), Q-03, Q-09, matching candidate scan |

### 1.3 forecast_collections

| Index | Columns | Serves |
|-------|---------|--------|
| `collections_dedup` (UNIQUE, partial) | (provider_id, location_id, COALESCE(model_run_time, requested_at)) WHERE status IN (success, partial) | Collection-level dedup |
| `collections_prov_loc_req` | (provider_id, location_id, requested_at DESC) | Q-07 health, Q-11 run history, latest-success queries |
| `collections_status_req` | (collection_status, requested_at DESC) | Admin failure listing (MVP: acceptable filtered scan; promote to partial index if measured) |

### 1.4 matched_evaluations

| Index | Columns | Serves |
|-------|---------|--------|
| `matches_dedup` (UNIQUE) | (forecast_snapshot_id, observation_id) | Idempotent matching |
| `matches_prov_loc_horizon_target` | (provider_id, location_id, forecast_horizon_minutes, target_time) | Batch aggregation scans, recomputation scope |
| `matches_observation` | (observation_id) | Correction impact lookup (rematch scope) |

### 1.5 accuracy_metrics

| Index | Columns | Serves |
|-------|---------|--------|
| `metrics_cell_period` | (provider_id, location_id, horizon_minutes, variable, metric_type, period_end DESC) | Q-05 trends, summary reads |
| `metrics_calculated` | (calculated_at DESC) | Engine-lag query (MAX) |

### 1.6 provider_rankings

| Index | Columns | Serves |
|-------|---------|--------|
| `rankings_loc_horizon_period` | (location_id, horizon_minutes, period_end DESC) | **Q-01 (additive index from reconciliation — verified added)** |
| `rankings_prov_loc_horizon_period` | (provider_id, location_id, horizon_minutes, period_end DESC) | Q-04 provider grid |
| `rankings_calculated` | (calculated_at DESC) | Batch freshness |

### 1.7 Remaining tables

| Table | Index | Serves |
|-------|-------|--------|
| collection_schedules | `(status, slot_time)` WHERE status = 'due' (partial); `(provider_configuration_id, slot_time)` unique | Slot claiming; generation dedup |
| schedule_runs | `(job_type, started_at DESC)` | Run history (S-13) |
| audit_events | `(created_at DESC)`; `(resource_type, resource_id, created_at DESC)` | S-14 listing |
| api_keys | `(key_prefix)` unique; `(user_id, revoked_at)` | Key lookup; user key listing |
| users | `(auth_subject)` unique; `(email)` unique | Provisioning; login mapping |
| export_jobs | partial unique (target_user_id) WHERE pending | 409 guard |
| provider_circuits | PK (provider_id) | Circuit reads |

**Total: 24 indexes.** Discipline: new indexes only with measured query path (pg_stat_statements review at Level 1 exit).

## 2. Query Catalog (operational additions to Q-01..Q-11)

### QX-01: Unmatched snapshots for matching batch

```sql
SELECT s.id, s.provider_id, s.location_id, s.target_time, s.forecast_horizon_minutes
FROM forecast_snapshots s
WHERE s.target_time BETWEEN $window_start AND $window_end
  AND s.target_time <= now() - interval '2 hours'
  AND NOT EXISTS (SELECT 1 FROM matched_evaluations m WHERE m.forecast_snapshot_id = s.id)
ORDER BY s.target_time
LIMIT 5000;
```

- Filter: time window (30 d lookback, bounded); anti-join via NOT EXISTS.
- Index: `snapshots_loc_target_horizon` scan within partitions + `matches_dedup` probe.
- Rows: ≤ 5,000/batch (chunked). Latency: < 2 s (batch context).
- Degradation: batch failure → retry next cycle (idempotent).

### QX-02: Aggregation input (pairs with values for one cell + period)

```sql
SELECT m.*, s.temperature_c AS f_temp, o.temperature_c AS o_temp, ...
FROM matched_evaluations m
JOIN forecast_snapshots s ON s.id = m.forecast_snapshot_id AND s.target_time = m.target_time
JOIN observations o ON o.id = m.observation_id AND o.observed_at = ...
WHERE m.provider_id = $1 AND m.location_id = $2 AND m.forecast_horizon_minutes = $3
  AND m.target_time BETWEEN $period_start AND $period_end;
```

- Join path: matches (leading index) → snapshots (PK composite) → observations (PK composite). All index probes.
- Rows: ≤ 2,160 per cell-period (90 d × 24 h). Batch processes ≤ 700 cells.
- Latency: < 10 min total (NFR-P06 at 100K pairs).

### QX-03: Coverage computation

```sql
SELECT COUNT(*) FILTER (WHERE temperature_c IS NOT NULL) AS with_value, COUNT(*) AS total
FROM forecast_snapshots
WHERE provider_id = $1 AND location_id = $2 AND target_time BETWEEN $ps AND $pe;
```

- Index: `snapshots_provider_loc_target` (index-only scan possible with INCLUDE — deferred until measured; heap fetch acceptable at this volume).
- Expected snapshots denominator: from schedule config × active days (application-side).

### QX-04: Reliability computation

```sql
SELECT COUNT(*) FILTER (WHERE collection_status IN ('success','partial')) AS ok,
       COUNT(*) AS total
FROM forecast_collections
WHERE provider_id = $1 AND location_id = $2 AND requested_at BETWEEN $ps AND $pe
  AND error_code IS DISTINCT FROM 'system_error';  -- FC-13: our failures excluded
```

- Index: `collections_prov_loc_req`. Rows: ≤ 2,160 per cell-period.

### QX-05: Slot claiming (scheduler hot path)

```sql
SELECT id, job_type, location_id, slot_time FROM collection_schedules
WHERE status = 'due' AND slot_time <= now()
  AND (next_retry_at IS NULL OR next_retry_at <= now())
ORDER BY slot_time
LIMIT 10
FOR UPDATE SKIP LOCKED;
-- then: UPDATE ... SET status='claimed', claimed_by=$1, claimed_at=now(),
--        lease_expires_at = now() + interval '5 minutes' WHERE id = ...
```

- Index: partial `(status, slot_time) WHERE due`. Short tx (< 10 ms). Multi-instance safe.

### QX-06: Expired lease recovery

```sql
UPDATE collection_schedules
SET status = 'due', claimed_by = NULL, claimed_at = NULL, lease_expires_at = NULL,
    attempts = attempts + 1, next_retry_at = now() + backoff(attempts)
WHERE status = 'claimed' AND lease_expires_at < now();
```

- Runs each scheduler cycle. Rows: ~0 normally.

## 3. Risk Audit (extends reconciliation §3)

| Risk | Verdict |
|------|---------|
| N+1: per-location rankings | Eliminated (single-payload endpoints; Q-04 single query) |
| N+1: per-provider metrics | Eliminated (`/accuracy/summary` all-in-one) |
| Unbounded scans: /forecasts, /observations | Required filters (location_id + time range ≤ 31 d) + limit ≤ 200 (OpenAPI-enforced) |
| Expensive COUNT(*) | No total_count anywhere (API-02); coverage counts bounded by period+cell |
| High-cardinality filters | All filter columns are low-cardinality enums/UUIDs with composite indexes |
| Materialization candidates | None needed: metrics/rankings already persisted projections; materialized views add refresh-state complexity for zero measured benefit (reconsider only at TimescaleDB promotion) |
| Partition over-scans | Day queries touch 1–2 monthly partitions (Q-09); ranking/metric tables unpartitioned (small: ≤ 10⁵ rows) |
| JSONB filtering | None in hot paths (payload only) |

## 4. Pagination Strategy

- Cursor = base64(keyset tuple) on (sort_field, id) — stable under inserts (append-only tables).
- Keyset pagination everywhere (no OFFSET): `WHERE (sort_col, id) < ($last_sort, $last_id) ORDER BY sort_col DESC, id DESC LIMIT $n`.
- Whitelisted sort fields per endpoint (OpenAPI enum).

## 5. Cacheability Summary (per endpoint class)

| Class | LRU | ETag | Cache-Control |
|-------|-----|------|---------------|
| Rankings/summary/methodology | 60 s | strong | public, max-age=60 |
| FvA past dates | 300 s | strong | public, max-age=300 |
| FvA today | 60 s | strong | public, max-age=60 |
| Locations/providers | 300 s | strong | public, max-age=300 |
| Trends | 60 s | strong | public, max-age=60 |
| Admin/* | none | none | no-store |
| Mutations | none | none | no-store |

Detail: `docs/api/08-caching-and-partial-results.md`.

## 6. Cross-Reference

- Reconciliation queries (Q-01..Q-11, authoritative): `docs/data/01-query-and-index-requirements.md`
- Table DDL: `docs/data/03-table-design.md`
- Growth model: `docs/data/06-data-growth-and-cost-model.md`
