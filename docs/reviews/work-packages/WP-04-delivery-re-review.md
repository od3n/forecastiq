# ForecastIQ — WP-04 Location Management: Delivery Review Board **Re-Review** Report

**Version**: 1.0
**Re-review date**: 2026-07-23
**Work package**: WP-04 — Location Management
**Previous decision**: CHANGES REQUIRED (`WP-04-delivery-review.md`, 2026-07-22)
**Remediation range reviewed**: `5445435` (original WP-04 tip) → `fc72f08` (HEAD), i.e. `d5f3f9b`, `e7dab5a`, `366509b`, `fc72f08`
**Companion documents**: `WP-04-findings.md` (finding cards, statuses updated), `WP-04-traceability.md` (matrices)

> Scope discipline: this is a **re-review only**. No fixes were implemented, no production code was modified, WP-05 was not started, and no new product requirements were introduced. Only review/documentation artifacts were written.

---

## 1. Executive summary

- **Package re-reviewed**: WP-04 — Location Management.
- **Previous decision**: CHANGES REQUIRED (one High finding open: DRB-WP04-001).
- **Remediation scope**: DRB-WP04-001..005, plus companion review docs and a risk-register row. No unrelated feature work; no schema changes; change set stays within remediation scope.
- **Overall outcome**: **All five original findings (DRB-WP04-001..005) are independently verified RESOLVED**, with meaningful tests — including a **real-PostgreSQL concurrency proof** for the High finding. No remediation-caused Critical/High regression was found. **However, the tracked condition TC-04 (push the remediation branch and obtain CI evidence) cannot be satisfied or verified in this environment**, and the full `make test-integration` gate is currently **red** due to a **pre-existing** (not remediation-caused) test-assertion bug plus environmental flakiness.
- **Final decision**: **BLOCKED**.

The code remediation is correct and complete. The block is on **evidence**, not on the fixes: TC-04 explicitly requires pushed-branch CI, the git remote rejects authentication (`git ls-remote origin` → "Authentication failed"), there is no distinct remediation branch, and the local `origin/main` tracking ref was moved to HEAD mid-session in a way inconsistent with the session's initial `git status` and unverifiable against the real remote. Per the board's own decision rules ("If required CI evidence is unavailable … BLOCKED"; "Do not treat local checks as full satisfaction of TC-04 when pushed-branch CI was required"), the package cannot move to Accepted until (a) real pushed-branch CI evidence exists and (b) the WP-04 integration suite runs green.

## 2. Re-review scope

- **Original findings reviewed**: DRB-WP04-001, -002, -003, -004, -005 (acceptance contract). Low findings DRB-WP04-006/-007 assessed for closure status only (non-blocking).
- **Commits inspected**: `d5f3f9b` (boundary), `e7dab5a` (create hardening + status lifecycle + advisory lock + error mapping + tests), `366509b` (doc corrections), `fc72f08` (review docs + risk row). Base: `5445435`.
- **Files reviewed**: `internal/catalog/domain/{location.go, status.go, errors.go, location_test.go}`, `internal/catalog/{location_service.go, location_service_test.go}`, `internal/catalog/ports/repositories.go`, `adapters/persistence/catalogpg/location.go`, `internal/api/respond/errors.go`, `internal/api/handlers/location.go`, `api/openapi/openapi.json`, `test/integration/location_test.go`, `docs/api/05-endpoint-catalog.md`, `README.md`, `docs/risk/02-phase-1-risk-register.md`, `docs/planning/06-work-package-status-registry.md`.
- **Commands run**: `go build ./...`, `go vet ./...`, `go test -race -count=1 ./...`, `golangci-lint run ./...`, focused unit tests, focused WP-04 remediation **integration** tests (testcontainers, real PostgreSQL 16), full integration suite (×2), `git ls-remote origin`, `git rev-parse`, scope diff analysis. Full table in §10.
- **Limitations**: The git remote rejects authentication in this environment, so pushed-branch state and GitHub Actions results are **not inspectable** — TC-04 cannot be independently verified. Full-integration-suite runs exhibit environmental flakiness (each test provisions its own PostgreSQL container; Docker Desktop shows 7.8 GB).

## 3. Change-set assessment

The remediation remained **within scope**. `git diff --stat 5445435..HEAD`: 19 files, +848/-12. Production changes are confined to the catalog module (domain, service, port, pg adapter), the shared error classifier, one handler godoc, and the OpenAPI spec — every one traceable to a specific finding. Documentation changes are the endpoint catalog, README, risk register, status registry, and the three review artifacts. No unrelated feature additions, no package restructuring, no future-package (WP-05) work, no rewritten migrations, no `go.mod` change, no generated-file churn. Classification: all changes are **necessary remediation dependency** or **harmless incidental** (review docs). **No scope expansion, no architectural drift.**

