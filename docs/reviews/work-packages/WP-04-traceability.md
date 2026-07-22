# ForecastIQ — WP-04 Location Management: Traceability Matrices

**Version**: 1.0
**Review date**: 2026-07-22
**Work package**: WP-04 — Location Management
**Companion documents**: `WP-04-delivery-review.md`, `WP-04-findings.md`

Results: PASS | PARTIAL | FAIL | NOT APPLICABLE | DEFERRED BY APPROVED DECISION.

---

## 1. Requirement traceability

| Requirement | Implementation evidence | Test evidence | Result | Finding |
|-------------|------------------------|---------------|--------|---------|
| Location CRUD (create/read/list/update) — objective | `internal/catalog/location_service.go`; `internal/api/handlers/location.go`; routes in `internal/api/router.go` | `location_service_test.go` (create/update/list/pagination); integration `location_test.go` (PUT, GET); live golden path (201/200/200) | PASS | — |
| BR-LOC-01 haversine dedup (0.05°) with 409 + existing ref | `domain/location.go` (`HaversineDegrees`, `IsNearDuplicate`); `location_service.go` CreateLocation tx; `respond/errors.go` duplicate class | `TestCreateLocation_DuplicateRejected`; `TestAPI_DuplicateLocation409` (asserts `existing_resource{id,name,distance_degrees}`); live 409 with contract fields | PASS | — |
| BR-LOC-01 under concurrent requests | None — check-then-insert in READ COMMITTED tx, no serialization | None | FAIL | DRB-WP04-001 |
| BR-LOC-01 boundary: exactly 0.05° permitted | `IsNearDuplicate` strict `<` | `TestCreateLocation_DuplicateBoundaryPermitted` (single favorable pair) | PARTIAL | DRB-WP04-002 |
| BR-LOC-01 override flag + audit | `AllowNearDuplicate` + audit details in CreateLocation | `TestCreateLocation_OverrideWithReason`; `TestAPI_DuplicateOverrideWithReason` (audit row asserted); live 201 + audit row | PASS | — |
| Mandatory override reason | Field captured and audited, not enforced | `TestCreateLocation_OverrideWithReason` (reason present, not required); live no-reason → 201 | PARTIAL | DRB-WP04-003 |
| Override audit event | `audit.Record` in create tx with `allow_near_duplicate` + `override_reason` | Integration audit query; live audit rows verified in `audit_events` | PASS | — |
| IANA timezone validation | `ValidateCreation` → `time.LoadLocation` | `TestCreateLocation_Validation`; live `Mars/Olympus_Mons` → 422 | PASS | — |
| Coordinate validation (lat/lon ranges) | `ValidateCreation`; DB CHECK constraints | `TestCreateLocation_Validation` (lat 91, ≥2 fields); migration CHECK verified | PASS | — |
| Country code validation (ISO 3166-1 alpha-2) | `ValidateCreation` (2-letter uppercase alpha) | `TestCreateLocation_Validation` (`my` rejected) | PASS | — |
| Status lifecycle (enable/disable) | `SetLocationStatus` + `UpdateStatus` | `TestSetLocationStatus_Disable/Invalid/NotFound`; `TestAPI_SetLocationStatus*`; live disable/re-enable | PASS | — |
| Lifecycle restricted to approved transitions (`archived` reserved) | Any known status accepted; no transition validation | No invalid-transition test; live `archived` → 200 | PARTIAL | DRB-WP04-004 |
| BR-LOC-02 (no per-user limits at MVP) | No limit logic (documented rule) | n/a — absence by design | NOT APPLICABLE | — |
| BR-LOC-03 disable stops future collection only | `ListActiveLocations` feeds scheduler; trigger guards inactive → 422 | `TestListActiveLocations_ExcludesDisabled`; `TestAPI_DisabledLocationBlocksCollection` (trigger 422 + history queryable); `TestSchedulerEligibility_DisabledLocationExcluded`; live: trigger 422 when disabled, `forecasts/latest` 200 after disable | PASS | — |
| Historical-data retention on disable | Soft status change only; no deletes | `TestAPI_DisabledLocationBlocksCollection` (3 snapshots still served); live query after disable | PASS | — |
| Audit integration (create/update/status) | `audit.Record` in every command tx | Unit audit assertions; integration audit queries; live `audit_events` rows for all actions with actor | PASS | — |
| (workspace_id, name) unique among active | Partial unique index `locations_active_name_uidx` + `NameConflictError` mapping | `TestAPI_LocationNameConflict409`; live same-name far-away → 409 `conflict` | PASS | — |
| Authorization (admin) on mutations | `RequireAdmin` on POST/PUT/PATCH | `TestAPI_UpdateLocationRequiresAuth`, `TestAPI_SetLocationStatusRequiresAuth`; live no-token → 401 | PASS | — |
| Idempotency-Key on POST /locations | Not implemented | — | DEFERRED BY APPROVED DECISION | Registry deferral (WP-15); tracked condition TC-01 |
| Optimistic concurrency on update | Not implemented ("if approved" — no approval exists) | — | DEFERRED BY APPROVED DECISION | Registry deferral; tracked condition TC-02 |

