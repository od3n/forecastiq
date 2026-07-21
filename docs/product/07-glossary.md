# ForecastIQ — Glossary

**Version**: 1.0 (Phase 0 Amendment)
**Status**: Authoritative — binding terminology for all documents, API, UI, and code

---

| Term | Definition |
|------|-----------|
| **Accuracy** | NEVER used as a bare term. See `occurrence_agreement` (defined, warned, unranked) in the methodology. |
| **Bias** | Mean signed error `(1/n)Σ(f−o)`. Positive = forecasts higher than observed. |
| **Canonical condition code** | A provider-independent weather condition label from the ForecastIQ taxonomy (v1: clear, partly_cloudy, cloudy, fog, drizzle, rain, heavy_rain, thunderstorm, snow, sleet, unknown). |
| **Collection reliability** | Fraction of scheduled collections that succeeded, with our-side failures classified separately from provider-side outages. |
| **Composite score** | Weighted, normalized, coverage-penalized provider score per methodology §6. Always versioned and decomposable. |
| **Coverage (provider data coverage)** | Fraction of expected snapshots actually delivered with non-null values for a variable over the evaluation period. |
| **Evaluation period** | The trailing time window (default 30 days; 90 for slow horizons) over which metrics and rankings are computed. |
| **F1** | Harmonic mean of Precision and Recall: `2TP/(2TP+FP+FN)`. |
| **False Alarm Rate (FAR)** | `FP/(FP+TN)` — fraction of observed dry hours wrongly forecast as rain. NOT the "false alarm ratio" `FP/(TP+FP)` (which equals 1−Precision). |
| **ForecastCollection** | One provider API collection attempt for one location. Parent of snapshots. Carries raw payload key, checksum, status, and error accounting. |
| **ForecastSnapshot** | One prediction for one target time, extracted from a ForecastCollection. Immutable. ("Snapshot" never means a whole provider response.) |
| **Hit Rate** | DEPRECATED/REMOVED term — ambiguous. Use **Recall (POD)**. |
| **Horizon** | Time offset between forecast issuance and target time (`target_time − issued_at`), expressed in minutes; UI labels +1h…+7d. |
| **MAE** | Mean Absolute Error `(1/n)Σ|f−o|`. |
| **MatchedEvaluation** | One forecast–observation pair eligible for comparison under matching rules. |
| **methodology_version** | Version identifier of the metric & ranking methodology (currently `2026.1`). |
| **Observation** | A measured or derived weather record for a location and time, typed by provenance. Immutable; corrections are new records. |
| **observation_type** | Provenance class: `station_observation`, `interpolated`, `reanalysis`, `provider_estimated`. Drives quality weighting. |
| **Occurrence agreement** | `(TP+TN)/n` for rain/no-rain. Published with class-imbalance warning; never used for ranking. |
| **POD (Probability of Detection)** | Synonym of Recall: `TP/(TP+FN)`. |
| **Precision** | `TP/(TP+FP)` — fraction of forecast rain hours that actually had rain. |
| **Provisional (ranking status)** | 10–29 matched pairs or coverage in [0.5, 0.8): shown with explicit "provisional" label, listed after ranked providers. |
| **Quality flag** | Observation validity class: `valid`, `suspect` (out of range, excluded from metrics), `corrected`. |
| **Ranked (ranking status)** | ≥ 30 matched pairs per required variable, coverage ≥ 0.5, ≥ 7 days of data: full numeric rank published. |
| **Recall** | `TP/(TP+FN)` — fraction of actual rain hours that were forecast. The only "hit rate" ForecastIQ uses. |
| **RMSE** | Root Mean Squared Error `√((1/n)Σ(f−o)²)`. |
| **Snapshot** | See ForecastSnapshot. Never means a full API response. |
| **Threat Score (CSI)** | `TP/(TP+FP+FN)` — supplementary occurrence metric. |
| **Unranked** | < 10 pairs or coverage < 0.5: "insufficient data" — no ordering published. |
| **weights_version** | Identifier of the weight set used for a composite score (`w-2026.1` default or `custom:<hash>`). |
| **Workspace** | Ownership boundary. MVP: single `system` workspace; schema supports personal workspaces (Level 3: organizations). |