## 4. Finding-by-finding results

### DRB-WP04-001 — BR-LOC-01 dedup not concurrency-safe (TOCTOU) — **High**

- **Original issue**: `CreateLocation` did `ListActive` → haversine scan → `Insert` in a READ COMMITTED tx with no serialization; 6 concurrent identical-coordinate POSTs produced 2 rows (reproduced). No concurrency test existed.
- **Original acceptance condition**: concurrency test green in CI; live re-verification shows a single row for the reproduction.
- **Remediation**: a new port method `LocationRepository.AcquireDedupLock` takes `pg_advisory_xact_lock($1)` (fixed key `0x4C4F43_44454450`) at the **start of the create transaction**, before `ListActive`; released automatically on commit/rollback. Wired in `CreateLocation`.
- **Root-cause assessment**: addresses the **real cause** (the unserialized check-then-insert window), not a symptom. Transaction-scoped advisory lock is an appropriate, minimal MVP-grade serialization for admin write traffic; no hidden fallback bypasses it (the lock is unconditional on the create path).
- **Code-change assessment**: complete, minimal, preserves existing behaviour; the fake repo implements the method as a no-op so unit tests still exercise the service logic. No brittle workaround.
- **Test assessment**: `TestAPI_ConcurrentDuplicateCreates` (integration, real PostgreSQL) launches 6 concurrent POSTs at identical coordinates and asserts **exactly one 201, five 409s, and exactly one persisted row** (`SELECT count(*) … WHERE latitude/longitude`). **Executed and PASSED** (7.49 s) — this is the acceptance condition met against a real database, which the original review could not run (no Docker then).
- **Result**: **RESOLVED**.

### DRB-WP04-002 — 0.05° dedup boundary floating-point fragile — **Medium**

- **Original issue**: strict `dist < 0.05` rejected a mathematically exact 0.05° pair (20.001→20.051) at `0.04999999999999716°`; passing test used a single favorable pair.
- **Original acceptance condition**: multi-coordinate boundary table test green; the (20.001→20.051) pair permitted.
- **Remediation**: `dedupTolerance = 1e-9` guard band; `IsNearDuplicate` returns `dist < DedupThresholdDegrees - dedupTolerance`. Documented precision (≈0.0001 mm at equator).
- **Root-cause assessment**: correctly targets IEEE-754 rounding at the threshold; epsilon direction is correct (shrinks the "duplicate" zone so exact-0.05° pairs are permitted).
- **Code-change assessment**: minimal one-line comparison change plus a documented constant. No behavioural change for clearly-inside (0.049) or clearly-outside (0.051) cases.
- **Test assessment**: `TestIsNearDuplicate_BoundaryTable` covers equator/mid-lat/high-lat, meridional **and** zonal offsets, the exact DRB pair (permitted), 0.049 (rejected), 0.051 (permitted). **Executed and PASSED** — including the previously-failing DRB pair.
- **Result**: **RESOLVED**.

### DRB-WP04-003 — Override reason not mandatory when `allow_near_duplicate` set — **Medium**

- **Original issue**: override accepted with empty reason (201), audit row recorded `override_reason: ""` — unaccountable bypass.
- **Original acceptance condition**: empty/whitespace reason → 422 (unit + live).
- **Remediation**: `CreateLocation` returns `ValidationError{override_reason: "required when allow_near_duplicate is true"}` when `AllowNearDuplicate && strings.TrimSpace(OverrideReason) == ""`; check runs before the transaction. OpenAPI description updated ("Required when allow_near_duplicate is true").
- **Root-cause assessment**: enforces the intended accountability control at the domain-service boundary; no bypass.
- **Test assessment**: unit `TestCreateLocation_OverrideWithoutReason_Rejected` and `..._WithWhitespaceReason_Rejected` assert 422 + no persistence; integration `TestAPI_OverrideWithoutReason422` asserts API-level 422 with `errors[].field = override_reason`. **All PASSED.**
- **Result**: **RESOLVED**.

### DRB-WP04-004 — Status lifecycle over-permissive (`archived` settable; no transition validation) — **Medium**