**Totals**: 15 PASS, 3 PARTIAL, 1 FAIL, 1 NOT APPLICABLE, 2 DEFERRED BY APPROVED DECISION.

---

## 2. Acceptance-criteria traceability

Approved acceptance: "BR-LOC-01..03 behaviors proven by tests" + DRB WP-04 test list.

| Acceptance criterion | Verification method | Evidence | Result | Finding |
|----------------------|--------------------|----------|--------|---------|
| Identical coordinates rejected | Unit + integration + live | `TestCreateLocation_DuplicateRejected`; `TestAPI_DuplicateLocation409`; live 409 (0.01°) | PASS | — |
| Near-duplicate coordinates rejected | Unit + integration + live | as above (0.01° inside boundary) | PASS | — |
| Exactly 0.05° permitted | Unit + live | `TestCreateLocation_DuplicateBoundaryPermitted` passes; live (20.001→20.051) rejected at 0.04999999999999716° | PARTIAL | DRB-WP04-002 |
| Different timezone at same coords (active) rejected | Code path identical to coords-only dedup; dedup ignores tz by design | `TestCreateLocation_DuplicateRejected` (same tz); tz not a dedup input per BR-LOC-01 | PASS | — |
| Invalid coordinates rejected | Unit + live | `TestCreateLocation_Validation`; live 422 | PASS | — |
| Inactive location blocks collection eligibility | Unit + integration + live | `TestListActiveLocations_ExcludesDisabled`; `TestAPI_DisabledLocationBlocksCollection`; `TestSchedulerEligibility_*`; live trigger 422 | PASS | — |
| Duplicate concurrent requests | None | Reproduced violation: 6 concurrent creates → 2 rows | FAIL | DRB-WP04-001 |
| Override without reason | Live probe | 201 with empty audited reason (expected 422 per guidance) | PARTIAL | DRB-WP04-003 |
| Unauthorized update | Integration + live | `TestAPI_UpdateLocationRequiresAuth`; live 401 | PASS | — |
| Deactivation with historical forecasts | Integration + live | `TestAPI_DisabledLocationBlocksCollection` (history queryable); live `forecasts/latest` 200 post-disable | PASS | — |
| Dedup override audited with reason | Integration + live | `TestAPI_DuplicateOverrideWithReason`; live audit row | PASS | — |
| Stable API contracts (OpenAPI) | Spec diff + served spec + `make docs` | PUT/PATCH + schemas added; served `/api/v1/openapi.json` contains both; `make docs` green | PASS | — |

---

## 3. Architecture compliance

| Architecture rule | Evidence | Result | Finding |
|-------------------|----------|--------|---------|
| Modular-monolith boundaries (catalog owns locations; handlers thin) | Handlers call `catalog.LocationManager` only; no domain logic in HTTP layer | PASS | — |
| Dependency direction (domain ← application ← adapters; depguard) | `domain` imports stdlib + uuid only; `catalogpg` depends on ports; `golangci-lint` (depguard) green | PASS | — |
| Domain has zero infrastructure imports | `internal/catalog/domain/*.go` imports | PASS | — |
| Transaction boundary: one tx per command (dedup + insert + audit atomic) | `dbtx.Runner` wraps ListActive→Insert→Record; audit inside tx (rejected creates not audited — asserted) | PASS | — |
| Repository responsibilities (no business rules in SQL adapters) | SQL is storage-only; dedup/validation in domain+service | PASS | DRB-WP04-006 (SQL writes immutable column — hygiene, not logic) |
| Configuration conventions (12-factor, fail-fast) | `config.Load` validation; prod rejects dev token (`TestLoad_ProductionRejectsDevToken`) | PASS | — |
| Error-classification conventions (RFC 7807 classes) | `respond.Classify` maps domain errors → `duplicate`/`conflict`/`validation`/404; live envelopes match `docs/api/03` | PASS | — |
| Identifier strategy (UUIDv7, ADR-022) | `ids.New()`; keyset pagination by id | PASS | — |
| No circular dependencies / god services / global state | Package graph inspected; `LocationService` single responsibility | PASS | — |
| Material changes require ADR | No new architectural decisions introduced; DR-01 resolved by correcting doc to match authoritative domain architecture | PASS | — |

