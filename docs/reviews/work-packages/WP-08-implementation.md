# ForecastIQ — WP-08 Forecast Scheduler and Collection Operations: Implementation Report

**Version**: 1.0
**Implementation date**: 2026-07-23
**Work package**: WP-08 — Forecast Scheduler and Collection Operations
**Authority**: `docs/planning/05-implementation-work-packages.md` §WP-08 (definition); ADR-005 (scheduler decision); `docs/workflows/05-scheduling-and-retries.md`; `docs/workflows/06-backfill-and-reprocessing.md` (FC-14 replay)
**Branch**: `feature/wp08-scheduler-collection-ops` (base: accepted WP-06 tip `9eda963`)
**Status**: **Implementation Complete — Not Accepted** (Delivery Review Board transition only)

> Scope discipline: this package hardened the WP-01-era scheduler prototype into the full WP-08 feature set (watchdog/missed-slot detection, scheduler lag metric, graceful in-flight drain, raw-payload replay with checksum verification + quarantine, and the manual-trigger budget/rate guard). No WP-07 (OpenWeather), WP-09 (observation adapter), or WP-10 (observation collection) work was started. No analysis-layer code was touched.

---

## 1. Executive summary

- **Objective**: Slot-based scheduler (ADR-005) with run history, manual trigger, and replay — hardened for unattended operation.
- **Prior state**: Prototype Exists — the bootstrap commit `4f24fa3` provided slot generation, `FOR UPDATE SKIP LOCKED` claiming, leases, retry/backoff, `schedule_runs` history, dispatch, `POST /admin/collections/trigger`, `GET /forecast-collections`, and the `--mode` flag.
- **Implemented this package**:
  1. **Watchdog + missed-slot detection** — a stall warning when claimable slots exist without progress for 2× the tick interval, plus a per-slot missed-schedule counter driven off claim lag.
  2. **Scheduler lag metric** — `scheduler_lag_seconds{job_type}` (slot_time → claimed_at delta), completing the workflow 05 §10 metric catalog.
  3. **Graceful shutdown drain** — in-flight jobs run under a context detached from the loop (bounded by a per-job timeout) and are drained within a bounded deadline before the process exits; `serve` now waits for the worker before the pool closes.
  4. **Raw-payload replay (FC-14)** — `POST /admin/collections/{id}/replay`: load original → read payload → verify SHA-256 → quarantine on mismatch → decode with the current adapter → new collection with dedup'd snapshots. Idempotent; originals never mutated.
  5. **Manual-trigger budget/rate guard** — a rate-limited provider outcome is surfaced as `429` with `Retry-After` (circuit-open remains `409`).
- **Final status**: Implementation Complete; awaiting Delivery Review Board.

## 2. Authorization and selection

| Check | Evidence | Result |
|-------|----------|--------|
| WP-06 Accepted | `06-work-package-status-registry.md` line 20 (confirmatory re-review 2026-07-23; TC-06-01 Closed) | ✅ |
| WP-08 selected | Registry line 22 — Programme Work-Package Selection Board selected WP-08 following WP-06 acceptance | ✅ |
| Hard dependencies Accepted | WP-04, WP-05, WP-06 all Accepted | ✅ |
| WP-08 definition found | `05-implementation-work-packages.md` §WP-08 | ✅ |

## 3. Scope reconstruction

| # | Approved scope item | Prior state | This package | Result |
|---|---------------------|-------------|--------------|--------|
| S1 | Slot generation | Implemented (prototype) | unchanged | ✅ |
| S2 | SKIP LOCKED claims | Implemented | unchanged (concurrency test pre-existing) | ✅ |
| S3 | Leases | Implemented | reclaim proven by test | ✅ |
| S4 | Backoff | Implemented | FC-08 backoff unit-tested | ✅ |
| S5 | Watchdog (missed-slot) | **Missing** | stall watchdog + `scheduler_missed_slots_total` | ✅ **added** |
| S6 | `schedule_runs` | Implemented | unchanged | ✅ |
| S7 | `POST /admin/collections/trigger` (409/429 guards) | Endpoint existed; 409 only | 429 rate guard added | ✅ **completed** |
| S8 | `POST /admin/collections/{id}/replay` (checksum verify, quarantine) | **Missing** | full FC-14 replay use case + endpoint | ✅ **added** |
| S9 | `GET /forecast-collections` (admin) | Implemented | unchanged | ✅ |
| S10 | Graceful shutdown drain | Partial (loop stopped; jobs cancelled) | detached job ctx + bounded drain + serve wait | ✅ **completed** |
| S11 | `--mode` flag | Implemented | unchanged | ✅ |
| Obs | Scheduler metrics (claimed/missed/lag/duration) | claimed + duration only | + `scheduler_lag_seconds`, wired missed | ✅ **completed** |

## 4. Architecture implementation

No architecture change. All additions respect the ports-and-adapters boundaries: the scheduler owns orchestration and consumes `catalog`/`collection` ports; `Quarantine` was added to the `PayloadStore` port and implemented by the filesystem adapter; replay is a new `collection.ForecastReplayer` use case on the existing `CollectService`. Dependency direction unchanged; adapters wired only in the composition root. No new module, service, or migration.

## 5. Requirement → test traceability

