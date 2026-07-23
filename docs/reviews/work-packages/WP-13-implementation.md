# ForecastIQ — WP-13 Aggregated Metrics: Implementation Report

**Version**: 1.0
**Implementation date**: 2026-07-23
**Work package**: WP-13 — Aggregated Metrics
**Authority**: `docs/planning/05-implementation-work-packages.md` §WP-13; `docs/workflows/04-evaluation-and-ranking.md` §2–3/§5; `docs/domain/03-metric-methodology.md` §4/§5/§6.4/§7.4/§9; `docs/data/03-table-design.md` §4; ADR-010
**Branch**: `feature/wp13-aggregated-metrics` (base: `main` `0d0d3ae`)
**Status**: **Implementation Complete — Not Accepted** (Delivery Review Board transition only)

> Scope discipline: this package aggregates matched pairs into `accuracy_metrics` rows (per cell-period, with CIs, coverage/reliability, null rules, supersede-on-recompute). It does **not** rank providers (composite normalization, weights, statuses, tie-grouping, horizon profiles → WP-14) and adds no public API (WP-15+).

---

## 1. Executive summary

- **Objective**: `AccuracyMetric` rows with CIs, null rules, and coverage/reliability, produced by an aggregation batch and superseded on recompute.
- **Implemented**:
  - **Migration `20260801000009`** — `accuracy_metrics` (per §4 DDL), a partial live-row index for latest-serving/supersede lookup, and a **supersede-only immutability trigger** (a row changes only by having `superseded_by` set once).
  - **CI primitives** (`internal/analysis/eval/ci.go`, additive to the WP-12 kernel): the **Wilson** score interval for ratio metrics and a frequency-weight-unbiased **±1.96·s/√n** interval for continuous/Brier metrics (reduces to sample variance ÷(n−1) at unit weights). `Continuous`/`Brier` gained Σw² / 4th-moment fields to support the std computation.
  - **Aggregation** (`internal/analysis/aggregate.go` + `analysispg` metric repository): per cell (provider × location × horizon) × period, reads live matched pairs, runs the WP-12 kernel per variable, and emits rows for continuous MAE/RMSE/Bias, rain MAE (all/wet), categorical Recall/Precision/F1/FAR/ThreatScore/OccurrenceAgreement (Wilson CI), Brier, per-variable **coverage**, and **reliability** — applying the §5 null rule (zero denominator → value NULL, sample_count 0) and supersede-on-recompute.
  - **Pipeline wiring**: the `analysis_batch` dispatcher now runs **matching → aggregation** sequentially (workflow §1); `aggregation_metric_rows_written_total` metric; composition-root wiring.
- **Deferred**: ranking (WP-14); API (WP-15+). Rolling multi-day daily windows beyond the current day/week/month set are a straightforward extension of `standardPeriods` when needed.
- **Final status**: Implementation Complete; awaiting pushed-branch CI + Delivery Review Board.

## 2. Authorization + selection

| Check | Evidence | Result |
|-------|----------|--------|
| WP-12 Accepted + merged | registry line 12; PR #10 merged `0d0d3ae` | ✅ |
| WP-13 dependency (WP-12) | evaluation kernel in place | ✅ |

## 3. Scope reconstruction (§WP-13)

| # | Approved scope item | This package | Result |
|---|---------------------|--------------|--------|
| S1 | Aggregation batch (per cell-period); weighted formulas | `AggregateService` over cells × periods, using the weighted WP-12 kernel | ✅ |
| S2 | Wilson CIs | `eval.Wilson` for all ratio metrics | ✅ |
| S3 | Zero-denominator → null | `metric()` null coupling; empty accumulators → nil | ✅ |
| S4 | Coverage (schedule-derived) + reliability (FC-13 classified) | `coverage` per variable (delivered/scheduled); `reliability` (successful/scheduled) | ✅ |
| S5 | Supersede on recompute | insert-new then `SupersedePrevious` on the prior live row | ✅ |
| S6 | Daily/weekly/monthly periods | `standardPeriods` (current day/week/month); explicit `AggregatePeriod` for arbitrary windows | ✅ |

## 4. Architecture + key decisions

