# ForecastIQ — Forecast Collection Workflow (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: FC-01..FC-15; domain model §4; ADR-012; `docs/architecture/05-data-flow-architecture.md` §2

---

## 1. Sequence Diagram

```mermaid
sequenceDiagram
    participant SCH as Scheduler
    participant DB as PostgreSQL
    participant CB as CircuitState
    participant AD as ProviderAdapter
    participant PROV as Provider API
    participant VOL as Payload volume

    SCH->>DB: claim due slot (FOR UPDATE SKIP LOCKED)
    DB-->>SCH: slot (provider_config, location)
    SCH->>CB: check circuit(provider)
    alt circuit open
        CB-->>SCH: open, next_probe_at
        SCH->>DB: mark run failed (circuit_open)
    else closed / half-open probe due
        SCH->>AD: collect(config, location)
        AD->>AD: check token bucket (rate limit)
        AD->>PROV: GET forecast (timeout 10s)
        alt success
            PROV-->>AD: JSON response
            AD->>AD: SHA-256 checksum (before parse)
            AD->>VOL: write gzip payload
            AD->>AD: schema validate (adapter schema_version)
            AD->>AD: decompose array → snapshot structs
            AD->>AD: normalize (UTC, units, condition map)
            AD->>AD: row validation (ranges, temporal)
            AD->>DB: BEGIN tx
            AD->>DB: INSERT collection row (status, counts)
            AD->>DB: INSERT snapshots (ON CONFLICT DO NOTHING)
            AD->>DB: UPDATE circuit (success → closed)
            AD->>DB: emit forecast.collected (in-tx)
            AD->>DB: COMMIT
            AD->>DB: mark slot completed + schedule_run
        else HTTP failure / timeout
            PROV-->>AD: error
            AD->>AD: retry with backoff (1,2,4,8,16s; max 5)
            AD->>DB: INSERT collection row (failed/timeout/rate_limited)
            AD->>DB: UPDATE circuit (failure count; open at 5)
            AD->>DB: mark slot failed + schedule_run
        end
    end
```

## 2. Step-by-Step Specification

| # | Step | Specification |
|---|------|---------------|
| 1 | Schedule becomes due | Slot row (status=due, slot_time ≤ now) per `collection_schedules`; generated hourly from active configurations |
| 2 | Job claimed | `SELECT ... FOR UPDATE SKIP LOCKED LIMIT 10` → UPDATE claimed_by + lease 5 min; concurrent workers never double-claim |
| 3 | Pre-checks | Circuit state (open → short-circuit fail); provider + location + config active; rate-limit token bucket (per provider; OpenWeather daily budget tracked) |
| 4 | Provider request prepared | Adapter builds URL from config (base_url + coords + variables + horizon params); auth header from env-resolved credential_ref |
| 5 | Provider API called | HTTP GET, timeout 10 s, gzip accept; response size limit 10 MB |
| 6 | Response metadata captured | HTTP status, latency_ms, provider_request_id / model_run_time when exposed |
| 7 | Raw payload persisted | SHA-256 computed on raw bytes **before parsing**; gzip write to `payloads/{provider}/{yyyy}/{mm}/{dd}/{collection_id}.json.gz`; write failure → continue with `error_code = payload_write_failed` (alerted) |
| 8 | Schema validated | Adapter validates against declared schema_version; missing/unknown required fields → row invalid |
| 9 | Response normalized | issued_at → UTC (BR-PROV-01, adapter contract-tested); percentages → [0,1]; condition codes → canonical taxonomy v1 (unmapped → `unknown` + WARN + metric; > 1%/day → alert FC-15) |
| 10 | ForecastCollection updated | Single final state: success / partial (> 0 invalid rows) / failed (> 50% invalid → `schema_drift` + alert) / deduplicated / rate_limited / timeout; accounting: received = stored + deduplicated + invalid |
| 11 | Snapshot rows written | Batch INSERT ... ON CONFLICT (provider_id, location_id, issued_at, target_time) DO NOTHING; same tx as collection row |
| 12 | Operational metrics emitted | Counters/histograms per observability doc §3.2; event `forecast.collected` |
| 13 | Retry/failure recorded | schedule_run row (status, error_code, duration); slot: completed / failed (attempts++, next_retry_at per backoff) |

## 3. Job Claiming and Concurrency

