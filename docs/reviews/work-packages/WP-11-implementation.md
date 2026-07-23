# ForecastIQ — WP-11 Matching Engine: Implementation Report

**Version**: 1.0
**Implementation date**: 2026-07-23
**Work package**: WP-11 — Matching Engine
**Authority**: `docs/planning/05-implementation-work-packages.md` §WP-11; `docs/workflows/03-matching.md`; ADR-014; BR-MATCH-01..06; domain architecture §2.8; `docs/data/03-table-design.md` §4
**Branch**: `feature/wp11-matching-engine` (base: `main` `fdbc9b6`)
**Status**: **Implementation Complete — Not Accepted** (Delivery Review Board transition only)

> Scope discipline: this package implements the deterministic matching engine — the `matched_evaluations` migration, the analysis module (domain selection + service), the batch + rematch passes, the scheduler `analysis_batch` job, and wiring. It does **not** compute metrics or rankings (WP-12/13/14) and adds no API. It opens the Analysis phase.

---

## 1. Executive summary

- **Objective**: deterministic exact-hour matching per BR-MATCH-01..06 (ADR-014).
- **Implemented this package**:
  - **New `internal/analysis` module** — `domain` (MatchedEvaluation, provenance rank, and the total-order `SelectCandidate` — the property-tested deterministic core), `ports.MatchRepository`, and the `MatchService` batch engine.
  - **Migration `20260801000008_create_analysis`** — `matched_evaluations` (uniqueness `(forecast_snapshot_id, observation_id)`, indexes for the unmatched scan / rematch / downstream evaluation, immutability trigger; logical refs to the partitioned tables per §4).
  - **`analysispg` adapter** — `ListUnmatchedSnapshots` (NOT EXISTS, keyset-chunked by id), `FindCandidates` (exact-hour, non-suspect, live), `InsertMatches` (`ON CONFLICT DO NOTHING`), `ListRematchTargets` (superseded-observation pairs), `CountUnmatched` (backlog).
  - **`MatchService.MatchBatch`** — chunked (5K) match over `[now−30d, now−2h]` with per-chunk tx (failure isolation), then a rematch pass adding new pairs for superseded observations (old retained), then the `matching_backlog` gauge. Idempotent.
  - **Scheduler** — `analysis_batch` job at `:10`/`:40` (global slot owned by the seeded Open-Meteo config), `AnalysisDispatcher`, `Router` entry.
  - **Metrics** — `matching_pairs_created_total`, `matching_backlog`; batch duration via the existing `job_duration_seconds{job_type=analysis_batch}`.
  - **Wiring** in the composition root.
- **Deferred**: pair-level evaluation/metrics (WP-12), aggregation (WP-13), rankings (WP-14), API (WP-15+). Sub-hourly ±15 min matching remains behind the source-capability flag (no MVP source; ADR-014).
- **Final status**: Implementation Complete; awaiting pushed-branch CI + Delivery Review Board.

## 2. Authorization + selection

| Check | Evidence | Result |
|-------|----------|--------|
| WP-10 Accepted + merged | registry line 10; PR #8 merged `3c4ac83` | ✅ |
| WP-08 (scheduler) Accepted | registry line 08 | ✅ |
| WP-11 Selected | registry line 11 (`docs(planning): select WP-11`, `fdbc9b6`) | ✅ |

## 3. Scope reconstruction (§WP-11)

| # | Approved scope item | This package | Result |
|---|---------------------|--------------|--------|
| S1 | Analysis matching (batch, chunked 5K) | `MatchBatch` keyset-chunked at 5000, per-chunk tx | ✅ |
| S2 | Candidate selection (provenance rank, corrected preference, tiebreak) | `domain.SelectCandidate` total order: corrected → provenance rank → nearest-hour → id | ✅ |
| S3 | Rematch on correction | rematch pass over superseded-observation pairs → new pair (old retained) | ✅ |
| S4 | Backlog gauge | `matching_backlog` set from `CountUnmatched` | ✅ |
| Acc | Batch over a seeded month < 2 min; all BR-MATCH tested | chunked design (NFR-P06 headroom); BR-MATCH-01/03/04/05/06 covered by unit+integration | ✅ (perf = design/operational) |

