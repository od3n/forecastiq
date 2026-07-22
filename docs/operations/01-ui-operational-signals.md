# ForecastIQ — UI Operational Signals

**Version**: 1.0 (UI ↔ Backend Reconciliation Board)
**Status**: Authoritative — every serious UI/API failure mode has an operational signal
**Inputs**: NFR-OBS01..07; `docs/architecture/00-phase-0-architecture-constraints.md` §6; `docs/ui/06-ui-state-contracts.md` §1 (monitoring signal column)
**Constraint**: Signals use the approved MVP observability stack only (structured JSON logs → hosted log service, Prometheus `/metrics`, uptime checks, Grafana free tier). No Jaeger/Tempo, no additional services. Traces: not used in MVP (request IDs provide correlation, NFR-OBS02).

---

## 1. Failure-Mode → Signal Matrix (board-mandated coverage)

| Failure mode | Metric (Prometheus, `/metrics`) | Log event (structured JSON) | Alert threshold | Dashboard view | Runbook |
|--------------|--------------------------------|-----------------------------|-----------------|----------------|---------|
| Collection failure | `collections_total{provider,status}` (failed/partial/rate_limited/timeout) | `collection.failed` {provider, location, error_code, latency_ms, request_id} | failed rate > 5% of slots over 1h OR any cell stale > 180m (BR-FRESH) | Grafana: collection success rate per provider | collection-failures.md (backoff, circuit, key rotation) |
| Provider schema mismatch | `snapshots_invalid_total{provider}`, `collections_total{status="failed",error_code="schema_drift"}` | `collection.schema_drift` {provider, schema_version, invalid_ratio, first_n_reasons} | > 50% invalid in one collection (FC-11) OR unmapped condition > 1%/day (FC-15) | Schema-drift panel | schema-drift.md (adapter bump procedure) |
| High provider latency | `provider_request_duration_seconds{provider}` histogram | `collection.slow` {provider, latency_ms} when > p95 budget | p95 > 5s over 30m | Provider latency panel | provider-latency.md |
| Rate-limit exhaustion | `provider_rate_limit_exhausted_total{provider}`, `collections_total{status="rate_limited"}` | `collection.rate_limited` {provider, budget_state, retry_after_s} | Any exhaustion event (warning); > 3/hour (page) | Rate-budget panel | rate-limits.md (budget tuning, OpenWeather free-tier math) |
| Observation delay | `observation_freshness_state{location}` gauge (0=fresh..3=unavailable), `observations_total{status}` | `observation.delayed` {location, age_seconds} | Any location stale > 240m; unavailable > 24h (BR-FRESH) | Observation freshness grid | observation-gaps.md |
| Evaluation backlog (engine lag) | `engine_lag_seconds` (now − max accuracy_metrics.calculated_at) | `engine.batch_completed` {duration_s, pairs_processed}; `engine.batch_failed` | lag > 3600s (2 batch cycles, BR-INV-02) OR batch > 10m (NFR-P06) | Engine lag panel | engine-backlog.md |
| Stale rankings | `ranking_freshness_state{location,horizon}` gauge | `rankings.stale` {cell, age_seconds} | Any cell stale > 6h (BR-FRESH rankings) | Ranking freshness panel | (same as engine-backlog.md) |
| Failed exports | `export_jobs_total{status}` | `export.failed` {job_id, target_user, error} | Any failure (GDPR obligation) | Export jobs panel | gdpr-operations.md |
| Authentication anomalies | `auth_failures_total{reason}` (invalid_token, expired, disabled) | `auth.login_failed` {subject?, ip, reason} — no PII on unknown subjects | > 20 failures/10m from one IP (abuse); disabled-account attempts (info) | Auth panel | auth-anomalies.md |
| High API latency | `http_request_duration_seconds{route,method}` histogram (RED) | — (metrics only; request_id logs on > p99) | p95 > 200ms over 15m (NFR-P02); p99 > 500ms | API RED dashboard | api-latency.md (query plan review) |
| Elevated partial responses | `partial_responses_total{endpoint}` | `api.partial_response` {endpoint, warning_codes} when partial | partial rate > 10% of responses over 1h | Partial-response panel | (cross-ref collection-failures.md) |
| Database saturation | `db_pool_in_use`, `db_query_duration_seconds` histogram; pg_stat_statements top queries | `db.pool_exhausted` (waiting > 1s) | pool > 80% for 5m; query p95 > 100ms (NFR-P08) | DB panel | db-saturation.md (index review, promotion criteria check) |
| Circuit open | `provider_circuit_state{provider}` gauge (0 closed, 1 half-open, 2 open) | `circuit.opened` {provider, consecutive_failures}; `circuit.half_open_probe` | Any open event (page — provider data degrading) | Circuit panel | circuit-breaker.md |
| Payload volume full | `payload_volume_used_pct` | `storage.volume_high` {used_pct} | > 80% (NFR-OBS05); > 90% (page) | Storage panel | disk-full.md |
| Backup failure / restore test | Backup status file → `backup_last_status` gauge, `backup_age_hours` | `backup.completed/failed` (script-emitted) | Any failed backup (page); no successful backup > 26h; restore test > 32d old | Backup panel | backup-restore.md |
| Certificate expiry | `tls_cert_expiry_days` | — | < 14 days (NFR-OBS05) | Infra panel | cert-rotation.md (Caddy auto-renewal check) |
| Uptime (external) | Hosted uptime check against `/healthz` + dashboard origin | — | 2 consecutive failures | Availability SLO (NFR-OBS07) | outage.md |

