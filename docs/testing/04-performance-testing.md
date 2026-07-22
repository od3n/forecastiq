# ForecastIQ — Performance Testing (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: NFR-P01..P08, NFR-S01; `docs/data/06-data-growth-and-cost-model.md` §2.3

---

## 1. Objectives

1. Verify NFR latency/throughput targets at expected load.
2. Validate the data design at 2× MVP volume (NFR-S01 gate at Level 1 exit).
3. Establish baselines for promotion-trigger measurement (Redis, worker split, TimescaleDB decisions need evidence, not guesses).

## 2. Scenarios

| # | Scenario | Load | Target | Frequency |
|---|----------|------|--------|-----------|
| PT-1 | Dashboard read mix (rankings 50%, summary 25%, trends 15%, FvA 10%) | 100 VU, 10 min | p50 < 50 ms, p95 < 200 ms, p99 < 500 ms, 0 errors | Weekly + pre-release |
| PT-2 | Sustained throughput | Ramp to 100 req/s, 5 min hold | ≥ 100 req/s at p95 < 200 ms (NFR-P05) | Pre-release |
| PT-3 | Ingestion burst | 240 collections (simulated day) via fake provider in 5 min | Cycle completion < 5 min (NFR-P07); snapshot write p95 < 100 ms | Weekly |
| PT-4 | Analysis batch at volume | 100K matched pairs seeded | Batch < 10 min (NFR-P06) | Pre-release + on engine changes |
| PT-5 | Evolution query (Level 3 reserved) | Single provider-location-target across 30 d issuances | < 500 ms (documented baseline; not an NFR) | On snapshot index changes |
| PT-6 | Health assembly under polling | 2 concurrent operators, 60 s polling, 10 min | p95 < 200 ms (Q-07 contract) | Weekly |
| PT-7 | DB query baselines | Q-01/Q-04/Q-05/Q-09 patterns at 2× volume data | p95 < 100 ms (NFR-P08) | Level 1 exit (NFR-S01) + quarterly |
| PT-8 | Dashboard paint | Lighthouse CI on key screens | Meaningful paint < 2 s (NFR-P04); CLS < 0.1 | Every frontend PR |

## 3. Test Data (synthetic seeder)

| Dataset | Volume | Purpose |
|---------|--------|---------|
| Base | 10 locations × 2 providers × 30 d (≈ 1.5M snapshots, 3M matches) | PT-1/2/3/6 |
| Extended | 2× MVP annual rate subset (≈ 35M snapshots) | PT-7 NFR-S01 validation |
| Analysis | 100K pairs in matched_evaluations + inputs | PT-4 |

Seeder: deterministic (fixed seed) Go program generating physically plausible tropical weather values; committed to repo (`test/perf/seeder`). Never production data.

## 4. Environment

- k6 scripts in repo (`test/perf/k6/*.js`), run in CI (dedicated runner) against docker-compose environment with production-equivalent PostgreSQL 16 config (shared_buffers tuned to VPS class, not laptop defaults).
- Extended dataset runs on a scratch VPS (same class as production) quarterly — CI runner storage insufficient.
- Grafana dashboards capture test-run metrics (Go runtime, DB, latency) for comparison across runs.

## 5. Threshold Gates and Actions

| Result | Action |
|--------|--------|
| All targets met | Baseline recorded; ship |
| p95 miss < 20% on one scenario | Investigate (query plan, cache, pool); fix or document with ticket; not blocking if NFR still met at expected (not peak) load |
| NFR breach at expected load | Blocking; profile before any other work |
| PT-7 miss at 2× volume | TimescaleDB/index promotion evaluation with evidence (ADR-004 trigger process) |
| PT-4 > 10 min | Batch chunking/parallelism review; worker-split trigger evaluation |

## 6. Baseline Register (living table, updated per run)

| Date | Commit | PT-1 p95 | PT-2 req/s | PT-3 cycle | PT-4 batch | PT-7 p95 | Notes |
|------|--------|----------|-----------|-----------|-----------|----------|-------|
| (first run at Level 1 exit) | | | | | | | baseline |

## 7. Cross-Reference

- Growth model assumptions: `docs/data/06-data-growth-and-cost-model.md`
- Query plans under test: `docs/data/04-index-and-query-plan.md`
- Promotion triggers fed by these results: `docs/architecture/10-scaling-and-evolution.md`
