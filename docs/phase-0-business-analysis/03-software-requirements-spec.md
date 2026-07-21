# ForecastIQ — Software Requirements Specification (SRS)

**Version**: 1.0  
**Status**: Draft  

---

## 1. Introduction

### 1.1 Purpose

This document specifies the software requirements for ForecastIQ, a Weather Intelligence Platform. It is intended for engineers, QA, and stakeholders involved in building, testing, and operating the system.

### 1.2 Scope

ForecastIQ collects multi-provider weather forecasts, stores them immutably, collects observations, computes accuracy metrics, and exposes data via REST API and web dashboard.

### 1.3 Definitions & Acronyms

| Term | Definition |
|------|-----------|
| Forecast Snapshot | An immutable record of a provider's forecast at a specific issuance time |
| Observation | Actual measured weather data at a specific time/location |
| Horizon | Time offset between forecast issuance and target time (e.g., +6h) |
| MAE | Mean Absolute Error |
| RMSE | Root Mean Square Error |
| Bias | Mean Error (signed) |
| Hit Rate | True positive rate for categorical events (e.g., rain) |
| FAR | False Alarm Rate |
| Provider | External weather forecast API (OpenWeather, Tomorrow.io, etc.) |

---

## 2. Overall Description

### 2.1 System Context

ForecastIQ is a distributed system consisting of:
- **Collector services** — Poll external provider APIs on schedules
- **Comparison engine** — Batch job comparing forecasts to observations
- **API service** — Serves data to clients
- **Dashboard** — Web UI consuming the API
- **Admin portal** — Management interface
- **Alert engine** — Monitors conditions and dispatches notifications

### 2.2 User Classes

| Class | Description | Access Level |
|-------|-------------|-------------|
| Anonymous | Public dashboard viewer | Read-only, limited |
| Registered | Individual user | Read, personal settings |
| Business | Subscription user | API access, alerts, extended history |
| Enterprise | Custom contract | Full API, SLA, support |
| Admin | Platform operator | Full system access |
| System | Internal services | Service-to-service |

### 2.3 Operating Environment

- Cloud-native (Kubernetes)
- PostgreSQL + TimescaleDB for storage
- Redis for caching
- NATS JetStream for messaging
- Docker containers
- Linux-based infrastructure

---

## 3. Functional Requirements

### 3.1 Forecast Collection (FC)

| ID | Requirement | Priority |
|----|-------------|----------|
| FC-01 | System SHALL collect forecasts from OpenWeather API | Critical |
| FC-02 | System SHALL collect forecasts from Tomorrow.io API | Critical |
| FC-03 | System SHALL collect forecasts from Visual Crossing API | Critical |
| FC-04 | System SHALL collect forecasts from Open-Meteo API | Critical |
| FC-05 | System SHALL store each forecast as an immutable snapshot | Critical |
| FC-06 | System SHALL record collection timestamp, provider, location, and raw payload reference | Critical |
| FC-07 | System SHALL support configurable collection schedules per provider/location | High |
| FC-08 | System SHALL retry failed collections with exponential backoff (max 5 retries) | High |
| FC-09 | System SHALL deduplicate identical forecasts (same provider, location, issuance time) | Medium |
| FC-10 | System SHALL store raw API responses in object storage for audit | Medium |
| FC-11 | System SHALL respect provider rate limits via token bucket | Critical |
| FC-12 | System SHALL emit metrics for collection success/failure rates | High |

### 3.2 Observation Collection (OC)

| ID | Requirement | Priority |
|----|-------------|----------|
| OC-01 | System SHALL collect actual weather observations from NOAA/NWS | Critical |
| OC-02 | System SHALL collect observations from Open-Meteo historical API as fallback | High |
| OC-03 | System SHALL store observations separately from forecasts | Critical |
| OC-04 | System SHALL record observation time, location, source, and measured values | Critical |
| OC-05 | System SHALL validate observation data ranges (e.g., temp -90°C to +60°C) | High |
| OC-06 | System SHALL support hourly observation collection | High |

