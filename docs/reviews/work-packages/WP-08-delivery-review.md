# ForecastIQ — WP-08 Delivery Review Board Report

**Version**: 1.0
**Review date**: 2026-07-23
**Work package**: WP-08 — Forecast Scheduler and Collection Operations
**Reviewed branch**: `feature/wp08-scheduler-collection-ops`
**Reviewed commit (full review)**: `7e295715cd727d85395f738507959b8e5a49e687` (`7e29571`)
**Authority**: `docs/planning/05-implementation-work-packages.md` §WP-08; ADR-005; `docs/workflows/05-scheduling-and-retries.md`, `06-backfill-and-reprocessing.md`
**Panel**: Independent Delivery Review Board (separation of duties — not the implementation team)

---

## 1. Review readiness

| Item | Evidence | Result |
|------|----------|--------|
| Implementation report | `docs/reviews/work-packages/WP-08-implementation.md` | ✅ |
| Final implementation SHA | `7e29571` (local == remote == CI head) | ✅ |
| Remote branch | `origin/feature/wp08-scheduler-collection-ops` | ✅ |
| Dependency acceptance | WP-04, WP-05, WP-06 Accepted | ✅ |
| CI evidence | run `29998226061` (`pull_request`, head `7e29571`) — six jobs green | ✅ |
| Scope contamination | diff touches only scheduler/collection/api/config/tests/docs; no migration/`ci.yml`/`.github` changes | ✅ |

## 2. Requirement verification

| Requirement | Implementation | Test evidence | Result |
|-------------|----------------|---------------|--------|
| Watchdog / missed-slot detection | `scheduler.watchdog` + `MissedSlots` on late claim | `TestSchedulerRunCollectsAndDrains` | PASS |
| Scheduler lag metric | `scheduler_lag_seconds` | run test asserts observation | PASS |
| Leases + reclaim | `ClaimDue` expired-lease branch | `TestLeaseExpiryReclaim` | PASS |
| Backoff (FC-08) | `retryBackoff` | `TestRetryBackoff` | PASS |
| Graceful drain | detached job ctx + bounded `drain` + serve wait | `TestSchedulerRunCollectsAndDrains` | PASS |
| Double-fire → zero duplicates (acceptance) | SKIP LOCKED + collection dedup | `TestSchedulerNoDoubleCollection` | PASS |
| `POST /admin/collections/{id}/replay` | `CollectService.Replay` | replay suite | PARTIAL (see DRB-WP08-001) |
| Checksum verify + quarantine | `Replay` + `PayloadStore.Quarantine` | `TestReplayChecksumMismatchQuarantine` | PASS |
| Trigger 409/429 guards | circuit `409`; rate-limited `429` | `TestAPI_TriggerRateLimited429` | PASS |
| `--mode` flag | pre-existing | — | PASS |

## 3. Architecture review

Dependency direction preserved; `Quarantine` added to the `PayloadStore` port and implemented by the filesystem adapter; replay is a `collection.ForecastReplayer` use case wired only in the composition root. No new module/migration. **Acceptable.**

## 4. Findings

### DRB-WP08-001 — Replay regresses `/forecasts/latest` — **High — blocks acceptance**

- **Category**: functional / regression.
- **Affected requirement**: FC-14 replay; regression against the accepted `/forecasts/latest` read.
- **Evidence**: `Replay` sets `collection_status = success` with `requested_at = now`; `CollectionRepository.LatestSuccessful` orders `success|partial` by `requested_at DESC`, so the replay row becomes "latest". Snapshots insert `ON CONFLICT DO NOTHING`, so pre-existing rows keep the original `forecast_collection_id` and the replay collection owns zero snapshots. `ReaderService.LatestForecast` then returns the replay collection with an empty snapshot list.
- **Impact**: after replaying the most recent collection, the public latest-forecast for that provider+location returns no snapshots until the next scheduled collection supersedes it (up to one hour).
- **Required remediation**: the replay collection must not compete in `LatestSuccessful`. Record it as `deduplicated` (excluded from both the success dedup index and the latest-successful query) while still inserting any genuinely new-key snapshots; add a regression test asserting latest-forecast is unchanged after a replay.
- **Acceptance condition**: latest-forecast returns the original complete snapshot set after a replay, proven by test; CI green on the remediation SHA.

### DRB-WP08-002 — Watchdog issues a count query per idle tick — **Informational (non-blocking)**

- Once `lastProgress` is stale (normal between hourly slots), the watchdog runs `CountClaimable` every tick. Harmless (indexed partial count) but noted for a future cheap short-circuit.

## 5. Full-review decision

```text
CHANGES REQUIRED
```

Blocking finding: DRB-WP08-001. Route to WP-08 Remediation → DRB Re-Review.

---

## 6. Remediation (WP-08 Remediation Team)

**DRB-WP08-001** — `internal/collection/replay.go`: the replay collection is now recorded with `collection_status = deduplicated` (excluded from the success dedup unique index and from `LatestSuccessful`); `requested_at`/`provider_model_run_time` mirror the original for lineage (dedup-safe). Snapshot insertion is decoupled from the collection status (`storeSnapshots`), so genuinely new-key rows still land while the original remains the authoritative latest-forecast source. A failed decode is recorded honestly as `failed`.

Regression test added: `TestReplayDoesNotShadowLatestForecast` — after replaying the most recent collection, `LatestForecast` still returns the original collection with its full 3-snapshot set, and the replay row's status is `deduplicated`.

**DRB-WP08-002** — accepted as informational; no change.

## 7. Delivery Review Board Re-Review

```text
Role: Independent Delivery Review Board Re-Review Panel
```

| Finding | Original issue | Remediation evidence | Test evidence | Result |
|---------|----------------|----------------------|---------------|--------|
| DRB-WP08-001 | replay shadowed `/forecasts/latest` with a snapshot-less success row | replay recorded as `deduplicated`; snapshot insertion decoupled | `TestReplayDoesNotShadowLatestForecast` | CLOSED |
| DRB-WP08-002 | idle watchdog count query | none (informational) | — | ACKNOWLEDGED |

**Change-scope check**: remediation touched only `internal/collection/replay.go` and added one integration test; no scope broadening, no test weakening, no architecture change, no migration/CI-config change. `go build`, `go vet` (incl. integration tag), `go test -race ./...` (unit), `golangci-lint`, and `make docs` all green locally; integration suite to be confirmed by CI on the remediation SHA.

**Re-review decision** (pending CI on the remediation SHA):

```text
ACCEPTED — conditional on six mandatory CI jobs green on the pushed remediation SHA
```
