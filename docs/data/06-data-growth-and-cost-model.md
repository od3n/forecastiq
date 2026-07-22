# ForecastIQ — Data Growth and Cost Model (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: ADR-004 (workload assumptions); constraints §5; NFR-S01; MVP scope §2.3

---

## 1. Workload Assumptions (explicit, per ADR-004 mandate)

| Parameter | MVP value | Basis |
|-----------|-----------|-------|
| Locations | 5–10 (design headroom: 40) | MVP scope §2.3 (≤ 10; OpenWeather free key headroom 4×) |
| Forecast providers | 2 (Open-Meteo, OpenWeather) | ADR-002 |
| Collection frequency | Hourly (one call yields all horizons) | MVP scope §2.3 |
| Forecast periods per response | Open-Meteo: 168 (7 d hourly); OpenWeather OneCall: 48 (hourly) | Provider API shapes |
| Observation sources | 1 (Open-Meteo Historical) | ADR-003 |
| Observation frequency | Hourly per location (2 h backfill window per call) | OC-01 |
| Evaluation batch | Every 30 min | CE-09 |
| Retention | Snapshots 2 y, observations 5 y, metrics indefinite, payloads 90 d | BR-09 |
| API traffic | ~10 concurrent users typical; 100 concurrent design (NFR-S03); ~500K requests/month assumed | Portfolio MVP |

## 2. Row Growth Model

### 2.1 Daily (10 locations × 2 providers)

| Table | Formula | Rows/day |
|-------|---------|----------|
| forecast_collections | 10 loc × 2 prov × 24 h | 480 |
| forecast_snapshots | OM: 10 × 24 × 168 = 40,320; OW: 10 × 24 × 48 = 11,520 | ~51,840 (worst case; dedup reduces ~10–20% for overlapping model runs) |
| observations | 10 loc × 24 h (2 h window deduped) | 240 |
| matched_evaluations | ≈ snapshots with elapsed target + available observation ≈ 48,000 eligible × ~95% | ~45,000 |
| accuracy_metrics | 2 prov × 10 loc × 7 horizons × ~6 variables × ~14 metric types × daily period | ~11,760 (daily granularity; weekly/monthly add ~20%) ≈ 14,000 |
| provider_rankings | 2 prov × 10 loc × 7 horizons × 48 batches/day (rolling, superseded) | ~6,720 |
| audit_events | Sporadic admin + auth | ~100 |
| schedule_runs | 480 collections + 240 obs + 48 batches + maintenance | ~800 |

**Snapshot rate:** ~51,840/day ≈ 0.6 rows/s average; burst at collection ≈ 700 rows/s for ~30 s twice per hour. Trivial for PostgreSQL (ADR-004 validated).

### 2.2 Annual and Steady-State

| Table | Rows/year | Steady-state (with retention) | Size (steady) |
|-------|-----------|-------------------------------|---------------|
| forecast_snapshots | ~18.9M (with dedup ~16M) | 2 y → ~32M rows | ~8–10 GB (rows ~250 B + index overhead) |
| observations | ~87,600 | 5 y → ~438K rows | < 200 MB |
| forecast_collections | ~175K | indefinite → ~350K at 2 y | < 300 MB |
| matched_evaluations | ~16.4M | 2 y → ~33M rows | ~5 GB |
| accuracy_metrics | ~5.1M | indefinite (superseded accumulate) → 10 y ~51M... **mitigated**: superseded daily metrics purged after 2 y (only latest + monthly kept indefinitely — see §4) | ~2 GB managed |
| provider_rankings | ~2.5M | same supersede policy → ~1 GB managed | ~1 GB |
| raw payloads | ~480 files/day × ~15 KB gzipped ≈ 7 MB/day | 90 d → ~630 MB | < 1 GB |

**Total steady-state DB: ~17–19 GB** (snapshots + matches dominate). Well within managed tiers (3 GB entry → 25 GB tier at ~18 months) and far below the 100 GB TimescaleDB trigger.

### 2.3 NFR-S01 Validation (100M snapshot rows)

- Partitioning: monthly partitions of ~1.6M rows each — B-tree depth 3–4, all hot queries partition-prune.
- Load test requirement: at Level 1 exit, synthetic load at 2× MVP volume (100K snapshots/day) validates p95 < 100 ms on Q-01/Q-05/Q-09 patterns; extrapolation to 100M documented.
- Beyond 100M: TimescaleDB promotion per ADR-004 triggers (not anticipated before year 3 at MVP cadence).

## 3. API Traffic Model

| Endpoint class | Requests/month (assumed) | Notes |
|----------------|--------------------------|-------|
| Public rankings/summary | 300K | 60 s LRU + ETag absorb polling |
| FvA | 100K | Past dates cached 300 s |
| Trends | 50K | ≤ 365-row scans |
| Raw data (user+) | 30K | Key rate-limited |
| Admin | 5K | Operator only |
| Auth-adjacent | 15K | Supabase absorbs most |

DB reads: ~80% served from LRU or ETag 304 → effective DB query rate < 20/s at 500K req/month. No read pressure concerns.

## 4. Metric/Ranking Supersede Management

Indefinite retention of all superseded metric rows would grow unbounded. Policy:
- **Rankings:** keep all rows for 2 y; older superseded rows purged (latest per cell-period + monthly snapshots retained indefinitely for reproducibility claims).
- **Metrics:** daily-granularity superseded rows purged after 2 y; weekly/monthly and the current version retained indefinitely.
- Reproducibility preserved: current rows + monthly aggregates suffice for any published claim; purge documented as acceptable (methodology §9 recomputation remains possible from immutable matches within their 2 y window).

## 5. Cost Model (monthly)

### 5.1 Low-use (5 locations, light traffic)

| Item | $/mo |
|------|------|
| VPS CX22-class (2 vCPU/4 GB) | 7 |
| Volume 20 GB | 2 |
| Managed DB entry paid tier (3 GB) | 20 |
| Everything else | 1.5 |
| **Total** | **~$31** |

### 5.2 Expected portfolio (10 locations, moderate traffic)

| Item | $/mo |
|------|------|
| VPS CX32 (4 vCPU/8 GB) | 12 |
| Volume 50 GB | 5 |
| Managed DB (25 GB tier at ~18 mo; 10 GB before) | 20–25 |
| Domain | 1.5 |
| Offsite backup (B2) | 3 |
| **Total** | **~$42–47** |

### 5.3 Growth (10× traffic, 40 locations — requires provider tier upgrade)

| Item | $/mo |
|------|------|
| VPS CX42-class | 25 |
| Volume 100 GB | 10 |
| DB 50 GB tier | 45 |
| OpenWeather paid tier (if 40 locations) | 40+ |
| **Total** | **~$120–160** |

Still within the $500 constraint ceiling (C-03) at 10× — the architecture's cost ceiling is the provider API tier, not infrastructure.

### 5.4 Largest Cost Drivers

1. Managed DB storage tier (driven by snapshot/match volume — retention is the lever).
2. Provider API tiers (driven by location count — scope is the lever).
3. VPS compute (fixed; cheapest lever).

### 5.5 Cost Controls

- Grafana Cloud billing alert at free-tier 80% usage.
- Hetzner budget notification.
- Monthly volume review (retention doc §7) catches storage drift before tier jumps.
- DB tier upgrade is a deliberate operator action (managed vendor), never automatic.

## 6. Cross-Reference

- ADR-004 (PostgreSQL decision with these assumptions)
- Retention: `docs/data/05-retention-and-archival.md`
- Deployment cost table: `docs/architecture/06-deployment-architecture.md` §10
- Risk: R-13 (storage growth)