### 3.3 Comparison Engine (CE)

| ID | Requirement | Priority |
|----|-------------|----------|
| CE-01 | System SHALL compare forecasts against observations at horizons: +1h, +3h, +6h, +12h, +24h, +3d, +7d | Critical |
| CE-02 | System SHALL calculate MAE for temperature, wind speed, humidity, pressure | Critical |
| CE-03 | System SHALL calculate RMSE for temperature, wind speed, humidity, pressure | Critical |
| CE-04 | System SHALL calculate Bias (mean signed error) | Critical |
| CE-05 | System SHALL calculate Rain Hit Rate (probability of detection) | Critical |
| CE-06 | System SHALL calculate False Alarm Rate for precipitation | Critical |
| CE-07 | System SHALL calculate Precision, Recall, F1 for rain/no-rain classification | Critical |
| CE-08 | System SHALL support comparison by provider, location, horizon, and time range | High |
| CE-09 | System SHALL store computed metrics for fast retrieval | High |
| CE-10 | System SHALL recalculate metrics when late observations arrive | Medium |
| CE-11 | System SHALL handle missing observations gracefully (skip, not fail) | High |

### 3.4 REST API (API)

| ID | Requirement | Priority |
|----|-------------|----------|
| API-01 | System SHALL expose `/api/v1/providers` — list providers | Critical |
| API-02 | System SHALL expose `/api/v1/locations` — CRUD locations | Critical |
| API-03 | System SHALL expose `/api/v1/forecasts` — query forecasts with filters | Critical |
| API-04 | System SHALL expose `/api/v1/observations` — query observations | Critical |
| API-05 | System SHALL expose `/api/v1/accuracy` — query accuracy metrics | Critical |
| API-06 | System SHALL expose `/api/v1/rankings` — provider rankings | Critical |
| API-07 | System SHALL support pagination (cursor-based) on all list endpoints | Critical |
| API-08 | System SHALL support filtering via query parameters | High |
| API-09 | System SHALL support sorting via `sort` parameter | High |
| API-10 | System SHALL return RFC 7807 Problem Details for errors | High |
| API-11 | System SHALL version API via URL path (`/api/v1/`) | Critical |
| API-12 | System SHALL enforce rate limits per API key | High |
| API-13 | System SHALL return `X-RateLimit-*` headers | Medium |
| API-14 | System SHALL support `Accept: application/json` only (MVP) | Low |

### 3.5 Dashboard (DB)

| ID | Requirement | Priority |
|----|-------------|----------|
| DB-01 | Dashboard SHALL display current provider ranking by selected location | Critical |
| DB-02 | Dashboard SHALL display accuracy trends over time (line charts) | Critical |
| DB-03 | Dashboard SHALL display forecast vs. actual comparison charts | High |
| DB-04 | Dashboard SHALL display location heatmap of accuracy | High |
| DB-05 | Dashboard SHALL support date range selection | High |
| DB-06 | Dashboard SHALL support provider selection/filtering | High |
| DB-07 | Dashboard SHALL support location selection | Critical |
| DB-08 | Dashboard SHALL load within 2 seconds on standard connection | High |
| DB-09 | Dashboard SHALL be responsive (desktop + tablet) | Medium |

### 3.6 Authentication & Authorization (AUTH)

| ID | Requirement | Priority |
|----|-------------|----------|
| AUTH-01 | System SHALL support JWT-based authentication for dashboard users | Critical |
| AUTH-02 | System SHALL support API key authentication for programmatic access | Critical |
| AUTH-03 | System SHALL implement RBAC with roles: admin, user, readonly | High |
| AUTH-04 | System SHALL support API key creation, rotation, and revocation | High |
| AUTH-05 | System SHALL enforce least-privilege access per resource | High |
| AUTH-06 | System SHALL log all authentication events (audit trail) | Medium |
| AUTH-07 | System SHALL support token expiry (JWT: 15min access, 7d refresh) | High |

