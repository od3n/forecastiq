# ForecastIQ — WP-16 Forecast Evolution API: Implementation Report

**Version**: 1.0
**Implementation date**: 2026-07-24
**Work package**: WP-16 — Forecast Evolution API (Forecast-vs-Actual)
**Authority**: `docs/planning/05-implementation-work-packages.md` §WP-16; `docs/api/01-screen-api-contracts.md` §5; C-19; DR-02; `docs/domain/03-metric-methodology.md` §4
**Branch**: `feature/wp16-forecast-evolution-api` (base: `main` `9c20c8c`)
**Status**: **Implementation Complete — Not Accepted** (Delivery Review Board transition only)

> Scope discipline: this package adds the single public bounded endpoint `GET /forecast-comparison` (S-05) over pre-computed `forecast_snapshots` + `observations`. It reuses the WP-15 envelope/freshness/rounding/cache layer and the WP-12 evaluation kernel; it adds **no writes**, no migration, and no auth wiring (public per AUTH-08).

---

## 1. Executive summary

- **Objective**: bounded public `GET /forecast-comparison` for the Forecast-vs-Actual screen — one location, one day, one variable, selected providers.
- **Implemented**:
  - **`analysis.ReadService.ForecastComparison`** + two `ReadRepository` queries (`analysispg`): per-provider forecast lines, the day's observations, in-memory day metrics, pooled error band, provenance mix.
  - **DR-02 issuance selection**: `DISTINCT ON (provider_id, target_time) … ORDER BY provider_id, target_time, forecast_horizon_minutes DESC WHERE forecast_horizon_minutes ≤ requested` — the largest horizon not exceeding the requested one (exact when present, else the nearest shorter). Each point carries its actual `issued_at` + `horizon_minutes` (subtitle honesty); the series carries the earliest issuance.
  - **Observation gaps**: only observations that exist are returned (never interpolated; PC-10). A forecast hour without an observation stays on the line but is excluded from day metrics.
  - **Day metrics**: MAE/RMSE/Bias per provider via the WP-12 `eval.Continuous` kernel under observation-quality weights (§6.4); `error_band_mae` is the pooled MAE across all matched pairs; `sample_count` per provider.
  - **Handler**: required `location_id`/`date`/`variable`/`horizon_minutes` (+ optional `providers` CSV); `date` interpreted in the location's IANA timezone → UTC day window; 404 unknown location; 422 bad params; variable-precision rounding; per-provider `provider_unavailable` warnings; observation freshness + provenance envelope. Wired on the analysis cache class (ETag + LRU).
- **No migration / schema change.** OpenAPI at 15 paths; drift-gate required-path list extended.

## 2. Authorization + selection

| Check | Evidence | Result |
|-------|----------|--------|
| WP-15 Accepted + merged | registry line 15; PR #13 merged `9c20c8c` | ✅ |
| WP-08 + WP-10 Accepted (data) | registry lines 8, 10 | ✅ |
| Envelope dependency (WP-15) | reused | ✅ |

## 3. Scope reconstruction (§WP-16)

| # | Approved scope item | This package | Result |
|---|---------------------|--------------|--------|
| S1 | `GET /forecast-comparison` (public, bounded, C-19) | endpoint + envelope | ✅ |
| S2 | Issuance selection (DR-02 nearest-shorter-horizon) | `DISTINCT ON` max-horizon-≤-requested | ✅ |
| S3 | Day metrics in-memory (≤ 48 pairs) | WP-12 kernel; per-provider + pooled error band | ✅ |
| S4 | Observation gaps as absences | absent from `observations[]`; excluded from metrics (PC-10) | ✅ |
| S5 | Per-series freshness + provenance | top-level observation freshness + `issued_at` per series/point; provenance mix | ✅ |
| S6 | Date-in-location-tz interpretation | `time.ParseInLocation` in the location zone → UTC window | ✅ |
| Acc | DR-02 selection; gap rendering; day metrics vs kernel; size bound | unit + integration; < 20 KB asserted | ✅ |

## 4. Architecture + key decisions

- **FvA read lives in the analysis module** (which already reads `forecast_snapshots` + `observations` and owns the `eval` kernel), reusing the WP-15 `ReadService`/`analysispg.ReadRepository`. Correct dependency direction; no new cross-module edge (collection never imports analysis).
- **DR-02 as a single indexed `DISTINCT ON`**: elegantly expresses "exact horizon, else nearest shorter" as `max(horizon) ≤ requested` per (provider, target hour). The variable→column map is a validated closed switch; all values parameterized (no injection).
- **Metrics reuse the kernel**: day metrics call `eval.Continuous` (same functions as WP-12/13), so a day MAE is computed identically to the aggregated MAE.
- **Freshness**: one top-level block from the latest observation (conventions §2); per-series collection recency is conveyed by `issued_at`; absent providers → `provider_unavailable` warnings (all-absent ≠ partial, §4.2 rule 6).

## 5. Tests

| Layer | Test | Proves |
|-------|------|--------|
| Unit | `internal/analysis/read_test.go` | DR-02 series assembly incl. a gap hour on the line; day metrics matched-only (sample_count, MAE 1.0, Bias 0); pooled error band; provenance mix; no-data path |
| Integration (real PG16) | `test/integration/forecast_comparison_test.go` | seeded snapshots + observations with a mid-day gap → 3 points / 2 observations / day metric sample_count 2 / MAE 1.0 / methodology_version; provenance + attribution + ETag + < 20 KB size bound; required-param + bad-date + 404-unknown-location validation |

Full `go test -race ./internal/... ./adapters/...` green; `gofmt`/`go vet`/`golangci-lint` clean; `go vet -tags integration ./test/integration/...` compiles (Docker unavailable locally → real-PG runs in CI). `make docs` valid (15 paths).

## 6. Database / API / security

**No migration, no schema change.** One new public GET endpoint. All reads parameterized + live-row/quality-scoped (`superseded_observation_id IS NULL`, `quality_flag <> 'suspect'`). No credentials/external calls. Public per AUTH-08; caching public-class only.

## 7. CI evidence

Branch pushed; PR #14 → `main` triggered CI run **30060501063** (event `pull_request`) **success** on head SHA `14c168b29271694f689ad01dd4d69a4b2378039e` (`14c168b`) with all six mandatory jobs green (`backend-checks`, `backend-integration`, `migrations`, `api-contract`, `security`, `image`), none skipped/cancelled; local == `git ls-remote origin` == CI head SHA. `backend-integration` ran the `/forecast-comparison` API tests against real PostgreSQL 16 (DR-02 selection over seeded snapshots, gap handling, day metrics, provenance, ETag, size bound, validation). No earlier failure (green on the first run).

## 8. Deviations

```text
Cache-Control max-age is the analysis class (60 s) for /forecast-comparison; the
contract's per-date split (past dates 300 s, today 60 s) is a documented follow-on
— ETag/304 + LRU still apply, only the max-age of past-date responses differs.
error_band_mae is defined as the pooled MAE across all matched pairs (a single
"typical error" band), consistent with the kernel's weighted MAE.
```

## 9. Work-package transition

```text
WP-16 — Forecast Evolution API
Previous State: Selected — Not Started
New State: Implementation Complete
Acceptance State: Not Accepted (pending pushed-branch CI + Delivery Review Board)
```

## 10. Recommended next action

```text
Push feature/wp16-forecast-evolution-api and capture the six mandatory CI jobs on
the exact code+test SHA, then convene the Delivery Review Board for WP-16.
```