- **Claiming:** SKIP LOCKED (QX-05); lease 5 min; expired leases reclaimed with attempts++ (QX-06).
- **Concurrency within process:** provider calls parallelized across provider-locations (goroutine pool, max 8 concurrent) — NFR-S02; DB writes serialized per collection tx.
- **Multi-instance safety:** SKIP LOCKED + snapshot uniqueness = no double collection even with 2 instances (future worker split ready).
- **Advisory locks:** not needed (slot rows are the coordination point).

## 4. Rate-Limit Handling

| Provider | Budget | Mechanism |
|----------|--------|-----------|
| Open-Meteo | 10K req/day (non-commercial) | Token bucket 6 req/min effective; MVP uses ≤ 240/day (40× headroom) |
| OpenWeather | ~1,000 req/day free | Token bucket 1 req/min + daily counter; MVP uses ≤ 240/day (4× headroom); 429 response → collection `rate_limited` + budget pause until reset |

Rate-limit state: in-process token bucket + daily counter persisted in `provider_circuits`-adjacent config (reset on restart from collections count — derived, not stored separately).

## 5. Retry Policy (FC-08)

- Backoff: 1, 2, 4, 8, 16 s with ±20% jitter; max 5 attempts within one slot execution.
- Retryable: network errors, timeouts, 5xx, 429 (after Retry-After).
- Non-retryable: 4xx (except 429) → immediate `failed` (client error won't self-heal).
- After max attempts: slot `failed`, attempts recorded; next hourly slot proceeds independently (no cross-slot retry accumulation — natural cadence is the recovery).
- **Circuit breaker (FC-09):** 5 consecutive failures per provider → open; half-open probe after 60 s (one slot allowed through); success → closed. State in `provider_circuits` (persistent).

## 6. Partial Success

- HTTP 200 but some rows invalid → status `partial`; valid rows stored; `snapshots_invalid` counted; `error_message` lists first 5 rejection reasons (truncated).
- Partial is **not auto-retried** (provider served the data; next slot fills gaps naturally).
- > 50% invalid → `failed` + `error_code = schema_drift` + critical alert (FC-11).

## 7. Collection-Level Deduplication

If `(provider_id, location_id, COALESCE(provider_model_run_time, issued_at))` matches an existing successful collection → new collection row with status `deduplicated`, zero snapshots, no payload rewrite. Handles: scheduler double-fire, provider serving the same model run twice.

## 8. Cancellation and Manual Operations

- **Cancellation:** in-flight provider call cancelled via context on graceful shutdown (30 s drain); slot lease expires → reclaimed.
- **Manual retry:** `POST /admin/collections/trigger` → same use case as scheduled (CollectNow), bypasses schedule slot; 409 while circuit open; 429 if budget exhausted; idempotent by snapshot dedup regardless of Idempotency-Key.
- **Replay from raw payload:** `POST /admin/collections/{id}/replay` → read stored payload, verify checksum (mismatch → quarantine `.corrupt` + alert + 422 `payload_unavailable`), run through **current** adapter → new collection (error_code marker `replay`, provider_request_id copied) → new snapshots where uniqueness allows. Originals never mutated (domain §4.8).

## 9. Adapter Schema Versioning

- Each adapter declares `SchemaVersion` (e.g., `openmeteo-v1`) + `AdapterVersion` (semver).
- Both recorded on every collection row.
- Schema change support = new schema_version + adapter bump; historical rows keep original versions (never rewritten).
- Contract tests: recorded fixtures per provider × schema version run in CI (adapter cannot drift silently).

## 10. One Response → Multiple Rows (example)

Open-Meteo response for JB (issued 2026-07-22T10:00Z):
```text
hourly.time:             [10:00, 11:00, ..., 09:00+7d]   (168 entries)
hourly.temperature_2m:   [31.2, 31.8, ...]
hourly.precipitation_probability: [0.42, ...]
...
→ 1 ForecastCollection (records_received=168)
→ 168 ForecastSnapshots:
    issued_at = 2026-07-22T10:00Z (all)
    target_time = each hourly entry (UTC)
    forecast_horizon_minutes = 0, 60, 120, ... 10080
```

## 11. Cross-Reference

- Scheduling: `docs/workflows/05-scheduling-and-retries.md`
- Replay/backfill: `docs/workflows/06-backfill-and-reprocessing.md`
- Adapter contract tests: `docs/testing/03-contract-testing.md`
- Failure runbook: `docs/operations/06-provider-failure-runbook.md`
