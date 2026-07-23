# ForecastIQ — Work-Package Status Registry

**Version**: 1.0
**Status**: Living document — updated at each work-package completion
**Authority**: `docs/planning/05-implementation-work-packages.md` (definitions); delivery reports (evidence)

State model: Not Started → Prototype Exists → Partially Implemented → Implementation Complete → Review Findings Open → Accepted. Side states: Blocked, Deferred.

---

## Registry

| WP | Name | State | Last updated | Evidence / notes |
|----|------|-------|--------------|------------------|
| 01 | Repository + dev env | Accepted (bootstrap) | 2026-07-22 | Repository Bootstrap final report; `make dev-up`, CI green |
| 02 | DB foundation | Accepted (bootstrap) | 2026-07-22 | Migrations 20260801000001..05; integration suite |
| 03 | Identity + workspace | Not Started | 2026-07-22 | Audit recorder seam exists (used by WP-04); JWKS/API keys pending |
| 04 | Location management | **Accepted** | 2026-07-23 | DRB-WP04-001..005 verified **RESOLVED** at re-review (advisory-lock dedup proven vs real PostgreSQL; fp-tolerant boundary; mandatory override reason; restricted status lifecycle; doc corrections). RR-001/RR-002 resolved and **green in CI**. Two deferred mandatory CI gates **RESOLVED** (CI-WP04-001 backend-checks, CI-WP04-002 security). **Confirmatory re-review 2026-07-23: ACCEPTED** — all six mandatory CI jobs green on `15b8faa` (run 29952834878); commit identity, remote SHA, and CI headSha independently verified; no regression. Report: `docs/reviews/work-packages/WP-04-delivery-re-review.md` (§A, §B). |
| 05 | Adapter framework | **Accepted** (confirmatory re-review 2026-07-23; TC-05-01 **Closed — Satisfied**) | 2026-07-23 | Provider-agnostic framework hardened over the existing prototype: structured FC-13 `ProviderError` taxonomy + retry classification; shared `providerhttp` transport (User-Agent, capped redirects, bounded reads, FC-08 retry/backoff, rate-limit header normalization); `Capabilities()` declaration; explicit `collection.Registry` (duplicate/version/replay-capability validation, startup inspection log); `ReplayDecoder.DecodeStored` deterministic replay; Open-Meteo adapter refactored behind the framework with all contract tests green. New framework tests (`providerhttp`, registry, `ProviderError`, capabilities/replay/rate-limit) pass under `-race`; no DB/API change. Guide: `docs/development/08-provider-adapter-authoring-guide.md`. **DRB 2026-07-23: CONDITIONALLY ACCEPTED** — merits satisfied (no Critical/High; 8/8 scope; acceptance criterion proven live on real PG16; full local gate green), conditional on **TC-05-01** (push WP-05 commit `469560b` + capture six mandatory CI jobs green on that SHA). **TC-05-01 SATISFIED 2026-07-23**: branch `review/wp05-ci-evidence` pushed at exactly `469560b` (remote tip == local HEAD == CI headSha == `469560b3fe9eed8bfecf25b190f93d53cb136069`); CI run **29978249699** (PR #2, event `pull_request`) **success** — all six mandatory jobs green on `469560b`, none skipped; no framework-code change. WP-05 → **READY FOR CONFIRMATORY RE-REVIEW**. Reports: `docs/reviews/work-packages/WP-05-delivery-review.md` (§16), `WP-05-findings.md`, `WP-05-traceability.md`. **Not yet Accepted — WP-06 must not be selected.** **Confirmatory re-review 2026-07-23: ACCEPTED** — the board independently re-verified commit identity + ancestry (`9a6023f` parent of `469560b`), the real remote (`git ls-remote origin` → `refs/heads/review/wp05-ci-evidence` == `refs/pull/2/head` == `469560b3fe9eed8bfecf25b190f93d53cb136069`, == local), and CI run **29978249699** (headSha `469560b`, event `pull_request`) with all six mandatory jobs green (none skipped/cancelled); no post-review framework change (`311a4a7` is docs-only). **TC-05-01 → Closed — Satisfied; WP-05 → Accepted; WP-06 may now be selected** (report §17). TC-05-02/03 (Low) remain Open, non-blocking. |
| 06 | First provider (Open-Meteo) | **Accepted** (confirmatory re-review 2026-07-23; TC-06-01 **Closed — Satisfied**) | 2026-07-23 | Contract-test matrix (`docs/testing/03-contract-testing.md` §1.2) completed over the WP-05 prototype: +2 recorded fixtures (`forecast_localtime_offset`, `forecast_majority_invalid`) and +4 contract tests (BR-PROV-01 timezone conversion; attribution-field capture; >50%-invalid → `failed`/`schema_drift`; auth-failure no-retry) → 10/10 matrix rows green under `-race`. Live-collection acceptance documented (manual smoke procedure + captured-payload evidence under `data/payloads/open-meteo/2026/07/22/`). No production-adapter, DB, API, or CI change (tests/fixtures/docs only). Branch `feature/wp06-open-meteo-provider`. Report: `docs/reviews/work-packages/WP-06-implementation.md`. **DRB 2026-07-23: CONDITIONALLY ACCEPTED** — accepted on technical/architectural merits; sole tracked condition **TC-06-01** (synchronise final-SHA CI evidence in implementation report §15). **TC-06-01 SATISFIED by delivery team 2026-07-23, pending DRB confirmation**: §15 now cites final reviewed commit `da04fe4` (local HEAD == remote tip == CI head SHA == `da04fe49e4daed358e50d06d401c41f781e9e809`) and CI run **29984650522** (PR #3, event `pull_request`) **success** — all six mandatory jobs (`backend-checks`, `backend-integration`, `migrations`, `api-contract`, `security`, `image`) green on `da04fe4`, none skipped/cancelled; no code/test/fixture/CI change. Optional DRB-WP06-002 addressed: test count corrected to the verified 15 discovered top-level tests (`go test -list`). **Confirmatory re-review 2026-07-23: ACCEPTED** — the board independently re-verified commit identity (`da04fe4` preserved in remote branch history), the real remote (`git ls-remote origin` → `refs/heads/feature/wp06-open-meteo-provider` == local HEAD == `af53c5c181b0ff842490cfb29d55f50c69d4559e`, a documentation-only descendant of `da04fe4`), the docs-only closure diff (`da04fe4..af53c5c` touches only this registry + the implementation report), and CI run **29984650522** (headSha `da04fe4`, event `pull_request`) with all six mandatory jobs green (none skipped/cancelled); §15 synchronized to the final SHA and the `(see below)` reference resolved; the TC-06-01 closure commit `af53c5c` is now pushed (durable persistence confirmed); no CI run was triggered for the docs commit, so run `29984650522` on `da04fe4` remains the authoritative implementation CI evidence. No new Critical/High finding. **TC-06-01 → Closed — Satisfied; WP-06 → Accepted; WP-07 may now be selected.** |
| 07 | Second provider (OpenWeather) | Not Started | 2026-07-22 | |
| 08 | Scheduler + collection ops | **Implementation Complete** — Not Accepted | 2026-07-23 | **Selected then implemented 2026-07-23.** Hardened the WP-01-era prototype (bootstrap `4f24fa3`) into the full WP-08 feature set on branch `feature/wp08-scheduler-collection-ops` (base: accepted WP-06 tip `9eda963`): (1) **watchdog/missed-slot detection** — `scheduler.stalled` warning when claimable slots make no progress for 2× interval, plus per-slot `scheduler_missed_slots_total` on late claims; (2) **`scheduler_lag_seconds{job_type}`** histogram completing the workflow 05 §10 metric catalog; (3) **graceful drain** — in-flight jobs run under a loop-detached, per-job-timeout context and drain within a bounded deadline; `serve` waits for the worker before pool close; (4) **raw-payload replay (FC-14)** — `POST /admin/collections/{id}/replay` (load → SHA-256 verify → quarantine-on-mismatch → decode via current adapter → new dedup'd collection; idempotent, originals immutable); (5) **manual-trigger rate guard** — rate-limited outcome → 429 + Retry-After (circuit-open remains 409). New config: `FIQ_WORKER_JOB_TIMEOUT`, `FIQ_SCHEDULER_DRAIN_TIMEOUT`, `FIQ_SCHEDULER_MISSED_THRESHOLD`. OpenAPI +1 path (replay). Local gate green (gofmt, `go vet`, `go test -race ./...`, `golangci-lint`, `make docs`); integration suite (`TestLeaseExpiryReclaim`, `TestSchedulerRunCollectsAndDrains`, `TestSchedulerNoDoubleCollection`, `TestReplayIdempotency`, `TestReplayChecksumMismatchQuarantine`, `TestReplayUnknownCollection`, `TestAPI_ReplayCollection`, `TestAPI_TriggerRateLimited429`) authored — **CI to confirm** (no local Docker). Report: `docs/reviews/work-packages/WP-08-implementation.md`. **Not yet Accepted — Delivery Review Board pending; branch push + six-job CI evidence still required.** |
| 09–27 | (remaining) | Not Started | 2026-07-22 | |

---

## Recorded discrepancies

Documentation-vs-documentation conflicts discovered during implementation, with resolutions (master prompt: "record the discrepancy and resolve it before materially extending the affected behaviour").

| # | Discrepancy | Resolution | Affected docs | Packages impacted |
|---|-------------|------------|---------------|-------------------|
| DR-01 | Location timezone mutability: domain architecture §2.3 lists timezone as **immutable** (Mutable: name, status, updated_at); `docs/api/00-api-requirements.md` §4.1 listed PUT as "(name, timezone, status)"; UI design spec edit form includes timezone; security matrix lists "name/timezone/status" | Domain architecture authoritative (Phase 1 aggregate design; timezone derives from immutable coordinates; mutation would re-bucket historical display data). Implementation: PUT accepts name only. API requirements doc corrected 2026-07-22. | `docs/api/00-api-requirements.md` (corrected); `docs/ui/02-ui-design-specification.md` §S-12 (edit form should render timezone read-only — flagged for WP-21); `docs/security/01-ui-authorization-matrix.md` S-12 row (flagged for WP-19 review) | WP-21 (UI form), WP-19 (matrix) |
| DR-02 | Contract-testing doc §1.1 illustrative fixture name `forecast_success_v1_edge.json` differs from the committed `forecast_edge_nulls.json` | §1.2 matrix uses generic labels; committed names authoritative (existing tests reference them). No rename performed; recorded for documentation consistency. Discovered during WP-06. | `docs/testing/03-contract-testing.md` §1.1 (illustrative only) | None (documentation-only) |

---

## WP-04 Delivery Review Board outcome (2026-07-22)

Decision: **CHANGES REQUIRED** (report: `docs/reviews/work-packages/WP-04-delivery-review.md`).

| Finding | Severity | Summary | Remediation target |
|---------|----------|---------|--------------------|
| DRB-WP04-001 | High | BR-LOC-01 dedup race: concurrent creates bypass proximity check (reproduced: 6 parallel POSTs → 2 rows); no concurrency test | Serialize create tx (advisory lock or SERIALIZABLE+retry) + concurrency integration test |
| DRB-WP04-002 | Medium | Exact-0.05° boundary fp-fragile (live pair rejected at 0.04999999999999716°) | Epsilon/precision-tolerant comparison + multi-coordinate boundary tests |
| DRB-WP04-003 | Medium | `override_reason` not mandatory when `allow_near_duplicate` set | 422 on empty reason + tests |
| DRB-WP04-004 | Medium | Reserved `archived` settable via PATCH; no transition validation | Restrict to active\|disabled (or ADR) + transition tests |
| DRB-WP04-005 | Medium | `docs/api/05-endpoint-catalog.md` still lists PUT timezone; README endpoint list stale | Doc corrections |
| DRB-WP04-006 | Low | Repository UPDATE writes immutable timezone column | Remove column from statement |
| DRB-WP04-007 | Low | Malformed `active` query param silently coerced | 422 on invalid boolean |

Tracked conditions: TC-01 Idempotency-Key (→ WP-15, this registry's deferral accepted); TC-02 optimistic concurrency (deferred); TC-03 dev-token seam replacement (WP-03/19) + DR-01 UI/matrix follow-ups (WP-21/19); TC-04 CI green on pushed branch + testcontainers run (review env had no Docker).

## WP-04 Delivery Review Board **Re-Review** outcome (2026-07-23)

Decision: **BLOCKED** (report: `docs/reviews/work-packages/WP-04-delivery-re-review.md`). All five original findings independently verified **RESOLVED** (High concurrency finding proven fixed against real PostgreSQL 16; four Medium findings closed with executed unit + integration tests and matching OpenAPI/docs). No remediation-caused Critical/High regression. Blocked purely on evidence:

- **TC-04 NOT SATISFIED / BLOCKED BY EXTERNAL ACCESS** — `git ls-remote origin` fails authentication; no distinct remediation branch (all on `main`); local `origin/main` moved to HEAD unverifiably; GitHub Actions results not inspectable. Local checks do not substitute for pushed-branch CI.
- **`make test-integration` red** — DRB-WP04-RR-001 (Medium, **pre-existing** test-assertion bug: string `"true"` vs boolean `true` at `location_test.go:68`; production correct) + DRB-WP04-RR-002 (Low, flaky per-test containers).

Next action (implementation team): push the branch with working credentials and capture CI for `fc72f08`; fix RR-001 and stabilise the suite; then re-convene for a short confirmatory re-review. **WP-05 must not be selected until WP-04 is Accepted.**

## WP-04 team final remediation (2026-07-23)

Status: **PARTIALLY COMPLETE** (report addendum: `docs/reviews/work-packages/WP-04-delivery-re-review.md` §A1–A6). The implementation team addressed the two evidence blockers within strict scope (test-only + test-harness; no production, migration, OpenAPI, or CI changes):

- **DRB-WP04-RR-001 — RESOLVED (test-only).** `test/integration/location_test.go` now type-asserts `allow_near_duplicate` as a JSON boolean (`bool` + `true`) instead of the string `"true"`. Production audit payload unchanged. Commit `bf694a0`.
- **DRB-WP04-RR-002 — RESOLVED (test-harness).** Single package-wide PostgreSQL container via `TestMain`; each test gets a fresh migrated database (`it_<pid>_<counter>`) dropped `WITH (FORCE)`. Per-test container churn eliminated; isolation preserved; no retries/skips/weakened assertions. 5 consecutive local green runs (~40 s → ~7 s). Commit `b7f2479`.
- **TC-04 — PARTIALLY SATISFIED.** Branch `fix/wp04-final-review` (base `33ad0ab`, tip `1fa9105`) pushed via SSH; local == remote SHA verified; PR #1 triggered CI run `29945014559` (`pull_request`, headSha `1fa9105`). **`backend-integration` went red→green** (failure on `main` `fc72f08` → success on `1fa9105`), verifying RR-001+RR-002 in CI. `api-contract`, `migrations`, `image` also green.
- **Out of scope (deferred, separate work item).** Two mandatory but **pre-existing, unrelated** CI jobs remain red on the branch (they fail on `main` independently of WP-04): `backend-checks` (`govulncheck` flags Go 1.23.x stdlib CVEs GO-2025-4007/4008 + `golang.org/x/net@v0.25.0` GO-2025-3595) and `security` (`gitleaks` HTTP 403 `Resource not accessible by integration` — a `pull_request`-event token-permission quirk, not a detected secret). By explicit scope decision these were left unchanged; their remediation (Go toolchain/dependency bump; CI `permissions:` block) is a separate task.

Next action: raise the two pre-existing CI failures as a separate maintenance task, then re-convene the board for a short confirmatory re-review. **Only the Delivery Review Board may mark WP-04 Accepted. WP-05 must not be selected until WP-04 is Accepted.**

## WP-04 mandatory CI gate remediation (2026-07-23)

Status: **READY FOR CONFIRMATORY RE-REVIEW** (report addendum: `docs/reviews/work-packages/WP-04-delivery-re-review.md` §B). The two mandatory but pre-existing/unrelated CI jobs deferred by the team final remediation were fixed within strict scope (build/dependency + CI config only; no WP-04 production, migration, or OpenAPI changes):

- **CI-WP04-001 — `backend-checks` RESOLVED (code/dependency defect).** `govulncheck` flagged real Go 1.23.x stdlib CVEs plus outdated deps. Fixed by moving the module to `go 1.25.0` + `toolchain go1.25.12`, upgrading `pgx/v5 v5.6.0→v5.9.2`, `x/text v0.15.0→v0.39.0`, `x/net v0.25.0→v0.56.0` (`go mod tidy`), bumping the `Dockerfile` base to `golang:1.25-alpine`, and updating `ci.yml` (`go-version 1.25.12`; `govulncheck@v1.6.0`; `golangci-lint install-mode: goinstall` for v1.64.8-on-Go-1.25). No jobs disabled, no suppressions. One **uncalled** advisory (`GO-2026-5932`, `x/crypto@v0.53.0`, no fix) remains informational and does not fail the gate. Commit `542c808`.
- **CI-WP04-002 — `security` RESOLVED (CI-config defect).** `gitleaks-action@v2` needs `pull-requests: read` to list PR commits on `pull_request` events; the default token (`contents: read` only) caused HTTP 403 `Resource not accessible by integration` — not a detected secret. Fixed with a least-privilege job-scoped `permissions:` block (`contents: read` + `pull-requests: read`). Detection unchanged. Commit `b277fba`.
- **CI evidence.** Run `29952013546` (`pull_request`, headSha `b277fba`): **all six mandatory jobs green** (`backend-checks`, `security`, `backend-integration`, `migrations`, `api-contract`, `image`). Prior baseline `701a0ed` run `29946041618` had `backend-checks`+`security` red. <https://github.com/od3n/forecastiq/actions/runs/29952013546>

Next action: re-convene the Delivery Review Board for the short confirmatory re-review. **Only the board may mark WP-04 Accepted. WP-05 must not be selected until WP-04 is Accepted.**

## WP-04 Delivery Review Board confirmatory re-review outcome (2026-07-23)

Decision: **ACCEPTED** (report: `docs/reviews/work-packages/WP-04-delivery-re-review.md`). The board independently verified: commit `15b8faa` (local HEAD == `git ls-remote origin` == CI headSha); CI run **29952834878** (`pull_request`, headSha `15b8faa`) **completed success** with all six mandatory jobs green (`backend-checks`, `api-contract`, `image`, `backend-integration`, `migrations`, `security`) — no required job skipped or cancelled (all jobs unconditional/blocking in `ci.yml`). DRB-WP04-001..005 remain **RESOLVED** (remediation code artifacts present in-tree; no regression); RR-001/RR-002 remain **RESOLVED**; TC-04 **SATISFIED** (branch pushed, remote SHA verified, CI evidence, commit identity confirmed). `15b8faa` is documentation-only over the code-fix tip `b277fba`, so the CI surface is unchanged. Documentation is consistent; local build + catalog unit tests pass. No new Critical/High issue. **WP-04 dependency contracts (LocationManager, active-location queries) are stable and hardened — WP-05 may now depend on WP-04.**

**Package transition:** READY FOR CONFIRMATORY RE-REVIEW → **Accepted**. WP-05 may be selected for implementation (not started by the board).

## WP-05 Delivery Review Board outcome (2026-07-23)

Decision: **CONDITIONALLY ACCEPTED** (report: `docs/reviews/work-packages/WP-05-delivery-review.md`; findings: `WP-05-findings.md`; matrices: `WP-05-traceability.md`). First independent review of WP-05. Dependency gate satisfied (WP-04 **Accepted**). Reviewed commit `469560b` on the accepted WP-04 tip `15b8faa` (via `9a6023f`).

**Merits (accepted):** 8/8 approved scope items implemented; provider-agnostic framework (shared `providerhttp` transport, FC-13 `ProviderError` taxonomy, `Capabilities()`, explicit `Registry` with startup validation + `provider.registered` log, deterministic `DecodeStored` replay, Open-Meteo refactored behind the framework) is architecturally clean with correct dependency direction and no DB/API/migration change. The board **executed the full quality gate locally with Docker present**: `go build`, `go vet`, `golangci-lint` (zero findings), `go test -race` (all packages green), and the testcontainers integration suite against **real PostgreSQL 16 (green, 6.4 s)** — including `TestCollectionIdempotency`, which proves the WP-05 acceptance criterion (full pipeline green with a fake provider; idempotent re-execution). **No Critical or High finding.**

**Findings:**

| Finding | Severity | Summary | Condition |
|---------|----------|---------|-----------|
| DRB-WP05-001 | Medium (evidence) | WP-05 commit `469560b` unpushed; no pushed-branch CI on the WP-05 SHA (mandatory gate) | TC-05-01 (blocking) |
| DRB-WP05-002 | Low | Pipeline-level non-success classification (partial/failed/timeout/rate_limited) not driven through `CollectService.Collect` | TC-05-02 |
| DRB-WP05-003 | Low | `ports.VerifyChecksum` has no direct unit test | TC-05-03 |

Tracked conditions: **TC-05-01 (blocking)** — push `469560b` (and `9a6023f`) and capture all six mandatory CI jobs green on that exact SHA; on satisfaction this converts to a clean **ACCEPTED** with no framework code change. TC-05-02/TC-05-03 opportunistic (non-blocking).

Next action (implementation team): run a WP-05 remediation pass for the identified findings (satisfy TC-05-01; optionally close the two Lows), then re-convene the board for a short confirmatory re-review. **Only the Delivery Review Board may mark WP-05 Accepted. WP-06 must not be selected until WP-05 is Accepted.**

**TC-05-01 closure (2026-07-23, closure team).** Branch `review/wp05-ci-evidence` pushed at exactly `469560b` (remote tip == local HEAD == CI headSha == `469560b3fe9eed8bfecf25b190f93d53cb136069`; parent `9a6023f` contained). CI run **29978249699** (workflow `CI`, event `pull_request`, PR #2 → `main`) completed **success** with all six mandatory jobs green on `469560b`: `backend-checks`, `backend-integration`, `migrations`, `api-contract`, `security`, `image` (none skipped/neutral/cancelled). **No framework-code change.** DRB-WP05-001 **Resolved**; **TC-05-01 SATISFIED**. WP-05 transitions to **READY FOR CONFIRMATORY RE-REVIEW** (not Accepted — DRB transition only). TC-05-02 / TC-05-03 (Low) remain Open.

**Confirmatory re-review (2026-07-23, Delivery Review Board).** Decision: **ACCEPTED** (report: `docs/reviews/work-packages/WP-05-delivery-review.md` §17). The board independently re-verified: both commits exist and `9a6023f` is the parent of `469560b`; `git ls-remote origin` shows `refs/heads/review/wp05-ci-evidence` == `refs/pull/2/head` == local == `469560b3fe9eed8bfecf25b190f93d53cb136069` (history preserved); CI run **29978249699** (`pull_request`, headSha `469560b`) **success** with all six mandatory jobs green (`backend-checks`, `backend-integration`, `migrations`, `api-contract`, `security`, `image`) — none skipped/cancelled, no `continue-on-error`; the only post-review commit (`311a4a7`) is documentation-only and off the reviewed branch, so the CI-tested framework commit remains exactly `469560b`. No new Critical/High finding. **TC-05-01 → Closed — Satisfied; DRB-WP05-001 → Closed; WP-05 → Accepted.** WP-06 may now be selected in a separate action. TC-05-02 / TC-05-03 (Low) remain Open, non-blocking.

## Deferred items recorded during WP-04

| Item | Rationale | Revisit |
|------|-----------|---------|
| Idempotency-Key infrastructure for POST /locations | No `idempotency_keys` table in the approved 18-table schema; cross-cutting concern spanning all mutable POSTs; WP-04 acceptance (BR-LOC-01..03) does not require it; snapshot/collection dedup makes re-execution harmless for the pipeline | WP-15 or a dedicated cross-cutting package |
| Optimistic concurrency on location update | Master prompt conditions it on "if approved"; no ADR or domain-architecture section approves it; single-operator MVP with DB unique constraints suffices | If multi-operator admin emerges |
