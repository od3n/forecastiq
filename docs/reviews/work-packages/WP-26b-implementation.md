# WP-26b — Performance Validation Completion: Implementation Report

**Date**: 2026-07-28
**Branch**: `feature/wp26b-performance-validation` (base: accepted `main` `49c67b6`)
**Scope authority**: `docs/planning/05-implementation-work-packages.md` §WP-26b;
`docs/reviews/work-packages/WP-26-delivery-review.md` (DRB-WP26-001/002/006 remainders);
`docs/testing/04-performance-testing.md`

---

## 1. Scope delivered

WP-26b completes the WP-26 scaffold. All items from the §WP-26b scope table:

| Item | Status |
|------|--------|
| Functional seeder DB writes | ✅ `test/perf/seeder` writes all six data tables (COPY-streamed) |
| PT-3 ingestion burst | ✅ `test/perf/pt3` harness; run green |
| PT-4 analysis batch (NFR-P06) | ✅ `test/perf/pt4-analysis-batch.sh`; run green (30 s ≪ 600 s) |
| PT-7 query baselines (NFR-P08) | ✅ `test/perf/pt7`; green at base volume; extended-preset run pending scratch VPS (§6) |
| PT-8 Lighthouse | ✅ `frontend.yml` LHCI step + `web/lighthouserc.json`; local run green |
| 5 fault-injection reliability scenarios | ✅ `test/perf/reliability-faults.sh`; 15/15 checks green |
| Baseline register populated | ✅ perf doc §6 (this branch) |
| 2× volume load test (NFR-S01 Level-1 exit) | ◐ capability delivered (`--preset=extended`); execution requires the scratch-VPS environment per perf doc §4 (CI/laptop storage+runtime insufficient) — see §6 |
| Weekly scheduled wiring | ✅ `scheduled.yml` `weekly-perf-reliability` job (DRB-WP26-006) |

## 2. Functional seeder (slice 1)

`test/perf/seeder` (was: estimate-only scaffold, exit-1):

- **Deterministic generation** (`gen.go`): splitmix64-hashed physically plausible
  tropical weather (diurnal 24–33 °C, humid, afternoon convective rain);
  uuid-v5 row ids re-derivable across passes. Same `--seed` + anchor hour ⇒
  byte-identical dataset (verified via md5 over a re-seed).
- **Volume model** = doc §3 fan-out preserved from the scaffold (24 runs/day ×
  104 forecast rows: hourly to +72 h, 3-hourly to +168 h). Base preset measured:
  **1,497,600 snapshots** (≈1.5 M ✅), **2,712,518 pairs** (≈2× eligible
  snapshots — every observed hour carries an original pair + a rematch pair to
  the correcting row; future-target tails unmatched, mirroring production).
- **Correction realism**: per location-hour an original row (superseded) + a
  correcting live row (~2 % flagged `suspect`), satisfying the partial dedup
  index and supersession trigger. Observations use source `perf_synthetic` so
  the live collector's `openmeteo_historical` rows can never race the
  (source, location, hour) dedup boundary; matching/serving are source-agnostic.
