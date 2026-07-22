# ForecastIQ — Metric ↔ UI Contract

**Version**: 1.0 (UI ↔ Backend Reconciliation Board)
**Status**: Authoritative — every metric displayed in the MVP UI is traceable to a defined formula and dataset
**Source of truth for formulas**: `docs/domain/03-metric-methodology.md` (methodology_version `2026.1`, weights_version `w-2026.1`)
**Principle applied**: No unexplained metrics. Anything without a documented formula (consensus, confidence rating, disagreement index) is deferred — see `docs/reviews/03-ui-backend-conflicts.md` C-03.

---

## 1. Metric Traceability Matrix

Scope inherited by every metric below unless overridden: **location scope** (selected location), **provider scope** (per provider row), **observation source** (Open-Meteo Historical; mix disclosed via `observation_provenance_mix`), **methodology version** `2026.1` (displayed + linked to S-06).

| UI metric | Definition | Formula | Input data | Aggregation | Horizon | Period | Min samples | Confidence representation | Methodology version | API field | Database source | Test requirement |
|-----------|-----------|---------|------------|-------------|---------|--------|-------------|---------------------------|--------------------|-----------|-----------------|------------------|
| Composite score | Weighted normalized multi-metric provider score | `Σ w_c × norm_c` then coverage penalty (§6) | Component metrics below | Per (provider, location, horizon, period) | Selected or profile | 30d (90d slow horizons) | 30 ranked / 10 provisional | CI via component propagation (approximate, documented) | 2026.1 + weights echoed | `composite_score`, `ci_lower/upper`, `components{}` | provider_rankings | TV vectors + property tests §11 (10, 11); ranking integration test |
| Temperature MAE | Mean absolute temp error | `(1/n)Σ\|f−o\|` (weighted) | MatchedEvaluation temp pairs | Per cell | Selected | 30d | 30 | `±1.96·s/√n` | 2026.1 | `metrics[temperature].mae` + ci | accuracy_metrics | TV-1, TV-5; property 1, 6, 7 |
| Temperature RMSE | Root mean squared temp error | `√((1/n)Σe²)` | Same | Same | Same | Same | 30 | Same | 2026.1 | `.rmse` | accuracy_metrics | TV-1; property 2 |
| Temperature Bias | Systematic temp error direction | `(1/n)Σe` (signed) | Same | Same | Same | Same | 30 | CI on mean error | 2026.1 | `.bias` | accuracy_metrics | TV-1; property 3 |
| Recall (POD) | Rain-hour detection rate | `TP/(TP+FN)` (fractional w_i) | Occurrence pairs (event def §4.2) | Same | Same | Same | 30; null if TP+FN=0 | Wilson interval | 2026.1 | `.recall` (null allowed) | accuracy_metrics | TV-2, TV-3; property 4 |
| Precision | Rain-forecast hit rate | `TP/(TP+FP)` | Same | Same | Same | Same | 30; null if TP+FP=0 | Wilson | 2026.1 | `.precision` | accuracy_metrics | TV-2, TV-3; property 4 |
| F1 | Balanced occurrence score | `2TP/(2TP+FP+FN)` | Same | Same | Same | Same | 30; null if denom=0 | Wilson | 2026.1 | `.f1` | accuracy_metrics | TV-2; property 5 |
| False Alarm Rate | Dry hours wrongly forecast rain | `FP/(FP+TN)` | Same | Same | Same | Same | 30; null if FP+TN=0 | Wilson | 2026.1 | `.far` | accuracy_metrics | TV-2; property 4 |
| Threat Score (CSI) | Event-based agreement (supplementary) | `TP/(TP+FP+FN)` | Same | Same | Same | Same | 30; null if denom=0 | Wilson | 2026.1 | `.threat_score` | accuracy_metrics | TV-2 |
| Occurrence agreement | Simple agreement + **imbalance warning** | `(TP+TN)/n` | Same | Same | Same | Same | 30 | Wilson | 2026.1 | `.occurrence_agreement` + warning flag | accuracy_metrics | TV-2; UI test: warning always co-rendered |
| Brier Score | Probability calibration (supplementary, not ranked) | `(1/n)Σ(p−a)²` | Probability pairs | Same | Same | Same | 30 | Normal approx | 2026.1 | `.brier` | accuracy_metrics | TV-4 |
| Rain MAE (all) | Rain amount error, all pairs | Weighted MAE over all pairs | Amount pairs | Same | Same | Same | 30 | ±CI | 2026.1 | `.rain_mae_all` | accuracy_metrics | TV-1 pattern |
| Rain MAE (wet) | Conditional wet-hour amount error | MAE over pairs obs ≥ 0.1mm | Wet pairs only | Same | Same | Same | 30 (wet pairs); null if none | ±CI | 2026.1 | `.rain_mae_wet` | accuracy_metrics | Wet-subset test |
| Wind MAE / RMSE / Bias | Wind speed errors | As temperature family | Wind pairs | Same | Same | Same | 30 (wind optional for ranking) | ±CI | 2026.1 | `.wind_*` | accuracy_metrics | TV-1 pattern |
| Humidity / Pressure MAE etc. | Per-variable continuous errors | As temperature family | Per-variable pairs | Same | Same | Same | 30 | ±CI | 2026.1 | `.humidity_*`, `.pressure_*` | accuracy_metrics | TV-1 pattern |
| Coverage | Data completeness | `non_null_snapshots/expected` per schedule | Snapshot counts vs. schedule | Per provider-location-period | All | Period | n/a | n/a (direct ratio) | 2026.1 | `coverage` | accuracy_metrics / ranking components | Schedule-baseline test |
| Reliability | Collector success rate | `successful/scheduled` with error classification | Collection statuses | Same | n/a | Period | n/a | n/a | 2026.1 | `reliability` | forecast_collections aggregate | FC-13 classification test |
| Sample count | Matched pairs contributing | Count of eligible pairs | MatchedEvaluation | Per metric | Selected | Period | self | Highlighted < threshold | 2026.1 | `sample_count` | accuracy_metrics | Eligibility-rule tests (CE-01) |
| Ranking status | Eligibility classification | Thresholds 30/10 + coverage rules (§7) | sample_count, coverage, data age | Per cell | Selected | Period | self | n/a | 2026.1 | `ranking_status`, `reason` | provider_rankings | BR-RANK-02/03/09 tests |
| Tie groups | Statistical indistinguishability | 95% CI overlap | Composite CIs | Adjacent pairs | Selected | Period | n/a | Self-defining | 2026.1 | `significant_vs_next` | provider_rankings | BR-RANK-05 test |
| Coverage penalty | Incomplete-coverage score penalty | `×(coverage/0.8)` when [0.5,0.8) | Coverage | Per cell | Selected | Period | n/a | n/a | 2026.1 | `coverage_penalty_applied`, raw vs. penalized | provider_rankings | BR-RANK-04 + property 9 |
| Day MAE/Bias/RMSE (S-05 table) | Single-day error summary | Same formulas, day scope | That day's matched pairs | Per provider-day | Horizon-matching issuance | 1 day | None (n displayed, no ranking claim) | CI optional (n≤24 wide) | 2026.1 | `day_metrics[]` in `/forecast-comparison` | computed on demand | Day-scope integration test |
| Error band ±MAE (S-05 chart) | 30-day MAE as visual tolerance | 30d MAE for variable+horizon | AccuracyMetric | Per location-horizon | Selected | 30d | ≥30 (band hidden with note if insufficient) | Self is CI-bearing | 2026.1 | `error_band_mae` | accuracy_metrics | Band-insufficient-state test |

