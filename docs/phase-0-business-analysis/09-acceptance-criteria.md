# ForecastIQ — Acceptance Criteria (Detailed)

> **⚠️ SUPERSEDED (2026-07-22, Phase 0 Amendment).** Retained for the historical record
> only. Authoritative: `docs/requirements/04-acceptance-criteria.md` (AC-3.2 corrected;
> "hit rate" removed). See `docs/reviews/02-phase-0-amendment-summary.md`.

**Version**: 1.0  
**Status**: Draft  

---

## Format

Each acceptance criterion follows the structure:
- **Given** [precondition]
- **When** [action]
- **Then** [expected outcome]

---

## 1. Forecast Collection

### AC-1.1: Successful forecast collection

```gherkin
Given provider "OpenWeather" is enabled
  And location "New York" (40.7128, -74.0060) is active
  And the collection schedule triggers
When the collector calls the OpenWeather API
Then a forecast snapshot is stored with:
  | field            | value                          |
  | provider_id      | <openweather_provider_id>      |
  | location_id      | <new_york_location_id>         |
  | issued_at        | API response issuance time     |
  | collected_at     | current UTC timestamp          |
  | temperature_c    | value from API response        |
  | raw_payload_key  | forecasts/openweather/ny/...   |
And event "forecast.collected" is published to NATS
And metric "forecast_collection_total{provider=openweather,status=success}" is incremented
```

### AC-1.2: Immutability enforcement

```gherkin
Given a forecast snapshot with id "abc-123" exists
When any process attempts to UPDATE the snapshot
Then the operation is rejected
  And an error "forecast snapshots are immutable" is returned
  And an audit log entry is created
```

### AC-1.3: Deduplication

```gherkin
Given provider "Tomorrow.io" issued a forecast at "2024-01-15T12:00:00Z" for location "London"
  And that forecast is already stored
When the collector receives the same forecast (same provider, location, issued_at)
Then no new snapshot is created
  And metric "forecast_collection_total{status=deduplicated}" is incremented
```

### AC-1.4: Rate limit compliance

```gherkin
Given OpenWeather rate limit is 60 calls/minute
  And 60 calls have been made in the current minute
When the collector attempts call #61
Then the call is delayed until the next minute window
  And no 429 error is received from the provider
```

### AC-1.5: Retry with backoff

```gherkin
Given provider "Visual Crossing" returns HTTP 500
When the collector receives the error
Then it retries after 1 second
  And if still failing, retries after 2 seconds
  And if still failing, retries after 4 seconds
  And if still failing, retries after 8 seconds
  And if still failing, retries after 16 seconds
  And after 5 failures, the collection is marked as failed
  And an alert is emitted
```

### AC-1.6: Circuit breaker

```gherkin
Given provider "Tomorrow.io" has failed 5 consecutive collections
When the circuit breaker opens
Then no further calls are made to Tomorrow.io for 60 seconds
  And other providers continue collecting normally
  And after 60 seconds, a single test call is made (half-open)
  And if the test succeeds, the circuit closes
  And if the test fails, the circuit remains open for another 60 seconds
```

---

## 2. Observation Collection

### AC-2.1: Valid observation stored

```gherkin
Given location "Chicago" (41.8781, -87.6298) is active
  And NOAA reports temperature 22.5°C at "2024-01-15T14:00:00Z"
When the observation collector processes the report
Then an observation is stored with:
  | field          | value              |
  | location_id    | <chicago_id>       |
  | source         | "noaa_nws"         |
  | observed_at    | 2024-01-15T14:00Z  |
  | temperature_c  | 22.5               |
  | quality_flag   | "valid"            |
And event "observation.collected" is published
```

### AC-2.2: Out-of-range validation

```gherkin
Given an observation reports temperature_c = 75.0
When the validator checks the value
Then the observation is stored with quality_flag = "suspect"
  And a warning log is emitted
  And the observation is excluded from accuracy calculations
```