- **CI math lives in the kernel** (`eval`): additive to WP-12, keeping all statistics cohesive and unit-testable. Continuous/Brier CIs use the frequency-weight-unbiased variance so they reduce to the familiar ÷(n−1) sample variance at unit weights (matching hand computation and giving correct coverage). The Wilson interval provably brackets the point estimate, so the DB `CHECK (ci_lower ≤ value ≤ ci_upper)` never trips.
- **Aggregation reads live pairs only** (`o.superseded_observation_id IS NULL`), so a correction's rematched pair is used and the superseded one ignored — corrections flow into metrics automatically on the next batch (workflow §5).
- **Supersede ordering**: within one tx, new rows are inserted (live) and then the prior live row per logical key is superseded (`id <> new`); the live index is **non-unique** (per the §4 DDL) so the brief two-live-rows window is legal, and the supersede-only trigger permits exactly the `superseded_by` NULL→value mutation.
- **Null coupling** enforced both in code (`metric()`) and by the DB `CHECK ((value IS NULL) = (sample_count = 0))`.
- **Pipeline**: `AnalysisDispatcher` runs match then aggregate in the same `analysis_batch` (workflow §1), so metrics track fresh matches without a second scheduler job.

## 5. Tests

| Layer | Test | Proves |
|-------|------|--------|
| Unit | `eval/ci_test.go` | Wilson known value (50/100 → [0.4038, 0.5962]) + brackets/clamps (fuzz); continuous CI brackets estimate + nil for n<2; **bias-CI coverage-probability simulation ≈ 95%** |
| Integration (real PG16) | `test/integration/aggregation_test.go` | **hand-computed** TV-1 continuous (mae 1.375 / rmse 1.75 / bias 0.875, n=4, CI brackets); **null rule** (unprovided humidity → NULL, sample_count 0); **supersede + byte-identical recompute** (one live row, previous superseded, identical value) |

Full `go test -race ./internal/... ./adapters/...` green; `gofmt`/`go vet`/`golangci-lint` clean; `go build -tags integration ./test/integration/...` compiles.

## 6. Database / API / security

Adds migration `20260801000009` (up + down): `accuracy_metrics`, the live-row partial index, and the supersede-only trigger. No API change. No external calls/credentials. All SQL parameterized.

## 7. CI evidence

Branch pushed; PR #11 → `main` triggered CI run **30045318424** (event `pull_request`) **success** on head SHA `93f808b170c14815930687d3266db1051a32f5ab` (`93f808b`) with all six mandatory jobs green (`backend-checks`, `backend-integration`, `migrations`, `api-contract`, `security`, `image`), none skipped/cancelled; local == remote tip == CI head SHA. The `migrations` job applied `20260801000009`; `backend-integration` ran the aggregation hand-computed/null/supersede tests against real PostgreSQL 16.

## 8. Files changed

- **Migration**: `migrations/20260801000009_create_accuracy_metrics.{up,down}.sql`
- **Kernel CI**: `internal/analysis/eval/{ci.go,continuous.go,brier.go}`
- **Domain**: `internal/analysis/domain/metric.go`
- **Ports/persistence**: `internal/analysis/ports/ports.go`, `adapters/persistence/analysispg/metricspg.go`
- **Service**: `internal/analysis/aggregate.go`
- **Scheduler/metrics/wiring**: `internal/scheduler/analysis_dispatcher.go`, `internal/platform/metrics/metrics.go`, `cmd/forecastiq/app.go`
- **Tests**: `internal/analysis/eval/ci_test.go`, `test/integration/aggregation_test.go`
- **Docs**: this report; `docs/planning/06-work-package-status-registry.md`

## 9. Deviations

```text
No approved-scope deviations. Coverage/reliability use the schedule-derived
denominator (scheduled forecast slots in period); coverage numerator = delivered
snapshots with a non-null value at the cell horizon; reliability numerator =
successful collections. RMSE CI reuses the std of |e| per methodology §7.4
(documented approximation). standardPeriods currently emits the current
day/week/month; wider rolling daily windows are a config-only extension.
```

## 10. Work-package transition

```text
WP-13 — Aggregated Metrics
Previous State: Selected — Not Started
New State: Implementation Complete
Acceptance State: Not Accepted (pending pushed-branch CI + Delivery Review Board)
```

## 11. Recommended next action

```text
Convene the Delivery Review Board for WP-13. CI evidence is captured (§7):
run 30045318424 on head SHA 93f808b, six mandatory jobs green.
```