### 3.7 Admin Portal (ADMIN)

| ID | Requirement | Priority |
|----|-------------|----------|
| ADMIN-01 | Admin SHALL manage provider configurations (enable/disable, credentials) | Critical |
| ADMIN-02 | Admin SHALL manage monitored locations (add, edit, remove) | Critical |
| ADMIN-03 | Admin SHALL manage collection schedules | High |
| ADMIN-04 | Admin SHALL view collector health and recent run history | High |
| ADMIN-05 | Admin SHALL manage user accounts and API keys | High |
| ADMIN-06 | Admin SHALL view system metrics and alerts | Medium |

### 3.8 Alert Engine (ALERT) — Post-MVP

| ID | Requirement | Priority |
|----|-------------|----------|
| ALERT-01 | System SHALL detect significant forecast changes (>5°C temp swing in 3h) | Medium |
| ALERT-02 | System SHALL detect provider accuracy degradation (>20% MAE increase over 7d) | Medium |
| ALERT-03 | System SHALL support webhook delivery for alerts | Medium |
| ALERT-04 | System SHALL support email notification for alerts | Medium |
| ALERT-05 | System SHALL allow users to configure alert thresholds | Low |

---

## 4. Non-Functional Requirements

See `05-non-functional-requirements.md` for full specification.

Summary:
- Availability: 99.9%
- Latency: p95 < 200ms (API), < 2s (dashboard)
- Throughput: 1,000 req/s sustained
- Data retention: 2 years raw, indefinite aggregated
- Security: OWASP Top 10 compliance
- Observability: Metrics, logs, traces on all services

---

## 5. Interface Requirements

### 5.1 External Interfaces

| Interface | Protocol | Direction |
|-----------|----------|-----------|
| OpenWeather API | HTTPS/REST | Outbound |
| Tomorrow.io API | HTTPS/REST | Outbound |
| Visual Crossing API | HTTPS/REST | Outbound |
| Open-Meteo API | HTTPS/REST | Outbound |
| NOAA/NWS API | HTTPS/REST | Outbound |
| S3-compatible storage | HTTPS | Outbound |
| SMTP (email alerts) | SMTP/TLS | Outbound |
| Webhook endpoints | HTTPS | Outbound |

### 5.2 Internal Interfaces

| Interface | Protocol | Purpose |
|-----------|----------|---------|
| API ↔ Database | TCP (PostgreSQL wire) | Data access |
| API ↔ Redis | TCP (RESP) | Caching |
| Collectors ↔ NATS | TCP (NATS protocol) | Event publishing |
| Services ↔ Services | gRPC / NATS | Internal communication |

---

## 6. Data Requirements

### 6.1 Data Entities

- Provider
- Location
- ForecastSnapshot
- Observation
- AccuracyMetric
- User
- APIKey
- Alert
- AuditLog

### 6.2 Data Volume Estimates (Year 1)

| Entity | Estimated Records | Growth Rate |
|--------|------------------|-------------|
| ForecastSnapshot | 10M | ~30K/day |
| Observation | 1M | ~3K/day |
| AccuracyMetric | 5M | ~15K/day |
| AuditLog | 500K | ~1.5K/day |

---

## 7. Traceability Matrix

| Business Req | Functional Req | Module |
|-------------|---------------|--------|
| BO-01 | API-06, DB-01 | Ranking |
| BO-02 | CE-01..CE-09, DB-02 | Comparison |
| BO-03 | ALERT-01..ALERT-05 | Alerts |
| BO-04 | AUTH-01..AUTH-07, API-12 | Platform |
| BR-01 | FC-05 | Collection |
| BR-02 | OC-03 | Observation |
| BR-03 | CE-11 | Comparison |