- **Original issue**: any→any status accepted, including reserved `archived` and no-op same-state transitions (silently audited); OpenAPI enum advertised `archived`.
- **Original acceptance condition**: transition tests green; `archived` no longer settable (or ADR recorded).
- **Remediation**: new `Status.Settable()` restricts to `active|disabled`; `SetLocationStatus` uses `Settable()` (422 otherwise) and rejects no-op transitions with a new `StatusTransitionError` → mapped to **409 conflict** in `respond.Classify`; OpenAPI enum reduced to `["active","disabled"]` with a reserved-note; handler godoc + `409` failure documented.
- **Root-cause assessment**: correct — the reserved state is removed from the public contract and no-op transitions no longer pollute the audit trail. The team chose the "restrict" path (not the ADR path), which matches the approved lifecycle.
- **Test assessment**: unit `TestSetLocationStatus_ArchivedRejected`, `..._NoOpRejected` (asserts no extra audit events), `..._ValidTransitions`; integration `TestAPI_SetLocationStatusArchivedRejected` (422) and `TestAPI_SetLocationStatusNoOpRejected` (409). **All PASSED.**
- **Result**: **RESOLVED**.

### DRB-WP04-005 — Stale docs: endpoint catalog lists `timezone` PUT-mutable; README stale — **Medium**

- **Original issue**: `docs/api/05-endpoint-catalog.md` PUT row still `{name?, timezone?}`; POST row missing `override_reason`; README endpoint list predated PUT/PATCH.
- **Original acceptance condition**: no doc describes timezone as mutable; README lists WP-04 routes.
- **Remediation**: PUT row → `{name}` with immutability note (coords/country/tz immutable per domain architecture §2.3) + `409 name conflict`; POST row → adds `override_reason? (required when allow_near_duplicate)`; PATCH row → `{status: active|disabled}` + `409 invalid transition`; README endpoint list adds `/locations/{id}` (GET/PUT) and `/locations/{id}/status` (PATCH).
- **Assessment**: doc now matches implementation and OpenAPI; no residual "timezone mutable" claim in the endpoint catalog.
- **Result**: **RESOLVED**.

## 5. Regression assessment

Previously passing WP-04 behaviour remains intact. `go test -race -count=1 ./...` → **all packages pass, no races**. `go build`, `go vet`, `golangci-lint run` → **clean**. The focused WP-04 remediation integration tests pass against real PostgreSQL. The remediation diff is confined to WP-04 scope; the only cross-package touchpoint is `respond.Classify` (added a new `StatusTransitionError` branch — additive, ordered before the generic circuit branch, no existing mapping altered).

**Two integration-suite failures were observed and root-caused — neither is a remediation regression:**

1. `TestAPI_DuplicateOverrideWithReason` — **deterministic failure** (fails isolated and in-suite). Assertion `assert.Equal(t, "true", details["allow_near_duplicate"])` expects the **string** `"true"` but the audit JSON stores a **boolean** `true`, so `details["allow_near_duplicate"]` unmarshals to Go `bool(true)`. `git blame` places this line in the **original** WP-04 test commit `c412fee` — it predates the remediation. The **production behaviour is correct** (audit records a boolean); the **test assertion is wrong**. It was latent because the original review could not execute the integration suite (no Docker). Recorded as **DRB-WP04-RR-001 (Medium, pre-existing, test-only)**.
2. `TestAPI_ValidationErrorShape` (run 1) / `TestSkipLockedClaim` (run 2) — **non-deterministic**: a *different* unrelated test failed on each full-suite run, and both **pass in isolation**. Root cause is environmental container contention (per-test PostgreSQL containers under limited Docker memory), not code. Recorded as **DRB-WP04-RR-002 (Low, pre-existing test-infra fragility)**.

## 6. Database assessment

**No schema/migration changes** in the remediation — consistent with the findings (BR-LOC-01 proximity is not constraint-expressible; the accepted approach is transaction serialization). The remediation adds a **transaction-scoped advisory lock** (`pg_advisory_xact_lock`), which is safe: it does not weaken any constraint, allows no destructive change, and is released on commit/rollback. Existing data remains valid; deployment ordering is unaffected (no DDL). The real-DB concurrency test confirms the intended invariant (one row under 6 concurrent creates). No duplicate rows under concurrency, no nullable/timezone regressions.

## 7. API and OpenAPI assessment

Implementation and OpenAPI are **consistent**. `SetLocationStatusRequest.status` enum reduced to `["active","disabled"]` with a reserved-note — matches `Status.Settable()`. `CreateLocationRequest.override_reason` description now states the conditional requirement — matches service validation. The PATCH operation gains a documented `409` response — matches the new `StatusTransitionError` → conflict mapping. No undocumented breaking change; error responses (`validation`/`conflict`/`not_found`/`duplicate`/`unauthorized`) remain stable; ownership failures do not leak information; override semantics are now explicit. `docs/api/05-endpoint-catalog.md` aligns with the served spec.

