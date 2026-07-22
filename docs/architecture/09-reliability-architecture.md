# ForecastIQ — Reliability Architecture (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: NFR-A01..A08; constraints §5–§6; risk register R-12

---

## 1. Targets (right-sized, honest)

| Target | Value | Basis |
|--------|-------|-------|
| API availability | 99.5% (≈ 3.6 h/month budget) | Single VPS ceiling (NFR-A01); 99.9% is Level 3 |
| Dashboard availability | 99.5% | CDN-served static (typically better) |
| Collection success rate | ≥ 99% of scheduled slots completed per month (NFR-A03) | Provider outages excluded from reliability via error classification (FC-13) |
| Ranking freshness | Recomputed within 2 h of latest input (BR-FRESH rankings threshold) | 30-min batch + alert at 2 h |
| Observation freshness | < 90 min (fresh threshold, BR-FRESH) | Hourly at :05 |
| API latency | p50 < 50 ms, p95 < 200 ms, p99 < 500 ms | NFR-P01..P03 |
| RPO | < 1 h | Managed PITR + hourly WAL |
| RTO | < 4 h | Redeploy-to-new-VPS runbook |
| Stored data durability | 100% (no loss of committed forecasts/observations) | DB durability + immutability |

**Error budget use:** the 3.6 h/month budget explicitly covers: deploys (< 30 s each), VPS maintenance, provider-independent incidents. Provider outages consume observation/forecast *freshness* budget, not availability budget (the API remains up, serving what it has with honest staleness).

## 2. SLIs (measurement)

| SLI | Measurement | Source |
|-----|-------------|--------|
| Availability | Uptime check success rate against `/healthz` (1 min interval) | Hosted uptime |
| Latency | `http_request_duration_seconds` percentiles | /metrics |
| Collection success | successful+partial slots / total due slots (monthly) | schedule_runs + collections |
| Engine lag | now − MAX(accuracy_metrics.calculated_at) | engine_lag_seconds gauge |
| Freshness (per data type) | BR-FRESH state distribution | freshness gauges |
| Durability | Zero committed-data-loss events (incident count) | Incident log |

SLO detail and burn-rate policy: `docs/operations/02-sli-slo.md`.

## 3. Retry Policies

| Operation | Policy | Rationale |
|-----------|--------|-----------|
| Provider forecast call | Exponential backoff 1, 2, 4, 8, 16 s; max 5 attempts (FC-08); jitter ±20% | Provider-friendly; fits within slot lease |
| Observation call | Same policy | Same source characteristics |
| DB transient error (API) | No retry in handler → stale-cache degradation or 503 | Requests are short; client retries naturally |
| DB transient error (batch) | Retry whole batch next cycle (30 min) | Idempotent; simpler than mid-batch resume |
| Payload write | 1 immediate retry → degrade with alert | Payload is non-blocking for data correctness |
| JWKS fetch | Cache 15 min; on unknown kid: 1 fetch (rate-limited 1/min) | Tolerates rotation without thundering herd |
| Supabase Admin API (user disable) | 2 retries, 1 s apart → failure surfaces as 502 (local state unchanged) | No split-brain |
| Manual collection trigger | No auto-retry (operator decides) | Human in loop |

## 4. Circuit Breaking

Per provider (FC-09): open after 5 consecutive failures; half-open probe after 60 s; success closes. State persisted in `provider_circuits` (restart-safe, multi-instance-safe, admin-visible per S-10). While open: all slots for that provider short-circuit to `failed` with `error_code = circuit_open` (no provider calls); manual trigger returns 409 with next-probe time.

## 5. Graceful Degradation Matrix