---

## 4. Security controls

| Threat or control | Implementation | Test or review evidence | Result | Finding |
|-------------------|---------------|------------------------|--------|---------|
| Authentication on admin mutations | `RequireAdmin` dev-token seam (WP-03/19 will replace) | `TestAPI_UpdateLocationRequiresAuth`, `TestAPI_SetLocationStatusRequiresAuth`; live 401 | PASS | Interim seam; TC-03 |
| Dev token unusable in production | `config.Load` fails fast when `FIQ_DEV_ADMIN_TOKEN` set with `FIQ_ENV=production` | `TestLoad_ProductionRejectsDevToken` | PASS | — |
| Token comparison timing | `crypto/subtle.ConstantTimeCompare` | Code review | PASS | — |
| SQL injection | All queries parameterized (`$n`); no string-built SQL in changed files | Code review of `catalogpg/location.go` | PASS | — |
| Object-level authorization / workspace isolation | Single system workspace MVP (ADR-009); workspace_id defaulted + FK'd | Code review; multi-workspace is WP-03+ | PASS (scope) | — |
| Input validation | Domain `ValidateCreation` (name/lat/lon/cc/tz); binding on requests | Unit + live 422s | PASS | DRB-WP04-007 (query param coercion) |
| Error-message leakage | RFC 7807 envelopes; `Recovery` sanitizes panics; duplicate detail exposes only existing name/id/distance (admin-facing, intended per contract doc) | Live responses reviewed | PASS | — |
| Audit logging of admin actions | Every command audits actor + action + details in-tx | Integration audit assertions; live `audit_events` | PASS | DRB-WP04-003 (reason optional) |
| Secret logging | No credentials in changed code paths; logs carry ids/names only | Log review (live run) | PASS | — |
| Rate limiting | `RateLimit` middleware on router (pre-existing) | Code review | PASS | — |
| Hidden frontend controls not sole authorization | Server-side `RequireAdmin` on all three mutation routes | Router inspection | PASS | — |

---

## 5. Operational readiness

| Failure mode | Detection | Recovery | Test evidence | Result |
|--------------|-----------|----------|---------------|--------|
| Concurrent duplicate create slips through | None today (no metric/alert on near-duplicate inserts) | Manual dedup cleanup | Reproduced (DRB-WP04-001) | FAIL |
| DB failure mid-create | tx rollback (audit + insert atomic); error 5xx envelope | Retry create | Atomicity by construction; `dbtx` design | PASS |
| Unknown location on update/status | 404 `not_found` envelope | n/a | `TestUpdateLocation_NotFound`, `TestSetLocationStatus_NotFound`, `TestAPI_UpdateLocationNotFound` | PASS |
| Name collision on insert | 409 `conflict` from partial unique index mapping | Rename/retry | `TestAPI_LocationNameConflict409`; live | PASS |
| Invalid operator input | 422 with field errors | Correct + retry | Unit + live | PASS |
| Unauthorized access attempt | 401 + `api.request` log | n/a | Integration + live | PASS |
| Disabled location collected by mistake | Scheduler reads `ListActiveLocations`; trigger 422 guard | n/a (prevented) | `TestSchedulerEligibility_*`; live | PASS |

---

## 6. Documentation consistency

