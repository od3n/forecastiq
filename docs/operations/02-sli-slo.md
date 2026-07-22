# ForecastIQ — SLI / SLO (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: NFR-A01..A08, NFR-P01..P08, NFR-OBS07; `docs/architecture/09-reliability-architecture.md`

---

## 1. SLI Definitions

| SLI | Definition | Measurement | Good-event threshold |
|-----|-----------|-------------|---------------------|
| Availability | Proportion of successful `/healthz` probes | Hosted uptime, 1-min interval, 3-region median | Probe success |
| API latency (p50) | 50th percentile `http_request_duration_seconds` | /metrics histogram, 5-min rollup | < 50 ms |
| API latency (p95) | 95th percentile | same | < 200 ms |
| API latency (p99) | 99th percentile | same | < 500 ms |
| Collection success | (successful + partial slots) / due slots, monthly | schedule_runs + forecast_collections | Slot completed |
| Observation freshness | Proportion of hours with observation age < 90 min per active location | observation_freshness_age_seconds | < 90 min |
| Engine lag | now − MAX(accuracy_metrics.calculated_at) | engine_lag_seconds | < 2 h |
| Ranking freshness | Cells with BR-FRESH state = fresh | ranking_freshness_age_seconds | < 2 h |
| Durability | Committed-data-loss events | Incident count | 0 |

## 2. SLO Targets (monthly)

| SLO | Target | Error budget | Basis |
|-----|--------|--------------|-------|
| Availability | 99.5% | 3.65 h/month | NFR-A01 (single-VPS honest ceiling) |
| Latency p50 | 99% of 5-min windows < 50 ms | 1% | NFR-P01 |
| Latency p95 | 99% of windows < 200 ms | 1% | NFR-P02 |
| Collection success | ≥ 99% of slots | 1% of ~7,200 slots/mo ≈ 72 | NFR-A03 (provider-side failures classified out per FC-13) |
| Engine lag | < 2 h for 99% of time | 7.2 h/month | BR-FRESH rankings |
| Durability | 100% | 0 | NFR-A06 |

## 3. Error Budget Policy

| Budget state | Action |
|--------------|--------|
| > 50% remaining | Normal feature work |
| 25–50% consumed | Review: identify dominant consumer; no risky deploys without rollback rehearsal |
| < 25% remaining | Feature freeze on reliability-impacting changes; reliability work prioritized |
| Exhausted | Incident review mandatory; post-mortem; remediation plan before next feature deploy |

Burn-rate alerts: 2× consumption rate over 6 h → warning; 10× over 1 h → critical (page).

## 4. Recording Rules (Grafana/Prometheus)

```yaml
- record: forecastiq:availability:ratio_rate30d
  expr: avg_over_time(up{job="forecastiq"}[30d])
- record: forecastiq:latency_p95:5m
  expr: histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le))
- record: forecastiq:collection_success:ratio_rate30d
  expr: |
    sum(rate(schedule_runs_completed_total{job_type=~"forecast_collection|observation_collection"}[30d]))
    / sum(rate(schedule_runs_total{job_type=~"forecast_collection|observation_collection"}[30d]))
```

## 5. Review Cadence

- **Monthly SLO review** (NFR-OBS07): burn analysis, budget state, action items.
- Quarterly: target calibration against actual usage patterns (first review after 90 days production).
- SLO changes require: reliability doc version bump + architecture report note.

## 6. Exclusions (documented, honest)

- Provider API outages do not consume availability budget (API stays up serving cached/partial with warnings) — they consume freshness/collection SLIs where classified provider-side (FC-13).
- Scheduled maintenance (DB vendor windows) excluded from availability if announced > 24 h and < 30 min.
- Deploys: < 30 s gaps count against budget (no exclusion — incentive to keep deploys fast).

## 7. Cross-Reference

- Monitoring/alerting: `docs/operations/03-monitoring-and-alerting.md`
- Reliability architecture: `docs/architecture/09-reliability-architecture.md`