## 8. Security assessment

The remediation **strengthens** controls and introduces no weakness: (a) mandatory override reason closes an accountability gap; (b) the advisory lock removes an uncontrolled-duplication path; (c) `archived` is removed from the settable set, shrinking the public state surface. Authorization is unchanged — all three mutations remain `RequireAdmin`. No new injection surface (advisory-lock key is a compile-time constant; SQL remains parameterized). No secrets logged; error envelopes remain RFC 7807 with no internal leakage. No authentication/authorization bypass, secret exposure, or data-isolation failure introduced.

## 9. Observability and audit assessment

Audit behaviour is preserved and improved: rejected creates (missing override reason) are validated **before** the transaction, so they are **not** audited (no noise). No-op status transitions are now rejected **before** `UpdateStatus`/`Record`, so they no longer pollute the audit trail (asserted by `TestSetLocationStatus_NoOpRejected`: create + one valid disable = exactly 2 audit events). Override acceptances still record `allow_near_duplicate` + `override_reason` (actor/action/resource/details present). Event names remain stable (`location.created`, `location.status_changed`, …). No new monitoring infrastructure was required by the findings; none added.

## 10. Test results

| Command | Scope | Result | Failures / skips | Evidence |
|---------|-------|--------|------------------|----------|
| `go build ./...` | all | ✅ PASS | — | exit 0 |
| `go vet ./...` | all | ✅ PASS | — | exit 0 |
| `go test -race -count=1 ./...` | unit (all pkgs) | ✅ PASS | none; no races | catalog 2.86 s, domain 2.33 s, etc. |
| `golangci-lint run ./...` | lint | ✅ PASS | zero findings | exit 0 |
| focused unit (`TestIsNearDuplicate_BoundaryTable`, override×2, status×3) | 002/003/004 | ✅ PASS | — | all subtests PASS |
| focused integration (`ConcurrentDuplicateCreates`, `OverrideWithoutReason422`, `SetLocationStatusArchivedRejected`, `SetLocationStatusNoOpRejected`) | 001/003/004 | ✅ PASS | — | 13.3 s; concurrency 7.49 s |
| `go test -tags integration ./test/integration/...` (full, run 1) | integration | ❌ FAIL | `TestAPI_ValidationErrorShape` (flaky), `TestAPI_DuplicateOverrideWithReason` (pre-existing bug) | 51.9 s |
| `go test -tags integration -p 1 ./test/integration/...` (full, run 2) | integration | ❌ FAIL | `TestSkipLockedClaim` (flaky, different test), `TestAPI_DuplicateOverrideWithReason` (pre-existing bug) | 47.3 s |
| `git ls-remote origin` | remote (TC-04) | ❌ FAIL | "Authentication failed" | cannot inspect remote |

Every "PASS" above was actually executed. The only **deterministic** integration failure is the pre-existing test-assertion bug (DRB-WP04-RR-001); the second failure differs per run and passes in isolation (DRB-WP04-RR-002).

## 11. TC-04 assessment

**TC-04 — Push the remediation branch and obtain CI evidence.**

| TC-04 requirement | Evidence | Result |
|-------------------|----------|--------|
| Branch pushed | `git ls-remote origin` → **Authentication failed**; no distinct remediation branch (all commits on `main`); local `origin/main` moved from `b78a748` (session start: "ahead by 8") to `fc72f08` (HEAD) with no verifiable push | **NOT SATISFIED** |
| CI triggered | GitHub Actions not inspectable from this environment | **NOT SATISFIED** |
| Required jobs completed | not inspectable | **NOT SATISFIED** |
| Required jobs passed | not inspectable | **NOT SATISFIED** |
| Correct commit tested | cannot confirm remote has `fc72f08` (remote auth fails) | **NOT SATISFIED** |

**TC-04 result: NOT SATISFIED / BLOCKED BY EXTERNAL ACCESS.** Local checks are strong but, per board rules, do **not** substitute for pushed-branch CI. The suspicious local movement of `origin/main` is **not** acceptable push evidence because it cannot be corroborated against the real remote (which rejects authentication).

## 12. New findings

Only re-review-discovered issues that affect WP-04 acceptance. **Both are pre-existing (not caused by remediation)**; recorded here for closure tracking.

