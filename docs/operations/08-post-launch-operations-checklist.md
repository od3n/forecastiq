# ForecastIQ — Post-Launch Operations Checklist

**Version**: 1.0
**Status**: Draft — post-M4 steady-state operations
**Authority**: `docs/operations/02-sli-slo.md` (targets); `docs/operations/03-monitoring-and-alerting.md` (alerts); `docs/operations/04-backup-and-restore.md` (backup cadence); ADR-033 (EC2 + Docker topology); `docs/planning/01-scope-levels.md` (Level 2 → Level 3 exit criteria)
**Context**: M4 launch-ready reached 2026-07-28 (launch checklist 22 PASS / 0 FAIL; D-05 and D-06 closed — `docs/reviews/06-phase-1-decision-log.md` §6). This checklist governs the validation window that follows.

---

## 1. One-time post-launch actions

Executed 2026-07-30 against the co-tenant production host (`/opt/forecastiq`, secrets in `/opt/forecastiq/shared/secrets.env` — the co-tenant path, not the standalone `/etc/forecastiq/` one).

| # | Action | Result (2026-07-30) | Done |
|---|--------|---------------------|------|
| 1.1 | Activate OpenWeather in production | **Already active** — key present in `secrets.env`; `GET /api/v1/providers` shows OpenWeather `status=active`, `collecting_since=2026-07-25T07:00Z`. No action needed. | ☑ |
| 1.2 | Verify OpenWeather collection cycle | `collection_attempts_total{provider="openweather",status="success"} = 75` (zero failures), 3,600 snapshots stored, `circuit_state=0` (closed), no rate-limit exhaustion. Budget headroom fine (3 locations ≈ 72 calls/day vs 1000). | ☑ |
| 1.3 | Confirm external uptime checks are live | **GAP — no evidence of any uptime probe.** Also discovered: nothing scrapes the app's loopback `127.0.0.1:9090` metrics (CloudWatch agent collects host disk/mem only) → no alert in §2 can currently fire. See §1a. | ☐ |
| 1.4 | Record the Level-2 go-live date | Go-live 2026-07-28; 90-day mark = 2026-10-26. | ☑ |
| 1.5 | Baseline the SLO dashboard | Captured 2026-07-30 (see §1b). No scrape pipeline yet, so this is a point-in-time `/metrics` snapshot rather than percentile windows. | ☑ |

### 1a. Gaps discovered during §1 execution (2026-07-30)

| Gap | Evidence | Severity |
|-----|----------|----------|
| No metrics scraping / alerting in production | App exposes `127.0.0.1:9090`; no Prometheus/Alloy/Grafana agent on the host; CloudWatch agent config = disk+mem only | High — the entire §2/§3 alert surface is dark |
| No external uptime checks | No probe service found; `/healthz` returns 200 but nobody watches it | High |
| No backup cron on the production host | Deploy-user crontab empty; no `/etc/cron.d/forecastiq*`; `backup-status.json` is 0 bytes (created 2026-07-29, never written) | ~~Critical~~ **CLOSED 2026-07-30**: `/etc/cron.d/forecastiq` installed — nightly `backup.sh` 02:30 UTC + monthly `restore-test.sh` (both scripts gained a `FIQ_DB_CONTAINER` co-tenant override, dumping via `app-postgres-1`; local retention 7d for disk headroom). First backup (324 MB) + restore test (8/8 tables, ≤1% short) green; `forecastiq_backup_status 1` live on `/metrics`. **Fixed a latent WP-24 bug found during rollout: scratch containers leaked their anonymous PG volume (`docker rm -f` without `-v`), which filled the disk to 99% in two runs — now `rm -fv`.** **Offsite CONFIGURED 2026-07-30 (correct account)**: an initial setup in the wrong AWS account (590722478923) was fully reverted on operator instruction; replacement provisioned via Terraform `terraform/backup/` — S3 `forecastiq-backups-077101397287` (od3n.com account, ap-southeast-5, private + SSE + 180d lifecycle safety net) + bucket-scoped `forecastiq-backup` IAM user, account-guard precondition prevents wrong-profile applies; IAM policies documented in `terraform/backup/README.md` + `provisioner-policy.json`. Verified end-to-end on the correct bucket: fresh backup (342 MB) → offsite copy → offsite-sourced restore test 8/8 green; `FIQ_RCLONE_REMOTE` wired into cron; `forecastiq_backup_status 1`. Note: the restore-test ±2% tolerance trips if the dump is > ~1 h older than prod (`accuracy_metrics` grows ~2%/h) — the cron sequence (backup 02:30 → test 04:00) stays inside it. |
| Open-Meteo collections mostly `partial` | `collection_attempts_total{open-meteo}`: 69 partial / 3 success; `collection_records_rejected_total{reason="invalid_range"} = 828` | Medium — investigate which variable persistently fails OC-04 range checks |
| Disk headroom | Host `/` at 73%; `payload_volume_used_bytes` 14.9 GB / 20.6 GB (72%) — §3.3 warn threshold is 80% | Medium — pruning or volume growth plan needed soon |

### 1b. Baseline snapshot (2026-07-30T17:20Z)

| Signal | Value |
|--------|-------|
| Active locations | 3 |
| Providers | Open-Meteo (active, since 07-25 02:00Z), OpenWeather (active, since 07-25 07:00Z) |
| Collection attempts | open-meteo 69 partial + 3 success (828 `invalid_range` rejects); openweather 75 success |
| Snapshots stored | open-meteo 11,268; openweather 3,600 |
| Observations | 33 per location (`openmeteo_historical`); freshness age ≈ 312 s (fresh) |
| Engine lag | 625 s (< 3600 s threshold) |
| Circuits | both closed (0) |
| API latency (avg = sum/count; no histograms scraped yet) | `/rankings` 7.3 ms · `/accuracy` 13.5 ms · `/locations` 1.0 ms · `/forecast-comparison` 113 ms · `/forecasts/latest` 130 ms |
| Payload volume | 14.9 / 20.6 GB (72%) |
| `/healthz` (external via Cloudflare) | 200 |

