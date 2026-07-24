# ForecastIQ — Metric & Ranking Methodology

**Version**: 2026.1 (methodology_version `2026.1`)
**Status**: Authoritative (Phase 0 Amendment)
**Resolves**: ARB Blocker 1 (composite ranking score), ARB Blocker 3 (accuracy formulas)
**Supersedes**: metric definitions in `docs/phase-0-business-analysis/04-functional-requirements.md` §3.3 and `09-acceptance-criteria.md` AC-3.2

---

## 1. Purpose and Principles

ForecastIQ's core output is a statistically defensible comparison of weather forecast
providers against actual observations. This document is the single source of truth for:

1. Every statistical formula used by the comparison engine.
2. The provider ranking methodology, including weights, thresholds, and penalties.
3. The behavior of every metric under missing data, zero denominators, and low samples.

**Principles:**

- **Transparency**: every published score exposes its methodology version, weights,
  sample size, and confidence interval. No unexplained composite number is ever shown.
- **Reproducibility**: all inputs (forecasts, observations) are immutable; any metric
  can be recomputed from stored data.
- **Conservatism**: when data is insufficient, ForecastIQ says "insufficient data"
  rather than publishing a misleading ranking.

---

## 2. Terminology Decisions

| Decision | Rationale |
|----------|-----------|
| The term **"Hit Rate" is removed** from all documents. | Ambiguous: in some verification literature it means Recall/POD, in others Critical Success Index. ForecastIQ uses **Recall (POD)** explicitly. |
| The term **"Accuracy" is never used for precipitation occurrence** without qualification. | `(TP+TN)/all` is misleading when rain is rare (a provider forecasting "no rain" always achieves high accuracy in dry climates). It is published only as `occurrence_agreement` with an explicit warning, and is **excluded from ranking**. |
| **"False Alarm Rate"** means `FP/(FP+TN)` (fraction of observed dry periods wrongly forecast as rain). The "False Alarm Ratio" `FP/(TP+FP)` equals `1 − Precision` and is not published separately. |
| **"Miss Rate"** equals `1 − Recall` and is not published separately. |

---

## 3. Matched Evaluation Pairs

All metrics are computed over **matched forecast–observation pairs** (see
`docs/domain/01-domain-model.md`, entity `MatchedEvaluation`).

A pair is eligible for a given variable's metric when:

1. Same `location_id`.
2. Forecast `target_time` matches observation `observed_at` per the matching rules in §3.1.
3. Both records have non-null values for that variable.
4. The observation `quality_flag` is not `suspect` (suspect observations are excluded).
5. The forecast horizon equals the evaluation horizon
   (`forecast_horizon_minutes = target_time − issued_at`).

### 3.1 Matching rules (MVP)

| Rule | Specification |
|------|---------------|
| Time basis | All matching in UTC. No local-time or DST logic in the engine. |
| Exact-hour matching | Forecast `target_time` truncated to the hour must equal `observed_at` (hourly sources). This replaces the prior universal ±30 min window. |
| Tolerance for sub-hourly sources | ±15 minutes only when the observation source reports at intervals finer than 1 hour. |
| Precipitation aggregation | Compared as 1-hour accumulated amounts. Sub-hourly observations are summed to the hour before matching. |
| Multiple observations per target hour | Prefer by provenance rank: `station_observation` > `interpolated` > `reanalysis` > `provider_estimated`. If equal rank, use the observation closest to the top of the hour. The chosen observation's `observation_id` is recorded on the matched pair (lineage). |
| Corrected observations | Observations are immutable. A correction is a **new** observation record with `quality_flag = corrected` referencing the superseded record. Matching prefers `corrected` over `valid` for the same source/time, and affected metrics are recomputed (see §9). |
| Incomplete observations | An observation missing one variable does not invalidate other variables; eligibility is per-variable (§3, rule 3). |

