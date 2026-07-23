# ForecastIQ — WP-11 Matching Engine: Delivery Review Board Report

**Version**: 1.0
**Review date**: 2026-07-23
**Work package**: WP-11 — Matching Engine
**PR**: #9 (`feature/wp11-matching-engine` → `main`)
**Reviewed SHA**: `f02f720915faeec83a5988b948bf4dc10a1dd07f` (`f02f720`); code+test tip `674dfddf1bc62005d582ee05eeedb6c4aab510b2` (`674dfdd`)
**Authority**: `docs/planning/05-implementation-work-packages.md` §WP-11; `docs/workflows/03-matching.md`; ADR-014; BR-MATCH-01..06; domain architecture §2.8; `docs/data/03-table-design.md` §4
**Decision**: **ACCEPTED** — no Critical/High/Medium/Low finding.

---

## 1. Scope of this review

First independent Delivery Review Board review of WP-11. Dependency gate satisfied: WP-10 and WP-08 **Accepted**. The board independently re-verified commit identity, CI evidence, scope reconstruction, architecture/security, and ran an adversarial read of the diff `fdbc9b6..f02f720`, focusing on the deterministic candidate selection, the rematch cascade, keyset chunking, and SQL correctness.

## 2. Commit identity + CI evidence (independently verified)

| Check | Evidence | Result |
|-------|----------|--------|
| Local HEAD == remote tip | `git rev-parse HEAD` == `git ls-remote origin …` == `f02f720` | ✅ |
| Code+test lineage | code/test commits `ae6b6ac`/`8584e6b`/`4a710de`/`464f990`/`15ac6a1`/`0a190bc`/`674dfdd`; commit since (`f02f720`) is **docs-only** (`git diff --stat 674dfdd..HEAD` → registry + report) | ✅ |
| CI on the reviewed head | run **30039043834** (`pull_request`, head `f02f720`) **success**, all six mandatory jobs green (none skipped/cancelled) | ✅ |
| CI on the code+test tip | run **30038659743** (head `674dfdd`) **success**, six jobs green | ✅ |
| `migrations` job | applied `20260801000008` (up + verify + seed×2) | ✅ |
| `backend-integration` job | ran the matching batch/rematch tests against real PostgreSQL 16 | ✅ |
| PR mergeable | `MERGEABLE` / `CLEAN` | ✅ |
| `ci.yml` unchanged | not in diff | ✅ |

