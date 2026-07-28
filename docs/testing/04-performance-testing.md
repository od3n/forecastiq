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
| 2026-07-28 | `feature/wp26b-performance-validation` | 1.56 ms (0 err / 71,752 req) | 105.0 sustained (p95 2.77 ms) | 6.77 s (240/240; per-coll p95 79 ms) | 30 s @ ~147 K pairs | base volume: Q-01 4.5 / Q-04 1.7 / Q-05 5.3 / Q-09 36.6 ms | first populated baseline — local Docker `fiqperf` stack (postgres:16-alpine, Apple-silicon host), base preset (1,497,600 snapshots / 2.71 M pairs). PT-6 p95 123.5 ms; PT-8 FCP 273–353 ms, CLS ≈ 0.025. Baseline runs surfaced + fixed the missing `metrics_calculated` index (migration `20260801000013`). |
| 2026-07-28 | `328e960` (main, WP-26b accepted) | — | — | — | — | **2× volume (NFR-S01 Level-1 exit): Q-01 2.0 / Q-04 0.7 / Q-05 20.5 / Q-09 79.8 ms — ALL < 100 ms** | **TC-26b-01 executed**: scratch EC2 per §4 — seeded on r6i.large (extended preset, 111,560,766 rows / ~65 GB in 2 h 42 m: 36,441,600 snapshots = 2× MVP annual, 71,658,646 pairs), then **measured on t3.small** (production class, ADR-033; 2 vCPU / 2 GB, swap off, postgres:16-alpine `shared_buffers=512MB`), 200 iterations/pattern. No index/TimescaleDB promotion triggered (§5). Instance, SG, and key pair torn down after the run. |

> **NFR-S01 status: SATISFIED at Level-1 exit** — the 2×-volume PT-7 run above
> passed all four patterns under the p95 < 100 ms gate on production-class
> hardware. Quarterly re-runs per §2 (scratch VPS, `--preset=extended`).
> Weekly CI re-runs PT-1/PT-3/PT-6/PT-7 (base volume) + both reliability
> suites (`.github/workflows/scheduled.yml`).

## 7. Cross-Reference

- Growth model assumptions: `docs/data/06-data-growth-and-cost-model.md`
- Query plans under test: `docs/data/04-index-and-query-plan.md`
- Promotion triggers fed by these results: `docs/architecture/10-scaling-and-evolution.md`