- **Derived-table backfill**: daily `accuracy_metrics` (27 rows/cell-day — the
  aggregation engine's exact row plan) and live `provider_rankings`
  (`component_scores` in the engine's `ComponentScore` JSON shape, w-2026.1
  weights) so Q-01/Q-05 serve realistic volume immediately.
- **Safety**: historical partitions via `create_monthly_partition`; catalog
  writes insert-only (canonical seed ids; operator state never clobbered);
  refuses to write into non-empty data tables without `--reset` (TRUNCATE);
  dataset anchored 3 h behind now (live-stack collision guard);
  DRB-WP26-003 URL redaction and DRB-WP26-004 non-zero-exit-on-no-write kept.
- Presets: `base` 10×2×30 d; `extended` 20×2×365 d (2× MVP annual, PT-7/NFR-S01);
  `analysis` retuned to 2×2×10 d ⇒ ~100 K pairs *inside the 30 d match window*
  (the previous 5×2×60 d spec put most pairs outside it).

## 3. PT harnesses (slice 2a)

- **PT-3** (`test/perf/pt3`): 240 collections (24 simulated hourly model runs ×
  10 locations) through the **real** `CollectService` (classify → partitions →
  tx → snapshot COPY/dedup → payload store → events) with an in-process fake
  provider — the doc's "via fake provider". Distinct `ModelRunTime` per run so
  every collection inserts fresh rows; anchored 2 years back so burst rows can
  never shadow serving reads. Gates: cycle < 5 min (NFR-P07), per-collection
  p95 < 100 ms (brackets the snapshot write), 240/240 success.
- **PT-4** (`test/perf/pt4-analysis-batch.sh`): times `POST /admin/recompute`
  (match → aggregate → rank) over the seeded analysis preset; gate < 10 min.
- **PT-7** (`test/perf/pt7`): Q-01/Q-04/Q-05/Q-09 access patterns
  (query-requirements doc) with rotating parameters, warm-up excluded,
  p50/p95/p99/max per pattern; gate p95 < 100 ms (NFR-P08).
- **PT-8**: `frontend.yml` runs `lhci autorun` after the static-export build;
  `web/lighthouserc.json` pins the four key screens (dashboard, trends, FvA,
  methodology), desktop preset, assertions FCP < 2000 ms (NFR-P04) + CLS < 0.1,
  filesystem report only (no external upload).

## 4. Reliability fault injection (slice 2b — closes the DRB-WP26-001 remainder)

`test/perf/reliability-faults.sh` + `test/perf/fakeprovider.py` (stdlib fake
Open-Meteo: `ok` = fresh-timestamped valid openmeteo-v1 payload; `hang` = holds
the socket) + `test/perf/compose.perf.yml` (isolated `fiqperf` project: shifted
ports 28080/29090/25432, own volumes, dev-verifier auth, `PERF_RATE_LIMIT`
switch — the DRB-WP26-005 env exclusion made explicit):

| # | Scenario | Injection | Assertions (15 total, all green) |
|---|----------|-----------|----------------------------------|
| 1 | Provider timeout | `api_base_url` → hanging fake | `healthz` responsive during the stall; collection classified `timeout` |
| 2 | Duplicate job | same-hour double trigger | both 200; second recorded `deduplicated`, 0 stored, all received deduped |
| 3 | Late observation | supersede-then-insert correcting row → recompute | rematch adds pair(s) for the correcting row; original pairs retained (append-only) |
| 4 | Worker restart | `docker compose restart app` | `readyz` recovers; public read serves |
| 5 | DB reconnect | `stop`/`start postgres` | `readyz` drops while down, recovers after; public read serves |

Request-path checks remain in `reliability.sh` (WP-26 slice 1, unchanged);
both suites run in the weekly job under the DEFAULT limiter.

## 5. Defects found and fixed by the baseline runs

Running the scaffold against real data surfaced four defects (the point of
WP-26b):

1. **Missing documented index** (product defect, **migration
   `20260801000013`**): index doc §1.5 specifies `metrics_calculated
   (calculated_at DESC)` for the engine-lag query, but migration `…0009` never
   created it — at base volume `/admin/health` seq-scanned ~150 K rows
   (~140 ms/query; PT-6 p95 1.33 s ≫ 200 ms gate). With the index: warm health
   assembly 11–15 ms.
2. **PT-1 sent contract-invalid params** (`/accuracy` without required
   `variable`+`metric_type`; `/forecast-comparison` with `temperature_2m`) —
   25 % of the mix 422'd. Fixed to the S-04/S-05 contracts.
3. **PT-2 gate unsatisfiable**: a constant-arrival-rate of exactly 100/s
   aggregates fractionally under 100/s, so `iterations rate>=100` could never
   pass. Drive 105/s — the NFR-P05 floor is proven without relaxing the gate.
4. **`curl -w` + `|| echo 000`** concatenates to `000000` on transport failure
   (pt4/faults scripts); k6 default `LOCATION_ID` was the workspace id.

## 6. Baseline register (perf doc §6)

Populated from this branch's runs (local Docker perf stack `fiqperf`,
postgres:16-alpine, Apple-silicon host; commit context in the register row):

- PT-1 p95 **1.56 ms** (p50 439 µs; 0 errors / 71,752 requests; 100 VU × 10 min)
- PT-2 **105.0 req/s sustained** (p95 2.77 ms; 0 errors / 31,501 — ≥ 100 req/s NFR-P05 proven)
- PT-3 cycle 6.77 s (240/240 success; p95 79 ms)
- PT-4 batch 30 s at ~147 K in-window pairs (records_affected 34,112)
- PT-6 p95 **123.5 ms** (quiet stack; the first run breached at 1.33 s → 429 ms — missing index, then overlap with saturating k6 + a concurrent recompute; health assembly is 13–16 ms warm even during a recompute)
- PT-7 (base volume) Q-01 p95 4.5 ms · Q-04 1.7 ms · Q-05 5.3 ms · Q-09 36.6 ms
- PT-8 key screens FCP 273–353 ms, CLS ≈ 0.025 (gates 2000 ms / 0.1)

**NFR-S01 (2× volume) status**: the `extended` preset (20 loc × 2 prov × 365 d
≈ 35 M snapshots / ~70 M pairs) is implemented and the PT-7 gate encodes the
Level-1 exit criterion, but the run itself needs the scratch-VPS environment
the perf doc §4 prescribes (CI-runner/laptop storage and multi-hour seeding
budget). The register's PT-7 row is recorded at base volume with the 2× run
tracked as the remaining Level-1-exit execution step — an operational run, not
a code gap.

## 7. CI wiring

- `scheduled.yml` `weekly-perf-reliability` (weekly Monday tier + on-demand):
  isolated `fiqperf` stack → seeder base preset → PT-3 → PT-7 (base-volume
  trend) → k6 PT-1/PT-6 under `PERF_RATE_LIMIT=100000` → DEFAULT limiter
  restored → `reliability.sh` → `reliability-faults.sh`; logs dumped on
  failure. Closes DRB-WP26-006; the stale header comment is gone.
- `frontend.yml`: PT-8 LHCI step (every frontend PR, per doc §2).
- `Makefile`: `perf-up/down/seed/pt3/pt4/pt7/k6/reliability` targets.

## 8. Local gate

`gofmt` / `go vet` / `golangci-lint` clean; `go test -race ./...` all ok;
`bash -n` on all suite scripts; six-job CI evidence to be captured on push
(PR to `main`).

## 9. Deviations / notes

- PT-3 measures the per-collection service latency (which brackets the
  snapshot write) rather than a DB-only write timer; documented in the harness
  header — the gate is strictly harder than "snapshot write p95".
- PT-4's analysis preset is 2×2×10 d (in-window pairs), not the WP-26-era
  5×2×60 d (mostly outside the match window).
- Weekly tier runs PT-7 at base volume as an early-warning trend; the doc's
  quarterly 2× cadence is unchanged.
- `provider_condition_code` left NULL by the seeder (canonical codes set);
  no serving read depends on it.
