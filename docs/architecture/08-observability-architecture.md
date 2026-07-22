# ForecastIQ — Observability Architecture (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: NFR-OBS01..07; constraints §2 (observability row) and §6 (boundaries 5–6); `docs/operations/01-ui-operational-signals.md`

---

## 1. Stack (constraints-compatible)

| Signal | Technology | Cost |
|--------|-----------|------|
| Logs | slog → structured JSON → stdout → journald → hosted shipper (Grafana Cloud Logs or Better Stack free tier) | $0 (free tier, < 500 MB/day expected) |
| Metrics | Prometheus exposition at `/metrics` → Grafana Cloud Metrics scrape (agent: grafana-agent on VPS) | $0 (free tier) |
| Tracing | **Not in MVP.** Request-ID correlation is sufficient for a modular monolith (NFR-OBS02). Distributed tracing is a promotion (constraints §4). | — |
| Health | `/healthz`, `/readyz` + hosted uptime checks (Grafana Synthetic Monitoring free tier or UptimeRobot) | $0 |
| Dashboards | Grafana Cloud free tier (3 dashboards) | $0 |
| Alerts | Grafana alerting → email/webhook | $0 |

## 2. Structured Logging Standard

```json
{
  "ts": "2026-07-22T10:00:00.123Z",
  "level": "info",
  "msg": "collection.completed",
  "service": "forecastiq",
  "request_id": "0d1c…",
  "job_id": "…",
  "collection_id": "…",
  "provider": "openweather",
  "location_id": "…",
  "duration_ms": 812,
  "snapshots_stored": 48,
  "error_code": null
}
```

**Binding fields:** `ts` (RFC 3339), `level`, `msg` (event name, dot-namespaced), `service`. Context fields: `request_id` (API), `job_id`/`collection_id`/`location_id`/`provider` (jobs), `duration_ms`, `error_code`/`error_class` (failures). Workspace ID included where safe (single workspace — always safe in MVP).

**Never logged:** access tokens, refresh tokens, provider API keys, credential_ref values, raw credentials, provider response bodies, email addresses (use auth_subject reference), unnecessary personal information.

**Event name registry** (stable, alertable): `collection.started/completed/failed/deduplicated`, `observation.collected/rejected`, `matching.batch_completed`, `metrics.batch_completed`, `rankings.published`, `scheduler.slot_claimed/missed`, `circuit.opened/half_open/closed`, `payload.write_failed`, `schema_drift.detected`, `auth.login_failed`, `api.request` (RED summary at handler level).

## 3. Metrics (Prometheus `/metrics`)

### 3.1 HTTP (RED)

| Metric | Type | Labels |
|--------|------|--------|
| `http_requests_total` | counter | method, route_template, status_class |
| `http_request_duration_seconds` | histogram | method, route_template |
| `http_errors_total` | counter | route_template, error_type (envelope class) |

### 3.2 Collection

| Metric | Type | Labels |
|--------|------|--------|
| `collection_attempts_total` | counter | provider, status (success/partial/failed/rate_limited/timeout/deduplicated) |
| `collection_duration_seconds` | histogram | provider |
| `collection_snapshots_stored_total` | counter | provider |
| `collection_records_rejected_total` | counter | provider, reason (invalid_range/schema/missing_field) |
| `provider_rate_limit_hits_total` | counter | provider |
| `provider_latency_seconds` | histogram | provider |
| `circuit_state` | gauge | provider (0=closed, 1=half_open, 2=open) |

### 3.3 Observations

| Metric | Type | Labels |
|--------|------|--------|
| `observations_collected_total` | counter | source, location_id |
| `observations_suspect_total` | counter | source, reason |
| `observation_freshness_age_seconds` | gauge | location_id |

### 3.4 Engine

| Metric | Type | Labels |
|--------|------|--------|
| `matching_backlog` | gauge | — (unmatched eligible snapshots) |
| `evaluation_backlog` | gauge | — (matched pairs not yet aggregated) |
| `engine_lag_seconds` | gauge | — (now − last batch calculated_at) |
| `ranking_freshness_age_seconds` | gauge | location_id, horizon_minutes |
| `batch_duration_seconds` | histogram | batch_type (matching/aggregation/ranking) |

