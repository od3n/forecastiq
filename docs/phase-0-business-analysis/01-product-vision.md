# ForecastIQ — Product Vision

> **⚠️ SUPERSEDED (2026-07-22, Phase 0 Amendment).** This draft is retained for the
> historical record only. The authoritative document is
> `docs/product/01-product-vision.md`. See
> `docs/reviews/02-phase-0-amendment-summary.md` for the mapping and rationale.

## Tagline

> Measure. Compare. Improve weather forecast accuracy.

---

## Vision Statement

ForecastIQ is a **Weather Intelligence Platform** that continuously collects weather forecasts from multiple providers, stores immutable forecast snapshots, collects actual observations, compares forecasts against reality, calculates accuracy metrics, and visualizes provider performance over time.

ForecastIQ is **not** a weather app. It does not tell users what the weather will be. It tells users **which forecast provider to trust**.

---

## Problem Statement

Businesses and individuals depend on weather forecasts for operational decisions worth billions annually. Yet no accessible tool exists to answer:

| Question | Impact |
|----------|--------|
| Which forecast provider is most accurate for my location? | Operational efficiency |
| Which forecast horizon performs best? | Planning confidence |
| How accurate are rain predictions specifically? | Event preparation |
| Which locations consistently have inaccurate forecasts? | Risk management |
| Has forecast quality improved or degraded over time? | Vendor accountability |

Organizations currently rely on anecdotal evidence, single-provider lock-in, or expensive bespoke solutions. ForecastIQ democratizes forecast accuracy intelligence.

---

## Solution Overview

ForecastIQ provides:

1. **Multi-provider forecast collection** — Immutable snapshots from OpenWeather, Tomorrow.io, Visual Crossing, Open-Meteo
2. **Observation collection** — Ground-truth actual weather data
3. **Comparison engine** — Statistical comparison at multiple horizons (+1h through +7d)
4. **Accuracy metrics** — MAE, RMSE, Bias, Rain Hit Rate, False Alarm Rate, Precision, Recall, F1
5. **REST API** — Programmatic access to all data and metrics
6. **Dashboard** — Visual analytics for provider ranking, heatmaps, trends
7. **Alert engine** — Notifications on forecast changes, provider degradation, storms
8. **Admin portal** — Provider/location/schedule management

---

## Target Users

### Tier 1: Individual (Free/Low-cost)
- Weather enthusiasts
- Photographers planning shoots
- Hikers and outdoor recreationists

### Tier 2: Business (Subscription)
- Logistics companies (route planning)
- Construction firms (pour scheduling)
- Agriculture (irrigation, harvest timing)
- Waste management (collection scheduling)
- Outdoor event planners

### Tier 3: Enterprise (Custom)
- Airports (ground operations)
- Insurance companies (claims prediction)
- Shipping companies (route optimization)
- Utilities (load forecasting, storm prep)

---

## Success Metrics

| Metric | Target (Year 1) |
|--------|-----------------|
| Providers integrated | ≥ 4 |
| Locations monitored | ≥ 100 |
| Forecast snapshots stored | ≥ 10M |
| API uptime | 99.9% |
| Median API latency (p95) | < 200ms |
| Dashboard load time | < 2s |
| Registered users | ≥ 1,000 |
| Paying customers | ≥ 10 |

---

## Competitive Differentiation

| Feature | Weather Apps | ForecastIQ |
|---------|-------------|------------|
| Shows weather | ✓ | ✗ |
| Compares providers | ✗ | ✓ |
| Historical accuracy | ✗ | ✓ |
| Statistical metrics | ✗ | ✓ |
| API access | Limited | Full |
| Provider-agnostic | ✗ | ✓ |
| Audit trail | ✗ | ✓ (immutable snapshots) |

---

## Revenue Model (Future)

- **Free tier**: 3 locations, 7-day history, community dashboard
- **Pro tier**: 25 locations, 90-day history, API access, alerts
- **Enterprise tier**: Unlimited locations, full history, SLA, dedicated support, custom integrations

---

## Guiding Principles

1. **Immutability** — Forecasts are never overwritten. Every snapshot is preserved.
2. **Transparency** — All metrics are reproducible from stored data.
3. **Provider-agnostic** — No preferential treatment of any provider.
4. **API-first** — Every feature available via API before UI.
5. **Observability** — Every service is measurable, traceable, and debuggable.
