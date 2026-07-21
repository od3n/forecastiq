# ForecastIQ — Acceptance Criteria (Revised)

**Version**: 2.0 (Phase 0 Amendment)
**Status**: Authoritative
**Supersedes**: `docs/phase-0-business-analysis/09-acceptance-criteria.md`
**Resolves**: ARB Blocker 3 (formula correctness — AC-3.2 corrected), terminology cleanup

All formulas reference `docs/domain/03-metric-methodology.md` (methodology_version
`2026.1`). Given/When/Then format retained.

---

## 1. Forecast Collection

### AC-1.1: Collection decomposes array response (replaces old AC-1.1)

```gherkin
Given provider "Open-Meteo" is enabled for location "Johor Bahru" (1.4927, 103.7414)
  And the hourly schedule triggers
When the collector calls the Open-Meteo forecast API
  And the response contains an hourly array of 168 periods
Then one ForecastCollection is stored with:
  | field                    | value                        |
  | collection_status        | "success"                    |
  | records_received         | 168                          |
  | snapshots_stored         | 168 (first collection)       |
  | raw_payload_checksum     | SHA-256 of response body     |
  | schema_version           | adapter's declared version   |
  | adapter_version          | current adapter semver       |
And 168 ForecastSnapshot rows exist, one per target_time
And each snapshot has forecast_horizon_minutes = target_time − issued_at
And raw payload is stored gzip-compressed at its payload key
And metric forecast_collection_total{provider="open-meteo",status="success"} increments
```

### AC-1.2: Immutability

```gherkin
Given any ForecastSnapshot or Observation exists
When any process attempts UPDATE or DELETE
Then the operation is rejected by database trigger
  And error "pipeline records are immutable" is raised
```

### AC-1.3: Deduplication (collection-level and snapshot-level)

```gherkin
Given a successful collection exists for (provider, location, model_run_time T)
When the scheduler fires again and the provider serves the same model run
Then the new ForecastCollection has status "deduplicated"
  And snapshots_stored = 0
  And no duplicate ForecastSnapshot rows exist (uniqueness constraint holds)
  And metric forecast_collection_total{status="deduplicated"} increments
```

### AC-1.4: Partial collection

```gherkin
Given a provider response with 48 periods of which 3 fail range validation
When the collector processes it
Then collection_status = "partial"
  And records_received = 48, snapshots_stored = 45, snapshots_invalid = 3
  And error_message lists rejection reasons
  And the 45 valid snapshots are queryable
```

### AC-1.5: Schema drift detection

```gherkin
Given a provider response where > 50% of rows fail validation
When the collector processes it
Then collection_status = "failed" with error_code = "schema_drift"
  And an operational alert is emitted
  And the raw payload is retained for debugging
```

### AC-1.6: Rate limiting, retry, circuit breaker

```gherkin
Given OpenWeather's token bucket is exhausted
When the collector would call
Then the call is deferred to the next window (no 429 emitted)
Given the provider returns HTTP 500
Then retries occur at 1s, 2s, 4s, 8s, 16s
  And after 5 consecutive failures the circuit opens for 60s
  And other providers continue collecting
  And after 60s a half-open probe decides close/reopen
```

### AC-1.7: Idempotent replay

```gherkin
Given a stored raw payload for collection C
When admin triggers replay
Then a new ForecastCollection is created (error_code marker "replay")
  And snapshots already present are not duplicated
  And only previously-missing snapshots are added
  And an audit event records the replay
```

## 2. Observation Collection

### AC-2.1: Provenance-typed observation stored

```gherkin
Given location "Johor Bahru" is active
When the observation collector fetches Open-Meteo Historical for hour H
Then an Observation is stored with:
  | field            | value                          |
  | source           | "openmeteo_historical"         |
  | observation_type | per API provenance (default "reanalysis") |
  | quality_flag     | "valid"                        |
And UNIQUE (source, location_id, observed_at) prevents duplicates
```

### AC-2.2: Suspect values

```gherkin
Given an observation reports temperature_c = 75.0
Then it is stored with quality_flag = "suspect"
  And it is excluded from all metrics
  And a warning is logged
```