| Document | Claimed behaviour | Actual behaviour | Result | Required change |
|----------|-------------------|------------------|--------|-----------------|
| `docs/api/00-api-requirements.md` §4.1 | PUT name-only (corrected in this changeset) | Name-only | PASS | — |
| `docs/api/05-endpoint-catalog.md` PUT row | Body `{name?, timezone?}` | Name-only | FAIL | DRB-WP04-005 |
| `docs/api/05-endpoint-catalog.md` POST row | No `override_reason` | Field exists + audited | PARTIAL | DRB-WP04-005 |
| `docs/api/03-error-and-partial-result-contracts.md` `duplicate` | 409 + `existing_resource{id,name,distance_degrees}` | Exact match (live) | PASS | — |
| `docs/architecture/04-domain-architecture.md` §2.3 | Immutable coords/tz; lifecycle active→disabled (archived reserved); mutable name/status | Immutability honoured; lifecycle over-permissive | PARTIAL | DRB-WP04-004 |
| `docs/data/03-table-design.md` locations | "Coordinates immutable… application rule; not trigger-enforced" | Matches implementation | PASS | — |
| `README.md` endpoint list | Pre-WP-04 routes only | PUT/PATCH exist | PARTIAL | DRB-WP04-005 |
| `docs/planning/06-work-package-status-registry.md` | "Implementation Complete; PUT/PATCH live; BR-LOC-01..03 proven" | Confirmed except concurrency + boundary robustness | PARTIAL | Update per decision |
| `docs/security/01-ui-authorization-matrix.md` S-12 | "name/timezone/status" mutable via PUT | Name-only | PARTIAL | Already tracked (DR-01 → WP-19 review) |
| `docs/ui/02-ui-design-specification.md` S-12 edit form | Timezone editable in form | Backend rejects; form must render read-only | PARTIAL | Already tracked (DR-01 → WP-21) |

---

## 7. Re-review traceability (2026-07-23)

Companion: `WP-04-delivery-re-review.md`. Decision: **BLOCKED** (all code findings RESOLVED; TC-04 unverifiable + integration gate red).

### 7.1 Original-finding traceability

| Finding | Original acceptance condition | Remediation evidence | Test evidence (executed) | Result |
|---------|-------------------------------|----------------------|--------------------------|--------|
| DRB-WP04-001 | Concurrency test green; single row for the 6-way reproduction | `AcquireDedupLock` (`pg_advisory_xact_lock`) at start of create tx (`location_service.go`, `catalogpg/location.go`, `ports/repositories.go`) | `TestAPI_ConcurrentDuplicateCreates` (real PG): 1×201, 5×409, 1 row — **PASS** | **RESOLVED** |
| DRB-WP04-002 | Multi-coordinate boundary table green; (20.001→20.051) permitted | `dedupTolerance = 1e-9`; `dist < 0.05 - 1e-9` (`domain/location.go`) | `TestIsNearDuplicate_BoundaryTable` (incl. DRB pair) — **PASS** | **RESOLVED** |
| DRB-WP04-003 | Empty/whitespace reason → 422 | Pre-tx validation in `CreateLocation`; OpenAPI note | unit `OverrideWithoutReason`/`WithWhitespaceReason` + integration `TestAPI_OverrideWithoutReason422` — **PASS** | **RESOLVED** |
| DRB-WP04-004 | Transition tests green; `archived` not settable | `Status.Settable()`; no-op → `StatusTransitionError`→409; enum `[active,disabled]` | unit `ArchivedRejected`/`NoOpRejected`/`ValidTransitions` + integration `ArchivedRejected` 422 / `NoOpRejected` 409 — **PASS** | **RESOLVED** |
| DRB-WP04-005 | No doc says timezone mutable; README lists routes | endpoint-catalog PUT `{name}`+note, POST `override_reason?`, PATCH `{active|disabled}`; README updated | doc review | **RESOLVED** |

### 7.2 Regression traceability

| Existing WP-04 behaviour | Verification | Result | New finding |
|--------------------------|--------------|--------|-------------|
| Unit suite (all pkgs, `-race`) | `go test -race ./...` | PASS | — |
| Build / vet / lint | `go build`, `go vet`, `golangci-lint run` | PASS | — |
| create/read/list/update/status, dedup, override, disable→history | focused unit + integration | PASS | — |
| scheduler eligibility (disabled excluded) | `TestSchedulerEligibility_*` | PASS | — |
| override-audit integration assertion | `TestAPI_DuplicateOverrideWithReason` | **FAIL (deterministic)** | DRB-WP04-RR-001 (pre-existing test-assertion bug; not remediation-caused) |
| full integration suite stability | full runs ×2 | FLAKY | DRB-WP04-RR-002 (pre-existing infra) |

### 7.3 CI traceability (TC-04)

| Required CI check | Commit tested | Result | Evidence |
|-------------------|---------------|--------|----------|
| Pushed-branch CI (GitHub Actions) | `fc72f08` (unconfirmed on remote) | **NOT SATISFIED** | `git ls-remote origin` → auth failed; no distinct branch; CI not inspectable |