## 2. Per-Chart/Table Scope Declaration (binding)

Every chart and table states or inherits (board mandate):

| Screen element | Location scope | Provider scope | Variable | Horizon | Evaluation period | Observation source | Methodology version |
|----------------|---------------|----------------|----------|---------|-------------------|--------------------|--------------------|
| S-01 ranking table | Selected (context bar) | All active | Composite (all components) | Selected | 30d/90d (echoed) | Mix badge + footer | Footer + linked |
| S-01 breakdown | Inherited | Per row | Per component | Inherited | Inherited | Provenance mix in tooltip | Inherited |
| S-02 metric tables | Page location | Per row | Per section | Selected | Selected (7/30/90d) | Header badge | Footer |
| S-03 composite grid | Per row | Page provider | Composite | Per column | 30d/90d | Footer | Footer |
| S-03 per-horizon detail | Selected (control) | Page provider | Per column | Per row | Selected | Footer | Footer |
| S-04 trend chart | Selected | Per line | Selected | Selected | Selected + aggregation | Footer | Footer |
| S-05 chart | Selected | Per line + filter | Selected | Selected (issuance rule) | Day (band: 30d, labeled) | Legend badge | Footer |
| S-05 day table | Inherited | Per row | Selected | Inherited | Day | Inherited | Footer |

## 3. Computation Strategy Decision (board mandate)

**Stored immutable evaluation results** — AccuracyMetric rows batch-computed every 30 minutes (CE-09) + ProviderRanking rows; day metrics for S-05 computed on demand over ≤ 48 matched pairs (trivially cheap). Rejected alternatives:
- On-demand full aggregation: violates p95 200ms at 90d periods (hundreds of thousands of pairs).
- Continuous aggregates: TimescaleDB excluded (ADR-004).
- Cached projections over live queries: adds invalidation complexity with no benefit given batch freshness thresholds already defined (BR-FRESH rankings: 2h).

This meets NFR-P02 with the simplest mechanism. No TimescaleDB, no Redis, no read replica required (promotion criteria unchanged).

## 4. Excluded Metrics (deferred — no UI surface)

| Metric | Reason | Gate for introduction |
|--------|--------|----------------------|
| Consensus value (mean/median across providers) | No aggregation rule defined; skill-weighted variant is methodology research | New methodology section + version bump (C-03) |
| Confidence rating (High/Moderate/Low) | No formula; PC-02 prohibits unexplained numbers | Same |
| Provider disagreement index (σ across providers) | No threshold/interpretation methodology | Same |
| Feels-like (observation-derived) | Heat-index derivation not stored on observations; provider feels_like exists only on snapshots | Level 3 with derivation spec |
| Ranking sparklines / rank-change timeline | Requires historical ranking comparison UI methodology (tie semantics across batches) | Level 3 |

## 5. Missing-Data Behaviour Contract (UI-visible, methodology §5 binding)

| Situation | API | UI |
|-----------|-----|-----|
| Zero denominator | `null` + `sample_count: 0` | "—" + tooltip "No events in period — metric excluded per methodology" (never 0) |
| Null component in composite | Weight redistributed; echoed in `components{}` as absent | Breakdown shows remaining components; no visual gap implying zero |
| Suspect observations excluded | Pairs excluded; count not inflated | Provenance note in tooltip when mix includes exclusions |
| n < threshold | Status provisional/unranked | Amber highlight + explicit "{n}/{threshold}" copy |
| Wet-only MAE with no wet pairs | `null` | "—" + tooltip "No rain ≥ 0.1mm observed in period" |

## 6. Rounding and Formatting Chain (binding)

Storage (full precision) → API (methodology §5 rounding: 4dp ratios/scores, 2dp °C/mm, 1dp m/s, 2dp hPa) → UI display formatting (doc 02 §1.8: composite 3dp, ratios as % 1dp, coverage 0dp, bias signed). Each layer rounds only once from the layer above — no double rounding from storage.
