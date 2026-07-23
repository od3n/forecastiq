# ForecastIQ — WP-10 Observation Collection: Delivery Review Board Report

**Version**: 1.0
**Review date**: 2026-07-23
**Work package**: WP-10 — Observation Collection
**PR**: #8 (`feature/wp10-observation-collection` → `main`)
**Reviewed SHA**: `0f16adce96e64a606c3e04d3da7e84ee0f91891a` (`0f16adc`); code tip `b42e937800ff61a0c3f0d2d91d39ba54ed210b95` (`b42e937`)
**Authority**: `docs/planning/05-implementation-work-packages.md` §WP-10; `docs/workflows/02-observation-collection.md`; ADR-025, ADR-005; domain architecture §2.7; OC-01/OC-03/OC-04; `docs/data/03-table-design.md` §3
**Decision**: **ACCEPTED** — no Critical/High/Medium/Low finding.

---

## 1. Scope of this review

First independent Delivery Review Board review of WP-10. Dependency gate satisfied: WP-09 and WP-08 **Accepted**. The board independently re-verified commit identity, CI evidence, scope reconstruction, architecture/security, and ran an adversarial read of the diff `9900814..0f16adc` with particular attention to the correction cascade, the partial dedup index, the supersession trigger, and transaction ordering.

## 2. Commit identity + CI evidence (independently verified)

| Check | Evidence | Result |
|-------|----------|--------|
| Local HEAD == remote tip | `git rev-parse HEAD` == `git ls-remote origin …` == `0f16adc` | ✅ |
| Code-bearing lineage | code commits `74f4d9b`/`fe01fe8`/`661bf10`/`4e07ea3`; commits since (`b42e937`, `0f16adc`) are **docs-only** (`git diff --stat b42e937..HEAD` → registry + report) | ✅ |
| CI on the code tip | run **30031945500** (`pull_request`, head `b42e937`) **success**, all six mandatory jobs green (none skipped/cancelled) | ✅ |
| `migrations` job | applied `20260801000007` (up + verify + seed×2) | ✅ |
| `backend-integration` job | ran the observation dedup/correction/suspect tests against real PostgreSQL 16 | ✅ |
| PR mergeable | `MERGEABLE` (mergeState `UNSTABLE` only because the docs-commit CI run was still in progress at review time; the authoritative code-tip run is green) | ✅ |
| `ci.yml` unchanged | not in diff | ✅ |