### 7.4 Documentation traceability

| Document / contract | Required update | Actual update | Result |
|---------------------|-----------------|---------------|--------|
| `docs/api/05-endpoint-catalog.md` | PUT `{name}`; POST `override_reason?`; PATCH `{active|disabled}` | Applied | PASS |
| `README.md` | List PUT/PATCH routes | Applied | PASS |
| `api/openapi/openapi.json` | Enum `[active,disabled]`; override note; 409 on PATCH | Applied | PASS |
| `docs/risk/02-phase-1-risk-register.md` | R-35 status | Updated (see register) | PASS |
| `docs/planning/06-work-package-status-registry.md` | State = Blocked; re-review outcome | Updated | PASS |

### 7.5 Dependency readiness

| Dependent package | Required WP-04 capability | Ready? | Evidence or blocker |
|-------------------|---------------------------|--------|---------------------|
| WP-05 | `LocationManager`, active-location queries, stable create/status contract | Functionally yes; **gated** | Contracts stable + hardened; blocked by TC-04 + red integration gate |
| WP-06 | Location catalog for provider mapping | Functionally yes; gated | Same |
| WP-08 | `ListActiveLocations` for slot generation | Functionally yes; gated | Scheduler-eligibility tests pass |

WP-05 is included for readiness only; it was **not** reviewed or implemented during the re-review.

---

## 8. Team final-remediation traceability (2026-07-23)

Companion: `WP-04-delivery-re-review.md` §A1–A6. Records the implementation team's closure of the two evidence blockers (RR-001, RR-002) and the TC-04 CI evidence, within strict scope (test-only + test-harness; no production/CI changes). Branch `fix/wp04-final-review`, tip `1fa9105`.

### 8.1 Remediation-finding traceability

| Finding | Acceptance condition | Remediation evidence | Test evidence (executed) | Result |
|---------|----------------------|----------------------|--------------------------|--------|
| DRB-WP04-RR-001 | Assert audit boolean, not string; production payload unchanged | `location_test.go` type-asserts `bool` + `true` (`bf694a0`) | `go test -tags integration -run TestAPI_DuplicateOverrideWithReason ./test/integration/` — **PASS** | **RESOLVED** |
| DRB-WP04-RR-002 | Stabilise suite; preserve isolation; ≥5 consecutive green; no retries/skips | Shared `TestMain` container + per-test fresh DB dropped `WITH (FORCE)` (`b7f2479`) | 5 consecutive `make test-integration` green (~40 s→~7 s); `backend-integration` red→green in CI | **RESOLVED** |

### 8.2 CI traceability (TC-04) — superseding §7.3

Pushed-branch CI is now inspectable (SSH credentials working). Run **29945014559** (`pull_request`, headSha `1fa9105`): <https://github.com/od3n/forecastiq/actions/runs/29945014559>.

| Required CI check | Commit tested | Result | Evidence |
|-------------------|---------------|--------|----------|
| backend-integration | `1fa9105` | **PASS** | red on `main` `fc72f08` → green on `1fa9105`; RR-001+RR-002 verified in CI |
| api-contract (OpenAPI) | `1fa9105` | **PASS** | run 29945014559 |
| migrations | `1fa9105` | **PASS** | build + migrate up + verify + seed ×2 |
| image (container build) | `1fa9105` | **PASS** | distroless prod build |
| backend-checks | `1fa9105` | **FAIL (pre-existing, out of scope)** | `govulncheck` Go 1.23.x stdlib CVEs (GO-2025-4007/4008) + `x/net@v0.25.0` (GO-2025-3595); also red on `main`; gofmt/lint/unit-race pass |
| security (gitleaks) | `1fa9105` | **FAIL (pre-existing, out of scope)** | HTTP 403 `Resource not accessible by integration` on `pull_request` event; passes on `main` push; not a detected secret |
| Local == remote SHA | `1fa9105` | **PASS** | `git ls-remote --heads origin fix/wp04-final-review` = `1fa9105` |

### 8.3 Documentation traceability

| Document / contract | Required update | Actual update | Result |
|---------------------|-----------------|---------------|--------|
| `WP-04-delivery-re-review.md` | Addendum: RR-001/RR-002 resolved + TC-04 CI evidence + final status | Applied (§A1–A6) | PASS |
| `WP-04-findings.md` | RR-001/RR-002 cards → Resolved with evidence | Applied | PASS |
| `docs/planning/06-work-package-status-registry.md` | Team final-remediation subsection (PARTIALLY COMPLETE) | Applied | PASS |
| `docs/risk/02-phase-1-risk-register.md` | R-35 residual updated | Applied | PASS |
| `WP-04-traceability.md` §8 | Remediation + CI + doc traceability | This section | PASS |