Provider-specific matching policies (e.g., a provider issuing forecasts at :30 offsets)
are configured per adapter; the MVP default is exact-hour matching for all providers.

---

## 4. Canonical Formulas

Notation: for pair *i*, `f_i` = forecast value, `o_i` = observed value, `n` = number of
eligible pairs, `e_i = f_i − o_i`.

### 4.1 Continuous variables (temperature, wind speed, humidity, pressure, rain amount)

| Metric | Formula | Plain language | Range |
|--------|---------|----------------|-------|
| **MAE** | `(1/n) × Σ|e_i|` | Average magnitude of error, ignoring direction. | [0, ∞) |
| **RMSE** | `√((1/n) × Σ e_i²)` | Error magnitude penalizing large errors more heavily. | [0, ∞) |
| **Bias** | `(1/n) × Σ e_i` | Systematic tendency: positive = forecasts run higher than reality. | (−∞, ∞) |

- **Rainfall amount MAE** is computed two ways and both are stored:
  - `rain_mae_all`: over all matched pairs (includes dry hours).
  - `rain_mae_wet`: only over pairs where observed rain ≥ 0.1 mm (conditional skill).
- Lower is better for MAE, RMSE, and |Bias|.

### 4.2 Precipitation occurrence (categorical)

**Event definitions (MVP):**

- Forecast rain: `precipitation_probability ≥ 0.5` **or** `precipitation_amount > 0`.
- Observed rain: `precipitation_mm ≥ 0.1` (trace amounts below 0.1 mm count as dry).

**Confusion matrix:**

|  | Observed rain | Observed dry |
|--|---------------|--------------|
| **Forecast rain** | TP | FP |
| **Forecast dry** | FN | TN |

| Metric | Formula | Plain language | Zero-denominator behavior |
|--------|---------|----------------|---------------------------|
| **Recall (POD)** | `TP / (TP + FN)` | Of all hours it actually rained, how many did the provider predict? | `TP+FN = 0` (it never rained): metric = `null`, excluded. |
| **Precision** | `TP / (TP + FP)` | Of all hours the provider predicted rain, how many actually had rain? | `TP+FP = 0` (no rain forecasts): metric = `null`, excluded. |
| **F1** | `2TP / (2TP + FP + FN)` | Harmonic mean of Precision and Recall; single balanced occurrence score. | `2TP+FP+FN = 0`: metric = `null`, excluded. |
| **False Alarm Rate** | `FP / (FP + TN)` | Of all dry hours, how many were wrongly forecast as rain? | `FP+TN = 0`: metric = `null`, excluded. |
| **Threat Score (CSI)** | `TP / (TP + FP + FN)` | Fraction of rain events (forecast or observed) correctly forecast. Published as supplementary, not ranked. | Denominator 0: `null`. |
| **Occurrence agreement** | `(TP + TN) / n` | Simple agreement. Published **with imbalance warning**, never ranked. | n = 0: `null`. |

**Why occurrence "accuracy" is misleading:** in a dry location where it rains 5% of
hours, a provider that always forecasts dry scores 95% agreement while providing zero
rain-detection value. Recall, Precision, and F1 are immune to this base-rate effect in
the sense that they condition on events; agreement is not.

### 4.3 Probabilistic metric

| Metric | Formula | Plain language |
|--------|---------|----------------|
| **Brier Score** | `(1/n) × Σ (p_i − a_i)²` where `p_i` = forecast precipitation probability, `a_i` = 1 if observed rain else 0 | Calibration + reliability of rain probabilities. 0 = perfect, 1 = worst. Published supplementary in MVP; ranked from Level 3. |

### 4.4 Coverage and reliability

| Metric | Formula | Plain language |
|--------|---------|----------------|
| **Provider data coverage** | `snapshots_with_non_null_variable / expected_snapshots` over the evaluation period | How completely the provider delivered data for this variable. `expected_snapshots` = active days × collections/day × periods/collection per the configured schedule. |
| **Collection reliability** | `successful_collections / scheduled_collections` over the period | How reliably our collector retrieved this provider's data (distinguishes provider outages from our failures via `error_code` classification). |