### AC-2.3: Correction

```gherkin
Given observation O1 for (source, location, hour H) exists
When the source republishes a corrected value for H
Then a new Observation O2 is stored with quality_flag = "corrected"
  And O1.superseded_observation_id references O2 (O1 unchanged otherwise)
  And subsequent matching prefers O2
```

## 3. Comparison Engine (formulas corrected — Blocker 3)

### AC-3.1: Continuous metrics (worked vector TV-1)

```gherkin
Given matched pairs (forecast, observed) °C: (15.0,13.5), (20.0,21.0), (18.0,18.0), (25.0,22.0)
When metrics are calculated for (provider, location, +6h, temperature, period)
Then:
  | metric | value  |
  | mae    | 1.375  |
  | rmse   | 1.75   |
  | bias   | 0.875  |
And sample_count = 4
And the 95% CI for MAE is stored (±1.96·s/√n)
And methodology_version = "2026.1"
```

### AC-3.2: Precipitation occurrence metrics (CORRECTED — replaces flawed AC-3.2)

The term "hit_rate" is removed; Recall (POD) is the canonical name.

```gherkin
Given 100 matched pairs for (location, +24h): TP=40, FP=10, FN=5, TN=45
When occurrence metrics are calculated
Then:
  | metric              | value   | formula check        |
  | recall              | 0.8889  | TP/(TP+FN) = 40/45   |
  | precision           | 0.8000  | TP/(TP+FP) = 40/50   |
  | f1                  | 0.8421  | 2TP/(2TP+FP+FN)=80/95|
  | false_alarm_rate    | 0.1818  | FP/(FP+TN) = 10/55   |
  | threat_score        | 0.7273  | TP/(TP+FP+FN)=40/55  |
  | occurrence_agreement| 0.8500  | (TP+TN)/n = 85/100, flagged "imbalance-warning" |
And no metric named "hit_rate" or bare "accuracy" exists in the response
```

### AC-3.3: Zero-denominator behavior (vector TV-3)

```gherkin
Given a period with TP=0, FP=0, FN=0, TN=100 (no rain occurred or was forecast)
Then recall = null, precision = null, f1 = null (never 0, never NaN)
  And false_alarm_rate = 0.0
  And the occurrence component is excluded from the composite for ALL cohort providers
  And its weight is redistributed proportionally to remaining components
```

### AC-3.4: Matching rule

```gherkin
Given a snapshot with target_time 2026-07-01T18:00:00Z (horizon +24h)
  And an observation at 2026-07-01T18:00:00Z (hourly source)
Then a MatchedEvaluation is created linking both
Given an observation at 18:25 for an hourly source
Then NO match is created (exact-hour rule; ±30min universal window abolished)
```

### AC-3.5: Missing observation / late arrival

```gherkin
Given a forecast with no observation for its target hour
Then no metric is generated for that pair and no error is raised
Given a late observation arrives for a past hour
Then matching pairs it with existing snapshots
  And affected AccuracyMetric and ProviderRanking rows are recomputed as NEW rows
  And old rows carry superseded_by links
```

### AC-3.6: Ranking statuses and transparency

```gherkin
Given provider P has 720 pairs, coverage 0.98 for all required variables
  And provider Q has 15 pairs
  And provider R has 400 pairs but coverage 0.45
Then P.ranking_status = "ranked" with numeric rank
  And Q.ranking_status = "provisionally_ranked" (listed after ranked, badge shown)
  And R.ranking_status = "unranked" with reason "coverage below threshold"
  And every ranked row exposes: composite_score, component_scores, sample_count,
    coverage, reliability, ci_lower/upper, methodology_version, weights_version
  And providers with overlapping composite CIs are marked significant=false pairwise
```

### AC-3.7: Coverage penalty and outranking rule

```gherkin
Given provider A coverage 0.55 with raw composite 0.90
  And provider B coverage 0.85 with raw composite 0.80
Then A.final = 0.90 × (0.55/0.8) = 0.61875 (penalized)
  And A (provisional, coverage < 0.8) is listed after B regardless of raw score
```

