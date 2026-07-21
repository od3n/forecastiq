# ForecastIQ — Functional Requirements

> **⚠️ SUPERSEDED (2026-07-22, Phase 0 Amendment).** Retained for the historical record
> only. Authoritative: `docs/requirements/01-functional-requirements.md` and
> `docs/api/00-api-requirements.md`. See
> `docs/reviews/02-phase-0-amendment-summary.md`.

**Version**: 1.0  
**Status**: Draft  

---

## 1. Forecast Collection Module

### 1.1 Provider Adapter Interface

Each provider adapter MUST implement:

```
ForecastProvider:
  - Name() string
  - CollectForecast(ctx, location, params) → ForecastSnapshot
  - HealthCheck(ctx) → ProviderHealth
  - RateLimit() → RateLimitConfig
```

### 1.2 Collection Workflow

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐     ┌──────────────┐
│  Scheduler  │────▶│  Collector   │────▶│  Validator  │────▶│  Repository  │
│  (Temporal) │     │  (Adapter)   │     │             │     │  (Postgres)  │
└─────────────┘     └──────────────┘     └─────────────┘     └──────────────┘
                          │                                         │
                          ▼                                         ▼
                    ┌──────────────┐                         ┌──────────────┐
                    │  S3 (raw)    │                         │ NATS (event) │
                    └──────────────┘                         └──────────────┘
