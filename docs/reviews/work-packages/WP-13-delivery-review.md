# ForecastIQ — WP-13 Aggregated Metrics: Delivery Review Board Report

**Version**: 1.0
**Review date**: 2026-07-23
**Work package**: WP-13 — Aggregated Metrics
**PR**: #11 (`feature/wp13-aggregated-metrics` → `main`)
**Reviewed SHA**: `63081bb2a6629f8e5e140a89ac819dcb66a05a39` (`63081bb`); code+test tip `93f808b170c14815930687d3266db1051a32f5ab` (`93f808b`)
**Authority**: `docs/planning/05-implementation-work-packages.md` §WP-13; `docs/workflows/04-evaluation-and-ranking.md` §2–3/§5; `docs/domain/03-metric-methodology.md` §4/§5/§6.4/§7.4/§9; `docs/data/03-table-design.md` §4; ADR-010
**Decision**: **ACCEPTED** — no Critical/High/Medium/Low finding.

---

## 1. Scope of this review

First independent Delivery Review Board review of WP-13. Dependency gate satisfied: WP-12 **Accepted** and merged. The board independently re-verified commit identity, CI evidence, scope reconstruction, and ran an adversarial read of the diff `0d0d3ae..63081bb` — with particular attention to CI/statistical correctness, the DB CHECK constraints, the null rules, and the supersede mechanics.

## 2. Commit identity + CI evidence (independently verified)

| Check | Evidence | Result |
|-------|----------|--------|
| Local HEAD == remote tip | `git rev-parse HEAD` == `git ls-remote origin …` == `63081bb` | ✅ |
| Code+test lineage | code/test commits `89d596c`/`be0c4be`/`9de5362`/`7284e06`/`a7ddf9e`; commit since (`63081bb`) is **docs-only** (`git diff --stat 93f808b..HEAD` → WP-13 report) | ✅ |
| CI on the code+test tip | run **30045318424** (`pull_request`, head `93f808b`) **success**, all six mandatory jobs green (none skipped/cancelled) | ✅ |
| `migrations` job | applied `20260801000009` (up + verify + seed×2) | ✅ |
| `backend-integration` job | ran the aggregation hand-computed/null/supersede tests against real PostgreSQL 16 | ✅ |
| PR mergeable | `MERGEABLE` (mergeState `UNSTABLE` only because the docs-commit run was queued at review time; the authoritative code+test-tip run is green) | ✅ |
| `ci.yml` unchanged | not in diff | ✅ |