Local gate re-run on the reviewed tree: `gofmt -l` clean; `go vet` clean; `go test -race ./internal/analysis/...` green (incl. the 500-permutation determinism property); `go build -tags integration ./test/integration/...` compiles; `golangci-lint run` clean. (Docker unavailable in the review environment → the integration suite is validated via CI's green `backend-integration` on the exact SHA.)

**CI-history note:** two earlier `backend-integration` failures (`30037176879` on `55bd8f6`, `30038022889` on `0a190bc`) were **test-harness-only** defects — a missing parent `forecast_collections` FK row, then two live observations at the same hour vs the WP-10 partial dedup index — fixed in `0a190bc`/`674dfdd`. No product code changed for either fix. The board treats these as evidence the real-PG suite genuinely exercises the schema.

## 3. Scope reconstruction (§WP-11)

| # | Approved scope item | Verified | Result |
|---|---------------------|----------|--------|
| S1 | Analysis matching (batch, chunked 5K) | `MatchBatch` keyset-chunked at 5000, per-chunk tx (§7 failure isolation) | ✅ |
| S2 | Candidate selection (provenance rank, corrected preference, tiebreak) | `SelectCandidate` strict total order: corrected → provenance rank → nearest-hour → id | ✅ |
| S3 | Rematch on correction | rematch pass over superseded-observation pairs → new pair (old retained; §5) | ✅ |
| S4 | Backlog gauge | `matching_backlog` from `CountUnmatched` (best-effort) | ✅ |
| Acc | Batch over a seeded month < 2 min; all BR-MATCH tested | chunked design (NFR-P06 headroom); BR-MATCH-01/03/04/05/06 covered by unit + integration | ✅ (perf = design/operational) |

Exclusions respected: no metrics/rankings/API (WP-12+); sub-hourly ±15 min rule dormant behind the source-capability flag (BR-MATCH-02).

## 4. Architecture + security assessment

- **New analysis module, correct dependency direction**: the deterministic selection lives in `analysis/domain` (pure, I/O-free — unit/property-testable); `MatchService` → `ports` + `platform`; the scheduler depends on a small `BatchMatcher` interface it defines; wiring is only in the composition root.
- **Read models, not cross-module domain reuse**: the engine consumes lightweight `SnapshotToMatch`/`ObservationCandidate` read structs, keeping analysis independent of the collection module.
- **Immutability + append-only rematch** (domain §2.8; BR-INV-03): `matched_evaluations` is trigger-immutable; corrections add pairs, never edit.
- **Security**: no external calls or credentials; all SQL parameterized (the multi-row INSERT builds only `$n` placeholders).

## 5. Adversarial review — verified correct

The board's adversarial pass found **no** defects and independently confirmed:

1. **Keyset chunking** — `after` advances to the last chunk id unconditionally (even for zero-match chunks); no infinite loop; unmatched (no-candidate) snapshots retry next batch.
2. **Determinism** — `candidateLess` is a strict total order (irreflexive/asymmetric/transitive) ending in the globally-unique UUID tiebreak; the min-scan equals sort-and-take-first; provenance ties (interpolated=reanalysis=2) resolve by later keys. Backed by the 500-permutation property test.
3. **`time_delta_minutes`** — 0 for exact-hour hourly sources; int truncation cannot arise on hour-aligned timestamps.
4. **Rematch** — `NOT EXISTS` prevents recreating an existing corrected pair; `ON CONFLICT DO NOTHING` bounds cross-batch idempotency; supersession chains resolve one round per batch; no join fan-out (pair uniqueness).
5. **SQL injection** — none (positional params only).
6. **Scheduler** — `analysisSlotTimes` enumerates the window without duplicates; the `COALESCE(location_id, nil-uuid)` unique index dedups the global (NULL-location) slot; no nil-deref in the analysis dispatch path.
7. **Window boundary** — `[now−30d, now−2h)`; the freshest 2h are intentionally deferred (publication margin), picked up next cycle.
8. **Transaction isolation** — per-chunk tx wraps the insert; pool reads are READ COMMITTED; mid-batch supersession is handled by the next batch's rematch; concurrent batches stay idempotent.
9. **Backlog gauge** — best-effort; a count failure logs a warning and does not fail the batch.

## 6. Findings

None. No Critical/High/Medium/Low finding.

## 7. Regression assessment

- WP-05/06/07 forecast adapters + `providerhttp`, WP-09/10 observation pipeline: untouched.
- WP-08 scheduler: extended additively (new `analysis_batch` job + generation); forecast/observation generation unchanged; existing scheduler unit suite green.
- Mandatory CI controls (`ci.yml`): unchanged.

## 8. Completion-gate assessment

| Gate | Result |
|------|--------|
| Exact WP-11 definition located | ✅ |
| Dependencies (WP-10, WP-08) Accepted | ✅ |
| Scope reconstruction complete; exclusions respected | ✅ |
| Architecture boundaries preserved | ✅ |
| Determinism proven (property test) | ✅ |
| Security (no secrets/external calls; parameterized SQL) | ✅ |
| Migration + matching proven vs real PG16 | ✅ (CI `migrations` + `backend-integration`) |
| SHA identity (local == remote == CI head) | ✅ |
| No Critical/High/Medium finding | ✅ |

## 9. Decision

**ACCEPTED.** WP-11 delivers the deterministic matching engine (4/4 scope items + acceptance): a strict-total-order candidate selection (permutation-invariant, property-tested), chunked batch with per-chunk failure isolation, an append-only rematch cascade for corrections, and the `matched_evaluations` migration — all CI-verified green on the exact code+test SHA `674dfdd`, including the real-PG `backend-integration` job. The adversarial review found no defect on any high-risk mechanic.

**Accepted Implementation SHA `f02f720`** (code+test lineage `674dfdd`). PR #9 ready to merge to `main`. **WP-12 (Pair-Level Evaluation)** becomes eligible — it consumes `matched_evaluations` to compute per-pair errors/classification/Brier/weights.

## 10. Tracked conditions

None.