## 4. REST API

### AC-4.1: Pagination without total_count

```gherkin
Given 150 forecasts match the query
When GET /api/v1/forecasts?limit=50
Then 200 with 50 items, pagination.has_more = true, pagination.next_cursor set
  And NO total_count field is present
```

### AC-4.2: Idempotent POST

```gherkin
Given POST /api/v1/locations with Idempotency-Key "K1" created location L
When the same POST with Idempotency-Key "K1" is retried within 24h
Then the response returns L (same id) with no duplicate created
```

### AC-4.3: Request correlation and errors

```gherkin
Given any request
Then the response carries X-Request-Id
Given a validation failure
Then 422 with RFC 7807 body including type/title/status/detail/errors and request_id
```

### AC-4.4: Rate limiting

```gherkin
Given an API key at its per-minute limit
When the next request arrives
Then 429 with Retry-After and X-RateLimit-* headers
```

### AC-4.5: Provenance in ranking responses

```gherkin
When GET /api/v1/rankings?location_id=X&horizon_minutes=1440
Then each entry includes ranking_status, composite_score, component breakdown,
  sample_count, coverage, reliability, methodology_version, weights_version,
  freshness state, and attribution
And ETag supports If-None-Match → 304 when unchanged
```

## 5. Authentication

### AC-5.1: Registration and verification

```gherkin
Given signup with email+password via managed auth
Then the account requires email verification before first login
  And no password hash exists in the ForecastIQ database
  And audit event "user.registered" is recorded
```

### AC-5.2: Login, rotation, theft detection

```gherkin
Given valid credentials
Then login returns access token (≤1h) + refresh token with rotation
Given refresh token RT-1 was already used
When RT-1 is presented again
Then the whole token family is revoked (theft detection) and 401 returned
Given 10 failed logins for an account
Then further attempts are rate-limited/blocked by the managed auth policy
  And the response never reveals whether the email exists
```

### AC-5.3: Password reset and account lifecycle

```gherkin
Given a password reset request
Then a reset email is sent (single-use, expiring link)
Given admin disables an account
Then its refresh tokens are revoked and login is refused with a generic message
Given account deletion
Then personal data is removed; weather data retained per BR-09
```

## 6. Dashboard

### AC-6.1: States (binding for every data-bearing screen)

```gherkin
Given any data-bearing screen
Then it implements: loading (skeleton), loaded, empty, insufficient-data,
  partial-provider-failure (per-provider badge), stale (banner + last-updated),
  unavailable (explicit message), error (retry action), permission-denied
Given a first-time signed-in user
Then onboarding content explains the platform's purpose and links methodology
```

### AC-6.2: Ranking honesty in UI

```gherkin
Given a ranking view
Then every row shows sample size, coverage, freshness, and provenance
  And unranked providers display "Insufficient data — collecting since {date}"
  And the composite score links to the per-metric breakdown
  And the methodology version is visible and links to the methodology page
```

### AC-6.3: Performance and export

```gherkin
Given normal API conditions
Then meaningful paint < 2s with CLS < 0.1
Given CSV export of any view
Then the file includes metadata header rows (period, methodology_version,
  weights_version, observation provenance mix)
```

## 7. System-Wide

### AC-7.1: Health and readiness

```gherkin
When GET /healthz Then 200 {"status":"healthy","version":"x.y.z"}
When GET /readyz Then 200 if DB reachable and payload volume writable, else 503
```

### AC-7.2: Structured logging

```gherkin
Given any request processing
Then JSON logs include timestamp (RFC3339 UTC), level, request_id, service, message, fields
```

### AC-7.3: Graceful shutdown

```gherkin
Given SIGTERM
Then the scheduler stops claiming new slots
  And in-flight work drains (≤30s)
  And DB pool closes cleanly; exit code 0
```

### AC-7.4: Formula verification suite

```gherkin
Given the metric implementation
Then all test vectors TV-1..TV-5 of the methodology pass as unit tests
And property-based tests (methodology §11, properties 1-11) pass with ≥10k fuzz cases
And recomputation over identical inputs yields byte-identical stored values
```
