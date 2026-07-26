# ForecastIQ — WP-23 Local Observability Stack + S-16/S-17: Delivery Review Board

**Review date**: 2026-07-26
**Work package**: WP-23 (local slice) — Local observability stack + S-16/S-17 admin screens (PR #34, `feature/wp23-local-observability`)
**Reviewed SHA**: `e7c2cb8` (post WP-22 merge-up)
**Decision**: **REJECTED — 1 Critical + 3 High findings**

---

## 1. Verification of evidence

| Check | Result |
|-------|--------|
| Branch merged with `main` (contains accepted WP-22) | ✅ `e7c2cb8` |
| Seven jobs green on pre-merge SHA `406e7ca` (incl. frontend-checks) | ✅ (re-verification required on new SHA) |
| Dashboard panel queries cross-checked against registered metric names | ✅ all 10 Prometheus queries + Loki query valid; no stale WP-22 names |

## 2. Findings

### Critical

**DRB-WP23L-001 (Critical)** — `make obs-reset` destroys the application database.
`docker compose --profile obs down -v` removes **all** named volumes of the
project — including `pgdata` and `payloads` — because profile flags only *add*
services to the always-enabled set; `down`/`down -v` still operate project-wide.
`make obs-down` likewise stops app/db/frontend, not just the obs services.
Verified against the branch Makefile and compose file. A developer running
"destroy observability volumes and restart clean" silently wipes local Postgres.
Fix: use `rm -sf prometheus loki promtail grafana` + explicit
`docker volume rm` of the three obs volumes; never `down` for a profile subset.

### High

**DRB-WP23L-002 (High)** — Promtail promotes free-text `msg` to a Loki stream
label (`deploy/observability/promtail.yml`) — the identical unbounded-cardinality
defect fixed in grafana-agent.yaml under DRB-WP22-009; it must not be
reintroduced in the local stack. Drop `msg` from the labels stage.

**DRB-WP23L-003 (High)** — Visiting `/forecast-comparison` permanently overwrites
the user's saved horizon preference: the S-05 clamp calls `setParams`, which
unconditionally persists `1440` to localStorage, clobbering the stored selection
for every other screen. Persist only user-initiated changes (e.g. `persist: false`
for the clamp, or skip `storeHorizon` for forced values).

**DRB-WP23L-004 (High)** — `NEXT_PUBLIC_DEV_TOKEN` is attached as a Bearer header
in `useApi` with no environment guard; the only protection against an
admin-granting token entering a production bundle is convention. Gate in code:
inject only when `NODE_ENV === "development"`, and collapse the three duplicated
`authHeaders()` copies onto one helper.

### Medium

**DRB-WP23L-005 (M)** — horizon URL param lost its NaN guard
(`use-global-params.ts`): `?horizon_minutes=abc` propagates `NaN` into API paths.
Guard with `Number.isFinite(n) && n > 0`.
**DRB-WP23L-006 (M)** — S-16 renders `completed_at: null` (failed collections) as
the epoch date and always labels the summary "Latest collection" even for a
historical pick.

### Low / Info (tracked)

**DRB-WP23L-007 (L)** — prometheus.yml header comment claims host.docker.internal
but the target is `app:9090`; native-host runs are never scraped. Fix comment,
optionally add the second target.
**Info** — Grafana claims host port 3000 (collides with native `npm run dev`);
the ops dashboard has no panel for the engine metrics (engine_lag_seconds /
evaluation_backlog / ranking_freshness_age_seconds); admin screen timestamps use
unlabeled `toLocaleString()` (pre-existing pattern, not a regression).

## 3. Scope coverage

**Present**: 4 compose services under the `obs` profile (correct ports/mounts) ·
Grafana datasource provisioning (uids match) · ForecastIQ Operations dashboard
with 11 panels (all queries valid) · Makefile obs-up/down/reset targets (lifecycle
bug above) · S-16 Raw Forecasts (page + snapshots endpoint + OpenAPI + nav) ·
S-17 Admin Dashboard (page + ConditionsTimeline ±12h strips + nav) · backend
support endpoints reviewed clean (snapshots, provider-configurations,
condition_code plumbing, seed idempotency).

**Absent**: none against the stated scope.

## 4. Decision

**REJECTED.** DRB-WP23L-001 (data-destroying make target) is disqualifying on its
own; 002–004 must be fixed with it; 005–006 fixed or tracked. Scope itself is
fully delivered and the dashboard/query hygiene is good — this package is one
short fix-cycle from acceptance. Re-review on the new SHA with seven jobs green.