Local gate re-run on the reviewed tree: `gofmt -l` clean; `go vet` clean; `go test -race ./internal/... ./adapters/...` green; `go build -tags integration ./test/integration/...` compiles; `golangci-lint run ./...` clean. (Docker was unavailable in the review environment, so the integration suite is validated via CI's green `backend-integration` on the exact code-tip SHA.)

## 3. Scope reconstruction (§WP-10)

| # | Approved scope item | Verified | Result |
|---|---------------------|----------|--------|
| S1 | Observation slots (:05) | `generateSlots` emits `observation_collection` slots per active location at the `:05` offset, owned by the seeded Open-Meteo config (`job_type` discriminates; uniqueness index includes `job_type`) | ✅ |
| S2 | Storage tx + `observation.collected`/`observation.corrected` events | `ObserveService` single-tx store + post-commit events (shapes frozen in `platform/events`) | ✅ |
| S3 | Supersession update (the one permitted mutation) | `observationpg.Supersede` narrow UPDATE + DB trigger enforcing supersede-only | ✅ |
| S4 | Freshness gauge | `observation_freshness_age_seconds{location_id}` from newest observed_at | ✅ |
| Acc | 48 h unattended collection | scheduler wired; invariants proven by the real-PG dedup/correction/suspect integration tests | ✅ (soak = operational) |

Exclusions respected: no matching/analysis (WP-11+); no `/observations` API (WP-15+). The `observations` migration + enums land here (explicitly deferred by `20260801000001`).

## 4. Architecture + security assessment

- **Dependency direction correct**: `ObserveService` (collection module) → `ports`/`platform`; the scheduler defines its own small `ObservationCollector` interface; wiring is only in the composition root. The shared `providerhttp` transport is reused unchanged.
- **Security (verified)**: keyless source (no credential); base URL is seeded config (no SSRF); no payloads/URLs logged; **no raw-payload storage** (ADR-025). All SQL is parameterized (the multi-row INSERT builds only `$n` placeholders).
- **Domain §2.7 invariants** enforced at the database: supersession-only trigger blocks DELETE, re-supersede, and any non-`superseded_observation_id` change; the partial dedup index guarantees one live row per `(source, location, hour)`.

## 5. Adversarial review — verified correct

The board's adversarial pass (targeting the highest-risk mechanics) found **no** defects and confirmed:

1. **Supersede-before-insert ordering** — the tx supersedes each old row before `InsertBatch`, so the corrected row never collides with the live row it replaces on the partial index; a new-hour row and a corrected row in the same batch are both safe.
2. **`ON CONFLICT (…) WHERE superseded_observation_id IS NULL DO NOTHING`** — valid PostgreSQL; the predicate matches the partial index exactly, so it is correctly inferred as the arbiter.
3. **Supersession trigger** — blocks DELETE and re-supersede; the legitimate single-column UPDATE passes every `IS DISTINCT FROM` guard; no `DEFAULT now()`/coercion false-positive.
4. **Concurrency** — under READ COMMITTED, a competing supersede blocks then re-evaluates the `WHERE … IS NULL` predicate → 0 rows → clean rollback/retry; `ON CONFLICT DO NOTHING` keeps concurrent inserts idempotent; SKIP-LOCKED slots prevent same-slot double-run.
5. **Partition coverage** — `observationMonthStarts` derives months from all fetched rows, so a window spanning a month boundary ensures both partitions.
6. **Window semantics** — dispatcher `[truncate(hour)-2h, truncate(hour)]`, adapter `start_hour`/`end_hour` (inclusive), and `ListCurrentByWindow` `BETWEEN` all agree — no off-by-one.
7. **Forecast generation unaffected** — the early-return change (`len(locations)==0`) leaves the forecast loop a no-op when configs is empty; observation generation is independent.
8. **Freshness gauge from fetched rows** — intentional and correct (reflects source data availability, deduped rows included).

## 6. Findings

None. No Critical/High/Medium/Low finding. DR-05 (the table-design plain-index bug) was discovered and correctly resolved in-package (partial index in migration + doc correction), and DRB-WP09-001 was folded in (inclusive window comment) — both verified.

## 7. Regression assessment

- WP-05/06/07 forecast adapters + `providerhttp`: untouched.
- WP-08 scheduler: extended additively (new job type + Router + observation slot generation); forecast slot generation behavior unchanged (verified). Existing scheduler unit suite green.
- Existing `internal/collection` (forecast) + persistence suites: unchanged and green.
- Mandatory CI controls (`ci.yml`): unchanged.

## 8. Completion-gate assessment

| Gate | Result |
|------|--------|
| Exact WP-10 definition located | ✅ |
| Dependencies (WP-09, WP-08) Accepted | ✅ |
| Scope reconstruction complete; exclusions respected | ✅ |
| Architecture boundaries preserved | ✅ |
| Security (keyless; no payload/secret/URL logging; parameterized SQL) | ✅ |
| Migration correctness (partial index, trigger) proven vs real PG16 | ✅ (CI `migrations` + `backend-integration`) |
| Tests map to behaviour; unit green under `-race`; integration green in CI | ✅ |
| SHA identity (local == remote == CI head) | ✅ |
| No Critical/High/Medium finding | ✅ |

## 9. Decision

**ACCEPTED.** WP-10 delivers the full observation-collection pipeline (4/4 scope items + acceptance), is architecturally clean, keyless and payload-free (ADR-025), and CI-verified green on the exact code-tip SHA `b42e937` — including the `migrations` and real-PG `backend-integration` jobs that exercise the new migration, partial dedup index, supersession trigger, and correction cascade end-to-end. The adversarial review found no defect on any of the high-risk mechanics. DR-05 resolved; DRB-WP09-001 closed.

**Accepted Implementation SHA `0f16adc`** (code lineage `b42e937`). PR #8 ready to merge to `main`. With WP-10 accepted, the **Provider-2 + observations** phase is complete; **WP-11 (Matching Engine)** becomes eligible — it consumes `forecast_snapshots` + `observations` and the `observation.corrected` event this package emits.

## 10. Tracked conditions

None.