```

### 1.3 Forecast Snapshot Schema (Logical)

| Field | Type | Description |
|-------|------|-------------|
| id | UUID | Primary key |
| provider_id | UUID | FK → providers |
| location_id | UUID | FK → locations |
| issued_at | TIMESTAMPTZ | When provider issued the forecast |
| collected_at | TIMESTAMPTZ | When we collected it |
| target_time | TIMESTAMPTZ | The time being forecasted |
| temperature_c | DECIMAL(5,2) | Temperature in Celsius |
| humidity_pct | DECIMAL(5,2) | Relative humidity % |
| wind_speed_ms | DECIMAL(6,2) | Wind speed m/s |
| wind_direction_deg | DECIMAL(5,2) | Wind direction degrees |
| pressure_hpa | DECIMAL(7,2) | Atmospheric pressure hPa |
| precipitation_mm | DECIMAL(7,2) | Precipitation amount mm |
| precipitation_prob | DECIMAL(4,3) | Probability of precipitation [0,1] |
| condition_code | VARCHAR(50) | Normalized weather condition |
| raw_payload_key | VARCHAR(255) | S3 key for raw response |
| created_at | TIMESTAMPTZ | Record creation time |

### 1.4 Collection Rules

1. Collect every 15 minutes for +1h to +24h horizons
2. Collect every 6 hours for +3d and +7d horizons
3. Never update or delete a forecast snapshot (append-only)
4. If provider returns identical forecast (same issued_at), skip storage (dedup)
5. Store raw JSON response in S3 with key: `forecasts/{provider}/{location}/{date}/{snapshot_id}.json`
6. Publish `forecast.collected` event to NATS after successful storage

### 1.5 Error Handling

| Scenario | Action |
|----------|--------|
| Provider timeout (>30s) | Retry with backoff: 1s, 2s, 4s, 8s, 16s |
| Provider 429 (rate limit) | Respect `Retry-After` header, backoff |
| Provider 5xx | Retry up to 5 times, then alert |
| Invalid response schema | Log error, store raw in S3, skip processing |
| Provider unreachable | Circuit breaker (open after 5 failures, half-open after 60s) |

---

## 2. Observation Collection Module

### 2.1 Observation Sources

| Source | Coverage | Frequency | Priority |
|--------|----------|-----------|----------|
| NOAA/NWS (US stations) | United States | Hourly | Primary (US) |
| Open-Meteo Historical | Global | Hourly | Fallback/Global |

### 2.2 Observation Schema (Logical)

| Field | Type | Description |
|-------|------|-------------|
| id | UUID | Primary key |
| location_id | UUID | FK → locations |
| source | VARCHAR(50) | Observation source identifier |
| observed_at | TIMESTAMPTZ | Actual observation time |
| temperature_c | DECIMAL(5,2) | Measured temperature |
| humidity_pct | DECIMAL(5,2) | Measured humidity |
| wind_speed_ms | DECIMAL(6,2) | Measured wind speed |
| wind_direction_deg | DECIMAL(5,2) | Measured wind direction |
| pressure_hpa | DECIMAL(7,2) | Measured pressure |
| precipitation_mm | DECIMAL(7,2) | Measured precipitation |
| condition_code | VARCHAR(50) | Observed condition |
| created_at | TIMESTAMPTZ | Record creation time |

### 2.3 Collection Rules

1. Collect observations hourly (at :05 past each hour to allow station reporting)
2. Validate ranges before storage:
   - Temperature: -90°C to +60°C
   - Humidity: 0% to 100%
   - Wind speed: 0 to 120 m/s
   - Pressure: 870 to 1084 hPa
   - Precipitation: 0 to 500 mm/hr
3. If observation is outside valid range, mark as `suspect` but still store
4. Publish `observation.collected` event to NATS

---

## 3. Comparison Engine Module

### 3.1 Matching Logic

A forecast-observation pair is valid for comparison when:
- Same `location_id`
- Forecast `target_time` matches observation `observed_at` within ±30 minutes
- Both records have non-null values for the metric being compared

### 3.2 Horizon Definitions

| Horizon | Meaning | Comparison Window |
|---------|---------|-------------------|
| +1h | Forecast issued 1h before target | issued_at + 1h ≈ target_time |
| +3h | Forecast issued 3h before target | issued_at + 3h ≈ target_time |
| +6h | Forecast issued 6h before target | issued_at + 6h ≈ target_time |
| +12h | Forecast issued 12h before target | issued_at + 12h ≈ target_time |
| +24h | Forecast issued 24h before target | issued_at + 24h ≈ target_time |
| +3d | Forecast issued 3 days before target | issued_at + 72h ≈ target_time |
| +7d | Forecast issued 7 days before target | issued_at + 168h ≈ target_time |

### 3.3 Metric Calculations

**Continuous Metrics** (temperature, wind speed, humidity, pressure):

```
MAE  = (1/n) × Σ|forecast_i - actual_i|
RMSE = √((1/n) × Σ(forecast_i - actual_i)²)
Bias = (1/n) × Σ(forecast_i - actual_i)
```

**Categorical Metrics** (rain/no-rain, threshold: precipitation_prob ≥ 0.5 or precipitation_mm > 0):

```
Hit Rate (POD)  = TP / (TP + FN)
False Alarm Rate = FP / (FP + TN)
Precision       = TP / (TP + FP)
Recall          = TP / (TP + FN)  [same as Hit Rate]
F1              = 2 × (Precision × Recall) / (Precision + Recall)
```

Where:
- TP = Forecast rain AND actual rain
- FP = Forecast rain AND no actual rain
- FN = Forecast no rain AND actual rain
- TN = Forecast no rain AND no actual rain

### 3.4 Aggregation Dimensions

Metrics are aggregated across:
- Provider
- Location
- Horizon
- Time period (hourly, daily, weekly, monthly)
- Weather variable (temperature, wind, humidity, pressure, precipitation)

### 3.5 Execution

- Runs as batch job every 30 minutes
- Processes new observations and matches against existing forecasts
- Recalculates affected aggregations
- Publishes `accuracy.calculated` event

---

## 4. REST API Module

### 4.1 Endpoint Summary

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/providers` | List all providers |
| GET | `/api/v1/providers/:id` | Get provider details |
| GET | `/api/v1/locations` | List locations |
| POST | `/api/v1/locations` | Create location |
| GET | `/api/v1/locations/:id` | Get location |
| PUT | `/api/v1/locations/:id` | Update location |
| DELETE | `/api/v1/locations/:id` | Delete location |
| GET | `/api/v1/forecasts` | Query forecasts (filtered) |
| GET | `/api/v1/forecasts/:id` | Get specific forecast |
| GET | `/api/v1/observations` | Query observations |
| GET | `/api/v1/observations/:id` | Get specific observation |
| GET | `/api/v1/accuracy` | Query accuracy metrics |
| GET | `/api/v1/accuracy/summary` | Aggregated accuracy summary |
| GET | `/api/v1/rankings` | Provider rankings |
| GET | `/api/v1/rankings/:location_id` | Rankings for location |
| POST | `/api/v1/auth/login` | Login (returns JWT) |
| POST | `/api/v1/auth/refresh` | Refresh JWT |
| POST | `/api/v1/api-keys` | Create API key |
| GET | `/api/v1/api-keys` | List API keys |
| DELETE | `/api/v1/api-keys/:id` | Revoke API key |
| GET | `/api/v1/health` | Health check |
| GET | `/api/v1/ready` | Readiness check |