### 8.4 Team status

**PARTIALLY COMPLETE.** RR-001 and RR-002 RESOLVED and verified green in CI; TC-04 branch/SHA/trigger/correct-commit checks satisfied. Withheld from a clean READY FOR CONFIRMATORY RE-REVIEW only because two mandatory but pre-existing, unrelated CI jobs remain red (deferred by explicit scope decision). The board retains sole authority to mark WP-04 **Accepted**.

> **Superseded by §9 (2026-07-23).** The two deferred CI jobs are now RESOLVED; all six mandatory CI jobs are green on `b277fba`. Team status advances to **READY FOR CONFIRMATORY RE-REVIEW**.

---

## 9. Mandatory CI gate remediation traceability (2026-07-23)

Companion: `WP-04-delivery-re-review.md` §B; findings `CI-WP04-001`, `CI-WP04-002`. Records the separate maintenance task that cleared the two pre-existing mandatory CI gates deferred in §8. Branch `fix/wp04-final-review`, tip `b277fba`. Scope: build/dependency + CI config only; no WP-04 production, migration, or OpenAPI change.

### 9.1 Finding traceability

| Finding | Classification | Acceptance condition | Remediation evidence | Result |
|---------|----------------|----------------------|----------------------|--------|
| CI-WP04-001 | Code/dependency defect | `backend-checks` green in CI (no called vulns) | `go 1.25.0`+`toolchain go1.25.12`; `pgx/v5 v5.9.2`, `x/text v0.39.0`, `x/net v0.56.0`; `Dockerfile` `golang:1.25-alpine`; `ci.yml` go-version/govulncheck pin/goinstall (`542c808`) | **RESOLVED** |
| CI-WP04-002 | CI-config defect (token permission) | `security` green in CI; detection intact | Least-privilege `permissions:` block (`contents:read`+`pull-requests:read`) on `security` job (`b277fba`) | **RESOLVED** |

### 9.2 CI traceability — superseding §8.2

Run **29952013546** (`pull_request`, headSha `b277fba`): <https://github.com/od3n/forecastiq/actions/runs/29952013546>. Baseline `701a0ed` run `29946041618` had `backend-checks`+`security` red.

| Required CI check | Status on `701a0ed` | Status on `b277fba` | Evidence |
|-------------------|---------------------|---------------------|----------|
| backend-checks | FAIL | **PASS** | govulncheck exit 0 after toolchain+dep bump; gofmt/lint(goinstall)/unit-race pass |
| security | FAIL | **PASS** | gitleaks scans PR commits after `pull-requests:read` grant |
| backend-integration | PASS | **PASS** | RR-001+RR-002 remain green |
| api-contract (OpenAPI) | PASS | **PASS** | 8 paths validated |
| migrations | PASS | **PASS** | build + migrate up + verify + seed ×2 |
| image (container build) | PASS | **PASS** | distroless prod build |
| Local == remote SHA | — | **PASS** | `git rev-parse @{u}` = `b277fba` |

### 9.3 Documentation traceability

| Document | Required update | Actual update | Result |
|----------|-----------------|---------------|--------|
| `WP-04-delivery-re-review.md` | Addendum B: CI-WP04-001/002 + CI evidence + status | Applied (§B1–B6) | PASS |
| `WP-04-findings.md` | CI-WP04-001/002 finding cards → Resolved | Applied | PASS |
| `docs/planning/06-work-package-status-registry.md` | WP-04 row → Ready for confirmatory re-review + CI subsection | Applied | PASS |
| `docs/risk/02-phase-1-risk-register.md` | Watchlist: uncalled `GO-2026-5932` residual | Applied | PASS |
| `WP-04-traceability.md` §9 | CI gate remediation traceability | This section | PASS |

### 9.4 Team status

**READY FOR CONFIRMATORY RE-REVIEW.** All six mandatory CI jobs green on `b277fba`; DRB-WP04-001..005 + RR-001/002 verified resolved; TC-04 satisfied. WP-04 remains **not Accepted** — only the Delivery Review Board may convene the confirmatory re-review and mark it Accepted. WP-05 must not be selected until then.