Both are in [0, 1]. Coverage below 0.5 makes a provider **provisionally ranked** (§7.4).

---

## 5. Null, Missing-Data, and Rounding Rules

| Situation | Rule |
|-----------|------|
| Zero denominator in any ratio metric | Metric value = `null` (never 0, never NaN). `sample_count = 0` for that metric. Null metrics are excluded from normalization and composite scoring; the affected weight is redistributed proportionally across remaining metrics (§6.2). |
| Variable missing on one side of a pair | Pair excluded from that variable's metrics only. |
| Observation `quality_flag = suspect` | Pair excluded from all metrics; pair is still stored for audit. |
| Rounding (storage) | Full double precision stored. |
| Rounding (API) | 4 decimal places for ratios and scores; 2 for temperature °C and rain mm; 1 for wind m/s; 2 for pressure hPa. |
| Rounding (UI) | 2 decimal places for error metrics; percentages shown as e.g. "88.9%". |
| Ties in ranking | Providers whose composite score 95% CIs overlap are displayed as a tied group (same rank number). |

---

## 6. Composite Provider Score

### 6.1 Inputs

Per provider, location, horizon, and evaluation period, the engine produces the
**metric-level results** of §4. The composite score is built only from:

| Component | Source metric | Direction |
|-----------|--------------|-----------|
| Temperature error | `temp_mae` | lower better |
| Temperature bias | `|temp_bias|` | lower better |
| Rain occurrence | `precip_f1` | higher better |
| Rain amount | `rain_mae_all` | lower better |
| Wind error | `wind_mae` | lower better |
| Data coverage | `coverage` (min across ranked variables) | higher better |
| Collection reliability | `reliability` | higher better |

### 6.2 Normalization (ratio-to-cohort-best)

For each component, normalize across the **cohort** = all providers with non-null values
for that component at the same location, horizon, and period:

- Lower-is-better: `norm = best_value / value` (best provider scores 1.0; others < 1.0).
  Guard: if `best_value = 0` (a perfect provider), add ε = 1e-9 to all values first.
- Higher-is-better: `norm = value / best_value`.
- Coverage and reliability are used directly (already [0,1]).

If a provider has a null component (e.g., no rain events → F1 null), that component is
excluded for **all** providers in the cohort for that period, and its weight is
redistributed proportionally to the remaining components. This prevents a dry period
from penalizing anyone.

**Known limitation (documented):** ratio-to-best is cohort-relative; a weak cohort inflates
scores. Mitigations: minimum sample thresholds (§7), coverage penalty (§7.3), and the
fact that metric-level raw values are always published alongside the composite score.

### 6.3 Default weights (weights_version `w-2026.1`)

| Component | Weight | Rationale |
|-----------|--------|-----------|
| Temperature MAE | 0.30 | Most universally compared variable; users' primary trust signal. |
| Precipitation occurrence F1 | 0.25 | Rain prediction is the highest-stakes, most-asked comparison. |
| Rainfall amount MAE | 0.15 | Amount matters for operations (flooding, irrigation). |
| Wind speed MAE | 0.15 | Important for logistics/outdoor use. |
| |Bias| (temperature) | 0.05 | Systematic bias indicates model quality even when MAE is similar. |
| Data coverage | 0.05 | Rewards providers that actually deliver data. |
| Collection reliability | 0.05 | Rewards retrievable providers (our-side failures excluded via error classification). |
| **Total** | **1.00** | |

Composite (per horizon): `score_h = Σ w_c × norm_c`, then coverage penalty (§7.3).

**Horizon weighting** (cross-horizon composite): the API accepts a `horizon_profile`:

