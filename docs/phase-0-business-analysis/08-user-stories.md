# ForecastIQ — User Stories

**Version**: 1.0  
**Status**: Draft  

---

## Epic 1: Forecast Collection

### US-1.1: Collect forecasts from multiple providers
**As a** platform operator  
**I want** the system to automatically collect forecasts from OpenWeather, Tomorrow.io, Visual Crossing, and Open-Meteo  
**So that** I have comprehensive multi-provider data for accuracy comparison  

**Acceptance Criteria:**
- [ ] System collects from all 4 providers on configured schedule
- [ ] Each collection stores an immutable snapshot
- [ ] Raw API response stored in S3
- [ ] Collection respects provider rate limits
- [ ] Failed collections are retried with exponential backoff
- [ ] Metrics emitted for success/failure per provider

**Story Points:** 13  
**Priority:** Critical  

---

### US-1.2: Configure collection schedules
**As an** admin  
**I want** to configure how frequently forecasts are collected per provider  
**So that** I can balance data freshness against API rate limits and costs  

**Acceptance Criteria:**
- [ ] Admin can set cron expression per provider
- [ ] Changes take effect on next scheduler cycle
- [ ] Invalid cron expressions are rejected with clear error
- [ ] Default schedule: every 15 minutes

**Story Points:** 5  
**Priority:** High  

---

### US-1.3: Handle provider API failures gracefully
**As a** platform operator  
**I want** the system to handle provider outages without losing data or crashing  
**So that** collection resumes automatically when providers recover  

**Acceptance Criteria:**
- [ ] Circuit breaker opens after 5 consecutive failures
- [ ] Half-open state attempted after 60 seconds
- [ ] Alert emitted when circuit opens
- [ ] Other providers continue collecting unaffected
- [ ] No duplicate snapshots on retry

**Story Points:** 8  
**Priority:** High  

---

## Epic 2: Observation Collection

### US-2.1: Collect actual weather observations
**As a** platform operator  
**I want** the system to collect actual weather observations hourly  
**So that** I have ground-truth data to compare forecasts against  

**Acceptance Criteria:**
- [ ] Observations collected from NOAA/NWS (US) and Open-Meteo (global)
- [ ] Stored separately from forecasts
- [ ] Data validated against physical ranges
- [ ] Out-of-range values flagged as `suspect` but stored
- [ ] Event published on successful collection

**Story Points:** 8  
**Priority:** Critical  

---

## Epic 3: Accuracy Comparison

### US-3.1: Compare forecasts against observations
**As a** data engineer  
**I want** the system to automatically match forecasts to observations and calculate errors  
**So that** accuracy metrics are always up to date  