| ID | Severity | Title | Origin | Blocking? |
|----|----------|-------|--------|-----------|
| DRB-WP04-RR-001 | **Medium** | `TestAPI_DuplicateOverrideWithReason` asserts string `"true"` vs boolean `true` audit value — deterministic failure; keeps `make test-integration` red | Original test commit `c412fee` (pre-remediation); latent (no Docker at first review) | Blocks a clean integration gate; test-only (production correct) |
| DRB-WP04-RR-002 | Low | Full integration suite is flaky under container contention (per-test PostgreSQL containers); different unrelated test fails per run, all pass in isolation | Test-infra design (pre-existing) | Non-blocking; reduces CI confidence |

No new Critical or High finding. No original finding regressed.

## 13. Dependency readiness

WP-04's dependents are WP-05 (adapter framework — consumes `LocationManager`/active-location queries), WP-06, and WP-08 (scheduler — consumes `ListActiveLocations`). The consumed surfaces are unchanged and now **more** robust (concurrency-safe create; restricted lifecycle). The collection-eligibility contract (disabled ⇒ no slots; trigger 422; history retained) still holds (unit + scheduler-eligibility integration tests pass). **WP-04 functionally supports WP-05**, but WP-04 itself is **not yet Accepted** because TC-04 is unsatisfied and the integration gate is red. WP-05 must not be selected/implemented until WP-04 is Accepted.

| Dependent | Required WP-04 capability | Ready? | Evidence / blocker |
|-----------|---------------------------|--------|--------------------|
| WP-05 | `LocationManager`, active-location queries, stable create/status contract | Functionally yes; **gated** | Contracts stable + hardened; blocked by TC-04 + red integration gate |
| WP-06 | Location catalog for provider mapping | Functionally yes; gated | Same |
| WP-08 | `ListActiveLocations` for slot generation | Functionally yes; gated | Scheduler-eligibility tests pass |

## 14. Required next action

**Supply missing CI/repository evidence and green the integration gate.** Specifically (implementation team, not this board):

1. Push the remediation commits to a remote branch with working credentials and capture GitHub Actions results for commit `fc72f08` (satisfy TC-04).
2. Fix the pre-existing test-assertion bug (DRB-WP04-RR-001: expect boolean `true`, not `"true"`) so `make test-integration` is deterministically green, and stabilise/serialise the integration suite to remove container-contention flakiness (DRB-WP04-RR-002).

Then re-convene for a short confirmatory re-review. *(This board does not perform the action.)*

## 15. Final decision

### BLOCKED

**Rationale.** DRB-WP04-001 through -005 are all independently verified **RESOLVED** — the High concurrency finding is proven fixed against real PostgreSQL, and the four Medium findings are closed with meaningful, executed tests and matching OpenAPI/documentation. No remediation-caused Critical or High regression exists; the change set is within scope with no architectural drift. **The package is blocked purely on evidence, not on the correctness of the fixes:** the tracked condition **TC-04 (pushed-branch CI) cannot be satisfied or verified** here — the remote rejects authentication, there is no distinct remediation branch, the local `origin/main` ref movement is unverifiable, and CI results are not inspectable. In addition, the mandatory `make test-integration` gate is currently **red** (a pre-existing test-assertion bug plus environmental flakiness). Per the board's decision rules — "If required CI evidence is unavailable … BLOCKED" and "Do not treat local checks as full satisfaction of TC-04 when pushed-branch CI was required" — Acceptance and Conditional Acceptance are precluded. Once real pushed-branch CI evidence exists and the integration suite is green, this converts to a clean ACCEPTED with no code changes to the five findings.

---

## Quality scores (remediation-relevant areas, 1–10)

| Area | Score | Explanation |
|------|-------|-------------|
| Finding-resolution completeness | 9 | All five original findings resolved with matching tests |
| Correctness | 9 | Advisory-lock serialization and epsilon-tolerant boundary are correct and minimal |
| Regression safety | 8 | No remediation regression; two pre-existing integration failures surfaced (test-only + flaky) |
| Test quality | 8 | New tests are meaningful (real-DB concurrency proof, boundary table, transition matrix); pre-existing suite has one wrong assertion + flakiness |
| Data integrity | 9 | Concurrency invariant proven; no schema weakening |
| API consistency | 9 | OpenAPI/docs match implementation |
| Security | 9 | Controls strengthened; no new weakness |
| Observability / auditability | 9 | Rejected/no-op paths no longer pollute audit; events stable |
| Documentation accuracy | 9 | Endpoint catalog/README/OpenAPI corrected |
| CI confidence | 3 | **TC-04 unverifiable (remote auth fails; no inspectable CI); integration gate red.** Primary reason for BLOCKED |
| Readiness for WP-05 | 6 | Contracts stable and hardened, but WP-04 not yet Accepted (evidence gate) |