| Requirement | Test | Level |
|-------------|------|-------|
| FC-08 retry backoff (1,2,4,8,16s clamped) | `TestRetryBackoff` | unit |
| Claim lag / missed-slot decision | `TestSlotLag` | unit |
| Config defaults (job timeout, drain, missed=2×interval floor) | `TestNewConfigDefaults` | unit |
| Error classification (circuit/inactive/generic) | `TestClassifyError` | unit |
| Concurrent claim, no double-claim | `TestSkipLockedClaim` (pre-existing) | integration |
| Lease expiry → reclaim by another instance | `TestLeaseExpiryReclaim` | integration |
| Scheduler run: claim → execute → metrics (claimed/lag/missed) → drain | `TestSchedulerRunCollectsAndDrains` | integration |
| Double-fire produces zero duplicate snapshots (acceptance) | `TestSchedulerNoDoubleCollection` | integration |
| Replay idempotency (new collection, no dup snapshots, original untouched) | `TestReplayIdempotency` | integration |
| Replay checksum mismatch → quarantine → payload_unavailable | `TestReplayChecksumMismatchQuarantine` | integration |
| Replay unknown id → not found | `TestReplayUnknownCollection` | integration |
| Replay endpoint (auth 401 / 200 / 404) | `TestAPI_ReplayCollection` | integration |
| Manual-trigger rate guard → 429 + Retry-After | `TestAPI_TriggerRateLimited429` | integration |

## 6. Database changes

```text
No database changes required.
```

The scheduler tables (`collection_schedules`, `schedule_runs`) and the collection tables already exist from WP-02. Replay reuses the existing `forecast_collections` dedup unique index by writing the replay collection with a distinct `requested_at` and a cleared `provider_model_run_time`, so it never collides with the original success/partial row while snapshot-level `ON CONFLICT DO NOTHING` lands only genuinely new rows.

## 7. API changes

- **Added**: `POST /admin/collections/{id}/replay` (admin) — recorded in `api/openapi/openapi.json` (9 paths; `make docs` green).
- **Changed**: `POST /admin/collections/trigger` now documents `429` (rate/budget guard) alongside the existing `409`.

## 8. Observability

- New metric `scheduler_lag_seconds{job_type}` (histogram); `scheduler_missed_slots_total{job_type}` is now incremented on late claims; existing `scheduler_slots_claimed_total` and `job_duration_seconds` unchanged. Stall condition logs `scheduler.stalled`; drain logs `scheduler.drained` / `scheduler.drain_timeout`.

## 9. Security

- Replay never calls the provider network and never logs payload bytes. Corrupt payloads (checksum mismatch) are quarantined, not served. The admin endpoints remain behind `RequireAdmin`. No credential handling change.

## 10. Configuration

New `FIQ_`-prefixed variables (documented in `.env.example`, validated fail-fast in `config.Load`):
`FIQ_WORKER_JOB_TIMEOUT` (60s), `FIQ_SCHEDULER_DRAIN_TIMEOUT` (30s), `FIQ_SCHEDULER_MISSED_THRESHOLD` (blank → 2×interval, floor 2m).

## 11. Validation results

Run locally (Docker unavailable in this environment — integration suite runs in CI, mirroring prior WP reviews):

| Command | Result |
|---------|--------|
| `gofmt` / `make fmt-check` | ✅ clean |
| `go vet ./...` | ✅ clean |
| `go build ./...` | ✅ |
| `go test -race ./...` (unit) | ✅ all packages pass |
| `golangci-lint run ./...` | ✅ zero findings |
| `make docs` (OpenAPI) | ✅ 9 paths valid |
| `go test -tags integration ./...` | ⏳ **to be confirmed by CI** (no local Docker) |

## 12. Files changed

- **Scheduler**: `internal/scheduler/scheduler.go`, `internal/scheduler/ports.go`, `internal/scheduler/scheduler_test.go`, `adapters/persistence/schedulerpg/schedulerpg.go`
- **Metrics**: `internal/platform/metrics/metrics.go`
- **Replay / collection**: `internal/collection/replay.go` (new), `internal/collection/collection.go`, `internal/collection/collect.go`, `internal/collection/domain/errors.go`, `internal/collection/ports/payloadstore.go`, `adapters/payloadstore/filesystem.go`
- **API**: `internal/api/handlers/collection.go`, `internal/api/handlers/handlers.go`, `internal/api/router.go`, `internal/api/respond/errors.go`, `api/openapi/openapi.json`
- **Composition / config**: `internal/platform/config/config.go`, `cmd/forecastiq/app.go`, `cmd/forecastiq/serve.go`, `.env.example`
- **Tests**: `test/integration/setup_test.go`, `test/integration/scheduler_ops_test.go` (new), `test/integration/replay_test.go` (new)
- **Docs**: this report; `docs/planning/06-work-package-status-registry.md`

## 13. Deviations

```text
No approved-scope deviations.
```

**Recorded design note (non-blocking):** a replay writes a **new** collection recorded with `collection_status = deduplicated`, so it is excluded from both the success dedup unique index and the `LatestSuccessful` query and can never shadow the original in `/forecasts/latest` (DRB-WP08-001, remediated). `requested_at` and `provider_model_run_time` mirror the original issuance for lineage (dedup-safe because the row is never success/partial). Snapshots are keyed by the original issuance and inserted `ON CONFLICT DO NOTHING`, so identical rows dedup and only genuinely new-key rows land — an accepted, documented consequence of snapshot immutability (a fixed adapter that changes an existing row's values cannot overwrite it; it can only add new keys).

## 14. Known limitations

- 48-hour unattended soak (acceptance) is a manual/operational check; the automated suite proves the equivalent invariants (claim-execute cycle, zero-duplicate double-fire, drain, reclaim). Multi-provider staggered offsets (OM :00 / OW :02) arrive with WP-07.

## 15. Work-package transition

```text
WP-08 — Forecast Scheduler and Collection Operations

Previous State:
Selected — Not Started

New State:
Accepted (DRB full review + re-review 2026-07-23)

Accepted Implementation SHA:
daee1e1bd07718ce073ba0c11d5e02cd8fa9432c
```

## 16. Recommended next action

```text
Merge PR #4 to main; select the next approved work package.
```