**Acceptance Criteria:**
- [ ] Matching uses location + target_time (±30min window)
- [ ] Supports all 7 horizons (+1h through +7d)
- [ ] Calculates MAE, RMSE, Bias for continuous variables
- [ ] Calculates Hit Rate, FAR, Precision, Recall, F1 for precipitation
- [ ] Handles missing data gracefully (skip, don't fail)
- [ ] Runs every 30 minutes

**Story Points:** 13  
**Priority:** Critical  

---

### US-3.2: View accuracy by provider and location
**As a** weather enthusiast  
**I want** to see which provider is most accurate for my city  
**So that** I know which forecast to trust  

**Acceptance Criteria:**
- [ ] Rankings displayed for selected location
- [ ] Filterable by horizon (+1h to +7d)
- [ ] Filterable by variable (temp, wind, rain, etc.)
- [ ] Shows MAE, RMSE, and sample count
- [ ] Updates when new data arrives

**Story Points:** 8  
**Priority:** Critical  

---

### US-3.3: View accuracy trends over time
**As a** business analyst  
**I want** to see how forecast accuracy changes over weeks/months  
**So that** I can identify improving or degrading providers  

**Acceptance Criteria:**
- [ ] Line chart showing MAE/RMSE over time
- [ ] Date range selectable (7d, 30d, 90d, custom)
- [ ] Multiple providers overlaid for comparison
- [ ] Hover shows exact values
- [ ] Exportable as CSV

**Story Points:** 8  
**Priority:** High  

---

## Epic 4: REST API

### US-4.1: Query forecasts programmatically
**As a** business developer  
**I want** to query forecast data via REST API  
**So that** I can integrate ForecastIQ data into my applications  

**Acceptance Criteria:**
- [ ] GET `/api/v1/forecasts` with filters (provider, location, time range)
- [ ] Cursor-based pagination
- [ ] Response time < 200ms (p95)
- [ ] Rate limited per API key
- [ ] Returns structured JSON with metadata

**Story Points:** 8  
**Priority:** Critical  

---

### US-4.2: Query accuracy metrics programmatically
**As a** data scientist  
**I want** to query aggregated accuracy metrics via API  
**So that** I can build custom analytics and reports  

**Acceptance Criteria:**
- [ ] GET `/api/v1/accuracy` with filters (provider, location, horizon, variable, period)
- [ ] Supports aggregation levels (hourly, daily, weekly, monthly)
- [ ] Returns metric value + sample count + confidence interval
- [ ] Paginated response

**Story Points:** 8  
**Priority:** Critical  

---

### US-4.3: Get provider rankings via API
**As a** logistics developer  
**I want** to get provider rankings for specific locations via API  
**So that** my routing system can select the best forecast source  

**Acceptance Criteria:**
- [ ] GET `/api/v1/rankings?location_id=X&horizon=24h`
- [ ] Returns ordered list with scores
- [ ] Includes breakdown by variable
- [ ] Cacheable (ETag/Last-Modified)

**Story Points:** 5  
**Priority:** High  

---

## Epic 5: Authentication & Authorization

### US-5.1: Authenticate via JWT
**As a** registered user  
**I want** to log in with email/password and receive a JWT  
**So that** I can access the dashboard and manage my settings  

**Acceptance Criteria:**
- [ ] POST `/api/v1/auth/login` returns access + refresh tokens
- [ ] Access token expires in 15 minutes
- [ ] Refresh token expires in 7 days
- [ ] Refresh tokens are single-use (rotation)
- [ ] Invalid credentials return 401 with no user enumeration

**Story Points:** 8  
**Priority:** Critical  

---

### US-5.2: Create and manage API keys
**As a** business user  
**I want** to create API keys with specific scopes  
**So that** I can grant programmatic access with least privilege  

**Acceptance Criteria:**
- [ ] Key shown only once at creation
- [ ] Stored as hash (never plaintext)
- [ ] Scopes limit accessible endpoints
- [ ] Rate limit configurable per key
- [ ] Keys can be revoked immediately
- [ ] Audit log entry on create/revoke

**Story Points:** 8  
**Priority:** High  

---

## Epic 6: Dashboard

### US-6.1: View overview dashboard
**As a** weather enthusiast  
**I want** to see a dashboard showing provider rankings and accuracy at a glance  
**So that** I can quickly identify the best forecast source  

**Acceptance Criteria:**
- [ ] Provider ranking cards (top 4)
- [ ] Location selector
- [ ] Horizon selector
- [ ] Last updated timestamp
- [ ] Loads in < 2 seconds
- [ ] Responsive layout (desktop + tablet)

**Story Points:** 13  
**Priority:** Critical  

---

### US-6.2: View forecast vs actual comparison
**As a** photographer  
**I want** to see how yesterday's forecast compared to actual weather  
**So that** I can plan future shoots with the most accurate source  

**Acceptance Criteria:**
- [ ] Overlay chart: forecast line + observation line
- [ ] Variable selector (temp, wind, precip)
- [ ] Date selector
- [ ] Provider selector (multi)
- [ ] Error bands shown

**Story Points:** 8  
**Priority:** High  

---

## Epic 7: Administration

### US-7.1: Manage monitored locations
**As an** admin  
**I want** to add, edit, and remove monitored locations  
**So that** the platform covers the locations users care about  

**Acceptance Criteria:**
- [ ] Add location with name, lat, lon, timezone
- [ ] Validate coordinates (-90 to 90, -180 to 180)
- [ ] Enable/disable without deleting
- [ ] Collection starts automatically for new active locations
- [ ] Audit log for all changes

**Story Points:** 5  
**Priority:** Critical  

---

### US-7.2: Monitor collector health
**As an** admin  
**I want** to see the health status of all collectors  
**So that** I can quickly identify and resolve collection issues  

**Acceptance Criteria:**
- [ ] Dashboard showing last successful collection per provider
- [ ] Error count and last error message
- [ ] Circuit breaker state (closed/open/half-open)
- [ ] Collection latency metrics
- [ ] Alert when collection fails > 3 consecutive times

**Story Points:** 5  
**Priority:** High  

---

## Epic 8: Alerting (Post-MVP)

### US-8.1: Receive forecast change alerts
**As an** event planner  
**I want** to be notified when forecasts change dramatically  
**So that** I can adjust plans proactively  

**Acceptance Criteria:**
- [ ] Alert when temperature forecast changes > 5°C within 3 hours
- [ ] Alert when rain probability changes > 30% within 3 hours
- [ ] Notification via webhook and/or email
- [ ] Configurable thresholds per user
- [ ] Rate-limited notifications (max 5/hour per rule)

**Story Points:** 13  
**Priority:** Medium  

---

## Story Map Summary

| Epic | Stories | Total Points | MVP? |
|------|---------|-------------|------|
| Forecast Collection | 3 | 26 | ✓ |
| Observation Collection | 1 | 8 | ✓ |
| Accuracy Comparison | 3 | 29 | ✓ |
| REST API | 3 | 21 | ✓ |
| Auth & Authorization | 2 | 16 | ✓ |
| Dashboard | 2 | 21 | ✓ |
| Administration | 2 | 10 | ✓ |
| Alerting | 1 | 13 | ✗ |
| **Total** | **17** | **144** | |