| Dependency down | API behaviour | Collection behaviour | UI experience |
|-----------------|---------------|---------------------|---------------|
| One provider | Rankings served from last batch + `warnings[]` (provider_unavailable); unaffected providers normal | Other providers continue; circuit open for failing one | Partial state (banner + badge per state contracts) |
| Observation source | Existing observations remain; freshness → delayed/stale; matching pauses for new data | Observations slots record failures | Stale banner; rankings unchanged (batch stability) |
| Database (transient < 5 min) | Cached responses with explicit staleness (LRU); mutations → 503 `service_unavailable` + Retry-After | Slots unclaimed (lease expiry re-queues); in-flight tx rolls back safely | Stale label; loading states |
| Database (sustained) | 503 with error envelope (no cache after LRU TTL) | Paused | Full error state |
| Supabase Auth | Public endpoints unaffected; authenticated → 401/503 guidance | Unaffected (no auth dependency) | Sign-in unavailable; public data browsable |
| Payload volume | Collections proceed (payload_write_failed alert); replay unavailable | Snapshots stored | Unaffected |
| Log service | Local buffering (bounded 100 MB ring) | Unaffected | Unaffected |
| CDN | Direct-to-origin fallback (Caddy serves nothing for dashboard — Pages handles) | Unaffected | Pages edge cache serves stale |

## 6. Recovery Procedures (runbook references)

| Scenario | Procedure | RTO |
|----------|-----------|-----|
| Process crash | systemd Restart=always (automatic, < 5 s); drain on graceful stop | < 1 min |
| VPS failure | `docs/operations/05-deployment-and-rollback.md` §VPS rebuild: new VPS → bootstrap script → deploy pipeline → DB reconnect (managed, unaffected) → volume re-attach (payloads: 90 d loss acceptable) | < 4 h |
| DB corruption | Managed PITR to pre-corruption timestamp (`docs/operations/07-database-recovery-runbook.md`) | < 2 h |
| Accidental deletion | Immutability triggers prevent pipeline-table deletes; config tables from backup | < 1 h |
| Bad deploy | Redeploy previous artifact (< 5 min, NFR-M07) | < 5 min |
| Provider key compromise | Rotate key → update env → restart (runbook `docs/operations/06-provider-failure-runbook.md`) | < 30 min |
| Payload volume corruption | Normalized data unaffected; checksum verification quarantines bad files; future collections rebuild | < 24 h |

## 7. Backup and DR Summary (detail: `docs/operations/04-backup-and-restore.md`)

| Layer | Method | Frequency | Retention | RPO contribution |
|-------|--------|-----------|-----------|------------------|
| Managed DB | PITR (WAL archiving) | Continuous | 7–30 d (vendor tier) | < 1 h (primary) |
| Logical dump | pg_dump (compressed) → volume → weekly offsite (B2) | Nightly | 30 d local, 90 d offsite | 24 h (secondary) |
| Payload volume | Not backed up (90 d ephemeral by design; ADR-011) | — | — | Acceptable loss (documented) |
| Configuration | IaC in repo + bootstrap script + Caddyfile + systemd units | On change (git) | Forever | 0 (reproducible) |

**Restore testing:** monthly automated restore of nightly dump to a scratch DB + integrity check (row counts, checksum sample); result written to backup status file → visible in `/admin/health` (NFR-D06).

## 8. Worker Outage Behaviour

Scheduler/engine share the API process. If the process is down: slots go unclaimed (no data loss — providers keep serving; gap = missed hours, reflected in coverage/reliability metrics honestly). If only the scheduler goroutine stalls (watchdog: no claims for 2× interval → alert + metric): API unaffected; operator restarts process. Per-job context timeouts (collection: 60 s; batch: 10 min) prevent goroutine leaks from hanging the process.

## 9. Partial API Behaviour (summary; binding: `docs/api/03-error-and-partial-result-contracts.md`)

Partial = HTTP 200 + `warnings[]` + `partial_result: true`. All-failed = stale cache with staleness OR 503 (never a partial with zero data). Rankings stable during provider outages (batch-computed; no mid-batch reshuffling).

## 10. Cross-Reference

- SLI/SLO: `docs/operations/02-sli-slo.md`
- Backup/restore: `docs/operations/04-backup-and-restore.md`
- Runbooks: `docs/operations/05..07`
- Risk: R-12 (single VPS, accepted with mitigation)