### AC-2.3: Observation separate from forecast

```gherkin
Given the database schema
When I inspect the tables
Then observations are in table "observations"
  And forecasts are in table "forecast_snapshots"
  And there is no foreign key between them
  And they are linked only via location_id + time matching in the comparison engine
```

---

## 3. Comparison Engine

### AC-3.1: Successful comparison

```gherkin
Given a forecast from "OpenWeather" for "Denver" targeting "2024-01-15T18:00:00Z"
  And an observation for "Denver" at "2024-01-15T18:00:00Z"
  And the forecast was issued at "2024-01-15T12:00:00Z" (horizon = +6h)
  And forecast temperature = 15.0°C
  And observed temperature = 13.5°C
When the comparison engine runs
Then an accuracy metric is calculated:
  | field         | value        |
  | provider_id   | <openweather>|
  | location_id   | <denver>     |
  | horizon       | "+6h"        |
  | variable      | "temperature"|
  | metric_type   | "MAE"        |
  | value         | 1.5          |
  | sample_count  | ≥ 1          |
```

### AC-3.2: Rain classification metrics

```gherkin
Given 100 forecast-observation pairs for "Seattle" at +24h horizon
  And 40 are True Positives (forecast rain, actual rain)
  And 10 are False Positives (forecast rain, no actual rain)
  And 5 are False Negatives (forecast no rain, actual rain)
  And 45 are True Negatives (forecast no rain, no actual rain)
When metrics are calculated
Then:
  | metric          | value  |
  | hit_rate        | 0.889  |  (40/45)
  | false_alarm_rate| 0.182  |  (10/55)
  | precision       | 0.800  |  (40/50)
  | recall          | 0.889  |  (40/45)
  | f1              | 0.842  |
```

### AC-3.3: Missing observation handling

```gherkin
Given a forecast exists for "Miami" at +12h horizon
  And no observation exists for that time window
When the comparison engine runs
Then no accuracy metric is generated for that pair
  And no error is raised
  And the forecast remains available for future comparison
```

### AC-3.4: Late observation arrival

```gherkin
Given forecasts for "Boston" on 2024-01-10 were already compared
  And a late observation arrives for 2024-01-10T06:00Z
When the comparison engine processes the late observation
Then it matches against existing forecasts for that time
  And recalculates affected aggregated metrics
  And the updated metrics reflect the new sample count
```

---

## 4. REST API

### AC-4.1: Paginated forecast query

```gherkin
Given 150 forecasts exist for provider "OpenWeather" at location "NYC"
When client sends GET /api/v1/forecasts?provider_id=X&location_id=Y&limit=50
Then response status is 200
  And response contains 50 forecast objects
  And response contains pagination.next_cursor
  And response contains pagination.has_more = true
  And response contains pagination.total_count = 150
```

### AC-4.2: Rate limiting

```gherkin
Given API key "key-123" has rate limit of 100 requests/minute
  And 100 requests have been made in the current minute
When request #101 is sent
Then response status is 429
  And response contains Retry-After header
  And response body follows RFC 7807 format
```

### AC-4.3: Authentication required

```gherkin
Given endpoint GET /api/v1/forecasts requires authentication
When request is sent without Authorization header or X-API-Key
Then response status is 401
  And response body contains:
    """
    {
      "type": "https://forecastiq.com/errors/unauthorized",
      "title": "Unauthorized",
      "status": 401
    }
    """
```

### AC-4.4: Input validation

```gherkin
Given endpoint POST /api/v1/locations
When request body contains latitude = 95.0
Then response status is 422
  And response contains field-level error for "latitude"
  And no location is created
```

### AC-4.5: Ranking endpoint

```gherkin
Given accuracy metrics exist for 4 providers at location "London" for +24h horizon
When client sends GET /api/v1/rankings?location_id=X&horizon=24h
Then response contains providers ordered by composite accuracy score
  And each entry includes: provider_name, mae_avg, rmse_avg, sample_count
  And response is cacheable (ETag header present)
```

