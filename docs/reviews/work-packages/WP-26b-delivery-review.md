# ForecastIQ — WP-26b Performance Validation Completion: Delivery Review Board

**Review date**: 2026-07-28
**Work package**: WP-26b — Performance Validation Completion (PR #43, `feature/wp26b-performance-validation`)
**Reviewed SHA**: `752e8d4f34ae8b01a7b14700d4ec66cb8a760df8` (`752e8d4`)
**Scope authority**: `docs/planning/05-implementation-work-packages.md` §WP-26b; WP-26 delivery review (DRB-WP26-001/002/006 remainders)
**Decision**: **ACCEPTED** (one tracked operational follow-on: the NFR-S01 2×-volume run)

---

## 1. Evidence verification

| Check | Result |
|-------|--------|
| Commit identity: local HEAD == `git ls-remote origin` branch tip == `refs/pull/43/head` | ✅ all `752e8d4f34ae8b01a7b14700d4ec66cb8a760df8` |
| Six mandatory CI jobs green on the reviewed SHA | ✅ run **30346445627** (`pull_request`, head `752e8d4`): `backend-checks`, `backend-integration`, `migrations`, `api-contract`, `security`, `image` — none skipped/cancelled (`build-release`/`deploy-api` are main-only by design) |
| Frontend workflow (incl. the new PT-8 Lighthouse gate) green on the reviewed SHA | ✅ run **30346445680** |
| Prior SHA `da7b225` also fully green (CI + Frontend) | ✅ runs 30342781008 / 30342780226 |
| Local gate | ✅ gofmt/vet/golangci-lint clean; `go test -race ./...` all ok |

CI history on the PR: the first push (`6afb68c`) had a green six-job CI but a red
Frontend run — the OpenAPI client drift gate caught a **pre-existing `main`
defect** (PR #42 extended `openapi.json` without regenerating
`web/lib/api/generated.ts`; the path-filtered Frontend workflow never ran on
that PR). Fixed by regeneration in `da7b225`, no manual edits.

## 2. Scope coverage (§WP-26b: 9/9 delivered; 1 execution step tracked)

| Scope item | Verdict |
|------------|---------|
| Functional seeder DB writes | ✅ all six data tables, COPY-streamed, deterministic; base preset measured 1,497,600 snapshots / 2,712,518 pairs — matches doc §3 (~1.5 M / ~2× eligible) |
| PT-3 ingestion burst | ✅ harness through the real `CollectService`; run: 6.77 s cycle, 240/240 success, p95 79 ms (< 5 min NFR-P07, < 100 ms) |
| PT-4 analysis batch | ✅ 30 s at ~147 K in-window pairs (< 10 min NFR-P06) |
| PT-7 query baselines | ✅ Q-01/Q-04/Q-05/Q-09 p95 4.5/1.7/5.3/36.6 ms at base volume (< 100 ms NFR-P08) |
| PT-8 Lighthouse | ✅ CI-gated on every frontend PR; key screens FCP 273–353 ms, CLS ≈ 0.025 |
| 5 fault-injection scenarios | ✅ `reliability-faults.sh` 15/15 green from a bare invocation against the isolated `fiqperf` stack — the DRB-WP26-001 remainder is closed |
| Baseline register populated | ✅ perf doc §6, environment disclosed |
| Weekly scheduled wiring | ✅ `weekly-perf-reliability` job; DRB-WP26-005 env exclusion enforced by sequential re-ups; DRB-WP26-006 closed |
| 2× volume load test (NFR-S01) | ◐ capability delivered (`--preset=extended` + PT-7 gate); the run itself needs the scratch-VPS environment perf doc §4 prescribes. Honestly recorded in the report, register, and registry as the remaining **operational** step — not a code gap. Tracked below. |

## 3. Adversarial review + remediation (all closed)

A pre-submission adversarial pass raised 1 High / 2 Medium / 2 Low / 3
Informational; re-exercising the remediated suite raised 1 further High
(product) + scenario-robustness items. All are remediated in `752e8d4` and
recorded in the implementation report §9:

| Finding | Sev | Resolution |
|---------|-----|-----------|
| DRB-WP26b-001 — seeder `--reset` could TRUNCATE whatever ambient `FIQ_DATABASE_URL` pointed at | High | `--reset` requires explicit `--db`; refuses `FIQ_ENV=production`; marker gate (perf location 0, or < 10 K incidental rows/table). All refusal + allow paths verified live |
| DRB-WP26b-006 — **WP-13 kernel product bug**: `eval.Wilson` at p̂=0 emitted lower ≈ +2.8e-17, violating the `accuracy_metrics` CHECK and 500-ing every aggregation tx on data with a p̂=0/1 cell at small n | High | Clamp interval to bracket p̂; property test made strict (old `1e-12` epsilon hid the bug) + exact-bound regression test. Recompute re-verified 200 |
| DRB-WP26b-002 — fault-suite defaults targeted the developer stack | Medium | Defaults now the isolated `fiqperf` project; header corrected |
| DRB-WP26b-003 — `trigger()` timeout read `$2`, callers pass `$1` | Medium | Fixed |
| DRB-WP26b-004 — PT-8 workflow comment claimed auto-discovery | Low | Comment states the four config-pinned key screens |
| DRB-WP26b-005 — "byte-identical" determinism over-claim | Low | Qualified (ids + values; bookkeeping timestamps excluded) |
| Informational: migration-13 lock (sub-second at MVP volume, matches doc §1.5), perf-admin containment (dev verifier build-tag-excluded; prod rejects dev mode), dispatch comment | Info | Accepted / corrected |

The board notes DRB-WP26b-006 and the missing `metrics_calculated` index
(migration `20260801000013`, PT-6 finding) as exactly the defect classes this
work package exists to surface: both were latent product bugs invisible to the
unit/integration tiers and found only by running real load over real volume.

## 4. Verified correct

Enforcing (non-decorative) gates with propagating exit codes at every layer;
threshold values match perf doc §2 unweakened (PT-2 drives 105/s while gating
≥ 100 — strictly harder); seeded data satisfies every dedup index, CHECK
constraint, immutability trigger, and partition boundary; uuid-v5 keyed ids are
collision-safe across kinds; credential redaction preserved (DRB-WP26-003);
insert-only catalog writes; live-stack collision guards (3 h anchor,
`perf_synthetic` source, 2-year-old PT-3 partitions); rate-limiter mutual
exclusion enforced; no repository secrets in workflows; LHCI reports stay on
the filesystem.

## 5. Decision

**ACCEPTED.** SHA identity verified; six mandatory jobs + Frontend green on
`752e8d4`; 9/9 scope items delivered with rehearsed runs producing real
numbers (the WP-26 rejection's core demand); all adversarial findings closed;
two genuine product defects found and fixed with regression coverage. No
Critical/High finding remains open.

**Tracked follow-on (non-blocking, operational)**: **TC-26b-01** — execute
PT-7 at the extended (2×-volume) preset on a scratch VPS per perf doc §4 and
append the row to the §6 register. This is the NFR-S01 Level-1 exit evidence;
the capability, gate, and procedure are delivered and CI-proven at base
volume. Owner: operations; revisit at the Level-1 exit review.

**WP-26b → Accepted. Accepted Implementation SHA `752e8d4`.** PR #43 ready to
merge to `main`.
