---
kind: business_term
name: Business Glossary
category: business_term
scope:
    - '**'
---

### ForecastIQ
- Definition：Internal product name for the Weather Intelligence Platform - a system that measures, compares, and improves weather forecast accuracy by collecting multi-provider forecasts, storing them immutably, comparing against actual observations, and calculating statistical accuracy metrics.
- Aliases：Forecast IQ

### Weather Intelligence Platform
- Definition：Product category definition distinguishing ForecastIQ from consumer weather apps. Focuses on measuring forecast accuracy rather than displaying weather conditions, serving businesses and individuals who need to evaluate which forecast provider to trust.

### Forecast Snapshot
- Definition：Immutable record of a provider's weather forecast at a specific issuance time. Once stored, cannot be modified or deleted - only appended. Contains normalized weather variables (temperature, humidity, wind, pressure, precipitation) plus metadata including collection timestamp and raw payload reference.
- Aliases：Snapshot

### Horizon
- Definition：Time offset between forecast issuance and target prediction time, expressed as positive intervals (+1h, +3h, +6h, +12h, +24h, +3d, +7d). Determines how far into the future a forecast predicts, with shorter horizons typically more accurate.
- Aliases：Forecast Horizon、Lead Time

### Accuracy Metrics
- Definition：Statistical measures comparing forecast predictions against actual observations. Includes continuous metrics (MAE, RMSE, Bias) for temperature/wind/humidity/pressure and categorical metrics (Hit Rate, False Alarm Rate, Precision, Recall, F1) for precipitation prediction.
- Aliases：Metrics、Accuracy Measures

### Comparison Engine
- Definition：Core processing module that matches forecasts to observations by location and time window (±30 minutes), calculates per-pair errors across all variables, and aggregates into statistical accuracy metrics by provider/location/horizon/time period.
- Aliases：CE、Accuracy Engine

### Provider Adapter
- Definition：Implementation of the ForecastProvider interface for each weather data source. Standardizes how different APIs (OpenWeather, Tomorrow.io, etc.) are queried, parsed, and normalized into common forecast snapshot format while handling provider-specific error conditions and rate limits.
- Aliases：Adapter、Provider Interface

### Collection Schedule
- Definition：Configurable frequency for querying weather providers, varying by horizon length. Shorter horizons (+1h to +24h) collected every 15 minutes, longer horizons (+3d, +7d) collected every 6 hours. Expressed as cron expressions configurable per provider via admin portal.
- Aliases：Schedule、Collection Frequency

### Quality Flag
- Definition：Data validation indicator for observations. Values include 'valid' for measurements within physical ranges (-90°C to +60°C temp, 0-100% humidity, etc.) and 'suspect' for out-of-range values that are still stored but excluded from accuracy calculations.
- Aliases：Quality Indicator、Data Quality

### Circuit Breaker
- Definition：Resilience pattern protecting against cascading failures when external weather providers become unavailable. Opens after 5 consecutive failures, enters half-open state after 60 seconds for test calls, closes if test succeeds or remains open for another 60 seconds if failed.
- Aliases：Breaker、CB

### Immutability
- Definition：Core design principle ensuring forecast snapshots are never overwritten once stored. Every collection creates new records, enabling audit trails, reproducibility of accuracy calculations, and historical tracking of forecast evolution over time.
- Aliases：Append-only、Immutable Records

### Cursor-based Pagination
- Definition：Pagination strategy using opaque cursors instead of page numbers, providing stable navigation under concurrent writes and better performance for large datasets. Implemented across all list endpoints with next_cursor, has_more, and total_count metadata.
- Aliases：Cursor Pagination、Stable Pagination
