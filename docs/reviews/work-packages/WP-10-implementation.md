# ForecastIQ — WP-10 Observation Collection: Implementation Report

**Version**: 1.0
**Implementation date**: 2026-07-23
**Work package**: WP-10 — Observation Collection
**Authority**: `docs/planning/05-implementation-work-packages.md` §WP-10; `docs/workflows/02-observation-collection.md`; ADR-025 (collection model), ADR-005 (scheduler); domain architecture §2.7; OC-01/OC-03/OC-04; `docs/data/03-table-design.md` §3
**Branch**: `feature/wp10-observation-collection` (base: `main` `9900814`)
**Status**: **Implementation Complete — Not Accepted** (Delivery Review Board transition only)

> Scope discipline: this package built the scheduled observation-collection pipeline over the WP-09 adapter — the `observations` migration, storage with dedup, correction cascade (supersession + events), freshness gauge, scheduler `:05` slot generation, routing dispatcher, and composition-root wiring. It did **not** touch matching/analysis (WP-11+). It folds in DRB-WP09-001.

---

## 1. Executive summary

- **Objective**: scheduled observation pipeline + correction-cascade initiation (workflow §2).
- **Implemented this package**:
  - **Migration `20260801000007_create_observations`** (up/down): `observation_type`/`quality_flag` enums; monthly-partitioned `observations` table; **partial** live-row dedup index (DR-05); freshness/lookup indexes; a supersession-only immutability trigger (the sole permitted mutation is `superseded_observation_id`); initial partitions via the existing `create_monthly_partition` helper.
  - **`ObservationRepository` port** + `observationpg` adapter: `EnsurePartitions`, `ListCurrentByWindow` (non-superseded), `InsertBatch` (`ON CONFLICT … WHERE superseded_observation_id IS NULL DO NOTHING`), `Supersede` (narrow UPDATE), `LatestObservedAt`.
  - **`collection.ObserveService`**: fetch (WP-09 adapter) → in one tx: ensure partitions, load live rows, classify each fetched row as **new / correction (value-diff ε) / dedup**, supersede-then-insert (order preserves the partial index), → post-commit `observation.collected` (+ `observation.corrected` per correction), metrics, and the `observation_freshness_age_seconds` gauge.
  - **Scheduler**: `JobObservationCollection`; `:05` slot generation per active location (owned by the seeded Open-Meteo config; `job_type` discriminates); `Router` (job-type dispatch) + `ObservationDispatcher` (hour-aligned 2 h window).
  - **Metrics**: `observations_collected_total`, `observations_suspect_total`, `observation_freshness_age_seconds`.
  - **Composition root** (`app.go`): observation adapter + repo + service + dispatchers wired into the worker; observation events logged.
  - **Folded in DRB-WP09-001**: corrected the `ObservationRequest` window comment to inclusive `[start, end]`.
- **Deferred**: matching/rematch on `observation.corrected` and downstream recompute are **WP-11+** (this package emits the events; consumers arrive later). 48 h unattended acceptance is a manual/operational check; the automated suite proves the equivalent invariants.
- **Final status**: Implementation Complete; awaiting pushed-branch CI + Delivery Review Board.

## 2. Authorization + selection

| Check | Evidence | Result |
|-------|----------|--------|
| WP-09 Accepted + merged | registry line 09; PR #7 merged `e80a51e` | ✅ |
| WP-08 (scheduler) Accepted | registry line 08 | ✅ |
| WP-10 Selected | registry line 10 (`docs(planning): select WP-10`, `9900814`) | ✅ |
| Observation-slot owner decision | User-confirmed: reuse the seeded Open-Meteo config | ✅ |

## 3. Scope reconstruction (§WP-10)

| # | Approved scope item | This package | Result |
|---|---------------------|--------------|--------|
| S1 | Observation slots (:05) | Scheduler generates `observation_collection` slots per active location at the `:05` offset (config-owned; `job_type` discriminates) | ✅ |
| S2 | Storage tx + `observation.collected`/`observation.corrected` events | `ObserveService` single-tx store + post-commit events (shapes frozen in `platform/events`) | ✅ |
| S3 | Supersession update (the one permitted mutation) | `observationpg.Supersede` + DB trigger enforcing supersede-only | ✅ |
| S4 | Freshness gauge | `observation_freshness_age_seconds{location_id}` set from newest observed_at | ✅ |
| Acc | 48 h unattended observation collection | scheduler wired (worker `--mode`); invariants proven by dedup/correction integration tests | ✅ (soak = operational) |