---

## 5. Authentication

### AC-5.1: Successful login

```gherkin
Given user "alice@example.com" exists with valid password
When POST /api/v1/auth/login with correct credentials
Then response status is 200
  And response contains access_token (JWT, expires in 15min)
  And response contains refresh_token (expires in 7d)
  And access_token contains claims: sub, email, role, exp, iat
```

### AC-5.2: Failed login (no enumeration)

```gherkin
Given user "alice@example.com" exists
When POST /api/v1/auth/login with wrong password
Then response status is 401
  And response message is "Invalid credentials"
  And response does NOT indicate whether email exists
  And response time is similar to non-existent email attempt (timing attack prevention)
```

### AC-5.3: Token refresh rotation

```gherkin
Given user has refresh_token "RT-001"
When POST /api/v1/auth/refresh with RT-001
Then new access_token is returned
  And new refresh_token "RT-002" is returned
  And RT-001 is invalidated
  And if RT-001 is used again, all tokens for user are revoked (theft detection)
```

### AC-5.4: API key creation

```gherkin
Given authenticated user "alice"
When POST /api/v1/api-keys with name="production" scopes=["forecasts:read","accuracy:read"]
Then response contains the full API key (shown once)
  And database stores only the hash
  And key_prefix stores first 8 chars for identification
  And audit log records the creation
```

---

## 6. Dashboard

### AC-6.1: Initial load performance

```gherkin
Given the dashboard application
  And the API is responding normally
When a user navigates to the dashboard URL
Then the page renders meaningful content within 2 seconds
  And all provider ranking cards are visible
  And no layout shift occurs after initial render (CLS < 0.1)
```

### AC-6.2: Location filtering

```gherkin
Given the dashboard is loaded with default location
When user selects "San Francisco" from the location dropdown
Then all charts and rankings update to show San Francisco data
  And URL updates to include location parameter (shareable)
  And loading state is shown during data fetch
```

---

## 7. Admin Portal

### AC-7.1: Disable provider

```gherkin
Given admin is authenticated
  And provider "Visual Crossing" is enabled
When admin disables "Visual Crossing"
Then provider status changes to "disabled"
  And no new collections are scheduled for Visual Crossing
  And existing stored forecasts remain accessible
  And audit log records: admin disabled provider Visual Crossing
```

### AC-7.2: Add location

```gherkin
Given admin is authenticated
When admin creates location with:
  | name      | "Tokyo"          |
  | latitude  | 35.6762          |
  | longitude | 139.6503         |
  | timezone  | "Asia/Tokyo"     |
Then location is created with status "active"
  And next collection cycle includes Tokyo
  And response returns the created location with generated ID
```

---

## 8. System-Wide Criteria

### AC-8.1: Health endpoints

```gherkin
Given any ForecastIQ service is running
When GET /health is called
Then response is 200 with {"status": "healthy", "version": "x.y.z"}
When GET /ready is called
Then response is 200 if service can serve traffic
  And response is 503 if dependencies (DB, Redis) are unreachable
```

### AC-8.2: Structured logging

```gherkin
Given any service processes a request
Then logs are emitted as JSON with fields:
  | field     | description              |
  | timestamp | RFC3339 UTC              |
  | level     | debug/info/warn/error    |
  | trace_id  | OpenTelemetry trace ID   |
  | span_id   | OpenTelemetry span ID    |
  | service   | service name             |
  | message   | human-readable message   |
  | fields    | structured context       |
```

### AC-8.3: Graceful shutdown

```gherkin
Given the API service is running with active requests
When SIGTERM is received
Then the service stops accepting new connections
  And in-flight requests are allowed to complete (max 30s)
  And database connections are closed cleanly
  And NATS subscriptions are drained
  And the process exits with code 0
```