### 3.5 Scheduler

| Metric | Type | Labels |
|--------|------|--------|
| `scheduler_slots_claimed_total` | counter | job_type |
| `scheduler_missed_slots_total` | counter | job_type |
| `scheduler_lag_seconds` | gauge | job_type (due time → claim time) |
| `job_duration_seconds` | histogram | job_type |

### 3.6 Runtime

| Metric | Type | Labels |
|--------|------|--------|
| `db_pool_in_use` / `db_pool_idle` / `db_pool_wait_total` | gauge/counter | — |
| `payload_volume_used_bytes` / `payload_volume_total_bytes` | gauge | — |
| `process_restarts_total` | counter | — |
| `lru_cache_hits_total` / `lru_cache_misses_total` | counter | endpoint_group |
| Go runtime (default collector) | — | — |

## 4. Health Endpoints

| Endpoint | Checks | Failure meaning |
|----------|--------|-----------------|
| `/healthz` | Process alive (200 immediately) | Process dead → systemd restart |
| `/readyz` | DB ping (< 2 s) + payload volume writable (test file) + JWKS reachable (cached OK) | Not ready → Caddy 502; deployment smoke test gate |
| Scheduler health (in `/admin/health`) | Last slot claimed < 2× interval; no expired leases stuck | Scheduler stall → alert |
| Worker health (in `/admin/health`) | Batch lag < 2 h (freshness threshold); no failed runs unhandled | Engine stall → alert |

## 5. Dashboards (3, Grafana)

| Dashboard | Panels |
|-----------|--------|
| **API** | Request rate, p50/p95/p99 latency, error rate by class, 429 rate, cache hit ratio, DB pool |
| **Pipeline** | Collections/hour by provider+status, provider latency, circuit states, observation freshness per location, engine lag, matching/evaluation backlog, scheduler lag + missed slots |
| **System** | VPS CPU/memory/disk, payload volume usage, DB connections, process restarts, backup status (from status file), uptime check results |

## 6. Alert Rules (NFR-OBS05, minimum set)

| Alert | Condition | Severity | Channel |
|-------|-----------|----------|---------|
| Collection stale | No successful collection for any active provider-location > 180 min | warning | email |
| Circuit open | `circuit_state == 2` for > 10 min | critical | email + webhook |
| Schema drift | `collection_records_rejected_total{reason=schema}` > 50% of a collection | critical | email |
| Engine lag | `engine_lag_seconds` > 7200 (2 h) | warning | email |
| Scheduler missed slots | `scheduler_missed_slots_total` increase > 0 over 2 h | critical | email |
| Disk > 80% | Payload volume used_pct > 80 | warning | email |
| Backup failure | Backup status file `status != success` OR age > 26 h | critical | email |
| Cert expiry < 14 d | Caddy cert expiry probe | warning | email |
| API error rate | 5xx > 1% of requests over 15 min | critical | email |
| Uptime | `/healthz` failing 3 consecutive checks | critical | email + SMS (hosted) |
| Unmapped condition codes | > 1% of a day's rows for a provider (FC-15) | warning | email |

## 7. Correlation Model (tracing replacement)

- Every API request: `X-Request-Id` (validated or generated UUIDv4) → all logs for that request carry it.
- Every scheduled job: `job_id` (schedule_run id) → collection/observation/engine logs carry it + `collection_id` where applicable.
- Cross-hop: a collection triggered by a slot carries `slot_id`; replay carries original `collection_id` + new `collection_id`.
- Admin UI never queries log systems (binding, operations doc): triage data served from application tables via `/admin/health`.

## 8. SLO Tracking

Monthly SLO review (NFR-OBS07): availability (99.5%), latency SLOs (p50 < 50 ms, p95 < 200 ms), collection success rate (≥ 99% slots/month). Tracked via Grafana recording rules; burn-rate alerts at 2× error budget consumption. Detail: `docs/operations/02-sli-slo.md`.

## 9. Cross-Reference

- Operational signals (UI failure modes): `docs/operations/01-ui-operational-signals.md`
- Monitoring runbook: `docs/operations/03-monitoring-and-alerting.md`
- SLI/SLO: `docs/operations/02-sli-slo.md`