Exclusions respected: no matching/analysis (WP-11+); no API endpoints (`/observations` is WP-15+). The `observations` migration + enums land here (they were explicitly deferred by `20260801000001` to the table's owning work package).

## 4. Architecture + key decisions

- **Observation-slot ownership** (schema fit): `collection_schedules.provider_configuration_id` is NOT NULL, but observations use a `source` string, not a provider config. Per user confirmation, observation slots hang off the seeded **Open-Meteo** provider configuration; `job_type='observation_collection'` discriminates them from forecast slots (the slot uniqueness key includes `job_type`). `Config.ObservationConfigID = uuid.Nil` disables generation.
- **Correction cascade & the partial index (DR-05)**: a correction inserts a **new** row sharing `(source, location_id, observed_at)` and marks the old row superseded. A plain unique index (as the table-design DDL declared) would reject the new row; the dedup index is therefore **partial** on `WHERE superseded_observation_id IS NULL`. The tx **supersedes the old row before inserting** the corrected row so only one live row exists at a time.
- **No parent collection entity / no payload** (ADR-025): observations store no raw payload or checksum; collector health derives from the freshness gauge.
- Dependency direction preserved: `ObserveService` (collection module) depends on `ports` + `platform`; the scheduler depends on a small `ObservationCollector` interface it defines; wiring is only in the composition root.

## 5. Tests

| Layer | Test | Proves |
|-------|------|--------|
| Domain (WP-09, reused) | `observation_test.go` | OC-04 ranges; ε correction; type validity |
| Scheduler unit | `scheduler/dispatcher_test.go` — `TestRouter_RoutesByJobType`, `TestObservationDispatcher_Window`, `..._RejectsWrongJobType` | job-type routing; hour-aligned 2 h window |
| Integration (real PG16) | `test/integration/observation_test.go` — `TestObservationCollection_DedupAndCorrection`, `..._SuspectStored` | window dedup; correction → supersede + corrected event + partial-index coexistence; suspect stored not dropped; supersede-only trigger exercised |

Full `go test -race ./...` (unit) green; `gofmt`/`go vet`/`golangci-lint` clean; `go vet -tags integration ./test/integration/...` compiles.

## 6. Database changes

Adds migration `20260801000007_create_observations` (up + down): the `observations` table (monthly-partitioned by `observed_at`), `observation_type`/`quality_flag` enums, the partial live-row dedup index, freshness/lookup indexes, and the supersession-only immutability trigger. The `migrations` CI job (up + verify + seed×2) exercises it.

## 7. API changes

```text
No public API changes. /observations query endpoints are WP-15+.
```

## 8. Security

- Open-Meteo Historical is keyless; no credential handling; base URL is seeded config (no SSRF). No payloads/URLs logged; observations store no payload (ADR-025).

## 9. CI evidence

Branch pushed; PR #8 → `main` triggered CI run **30031945500** (event `pull_request`) **success** on head SHA `b42e937800ff61a0c3f0d2d91d39ba54ed210b95` (`b42e937`) with all six mandatory jobs green (`backend-checks`, `backend-integration`, `migrations`, `api-contract`, `security`, `image`), none skipped/cancelled; local == remote tip == CI head SHA. The `migrations` job applied `20260801000007`; `backend-integration` ran the observation dedup/correction/suspect tests against real PostgreSQL 16.

## 10. Files changed

- **Migration**: `migrations/20260801000007_create_observations.{up,down}.sql`
- **Ports**: `internal/collection/ports/repositories.go` (+`ObservationRepository`); `internal/collection/ports/observation.go` (DR-09-001 comment)
- **Persistence**: `adapters/persistence/observationpg/observationpg.go`
- **Service**: `internal/collection/observe.go`
- **Scheduler**: `internal/scheduler/{slot.go,scheduler.go,dispatcher.go,observation_dispatcher.go}`
- **Metrics**: `internal/platform/metrics/metrics.go`
- **Wiring**: `cmd/forecastiq/app.go`
- **Tests**: `internal/scheduler/dispatcher_test.go`, `test/integration/observation_test.go`
- **Docs**: this report; `docs/data/03-table-design.md` (§3 partial index, DR-05); `docs/planning/06-work-package-status-registry.md`

## 11. Deviations / recorded discrepancies

- **DR-05 (resolved in migration + doc):** `docs/data/03-table-design.md` §3 declared `observations_dedup` as a plain unique index, incompatible with the correction model (a correction shares the key). Resolved: the index is **partial** on `WHERE superseded_observation_id IS NULL`; the table-design DDL was corrected to match.
- **DRB-WP09-001 (folded in):** `ObservationRequest` window comment corrected to inclusive `[WindowStart, WindowEnd]`.

## 12. Work-package transition

```text
WP-10 — Observation Collection
Previous State: Selected — Not Started
New State: Implementation Complete
Acceptance State: Not Accepted (pending pushed-branch CI + Delivery Review Board)
```

## 13. Recommended next action

```text
Convene the Delivery Review Board for WP-10. CI evidence is captured (§9):
run 30031945500 on head SHA b42e937, six mandatory jobs green.
```