### 4.2 Pagination

Cursor-based pagination on all list endpoints:

```json
{
  "data": [...],
  "pagination": {
    "next_cursor": "eyJpZCI6MTAwfQ==",
    "has_more": true,
    "total_count": 1523
  }
}
```

Parameters: `?limit=50&cursor=eyJpZCI6MTAwfQ==`

### 4.3 Filtering

Query parameters per endpoint:
- Forecasts: `provider_id`, `location_id`, `issued_after`, `issued_before`, `horizon`
- Observations: `location_id`, `source`, `observed_after`, `observed_before`
- Accuracy: `provider_id`, `location_id`, `horizon`, `variable`, `period_start`, `period_end`
- Rankings: `location_id`, `horizon`, `variable`, `period`

### 4.4 Error Response (RFC 7807)

```json
{
  "type": "https://forecastiq.com/errors/validation",
  "title": "Validation Error",
  "status": 422,
  "detail": "Field 'latitude' must be between -90 and 90",
  "instance": "/api/v1/locations",
  "errors": [
    {"field": "latitude", "message": "must be between -90 and 90"}
  ]
}
```

---

## 5. Authentication Module

### 5.1 JWT Flow

1. User authenticates via `/api/v1/auth/login` (email + password)
2. Server returns `access_token` (15min) + `refresh_token` (7d)
3. Client sends `Authorization: Bearer <access_token>` on requests
4. On expiry, client calls `/api/v1/auth/refresh` with refresh token
5. Refresh tokens are single-use (rotation)

### 5.2 API Key Flow

1. Admin/user creates API key via portal or API
2. Key is shown ONCE at creation (hashed for storage)
3. Client sends `X-API-Key: <key>` header
4. Server validates hash, checks scopes, enforces rate limit

### 5.3 RBAC Roles

| Role | Permissions |
|------|-------------|
| admin | Full CRUD on all resources, user management, system config |
| user | Read forecasts/observations/accuracy, manage own locations/keys |
| readonly | Read-only access to assigned resources |

---

## 6. Dashboard Module

### 6.1 Views

| View | Description |
|------|-------------|
| Overview | Provider ranking cards, top-level accuracy stats |
| Accuracy Trends | Line charts of MAE/RMSE over time per provider |
| Forecast vs Actual | Overlay chart comparing forecast to observation |
| Heatmap | Geographic grid showing accuracy by location |
| Provider Detail | Deep-dive into single provider performance |
| Location Detail | All providers compared for single location |
| Settings | User preferences, API key management |

### 6.2 Interactions

- Date range picker (default: last 30 days)
- Provider multi-select filter
- Location selector (search + map pin)
- Horizon selector (+1h through +7d)
- Variable selector (temp, wind, humidity, pressure, precip)
- Export to CSV/PNG

---

## 7. Admin Portal Module

### 7.1 Capabilities

- Enable/disable providers
- Update provider API credentials (encrypted)
- Add/edit/remove monitored locations
- Configure collection schedules (cron expressions)
- View collector run history and errors
- Manage users (create, disable, assign roles)
- Generate/revoke API keys
- View system health dashboard
- View audit logs
