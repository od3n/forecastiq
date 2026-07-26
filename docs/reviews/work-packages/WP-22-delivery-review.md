# ForecastIQ — WP-22 Observability: Delivery Review Board

**Review date**: 2026-07-26
**Work package**: WP-22 — Observability
**Reviewed SHA**: `4a1e55b327cc98f6e6d7d6a59e47adf5dd84ae6e` (`4a1e55b`)
**Decision**: **REJECTED — fixes required (7 blocking findings)**

---

## 1. Verification of evidence

| Check | Result |
|-------|--------|
| Commit identity: local branch == remote == CI head | ✅ all `4a1e55b` |
| CI run **30208004211** (six jobs) | ✅ success |
| Six jobs green, none skipped/cancelled | ✅ backend-checks, backend-integration, migrations, api-contract, security, image |
| Dependency gate: WP-08 (scheduler metrics surface) Accepted | ✅ |

CI evidence is clean. Rejection is on **content**, not process: the alerting layer contains
expression-level defects that CI cannot catch (no PromQL evaluation gate exists).

## 2. Scope review

**Present**: metric catalog §3.1–§3.6 registration (RED, collection, observation, engine,
scheduler, db pool collector, payload-volume gauges, cache, Go/process collectors) ·
log event registry · sanitizing slog handler + secret-leak tests · grafana-agent config ·
3 dashboards as code (API/Pipeline/System) · alert rules A1–A17 present as YAML ·
uptime checks · SLO recording rules · metric-presence + log-field tests (unit-level).

**Absent/partial**: `process_restarts_total` (§3.6) not implemented · `evaluation_backlog`
and `ranking_freshness_age_seconds` registered but never populated · backup/restore metrics
consumed by A10/A11 not exported anywhere · integration-level `/metrics` scrape test missing ·
System-dashboard restart/backup/uptime panels missing.

## 3. Findings

Blocking findings (Critical/High) — all verified by direct code inspection, not just review-agent output.

### Critical

**DRB-WP22-001 (Critical)** — Burn-rate recording rules have a PromQL operator-precedence bug.
`forecastiq:error_budget_burn:{6h,1h}` evaluate as `1 - (ratio / 0.005)` (division binds
tighter than subtraction), yielding ≈ −198 on a healthy system instead of ≈ 0.2. The 2×/10×
burn thresholds of `02-sli-slo.md` §3 can never trigger. Parenthesize:
`(1 - (success/total)) / (1 - 0.995)`.
`deploy/observability/alerts/recording-rules.yaml`

**DRB-WP22-002 (Critical)** — A5 `CollectionStale` compares `time()` against a **counter value**
(`time() - max by (provider)(collection_attempts_total{status="success"} > 0) > 10800`), which
is always true once one success exists → permanently-firing false alert for every provider.
Rewrite as `sum by (provider)(increase(collection_attempts_total{status=~"success|partial"}[3h])) == 0`.
`deploy/observability/alerts/rules.yaml`

**DRB-WP22-003 (Critical)** — A2 `APIErrorRateHigh` is doubly dead: (a) numerator
`http_errors_total` is registered but **never incremented** anywhere in the codebase, and
(b) the division uses one-to-one vector matching over disjoint label sets
(`{route_template,error_type}` vs `{method,route_template,status_class}`) → empty vector.
A sustained 5xx outage would go undetected. Rewrite using
`sum(rate(http_requests_total{status_class="5xx"}[15m])) / sum(rate(http_requests_total[15m]))`
and either instrument `HTTPErrorsTotal` in middleware or remove it from the catalog + presence test.
`deploy/observability/alerts/rules.yaml`, `internal/platform/metrics/metrics.go`

### High

**DRB-WP22-004 (High)** — A7 `EngineLagHigh` is dead: the only write is `EngineLag.Set(0)`
after a successful batch, so the gauge structurally cannot exceed 0 — the exact failure mode
A7 exists to detect (batches stop running) leaves the metric frozen at 0. Export lag via a
`GaugeFunc` computed at scrape time (mirroring the payload-volume pattern in `app.go`).
`internal/scheduler/analysis_dispatcher.go`