## 2. Daily (~5 min, or alert-driven)

| # | Check | Signal | Threshold / action |
|---|-------|--------|--------------------|
| 2.1 | Collection success per provider | Grafana collection panel; `collections_total{status}` | Failed rate > 5% of slots over 1h, or any cell stale > 180m → `06-provider-failure-runbook.md` |
| 2.2 | Observation freshness | Freshness grid; `observation_freshness_state{location}` | Any location stale > 240m; unavailable > 24h |
| 2.3 | Engine lag | `engine_lag_seconds` | > 3600s (2 batch cycles) → engine-backlog investigation |
| 2.4 | Ranking freshness | `ranking_freshness_state{location,horizon}` | Any cell stale > 6h |
| 2.5 | Backup succeeded | S-10 backup panel; `backup_last_status` / `backup_age_hours` | Any failed backup, or no success > 26h → `07-database-recovery-runbook.md` |
| 2.6 | Circuit state | `provider_circuit_state{provider}` | Any open circuit → provider failure runbook |
| 2.7 | Nightly scheduled CI (01:30 UTC) | GitHub Actions `Scheduled` — extended property tests + full govulncheck | Red run → triage same day |

## 3. Weekly (~30 min)

| # | Check | Signal / tool | Action |
|---|-------|---------------|--------|
| 3.1 | Weekly scheduled CI (Mon 03:00 UTC) | k6 PT-1/PT-6, PT-3/PT-7 baselines, reliability slice, gitleaks full-history | Compare PT-7 query latencies against the perf doc §6 baseline register; investigate regressions > 20% |
| 3.2 | API latency SLO | `http_request_duration_seconds` p95/p99 | p95 > 200ms over 15m budget burn → api-latency review |
| 3.3 | Payload volume | `payload_volume_used_pct` | > 80% warn; plan pruning/expansion before 90% |
| 3.4 | Schema drift / unmapped conditions | `snapshots_invalid_total`, `collection.schema_drift` logs | Unmapped condition > 1%/day → adapter condition-map bump |
| 3.5 | OpenWeather budget headroom | Rate-budget panel; `provider_rate_limit_exhausted_total` | Any exhaustion event → budget tuning per rate-limits math |
| 3.6 | Auth anomalies | `auth_failures_total{reason}` | > 20 failures/10m from one IP → review; disabled-account attempts noted |
| 3.7 | Error-budget review | SLO dashboard vs `02-sli-slo.md` §2 monthly targets | Budget state drives the §3 error-budget policy |
| 3.8 | Level-3 gate metrics snapshot | Registered users, weekly actives | Track toward ≥ 100 registered / ≥ 5 weekly actives |

## 4. Monthly (1st of month)

| # | Task | Tool | Verification |
|---|------|------|--------------|
| 4.1 | Restore test (04:00 UTC, automated) | `deploy/scripts/restore-test.sh` — restores latest dump into a throwaway container | `last_restore_test` in the backup status JSON; failure → alert A11b; must never exceed 32 days old |
| 4.2 | Secret rotation drill | `deploy/scripts/rotation-drill.sh` (dry-run; `--live` quarterly) | All checks PASS; live mode proves container-recreation path + healthy collection |
| 4.3 | Rollback drill (05:00 UTC, automated) | Monthly scheduled workflow job | Drill green, or skipped cleanly without deploy secrets |
| 4.4 | Certificate expiry | `tls_cert_expiry_days` | > 14 days remaining (Cloudflare/Caddy auto-renewal sanity) |
| 4.5 | Dependency + vuln posture | Nightly govulncheck history; `go.mod` review | No unaddressed fixable advisory older than 30 days |
| 4.6 | SLO monthly report | `02-sli-slo.md` §2 table vs actuals | Record availability, latency windows, collection success ≥ 99%, durability = 100% |
| 4.7 | Offsite backup spot-check | List offsite dumps; sizes monotonic-plausible | At least 1 dump per day present for the month |

## 5. Validation-window exit review (~day 90)

Level 2 → Level 3 gate per `01-scope-levels.md`:

- [ ] ≥ 90 days live (from 2026-07-28 → on/after 2026-10-26)
- [ ] ≥ 100 registered users
- [ ] ≥ 5 weekly actives
- [ ] Positive signal from 5 user interviews (schedule during weeks 6–12)
- [ ] ToS gates cleared for any Level-3 provider candidates (Tomorrow.io, Visual Crossing)
- [ ] Infra headroom confirmed (PT-7 trend flat; no promotion trigger fired; NFR-S01 Level-1 exit already satisfied per WP-26b TC-26b-01)
- [ ] Level-3 selection/business-case document drafted from observed usage data

## 6. Follow-on backlog (fill-in work during the window)

Non-blocking items tracked at acceptance; burn down opportunistically:

| Item | Origin | Notes |
|------|--------|-------|
| Observability polish (Low findings 012–014) | WP-22 re-review | |
| Security-header residuals vs ADR-033 §4 | WP-25 re-review | |
| GDPR export physical retention sweep | DRB-WP19c-001 | Expiry is logical-only today |
| `/forecast-comparison` per-date cache max-age split | DRB-WP16-001 | Past 300s / today 60s |
| `/accuracy` cursor pagination | WP-15 follow-on | Currently bounded by range + limit |
| Cross-horizon profile composites + custom-weight serving | WP-15 follow-on | |
| Cache-hit metadata `request_id` staleness | DRB-WP15-001 | Informational |