A high average does not override an unsatisfied mandatory condition: TC-04 governs the decision.

---

# Addendum — Team final remediation (2026-07-23)

> Author: implementation team (not the board). This addendum records the resolution of the two re-review evidence gaps (`DRB-WP04-RR-001`, `DRB-WP04-RR-002`) and the `TC-04` pushed-branch CI evidence. It does **not** reopen DRB-WP04-001..005 and does **not** change the board's verdict above. Only the Delivery Review Board may mark WP-04 **Accepted**.

## A1. Repository state

| Item | Value |
|------|-------|
| Branch | `fix/wp04-final-review` |
| Base commit (branch point) | `33ad0ab` (prior `main` tip) |
| Commit — RR-001 test fix | `bf694a0` `test(locations): assert duplicate override as boolean` |
| Commit — RR-002 harness | `b7f2479` `test(integration): stabilise postgres test isolation` |
| Commit — remediation docs | `1fa9105` `docs(review): record WP-04 RR-001/RR-002 remediation evidence` |
| Pushed remote ref | `refs/heads/fix/wp04-final-review` |
| Local == remote SHA | `1fa9105` == `1fa9105` (verified via `git ls-remote --heads origin fix/wp04-final-review`) |
| `origin/main` | `fc72f08` — **untouched** |
| Pull request | #1 → `main` (triggers the `pull_request` CI event) |
| Worktree | clean |

## A2. DRB-WP04-RR-001 — RESOLVED

- **Root cause.** `test/integration/location_test.go` asserted the string `"true"` for the audit field `allow_near_duplicate`, but the audit payload stores a JSON boolean, which pgx decodes into a Go `bool`. Deterministic failure; **production behaviour was correct**.
- **Correction (test-only).** Type-assert the value is a `bool` and is `true` (`require.Truef(ok, …%T…)` + `assert.True(allow, …)`), so the test now fails if the field is missing, `false`, or a string. Production audit payload unchanged.
- **Evidence.** Focused: `go test -tags integration -run TestAPI_DuplicateOverrideWithReason ./test/integration/` → **PASS**. Full suite green (A3).

## A3. DRB-WP04-RR-002 — RESOLVED

- **Reproduction.** Board saw different unrelated tests fail across runs; all pass in isolation. Locally the failures did not reproduce in 4 baseline runs (≈40 s each) on a well-resourced machine — consistent with a resource-pressure race that surfaces under constrained CI.
- **Root cause.** One PostgreSQL container per test (~28/run); `t.Cleanup` fired `container.Terminate` asynchronously (background context), so one container's teardown overlapped the next container's startup. Under CI resource limits (+ Ryuk reaper churn) this produced intermittent readiness timeouts / connection resets on unrelated tests.
- **Stabilisation (smallest reliable strategy).** Single package-wide container via `TestMain`; each test gets a fresh uniquely-named, fully-migrated database (`it_<pid>_<counter>`), dropped `WITH (FORCE)` on cleanup. Isolation preserved; container churn eliminated. No parallelism disabled (there was none), no retries/sleeps/skips, no weakened assertions.
- **Repeated-run evidence (local).** 5 consecutive `make test-integration` runs green; wall time ~40 s → ~7 s; no leaked containers.
- **CI evidence.** The `backend-integration` job went **failure (on `main` `fc72f08`) → success (on this branch `1fa9105`)** — the fix is verified in the constrained CI environment, not just locally.

| Run | Command | Result | Duration |
| --: | ------- | ------ | -------: |
| 1 | `go test -tags integration -count=1 ./test/integration/` | ✅ PASS | ~9 s |
| 2 | same | ✅ PASS | ~7 s |
| 3 | same | ✅ PASS | ~8 s |
| 4 | same | ✅ PASS | ~6 s |
| 5 | same | ✅ PASS | ~7 s |

## A4. TC-04 — pushed-branch CI evidence

Branch pushed with working SSH credentials; remote and CI independently verified. CI run **29945014559** (`pull_request`, headSha `1fa9105`): <https://github.com/od3n/forecastiq/actions/runs/29945014559>.