## 2. UI-State Signals (frontend-originated)

The dashboard emits no telemetry service in MVP (privacy + simplicity). UI failure visibility comes from the API side (above) plus:

| UI state | Server-side signal | Rationale |
|----------|-------------------|-----------|
| Stale banner shown | `ranking_freshness_state` / `observation_freshness_state` gauges | Server knows the state it served (BR-FRESH-02: server-computed) |
| Partial badge shown | `partial_responses_total` | Emitted at response assembly |
| Full error panel | `http_5xx_total`, uptime checks | Client retries hit the same counters |
| Rate-limited toast | `rate_limited_total{bucket}` | 429 counter per bucket |
| 403 on admin | `forbidden_total` | Misconfiguration/privilege-confusion detector |
| Offline banner | None (client-only; acceptable — no server round trip possible by definition) | Recovery visible via normal traffic resumption |

## 3. SLO Tracking (NFR-OBS07)

| SLO | Target | Measurement | Review cadence |
|-----|--------|-------------|----------------|
| API availability | 99.5% (NFR-A01) | Uptime checks + 5xx ratio | Monthly |
| API latency | p95 < 200ms (NFR-P02) | Histogram | Monthly |
| Collection slot success | ≥ 99%/month (NFR-A03) | `collections_total` | Monthly |
| Engine freshness | Rankings fresh (< 2h) ≥ 95% of hours | Freshness gauge history | Monthly |
| Zero data loss | 100% (NFR-A06) | Immutability + PITR; no counter possible — audit via replay verification drills | Quarterly |

## 4. Signal Placement Rules (board mandate: what belongs where)

| Data | Application tables | Logs | Metrics | Alerting |
|------|-------------------|------|---------|----------|
| Last success per provider-location (S-10) | ✓ (served by `/admin/health`) | also logged on transitions | ✓ counters | ✓ thresholds |
| Circuit state (S-10) | ✓ (`provider_circuits` table — UI-serving) | ✓ transitions | ✓ gauge | ✓ open event |
| Error detail per collection (S-13) | ✓ (`error_code`, truncated `error_message`) | ✓ full detail + request_id | ✓ counters by code | ✓ drift/rate patterns |
| Payload volume (S-10) | — | — | ✓ gauge (scraped from app) | ✓ 80/90% |
| Engine lag (S-10) | derivable (max calculated_at) | batch events | ✓ gauge | ✓ 1h |
| Backup status (S-10) | status file (read by app) | ✓ script events | ✓ gauges | ✓ failures |
| Per-request debug detail | ✗ never in UI | ✓ (request_id correlated) | ✗ | ✗ |

**Binding rule**: the admin UI (S-10/S-13) never queries the log service or Prometheus — everything it displays is served from application tables, filesystem stats, or the status file via `/admin/health` (board principle: no log queries for normal application UI behaviour). Logs and metrics are the operator's *analytical* surface (Grafana); the admin screens are the *operational triage* surface.

## 5. Alert Routing

Hosted log/metric thresholds → email + webhook (NFR-OBS05). Page-vs-ticket classification per §1 thresholds. No PagerDuty-class tooling in MVP (1–2 engineer team; email + uptime webhook suffice; promotion is a Level 3 ops decision).

## 6. Observability Gap Verdict

All twelve board-mandated failure modes (collection failure, schema mismatch, provider latency, rate-limit exhaustion, observation delay, evaluation backlog, stale rankings, failed exports, auth anomalies, API latency, elevated partials, DB saturation) have metric + log + alert + dashboard + runbook coverage within the approved stack. **No new observability infrastructure required.**