Local gate re-run on the reviewed tree: `gofmt -l` clean; `go vet` clean; `go test -race ./internal/... ./adapters/...` green (incl. the CI unit + coverage-probability simulation); `go build -tags integration ./test/integration/...` compiles; `golangci-lint run` clean. (Docker unavailable in the review environment → the integration suite is validated via CI's green `backend-integration` on the exact SHA.)

## 3. Scope reconstruction (§WP-13)

| # | Approved scope item | Verified | Result |
|---|---------------------|----------|--------|
| S1 | Aggregation batch (per cell-period); weighted formulas | `AggregateService` over cells × periods on the weighted WP-12 kernel | ✅ |
| S2 | Wilson CIs | `eval.Wilson` on every ratio metric | ✅ |
| S3 | Zero-denominator → null | `metric()` null coupling + DB CHECK | ✅ |
| S4 | Coverage (schedule-derived) + reliability (FC-13 classified) | per-variable coverage + reliability rows | ✅ |
| S5 | Supersede on recompute | insert-new then supersede prior live row (`id <> new`) | ✅ |
| S6 | Daily/weekly/monthly periods | `standardPeriods` + explicit `AggregatePeriod` | ✅ |
| Acc | Seeded data matches hand-computed reference | TV-1 continuous proven against real PG16 | ✅ |

Exclusions respected: no ranking (WP-14), no API (WP-15+).

## 4. Architecture + security assessment

- **CI math cohesive in the kernel**: additive to WP-12; continuous/Brier CIs use the frequency-weight-unbiased variance (reduces to ÷(n−1) at unit weights), Wilson for ratios. Dependency direction and module boundaries preserved; the scheduler depends on a small `BatchAggregator` interface.
- **Corrections flow automatically**: aggregation reads live pairs only (`o.superseded_observation_id IS NULL`), so a rematched corrected pair supersedes the old one in the next batch (workflow §5).
- **Security**: no external calls/credentials; all SQL parameterized (the 13-column insert builds only `$n` placeholders).

## 5. Adversarial review — verified correct

The board's adversarial pass found **no** defects and independently confirmed:

1. **`(value IS NULL) = (sample_count = 0)` CHECK safety** — the `metric()` helper couples value/CI/sample; a non-nil value always carries `sample_count ≥ 1` (continuous `N()≥1`, ratios `occN≥1`, coverage/reliability `denom>0`), a nil value carries `0` + nil CI. No path violates it.
2. **`ci_lower ≤ value ≤ ci_upper` CHECK safety** — Wilson brackets pHat by score-test construction (clamp to [0,1] can't exclude pHat∈[0,1]); continuous/Brier intervals are `mean ± half` with `half ≥ 0`; nil CI satisfies the `IS NULL` branch.
3. **Wilson formula** matches the standard score interval; boundary cases pHat=0/1 collapse to the boundary.
4. **`ciFromMoments`** frequency-weight-unbiased variance is correct and reduces to the sample variance ÷(n−1) at unit weights; guards for `n<2`, `D≤0`, negative-float variance; `√n` uses the pair count.
5. **RMSE CI** approximation (std of |e|, centred on RMSE) still satisfies the bracket CHECK.
6. **Supersede ordering** — insert-live-then-supersede with `id <> new` and `superseded_by IS NULL`; the non-unique live index makes the in-tx two-live-rows window legal and MVCC-invisible; trigger permits only `NULL→value`.
7. **Null/eligibility** — occurrence gated on observed precip present + non-suspect; `rain_mae_wet` gated on observed ≥ 0.1; continuous via `Eligible` (both non-nil); no double-count.
8. **`ReadPairs`** live-pairs-only; `sample_count` is always an integer count (never a fractional weight).
9. **SQL injection** — none (positional params only).
10. **`standardPeriods`** — Monday-based week (`(weekday+6)%7`), correct month boundary, half-open `[start,end)`.
11. **Coverage denominator** — `ScheduledForecastSlots` joins `provider_configurations` on `provider_id`; `denom = 0 → null` (no division-by-zero).

## 6. Findings

None. No Critical/High/Medium/Low finding.

## 7. Regression assessment

- The WP-12 `eval` kernel changes are **additive** (new fields default-zero; existing MAE/RMSE/Bias/Score/Confusion behaviour unchanged) — WP-12's own tests (TV-1..5, properties) remain green.
- WP-11 matching untouched; the `analysis_batch` dispatcher is extended additively (match → aggregate).
- No migration/API/CI-control regression (`ci.yml` unchanged).

## 8. Completion-gate assessment

| Gate | Result |
|------|--------|
| Exact WP-13 definition located | ✅ |
| Dependency (WP-12) Accepted | ✅ |
| Scope reconstruction complete; exclusions respected | ✅ |
| Methodology fidelity (formulas, CIs, null, coverage/reliability, supersede) | ✅ |
| Acceptance (hand-computed reference) proven vs real PG16 | ✅ |
| SHA identity (local == remote == CI head) | ✅ |
| No Critical/High/Medium finding | ✅ |

## 9. Decision

**ACCEPTED.** WP-13 delivers the aggregation engine — `accuracy_metrics` rows per cell-period with Wilson/normal-approx CIs, the §5 null rule (DB-enforced), coverage + reliability, and supersede-on-recompute — CI-verified green on the exact code+test SHA `93f808b` including the `migrations` and real-PG `backend-integration` jobs. The adversarial review found no defect, and specifically confirmed both DB CHECK constraints can never be violated by emitted rows.

**Accepted Implementation SHA `63081bb`** (code+test lineage `93f808b`). PR #11 ready to merge to `main`. **WP-14 (Provider Ranking)** becomes eligible — it consumes `accuracy_metrics` to produce `provider_rankings` (cohort normalization, weights, coverage penalty, statuses, tie-grouping, horizon profiles; ADR-010 worked-example integration test).

## 10. Tracked conditions

None.