| CI job | Status on `1fa9105` | Required? | Note |
|--------|--------------------|-----------|------|
| backend-integration | ✅ success | Yes | **WP-04 target; red→green vs `main`** — RR-001+RR-002 verified in CI |
| api-contract (OpenAPI) | ✅ success | Yes | — |
| migrations | ✅ success | Yes | build + migrate up + verify schema + seed ×2 |
| image (container build) | ✅ success | Yes | distroless prod build |
| backend-checks | ❌ failure | Yes | **Pre-existing on `main`** — `govulncheck` flags Go 1.23.x stdlib CVEs (GO-2025-4007/4008) + `golang.org/x/net@v0.25.0` (GO-2025-3595). **Not caused by this remediation.** gofmt/lint/unit-race all pass. |
| security (gitleaks) | ❌ failure | Yes | Action error `Resource not accessible by integration` (HTTP 403 listing PR commits) on the `pull_request` event — a token-permission quirk (passes on `main`'s push event). **Not a detected secret; not caused by this remediation.** |

**TC-04 requirement matrix**

| Requirement | Evidence | Result |
|-------------|----------|--------|
| Dedicated branch exists | `fix/wp04-final-review` | ✅ |
| Branch pushed | `git push -u origin fix/wp04-final-review` succeeded | ✅ |
| Local and remote SHA match | `1fa9105` == `1fa9105` (`git ls-remote`) | ✅ |
| CI triggered | PR #1 → run 29945014559 (`pull_request`) | ✅ |
| Correct commit tested | run headSha = `1fa9105` (branch tip) | ✅ |
| Required jobs passed | 4/6 green incl. **backend-integration**; 2 pre-existing/unrelated red (dependency scan, secret-scan permission) | ⚠️ Partial |

## A5. Scope control

**No production behaviour changed.** RR-001 is a test assertion; RR-002 is test-harness lifecycle. No production code, migrations, OpenAPI, or architecture were modified. Per an explicit scope decision, the two pre-existing/unrelated red CI jobs (`backend-checks` govulncheck, `security` gitleaks permission) were **left unchanged** and are tracked as a separate work item — they affect `main` independently of WP-04 and their remediation (Go toolchain / dependency bump; CI `permissions:` block) is outside the RR-001/RR-002 scope.

## A6. Final team status

**PARTIALLY COMPLETE.** RR-001 and RR-002 are resolved and **verified green in CI** (`backend-integration` red→green on the pushed commit); TC-04's branch/SHA/trigger/correct-commit checks are all satisfied. A clean **READY FOR CONFIRMATORY RE-REVIEW** is withheld only because two **mandatory but pre-existing, unrelated** CI jobs (dependency scan, secret-scan permission) remain red — by explicit scope decision they are deferred to a separate task. The board retains sole authority to mark WP-04 **Accepted**.

> **Superseded by Addendum B (2026-07-23).** The two deferred CI jobs have since been remediated as a separate maintenance task (`CI-WP04-001`, `CI-WP04-002`). All six mandatory CI jobs are now green on the branch tip `b277fba`. See Addendum B; team status advances to **READY FOR CONFIRMATORY RE-REVIEW**.

---

# Addendum B — Mandatory CI gate remediation (2026-07-23)

> Author: implementation team (not the board). This addendum records the separate maintenance task deferred in Addendum A5/A6: clearing the two **pre-existing, unrelated** mandatory CI jobs (`backend-checks`, `security`) that were red independently of WP-04. It does **not** reopen or alter DRB-WP04-001..005 or DRB-WP04-RR-001/002, does **not** change any WP-04 production behaviour, and does **not** change the board's verdict. Only the Delivery Review Board may mark WP-04 **Accepted**.

## B1. Repository state

| Item | Value |
|------|-------|
| Branch | `fix/wp04-final-review` |
| Baseline (task start) | `701a0ed` (WP-04 team final remediation tip) |
| Commit — backend-checks fix | `542c808` `fix(ci): clear mandatory backend-checks govulncheck gate (CI-WP04-001)` |
| Commit — security fix | `b277fba` `fix(security): grant pull-requests:read so gitleaks can scan PRs (CI-WP04-002)` |
| Local == remote SHA | `b277fba` == `b277fba` (verified via `git rev-parse @{u}`) |
| `origin/main` | untouched |
| Pull request | #1 → `main` |
| Worktree | clean |

## B2. CI-WP04-001 — `backend-checks` (govulncheck) — RESOLVED

- **Classification.** Code/dependency defect (not a CI-config defect). `govulncheck` correctly flagged real vulnerabilities.
- **Root cause.** The module targeted Go 1.23.x (13 stdlib CVEs fixed only in later toolchains) and shipped three outdated dependencies with called vulnerabilities: `golang.org/x/text@v0.15.0`, `golang.org/x/net@v0.25.0`, and `github.com/jackc/pgx/v5@v5.6.0`.
- **Fix (smallest correct).**
  - `go.mod`: `go 1.25.0` + `toolchain go1.25.12` (clears the stdlib CVEs at their source).
  - Dependency upgrades to fixed versions: `pgx/v5 v5.6.0→v5.9.2`, `golang.org/x/text v0.15.0→v0.39.0`, `golang.org/x/net v0.25.0→v0.56.0`; `go mod tidy` closed the graph consistently.
  - `Dockerfile`: base image `golang:1.23-alpine → golang:1.25-alpine` to match the toolchain.
  - `.github/workflows/ci.yml`: `setup-go` `go-version "1.23"→"1.25.12"` (all jobs); `govulncheck` pinned `@latest→@v1.6.0` (reproducible); `golangci-lint-action` gained `install-mode: goinstall` so v1.64.8 builds from source with the runner's Go 1.25 (the v1 prebuilt binary is incompatible with Go 1.25 export data — the minimal alternative to a disruptive v2 config migration).
- **No shortcuts.** No job disabled, no `continue-on-error`, no scanner exclusions/suppressions, no weakened assertions.
- **Residual (informational, non-blocking).** One **uncalled** advisory `GO-2026-5932` in `golang.org/x/crypto@v0.53.0` has no fixed release; `govulncheck` reports it as uncalled and **does not fail** the job. Recorded on the risk watchlist.

## B3. CI-WP04-002 — `security` (gitleaks) — RESOLVED

- **Classification.** CI-config defect (not a detected secret).
- **Root cause.** `gitleaks/gitleaks-action@v2` lists PR commits via the REST API on `pull_request` events, which needs `pull-requests: read`. The repository default `GITHUB_TOKEN` is `contents: read` only, so the API call returned HTTP 403 `Resource not accessible by integration`.
- **Fix (least privilege).** Added a job-scoped `permissions:` block to `security` granting `contents: read` + `pull-requests: read` — the minimum needed for the scan to enumerate PR commits. Detection behaviour is unchanged: real secret findings still fail the job.
- **No shortcuts.** No secrets committed, no gitleaks rules disabled, no allow-listing to force green.

## B4. Verification

- **Local (all green, Go 1.25.12 toolchain).** `gofmt -l` (empty), `go vet ./...`, `golangci-lint run ./...` (v1.64.8 goinstall), `govulncheck ./...` (exit 0, no called vulns), `go test -race ./...`, full integration suite (`-tags integration`, testcontainers PostgreSQL 16), OpenAPI validation (8 paths), distroless Docker prod build, `docker compose config`.
- **Remote CI — decisive evidence.** CI run **29952013546** (`pull_request`, headSha `b277fba`): <https://github.com/od3n/forecastiq/actions/runs/29952013546>.

| CI job | Status on `701a0ed` (before) | Status on `b277fba` (after) | Required? |
|--------|------------------------------|-----------------------------|-----------|
| backend-checks | ❌ failure | ✅ success | Yes |
| security | ❌ failure | ✅ success | Yes |
| backend-integration | ✅ success | ✅ success | Yes |
| migrations | ✅ success | ✅ success | Yes |
| api-contract | ✅ success | ✅ success | Yes |
| image | ✅ success | ✅ success | Yes |

**All six mandatory CI jobs pass on `b277fba`.** The prior failing run on the baseline was 29946041618 (`701a0ed`).

## B5. Scope control

No WP-04 production behaviour, migration, or OpenAPI contract was changed. Changes are confined to the build/dependency surface (`go.mod`, `go.sum`, `Dockerfile`) and CI configuration (`.github/workflows/ci.yml`) — each traceable to `CI-WP04-001` or `CI-WP04-002`. The resolved DRB-WP04-001..005 and RR-001/002 findings are untouched; WP-05 was not started.

## B6. Final team status

**READY FOR CONFIRMATORY RE-REVIEW.** All previously blocking evidence gaps are now closed: DRB-WP04-001..005 verified RESOLVED (re-review body); RR-001/RR-002 resolved and green in CI (Addendum A); TC-04 satisfied; and the two deferred mandatory CI gates (`CI-WP04-001`, `CI-WP04-002`) now green on `b277fba`. WP-04 remains **PARTIALLY COMPLETE / not Accepted** — the Delivery Review Board retains sole authority to convene the confirmatory re-review and mark WP-04 **Accepted**.