**DRB-WP22-005 (High)** — A6 `SchemaDriftDetected` and A14 `UnmappedConditionCodes`: division
with mismatched label sets (`{provider,reason}` / `{provider}` and `{provider,code}` / `{provider}`)
returns an empty vector → both alerts dead. Aggregate both sides with `sum by (provider)`.
`deploy/observability/alerts/rules.yaml`

**DRB-WP22-006 (High)** — A10/A11 reference `forecastiq_backup_*` / `forecastiq_restore_test_*`
metrics that are exported nowhere (only occurrence in the repo is rules.yaml itself). Alerts on
absent vectors silently never fire → backup failure is unmonitored. Export from the backup
status file via GaugeFunc (the `backupstatus` adapter already parses it), or move A10/A11 to
WP-24 scope explicitly with a tracked TODO.
`deploy/observability/alerts/rules.yaml`

**DRB-WP22-007 (High)** — A15 `ProcessRestarts` uses `increase()` on the Unix-timestamp gauge
`process_start_time_seconds` — one benign restart produces a jump of thousands (pages as crash
loop); the intended ">2 restarts/h" semantic is unimplemented. Use
`changes(process_start_time_seconds[1h]) > 2`.
`deploy/observability/alerts/rules.yaml`

### Medium (fix or track — non-blocking individually, but 008/010 strongly recommended now)

**DRB-WP22-008 (Medium)** — `evaluation_backlog` / `ranking_freshness_age_seconds` registered
but never populated; Pipeline dashboard backlog panel shows a constant healthy 0.

**DRB-WP22-009 (Medium)** — grafana-agent promotes `msg` to a Loki stream label → unbounded
stream cardinality (free-tier killer). Drop `msg` from labels; filter via `| json | msg=...`.
`deploy/observability/grafana-agent.yaml`

**DRB-WP22-010 (Medium)** — Sanitizer bypass: camelCase/concatenated keys (`apiKey`, `passwd`,
`jwt`, `bearer`) evade the snake_case substring match. Normalize keys (strip `_`/`-`) before
matching and extend the pattern list. `internal/platform/logging/sanitize.go`

**DRB-WP22-011 (Medium)** — `collection_success` recording rule deviates from `02-sli-slo.md` §4
(spec: schedule_runs; impl: collection_attempts) and counts `deduplicated` as failure,
structurally deflating the 99% SLO.

### Low / Info (tracked, non-blocking)

**DRB-WP22-012 (Low)** — Metric-presence test is a unit test on `Registry.Describe`, not the
scoped integration `/metrics` scrape; payload-volume gauges claimed "tested via integration"
but no such test exists. `TestPoolCollectorDescribes` never exercises the pool collector.

**DRB-WP22-013 (Low)** — Payload-volume GaugeFuncs return 0/0 on `Usage()` error → A9 evaluates
`NaN > 0.8` = false exactly when the volume is broken; also two statfs calls per scrape.

**DRB-WP22-014 (Low)** — Dashboard drift: API "429" panel actually plots all 4xx
(`status_class` cannot isolate 429); System dashboard omits restart/backup/uptime panels (§5).

**Info** — grafana-agent `${GRAFANA_CLOUD_*}` requires `-config.expand-env` (document in unit
file); uptime check label `job="forecastiq-uptime"` must be verified against what Grafana
Synthetic Monitoring actually emits; `.gitleaks.toml` allowlist is narrowly scoped to the
sanitize fixture file — acceptable.

## 4. Assessment

The work package's *structure* is complete and well organized: every scope artifact exists,
the code-level instrumentation (RED middleware, pool collector, payload-volume gauges, batch
duration) is sound, the sanitizing handler with fixture tests is a genuine defense-in-depth
win, and observability-as-code layout under `deploy/observability/` is exactly right.

However, **8 of 17 alert rules (A2, A5, A6, A7, A10, A11, A14, A15) are dead or
permanently-firing as written, and both burn-rate recording rules are mathematically wrong**.
The alerting layer would ship as decoration, not protection — and a permanently-firing A5
would actively train operators to ignore pages. Since WP-22's objective is precisely
"dashboards + alerts", this fails acceptance.

## 5. Decision

**REJECTED.** Findings DRB-WP22-001…007 must be fixed on `feature/wp22-observability`;
008–011 fixed or explicitly tracked; re-review on the new SHA required. Process note for
re-review: consider adding `promtool check rules` to the CI `backend-checks` or a new job so
expression-level regressions are gated mechanically.
