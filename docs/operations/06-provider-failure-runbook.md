# ForecastIQ — Provider Failure Runbook (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: FC-08..FC-13; alerts A4/A5/A6/A13; `docs/workflows/01-forecast-collection.md`

---

## 1. Failure Classification (FC-13, drives everything)

| error_code | Side | Meaning | Reliability impact | Coverage impact |
|------------|------|---------|--------------------|-----------------|
| `timeout`, `provider_5xx`, `rate_limited` (provider 429) | Provider | Upstream failure | Excluded (not our fault) | Yes (data gap) |
| `schema_drift` | Provider (change) | Response shape broke adapter | Excluded | Yes |
| `network_local`, `db_error`, `payload_write_failed` | System | Our failure | Counts against reliability | No (retryable) |
| `circuit_open` | — | Breaker protecting provider | Excluded | Yes |
| `invalid_credentials` | Config | Key expired/revoked | Counts (our config) | Yes |

## 2. Scenario: Single Provider Outage (A4/A5)

**Detection:** circuit_state gauge = 2 (open); collection stale warning; freshness delayed/stale for affected cells.

```text
1. Verify scope: /admin/health — which provider, since when, circuit state
2. Check provider status page (Open-Meteo status / OpenWeather status)
3. IF provider-side outage:
   a. No action needed on circuits (breaker manages probes automatically)
   b. Verify other provider unaffected (partial results serving with warnings)
   c. Verify UI shows staleness banners (spot-check S-01)
   d. Log incident start
4. IF our-side (network_local errors):
   a. VPS connectivity check (curl provider from VPS)
   b. DNS / firewall / credential check
   c. Fix + manual trigger one collection to close circuit faster:
      POST /admin/collections/trigger {provider_id, location_id}
5. On recovery: circuit closes on first successful probe; verify next slot succeeds;
   gap hours remain in coverage metrics (honest); log incident end
```

**Do NOT:** manually reset circuits to force calls into a failing provider; disable the breaker; retry in a tight loop.

## 3. Scenario: Schema Drift (A6, critical)

**Detection:** collection failed with error_code=schema_drift (> 50% rows invalid) OR unmapped condition spike (A14).

```text
1. Confirm: /forecast-collections?status=failed&error_code=schema_drift — which provider, since when
2. Fetch current payload manually (or from volume — latest collection has it)
3. Diff against adapter fixture (test/fixtures/<provider>/latest.json)
4. Classify:
   a. Additive fields only → no action (adapter ignores unknowns); monitor
   b. Renamed/moved fields → adapter fix required:
      - Write new fixture from current payload
      - Update adapter + bump schema_version (+ adapter_version minor)
      - Contract tests against old + new fixtures
      - Deploy (normal pipeline — urgent)
      - Replay affected collections (≤ 90 d window, if values wrong):
        bulk replay procedure (docs/workflows/06-backfill-and-reprocessing.md §3)
      - Recompute affected scope
   c. Breaking API version change → evaluate new endpoint; adapter as (b); ADR note if material
5. Post-incident: add drift fixture to contract suite permanently
```

## 4. Scenario: Rate Limit Exhaustion

**Detection:** 429 responses; `rate_limited` collections; trigger endpoint returns 429 with budget reset.

```text
1. Check daily call count vs. tier (Grafana: collection_attempts by provider)
2. IF unexpected volume: verify no slot duplication (schedule_runs anomaly),
   no retry storm (attempts per slot)
3. IF legitimate (location count grew): reduce cadence or upgrade tier
   (config change: provider_configurations.collection_schedule)
4. Budget resets at provider's window (UTC midnight for OpenWeather daily tier) —
   collections resume automatically
```

## 5. Scenario: Credential Failure (invalid_credentials)

```text
1. Provider dashboard: key active? expired? ToS flag?
2. Rotate: generate new key → update env file (credential_ref resolves env name) →
   systemctl restart forecastiq → manual trigger to verify
3. Audit: provider.config_updated logged automatically
4. IF OpenWeather ToS issue → R-02 gate: pause provider (status disabled),
   swap to Tomorrow.io fallback (ADR-002 documented path)
```

## 6. Scenario: Observation Source Outage (A13)

```text
1. /admin/health observation_collector section — per-location last success
2. Source status check (Open-Meteo)
3. Impact containment (automatic):
   - Observations stop; freshness degrades (delayed → stale → unavailable)
   - Matching pauses for new data; existing metrics/rankings unchanged
   - Rankings NOT corrupted (batch stability + staleness warnings)
4. On recovery: 2 h backfill window auto-catches recent hours;
   older gap hours remain unmatched (honest coverage impact)
5. Outage > 48 h: incident review — consider emergency source (provider_estimated
   weight 0.5, never primary — ADR-003 rules still bind)
```

## 7. Escalation Matrix

| Condition | Action |
|-----------|--------|
| Both providers down > 4 h | Incident: API serves cached with staleness; status note on dashboard (manual banner via config flag); investigate common cause (our network?) |
| Schema drift unfixed > 48 h | Provider data degrading; consider temporary disable (coverage penalty honest) until fix |
| Provider permanent degradation (reliability < 90% for 30 d) | Product decision: swap provider (ADR-002 fallback) or document as known limitation |

## 8. Cross-Reference

- Collection mechanics: `docs/workflows/01-forecast-collection.md`
- Replay: `docs/workflows/06-backfill-and-reprocessing.md`
- Alerts: `docs/operations/03-monitoring-and-alerting.md` A4–A6, A13, A14
