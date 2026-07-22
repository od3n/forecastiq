# ForecastIQ — Monitoring and Alerting (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: NFR-OBS01..OBS06; `docs/architecture/08-observability-architecture.md` (metric catalog — normative)

---

## 1. Stack Setup

| Component | Configuration |
|-----------|---------------|
| grafana-agent (VPS) | Scrapes localhost:port `/metrics` (15 s); ships structured logs from journald unit `forecastiq`; remote-writes to Grafana Cloud |
| Grafana Cloud | Free tier: Metrics (10K series), Logs (500 MB/day), 3 dashboards, alerting, synthetic monitoring (uptime) |
| Uptime checks | `/healthz` every 1 min from 3 regions; alert on 3 consecutive failures |
| Alert channels | Email (primary); webhook to phone push for critical (hosted free option) |

Series budget: ~300 series (metrics catalog × label cardinality: providers 2, locations ≤ 10, job types 4, routes ~35) — well within free tier.

## 2. Alert Rules (complete set)

| # | Alert | Expression (simplified) | Severity | For | Action ref |
|---|-------|------------------------|----------|-----|-----------|
| A1 | API down | up == 0 | critical | 3 min | §3.1 |
| A2 | API 5xx rate | rate(http_errors_total{status_class="5xx"}[15m]) / rate(http_requests_total[15m]) > 0.01 | critical | 15 min | §3.2 |
| A3 | Latency p95 | forecastiq:latency_p95:5m > 0.2 | warning | 30 min | §3.3 |
| A4 | Circuit open | circuit_state == 2 | critical | 10 min | provider runbook |
| A5 | Collection stale | no success per cell > 180 min | warning | — | provider runbook |
| A6 | Schema drift | collection_records_rejected_total{reason="schema"} spike > 50% of collection | critical | immediate | provider runbook §schema |
| A7 | Engine lag | engine_lag_seconds > 7200 | warning | 1 h | §3.4 |
| A8 | Scheduler missed slots | increase(scheduler_missed_slots_total[2h]) > 0 | critical | — | §3.5 |
| A9 | Disk > 80% | payload_volume_used_bytes/total > 0.8 | warning | — | §3.6 |
| A10 | Backup failure | backup status file age > 26 h OR status != success | critical | — | backup runbook |
| A11 | Restore test overdue | last_restore_test age > 35 d | warning | — | backup runbook |
| A12 | Cert expiry | cert expiry < 14 d | warning | — | §3.7 |
| A13 | Observation stale | observation_freshness_age_seconds > 14400 (4 h) any location | warning | — | provider runbook §observation |
| A14 | Unmapped conditions | unmapped rate > 1%/day per provider | warning | daily eval | §3.8 |
| A15 | Process restarts | increase(process_restarts_total[1h]) > 2 | critical | — | §3.5 |
| A16 | DB pool saturation | db_pool_in_use / 20 > 0.9 for 10 min | warning | 10 min | §3.3 |
| A17 | Uptime failure | hosted check failing | critical | 3 checks | §3.1 |

## 3. Response Playbooks (summary; full runbooks referenced)

### 3.1 API down (A1/A17)
SSH → `systemctl status forecastiq` → journalctl last 100 → restart → if crash-loop: rollback deploy (`docs/operations/05-deployment-and-rollback.md`). RTO < 15 min.

### 3.2 Elevated 5xx (A2)
Check error_type label distribution → if `internal`: recent deploy? rollback. If `service_unavailable`: DB connectivity (managed status page). If `provider_unavailable`: expected during outages (not 5xx — verify classification).

### 3.3 Latency / pool (A3/A16)
pg_stat_statements top queries → missing index? (promotion-style: add with measured evidence) → cache hit ratio check → LRU undersized? → sustained: Redis promotion evaluation (`docs/architecture/10-scaling-and-evolution.md`).

### 3.4 Engine lag (A7)
Batch run history (S-13 / schedule_runs) → failed batches? error? → DB contention during batch window? → batch duration trend vs. volume growth.

### 3.5 Scheduler / restarts (A8/A15)
journalctl for panics/OOM → systemd restart count → lease-expiry pattern (jobs exceeding 5-min lease → timeout config review) → memory profile if OOM.

### 3.6 Disk (A9)
Payload retention job status → manual purge if job failed → volume usage trend → 50 GB = S3 promotion trigger evaluation.

### 3.7 Cert expiry (A12)
Caddy admin API cert check → DNS validation health → Let's Encrypt rate limits → manual renew via caddy reload.

### 3.8 Unmapped conditions (A14)
Identify codes from logs → adapter mapping table update + taxonomy review → contract test fixture addition → deploy (prospective only).

## 4. Dashboards

Per observability architecture §5: **API**, **Pipeline**, **System** (3 boards, JSON provisioned in repo → Grafana Cloud sync).

## 5. Log Query Patterns (runbook helpers)

| Need | Query |
|------|-------|
| All errors for a request | `{request_id="…"}` |
| Collection failures by provider (24 h) | `msg="collection.failed" provider="openweather"` |
| Schema drift detail | `error_code="schema_drift"` → collection_id → admin collections view |
| Circuit transitions | `msg=~"circuit.*"` |
| Batch performance | `msg="matching.batch_completed"` duration_ms |

## 6. Governance

- Alert changes: PR to alerts config (in repo) + review; quarterly alert-quality review (noise vs. actionability; silence ≠ fix).
- Every critical alert has a named response procedure (this doc or referenced runbook) — alerts without procedures are removed.
- Admin UI never queries log/metrics systems (binding): triage via `/admin/health` from application tables.

## 7. Cross-Reference

- Metric catalog (normative): `docs/architecture/08-observability-architecture.md` §3
- Runbooks: `docs/operations/05..07`
- SLI/SLO: `docs/operations/02-sli-slo.md`