| Profile | Weights |
|---------|---------|
| `uniform` (default) | Equal weight across requested horizons. |
| `short_term` | +1h .25, +3h .20, +6h .20, +12h .15, +24h .10, +3d .06, +7d .04 |
| `daily_planning` | +12h .20, +24h .40, +3d .25, +7d .15 |

Custom weights may be supplied via API (`weights` parameter); the response always echoes
the weights actually used. The UI always shows per-metric breakdowns so the composite
never stands alone.

### 6.4 Observation-quality weighting

Each pair carries weight `w_i` by observation provenance:

| Observation type | Weight |
|------------------|--------|
| `station_observation` | 1.0 |
| `interpolated` | 0.8 |
| `reanalysis` | 0.8 |
| `provider_estimated` | 0.5 |
| `corrected` | same as the type it corrects |
| `suspect` | excluded (§5) |

Continuous metrics use the weighted form, e.g. `MAE = Σ w_i|e_i| / Σ w_i`. For
categorical counts, pairs contribute fractionally (TP += w_i, etc.). Provenance is
always displayed with results.

---

## 7. Ranking Rules

### 7.1 Minimum sample threshold

**Default: 30 matched forecast–observation pairs** per (provider, location, variable,
horizon, evaluation period) for a variable to count toward a **ranked** status.

- The threshold applies per variable; the provider's overall status uses its **worst**
  required variable (temperature AND precipitation occurrence are required; wind is
  optional for ranking in MVP).
- Evaluation period default: trailing 30 days. For slow-accumulating horizons (+3d,
  +7d), the period automatically extends to 90 days to reach 30 pairs — the count
  threshold is **not** lowered.
- Where a different threshold is appropriate:
  - **10 pairs (provisional)**: exploratory views, new locations, new providers — always
    labeled "provisional".
  - **Higher (e.g., 90)**: commercial/SLA contexts or publication-grade claims.
  - Threshold is configurable per workspace (post-MVP) and per request (API parameter,
    echoed in response).

### 7.2 Ranking status

| Status | Condition | Display |
|--------|-----------|---------|
| `ranked` | ≥ 30 pairs for all required variables in period, coverage ≥ 0.5 | Numeric rank (1, 2, 3…). |
| `provisionally_ranked` | 10–29 pairs for any required variable, OR coverage in [0.5, 0.8) | Rank shown with "provisional" badge; listed after all `ranked` providers. |
| `unranked` | < 10 pairs for any required variable, OR coverage < 0.5 | "Insufficient data" — no score ordering published. |

### 7.3 Coverage penalty and the coverage-outranking rule

- If coverage ≥ 0.8: no penalty.
- If coverage in [0.5, 0.8): `final_score = score_h × (coverage / 0.8)` (linear penalty).
- **Rule**: a provider with coverage < 0.5 (provisional/unranked) can **never** outrank a
  provider with coverage ≥ 0.8, regardless of score. Between 0.5 and 0.8 it may outrank
  only via its penalized score. This directly answers the review question: incomplete
  coverage can partially compete (with penalty) but cannot dominate broad coverage.

### 7.4 Statistical confidence

- Each continuous metric publishes a 95% CI: `±1.96 × s/√n` (s = sample standard
  deviation of |e_i| or e_i). For ratios, Wilson score interval.
- Composite score CI is derived by propagating component CIs through the weighted sum
  (first-order approximation); documented as approximate.
- Two providers are **not significantly different** when their composite CIs overlap;
  the UI renders them as a tied group and the API returns `significant: false` for the pair.
- When `n < 30`, CIs are wide by construction — the provisional label communicates this.

### 7.5 Score versioning

Every stored ranking row carries:

- `methodology_version` (this document's version, e.g. `2026.1`)
- `weights_version` (e.g. `w-2026.1` or `custom:<hash>`)
- `evaluation_period_start/end`, `sample_count` per component.

Changing methodology or weights produces **new** ranking rows; old rows are retained
(immutable) so historical comparisons remain reproducible. The UI displays the
methodology version and links to this document.

---

## 8. Worked Example (three providers)

**Setup**: Location = Johor Bahru (1.4927, 103.7414). Horizon = +24h. Period = trailing
30 days. All observations from Open-Meteo reanalysis (weight 0.8 each). Cohort:
Open-Meteo Forecast (OM), OpenWeather (OW), ProviderX (PX).

**Matched pairs and raw results:**

| Component | OM | OW | PX |
|-----------|-----|-----|-----|
| Matched pairs (temp) | 720 | 700 | 380 |
| temp_mae (°C) | 1.20 | 1.50 | 1.10 |
| \|temp_bias\| (°C) | 0.30 | 0.90 | 0.25 |
| precip: TP/FP/FN/TN | 40/15/10/655 | 55/40/5/600 | 30/8/20/322 |
| precip_f1 | 0.769 | 0.710 | 0.682 |
| rain_mae_all (mm) | 0.90 | 1.40 | 0.85 |
| wind_mae (m/s) | 1.10 | 1.30 | 1.60 |
| Coverage | 0.98 | 0.92 | 0.55 |
| Reliability | 0.99 | 0.97 | 0.90 |

**Step 1 — normalize (ratio-to-best):**

| Component | OM | OW | PX | Best |
|-----------|-----|-----|-----|------|
| temp_mae (1.20/1.10 best) | 0.917 | 0.733 | 1.000 | PX |
| \|bias\| | 0.833 | 0.278 | 1.000 | PX |
| precip_f1 (best 0.769) | 1.000 | 0.923 | 0.887 | OM |
| rain_mae | 0.944 | 0.607 | 1.000 | PX |
| wind_mae | 1.000 | 0.846 | 0.688 | OM |
| coverage (direct) | 0.98 | 0.92 | 0.55 | — |
| reliability (direct) | 0.99 | 0.97 | 0.90 | — |

**Step 2 — weighted sum (weights §6.3):**

- OM: .30(.917)+.25(1.000)+.15(.944)+.15(1.000)+.05(.833)+.05(.98)+.05(.99) = **0.957**
- OW: .30(.733)+.25(.923)+.15(.607)+.15(.846)+.05(.278)+.05(.92)+.05(.97) = **0.777**
- PX: .30(1.000)+.25(.887)+.15(1.000)+.15(.688)+.05(1.000)+.05(.55)+.05(.90) = **0.897**

> **DR-06 (2026-07-23, WP-14):** the composite sums above were corrected from a prior
> transcription that listed OM **0.940** / OW **0.780** / PX **0.899**; recomputing the
> stated normalized values against the w-2026.1 weights yields **0.957 / 0.777 / 0.897**.
> The formula, weights, normalized values, ranking order, and statuses are unchanged;
> only the arithmetic totals are corrected. The WP-14 worked-example test asserts these
> corrected values.

**Step 3 — coverage penalty:** PX coverage 0.55 < 0.8 → ×(0.55/0.8)=0.6875 → PX final =
**0.617**. OM and OW unchanged.

**Step 4 — status:** OM (720 pairs, cov .98) → `ranked`. OW (700, .92) → `ranked`.
PX (380 pairs but coverage 0.55 ∈ [0.5,0.8)) → `provisionally_ranked`, listed after
ranked providers.

**Final published ranking (+24h, Johor Bahru, 30d, methodology 2026.1, weights w-2026.1):**

| Rank | Provider | Status | Composite | Sample | Coverage |
|------|----------|--------|-----------|--------|----------|
| 1 | Open-Meteo | ranked | 0.957 | 720 | 98% |
| 2 | OpenWeather | ranked | 0.777 | 700 | 92% |
| — | ProviderX | provisional | 0.617 | 380 | 55% |

Each row is accompanied by the full metric breakdown (§4 tables) in API and UI.

---

## 9. Recomputation and Correction Policy

| Trigger | Action |
|---------|--------|
| Late observation arrival | Matching engine pairs it with existing forecasts; affected aggregates and rankings recomputed in the next batch run. New metric rows are written (old rows retained with `superseded_by`). |
| Corrected observation | Same as late arrival; `corrected` observation takes precedence in matching. |
| Methodology/weights version change | Rankings recomputed on demand (admin action); historical metric rows are never rewritten — new version produces new rows. |
| Provider schedule change | `expected_snapshots` baseline updated prospectively only; coverage for prior periods uses the schedule that was active then (schedule versions stored). |

---

## 10. Test Vectors

### TV-1: Continuous metrics
Inputs (temp °C): pairs (f,o) = (15.0,13.5), (20.0,21.0), (18.0,18.0), (25.0,22.0).
Errors: 1.5, −1.0, 0.0, 3.0.

- MAE = (1.5+1.0+0.0+3.0)/4 = **1.375**
- RMSE = √((2.25+1.0+0.0+9.0)/4) = √3.0625 = **1.75**
- Bias = (1.5−1.0+0.0+3.0)/4 = **0.875**

### TV-2: Categorical metrics
TP=40, FP=10, FN=5, TN=45 (n=100).

- Recall (POD) = 40/45 = **0.8889**
- Precision = 40/50 = **0.8000**
- F1 = 2TP/(2TP+FP+FN) = 80/(80+10+5) = 80/95 = **0.8421**
- FAR = 10/55 = **0.1818**
- Threat Score = 40/55 = **0.7273**
- Occurrence agreement = 85/100 = **0.8500** (with imbalance warning)

### TV-3: Zero denominators
TP=0, FP=0, FN=0, TN=100 (no rain ever occurred or was forecast):
Recall = **null**, Precision = **null**, F1 = **null**, FAR = 0/100 = **0.0**, agreement = 1.0.
The precipitation component is excluded from the composite; weights redistributed.

### TV-4: Brier Score
Probabilities (0.9, 0.2, 0.6, 0.4) vs outcomes (1, 0, 0, 1):
BS = ((0.1²)+(0.2²)+(0.6²)+(0.6²))/4 = (0.01+0.04+0.36+0.36)/4 = **0.1925**

### TV-5: Weighted MAE (observation quality)
Pairs: (|e|=2.0, w=1.0), (|e|=1.0, w=0.5): MAE = (2.0+0.5)/1.5 = **1.6667**

## 11. Property-Based Testing Requirements

The implementation MUST pass these invariants (fuzzed inputs):

1. `MAE ≥ 0`; `MAE = 0` iff all errors are 0.
2. `RMSE ≥ MAE` always; `RMSE = MAE` iff all |errors| equal.
3. `|Bias| ≤ RMSE`.
4. `0 ≤ Precision, Recall, F1, FAR, TS, BS ≤ 1`.
5. `F1 = 2·P·R/(P+R)` whenever P+R > 0; F1 null iff P and R both null.
6. Adding a pair with error equal to current MAE does not change MAE beyond float ε.
7. Metrics are permutation-invariant over pair order.
8. Null denominators never produce NaN/±Inf in stored output.
9. Coverage penalty monotonically non-increasing in score as coverage falls below 0.8.
10. Composite score ∈ [0, 1] for any valid input set.
11. Recomputing over the same immutable inputs yields byte-identical stored values.

---

## 12. Cross-Reference

- Acceptance criteria implementing this methodology: `docs/requirements/04-acceptance-criteria.md` §3.
- Domain entities: `MatchedEvaluation`, `AccuracyMetric`, `ProviderRanking` in `docs/domain/01-domain-model.md`.
- Business rules: `docs/product/05-business-rules.md` BR-RANK-*.
- ADR: `docs/adr/ADR-010-composite-scoring-methodology.md`.