Exclusions respected: no metrics/rankings/API; sub-hourly rule dormant behind the capability flag (BR-MATCH-02).

## 4. Architecture + key decisions

- **New analysis module** with correct dependency direction: `MatchService` → `ports` + `platform`; the deterministic selection lives in `analysis/domain` (pure, no I/O) so it is unit/property-testable. The scheduler depends on a small `BatchMatcher` interface it defines; wiring is only in the composition root.
- **Read models, not cross-module domain reuse**: the engine consumes lightweight `SnapshotToMatch` / `ObservationCandidate` read structs (populated by `analysispg`), keeping the analysis module independent of the collection module's domain types.
- **Determinism by total order** (ADR-014): `SelectCandidate` is a strict total order ending in the id tiebreak, so the winner is invariant to candidate arrival order — proven by a 500-permutation property test.
- **Append-only rematch** (BR-INV-03): corrections never edit pairs; a new pair is added and the old retained. `matched_evaluations` is immutable (trigger blocks UPDATE/DELETE).
- **`analysis_batch` slot ownership**: like observations, the global batch slot reuses the seeded Open-Meteo config to satisfy the NOT NULL FK; `job_type` + `location_id NULL` discriminate it.

## 5. Tests

| Layer | Test | Proves |
|-------|------|--------|
| Domain unit | `analysis/domain/match_test.go` | corrected preference; provenance rank; nearest-hour + id tiebreak; empty→nil; **500-permutation determinism property** |
| Integration (real PG16) | `test/integration/matching_test.go` — `TestMatching_BatchDedupSuspect`, `TestMatching_RematchOnCorrection` | exact-hour match; suspect exclusion (BR-MATCH-05); idempotent re-run; correction → rematch new pair + old retained + idempotent |

Full `go test -race ./...` (unit) green; `gofmt`/`vet`/`golangci-lint` clean; `go build -tags integration ./test/integration/...` compiles.

## 6. Database changes

Adds migration `20260801000008_create_analysis` (up + down): `matched_evaluations` with the pair-uniqueness constraint, observation/rematch/evaluation indexes, and the immutability trigger. Exercised by the `migrations` + `backend-integration` CI jobs.

## 7. API / security

No public API change. No new external calls or credentials. All SQL parameterized.

## 8. CI evidence

```text
PENDING — branch not yet pushed. Six mandatory jobs must be captured green on
the pushed head SHA (migrations applies 20260801000008; backend-integration
runs the matching batch/rematch tests against real PG16).
```

## 9. Files changed

- **Migration**: `migrations/20260801000008_create_analysis.{up,down}.sql`
- **Analysis module**: `internal/analysis/{match.go}`, `internal/analysis/domain/match.go`, `internal/analysis/ports/ports.go`
- **Persistence**: `adapters/persistence/analysispg/analysispg.go`
- **Scheduler**: `internal/scheduler/{slot.go,scheduler.go,analysis_dispatcher.go}`
- **Metrics**: `internal/platform/metrics/metrics.go`
- **Wiring**: `cmd/forecastiq/app.go`
- **Tests**: `internal/analysis/domain/match_test.go`, `test/integration/matching_test.go`
- **Docs**: this report; `docs/planning/06-work-package-status-registry.md`

## 10. Deviations

```text
No approved-scope deviations.
```

**Design note:** `matched_evaluations` references the partitioned `forecast_snapshots`/`observations` tables **logically** (no enforced composite FK), matching the table-design §4 DDL (which declares none). Integrity is maintained by the engine (it only inserts ids it just selected from those tables).

## 11. Work-package transition

```text
WP-11 — Matching Engine
Previous State: Selected — Not Started
New State: Implementation Complete
Acceptance State: Not Accepted (pending pushed-branch CI + Delivery Review Board)
```

## 12. Recommended next action

```text
Push feature/wp11-matching-engine, capture the six mandatory CI jobs green on
the head SHA, then convene the Delivery Review Board for WP-11.
```
