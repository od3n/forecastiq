# ForecastIQ — WP-04 Location Management: Delivery Review Board Report

**Version**: 1.0
**Review date**: 2026-07-22
**Work package**: WP-04 — Location Management
**Reviewed commits**: `19035f9` feat(catalog) → `5445435` docs(planning), on accepted baseline `b78a748` (origin/main)
**Companion documents**: `WP-04-findings.md` (finding cards), `WP-04-traceability.md` (matrices)

---

## 1. Executive summary

WP-04 delivers the location aggregate with BR-LOC-01 haversine dedup, IANA timezone validation, the soft-delete status lifecycle, full audit integration, and — beyond the letter of the package definition but within the DRB WP-04 guidance — live PUT/PATCH endpoints with updated OpenAPI contracts. The implementation is architecturally clean: thin handlers, domain invariants in the domain layer, one transaction per command, parameterized SQL, and an exemplary discrepancy record (DR-01) resolving the timezone-mutability documentation conflict in favour of the authoritative domain architecture.

The board verified every claim against the repository and re-executed the golden path live against the real binary and a real PostgreSQL: dedup 409 with the documented `existing_resource` contract, override with audited reason, name-only PUT with immutability preserved, disable → collection 422 with history intact, 401 without the admin token, and a complete audit trail.

Two defects prevent acceptance. First, the BR-LOC-01 dedup check is a check-then-insert under READ COMMITTED with no serialization: the board **reproduced** a concurrent-duplicate bypass (6 simultaneous creates → 2 rows), and no concurrency test exists. Second, the "exactly 0.05° permitted" boundary is floating-point fragile — a mathematically exact 0.05° pair was rejected live at 0.04999999999999716°, and the passing unit test covers only one coordinate-favorable pair. Three Medium findings (optional override reason, over-permissive status lifecycle exposing the reserved `archived` state, one stale doc row) and three Low findings complete the list.

**Decision: CHANGES REQUIRED.** One High finding open (DRB-WP04-001). The remediation is small and well-understood; a focused remediation pass followed by re-review is the expected path.

## 2. Review scope

**Files/modules reviewed**: `internal/catalog/{catalog.go, location_service.go, location_service_test.go, domain/*, ports/repositories.go}`, `internal/api/{router.go, middleware.go, handlers/location.go, handlers/handlers.go, respond/errors.go}`, `adapters/persistence/catalogpg/location.go`, `api/openapi/openapi.json`, `migrations/2026080100000{1,2,3,5}_*`, `test/integration/{setup_test.go, location_test.go}`, `.github/workflows/ci.yml`, `Makefile`, `README.md`, `.env.example`.

**Documents reviewed**: `docs/planning/05-implementation-work-packages.md` (WP-04), `docs/planning/06-work-package-status-registry.md`, `docs/product/05-business-rules.md` (BR-LOC-01..03), `docs/architecture/03-module-architecture.md` §3.2, `docs/architecture/04-domain-architecture.md` §2.3, `docs/data/03-table-design.md`, `docs/api/{00,01,03,05}-*`, `docs/security/01-ui-authorization-matrix.md`, `docs/ui/{02,05,06}-*`, `docs/testing/01-requirement-test-traceability.md`, `docs/development/05-testing-guide.md`, `docs/risk/02-phase-1-risk-register.md`.

**Commands run**: `go build ./...` ✅; `go vet ./...` ✅; `go vet -tags integration ./test/...` ✅ (suite compiles); `go test -race -count=1 ./...` ✅ (all packages pass); `golangci-lint run ./...` ✅ (zero findings); `make docs` ✅ (OpenAPI valid, 8 paths); live golden path against `go run ./cmd/forecastiq serve` + PostgreSQL (create/dedup/override/PUT/PATCH/trigger/auth/audit/concurrency/boundary probes); `git diff` scope analysis.

**Limitations**: Docker unavailable → testcontainers integration suite (`make test-integration`) not executed; mitigated by compile check + live re-verification of every asserted behaviour against real PostgreSQL 14 (target is 16; no 16-specific DDL found). WP-04 commits are local-only; CI has not run on them (tracked condition TC-04). `docker compose config`/image build not exercised (no Docker).

## 3. Package status

| Attribute | Value |
|-----------|-------|
| Previous state | Implementation Complete |
| Review decision | **CHANGES REQUIRED** |
| Resulting state | **Review Findings Open** |

## 4. Requirement results

Total traced requirements: 22 (see traceability §1).

| Result | Count |
|--------|-------|
| PASS | 15 |
| PARTIAL | 3 |
| FAIL | 1 |
| NOT APPLICABLE | 1 |
| DEFERRED BY APPROVED DECISION | 2 |

The single FAIL is BR-LOC-01 under concurrency (DRB-WP04-001). PARTIALs: boundary robustness (002), mandatory override reason (003), lifecycle transitions (004). Deferrals (Idempotency-Key → WP-15; optimistic concurrency → if multi-operator emerges) are transparently recorded in the status registry with rationale.

## 5. Acceptance-criteria results

Approved acceptance — "BR-LOC-01..03 behaviors proven by tests" — is met for the sequential paths with strong evidence (unit + integration + live), including the subtle ones: rejected creates are not audited, disabled locations are excluded from scheduler eligibility while their history stays queryable, and the name-uniqueness partial index surfaces as a 409 `conflict`. The DRB WP-04 test list is 9/12 satisfied: **duplicate concurrent requests FAIL** (reproduced violation), **exactly-0.05° boundary PARTIAL** (coordinate-dependent), **override without reason PARTIAL** (accepted instead of rejected). Full table: traceability §2.

## 6. Architecture assessment

Boundary compliance is excellent. The catalog module exposes `LocationManager` per module architecture §3.2; handlers contain zero business logic; the domain package imports only stdlib + uuid (depguard-green); adapters implement ports and are wired solely in the composition root. Transaction boundaries follow the documented "one tx per command" rule, and the audit write shares the command tx — so a rolled-back create leaves no audit noise (asserted in tests). The DR-01 resolution correctly treated the domain architecture as authoritative and corrected the API-requirements doc rather than the code. No ADR-required changes were introduced; no drift detected. The only blemish is the prototype-leftover `timezone` write in the repository UPDATE (DRB-WP04-006, Low — the approved table design explicitly makes immutability an application rule, so this is hygiene, not drift).

## 7. Code-quality assessment

Idiomatic, small, well-documented code. Naming is clear; errors are classified domain types mapped once in `respond.Classify`; context propagation and DI are consistent; the clock is injectable (deterministic tests). The WP-04 hardening pass visibly improved the prototype: the mutable-timezone input was removed, structured log events were added for update/status, and constraint violations were mapped to domain errors. Remaining items: the `timezone` write-back SQL (006), silent `active` param coercion (007), and the absence of transition validation in `SetLocationStatus` (004). No dead code, no commented-out code, no unactionable TODOs observed.

## 8. Database assessment

No schema changes in this package (WP-02 schema stands; correctly so — WP-04 is additive). Existing controls verified: PK uuid, workspace FK, lat/lon CHECK constraints, `locations_active_name_uidx` partial unique index (proven live → 409), `set_updated_at` trigger (verified: `updated_at > created_at` after rename). Immutability of coordinates/timezone is an approved application rule per table-design doc — consistent with implementation. Migration safety: not applicable (no new migrations). Gaps: no DB-level guard for proximity dedup (by design — haversine is not constraint-expressible; hence the advisory-lock/SERIALIZABLE remediation in DRB-WP04-001) and the UPDATE statement touching the immutable column (006). No N+1 or full-scan concerns at MVP scale (`ListActive` over ≤ dozens of rows is the documented design).

## 9. API assessment

Routes match the corrected API requirements: `POST /locations` (admin), `GET /locations` + `GET /locations/{id}` (public), `PUT /locations/{id}` (admin, name-only), `PATCH /locations/{id}/status` (admin). OpenAPI was updated in the same changeset and the served spec matches (`make docs` green; live `/api/v1/openapi.json` contains both operations). Error envelopes conform to `docs/api/03`: the `duplicate` class carries `existing_resource{id, name, distance_degrees}` exactly as contracted; `conflict`, `validation`, `not_found`, and `unauthorized` all observed live with request IDs. Deviations: the request enum exposes the reserved `archived` status (004); malformed `active` query values are silently coerced (007); endpoint-catalog doc drift (005). Idempotency-Key remains deferred (recorded; TC-01).

## 10. Testing assessment

**Commands**: `go test -race -count=1 ./...` → all pass (catalog 2.8 s, domain 2.3 s, no races); `golangci-lint run` → clean; integration suite compiles but could not execute (no Docker — see §2 limitations).

**Quality**: tests assert meaningful outcomes, not status codes alone — audit event contents, immutability of specific fields, absence of audit on rejected create, `existing_resource` payload fields, scheduler-eligibility exclusion, and history-queryability after disable. Fakes are minimal and deterministic (fixed clock). The integration suite adds real-DB constraint coverage (partial unique index) and end-to-end envelope assertions.

**Gaps**: no concurrency test (001 — the gap that bit), boundary coverage limited to one coordinate pair (002), no "override without reason" rejection test (003), no invalid-transition tests (004). No skipped tests, no flaky time dependence, no live external APIs in the suite.

**Confidence**: High for sequential behaviour (triangulated: unit + compiled integration assertions + live re-execution). Reduced for concurrency behaviour (defect found) and for PG16-specific behaviour (verified on PG14; no 16-specific constructs identified).

## 11. Security assessment

Posture is sound for an interim package. All three mutation routes are server-side gated by `RequireAdmin`; the dev-token seam uses constant-time comparison and — critically — `config.Load` **refuses to start in production with a dev token set** (tested). SQL is fully parameterized. Error envelopes leak no stack/SQL; the duplicate detail exposes only the existing location's name/id/distance, which the error-contract doc explicitly specifies for the admin-facing dedup flow. Audit captures actor, action, resource, and details for every mutation. Residual risk: the dev-token seam is not build-tag-excluded (that hardening belongs to WP-03/19 per approved scope — TC-03), and override reasons are optional (003). No injection, SSRF, path-traversal, or secret-logging exposure introduced by this package.

## 12. Observability and reliability assessment

Observability: structured events with stable names (`location.created`, `location.updated`, `location.status_changed`, `api.request`) carrying location_id/name/status; request IDs echoed end-to-end; RED metrics keyed by route template (verified live: `http_requests_total` series present). WP-04 added the previously missing update/status log events. No secrets in logs.

Reliability: command atomicity is correct (dedup+insert+audit in one tx); 404/409/422 paths are clean; disable is a pure status flip with no data path. The one reliability defect is the dedup TOCTOU (001) — an uncontrolled-duplication path, reproduced. Retry/backoff/circuit concerns are WP-05/08 scope and untouched.

## 13. Developer-experience assessment

A new engineer can run the package flow end-to-end: `make setup` → `make dev-up` (or local `pg_ctl` + `make migrate seed run`) → curl the documented endpoints. The board executed exactly this path (local PG + `go run`) without undocumented steps. Makefile targets cover test/lint/migrate/seed/docs; `.env.example` documents every variable including the dev-token seam with a production warning. Gaps: README endpoint list and the endpoint catalog are stale relative to the new routes (005). Troubleshooting docs were not needed and not exercised.

## 14. Regression assessment

Previously accepted behaviour remains intact. The diff is confined to WP-04 scope: catalog module, location API surface, OpenAPI, one doc correction (DR-01), and the registry. Pre-existing endpoints re-verified live (`/healthz` 200, `/metrics` serving, `forecasts/latest` 200, trigger 200 when active / 422 when inactive). Unit suite (all packages, `-race`) and lint are green. No rewritten applied migrations; no dependency changes (`go.mod` untouched). The only cross-package touchpoints — scheduler slot generation via `ListActiveLocations` and the trigger's inactive guard — are covered by dedicated tests. No unapproved scope, no drift, no regressions found.

## 15. Findings

Ordered by severity; full cards in `WP-04-findings.md`.

| ID | Severity | Title | Status |
|----|----------|-------|--------|
| DRB-WP04-001 | **High** | BR-LOC-01 dedup not concurrency-safe (TOCTOU; reproduced: 6 concurrent creates → 2 rows) | Open |
| DRB-WP04-002 | Medium | 0.05° boundary floating-point fragile; exact-0.05° pair rejected live | Open |
| DRB-WP04-003 | Medium | Override reason audited but not mandatory with `allow_near_duplicate` | Open |
| DRB-WP04-004 | Medium | Reserved `archived` settable via API; no transition validation | Open |
| DRB-WP04-005 | Medium | Endpoint catalog still documents PUT timezone mutability; README stale | Open |
| DRB-WP04-006 | Low | Repository UPDATE writes immutable timezone column | Open |
| DRB-WP04-007 | Low | Malformed `active` query param silently coerced | Open |

No Critical findings. No disagreements within the board; the High classification of 001 was unanimous (explicit DRB guidance requires concurrent duplicate prevention, and the violation was reproduced, not hypothesized).

## 16. Required remediation

Blocking (must close before re-review):

1. **DRB-WP04-001** — Serialize the create dedup window (`pg_advisory_xact_lock` in the create tx, or SERIALIZABLE + bounded 40001 retry). Acceptance: integration test with ≥5 concurrent near-duplicate creates → exactly one 201 and one row; board reproduction yields a single row.
2. **DRB-WP04-002** — Epsilon-tolerant (or precision-rounded) boundary comparison with documented precision. Acceptance: multi-coordinate boundary table test green; the (20.001 → 20.051) pair accepted live.
3. **DRB-WP04-003** — Enforce non-empty `override_reason` when `allow_near_duplicate` is set (422). Acceptance: unit + live rejection of empty/whitespace reasons.
4. **DRB-WP04-004** — Restrict the status endpoint to `active|disabled` and reject no-op/invalid transitions, or record an ADR approving `archived`. Acceptance: transition-matrix tests green; OpenAPI enum corrected if restricted.
5. **DRB-WP04-005** — Correct `docs/api/05-endpoint-catalog.md` (PUT `{name}`, POST `override_reason?`) and refresh the README endpoint list.

Non-blocking (fix opportunistically, verify at re-review): DRB-WP04-006, DRB-WP04-007.

## 17. Tracked conditions

| ID | Condition | Owner | Target |
|----|-----------|-------|--------|
| TC-01 | Idempotency-Key infrastructure for POST /locations (registry deferral; rationale accepted — no table in approved schema, cross-cutting, dedup makes re-execution harmless) | Eng | WP-15 |
| TC-02 | Optimistic concurrency on location update (registry deferral; "if approved" — no approval exists) | Eng | If multi-operator admin emerges |
| TC-03 | Replace dev-token seam with Supabase JWKS; UI timezone read-only (DR-01 → WP-21); security-matrix S-12 row (DR-01 → WP-19) | Eng | WP-03/19/21 |
| TC-04 | CI green on the pushed WP-04 branch (commits currently local-only) + testcontainers suite executed (not possible in review env — no Docker) | Eng | Before state → Accepted |

## 18. Dependency readiness

WP-04's dependents are WP-05 (adapter framework: consumes `LocationManager`/active-location queries), WP-06, and WP-08 (scheduler: consumes `ListActiveLocations`). The consumed surfaces are stable, tested, and verified live; the collection-eligibility contract (disabled ⇒ no slots, trigger 422) holds. **The concurrency defect (001) does not block dependent packages** — it is an admin-write-path defect, not a contract instability — but it must close before WP-04 itself is Accepted. With 001–005 remediated, the package safely supports its dependents.

## 19. Recommended next action

**Run a Work-Package Remediation Prompt** for DRB-WP04-001..005 (plus opportunistic 006–007), then re-convene the Delivery Review Board. Push the branch and satisfy TC-04 as part of remediation. Do not begin WP-05 until re-review.

## 20. Final decision

### CHANGES REQUIRED

The package is close to completion: 15 of 22 traced requirements pass with strong triangulated evidence, the architecture is exemplary, and the sequential BR-LOC-01..03 behaviours are proven by tests and live execution. However, one High finding remains open — the reproduced concurrent-duplicate bypass of BR-LOC-01, with no covering test — and the exact-boundary guarantee is proven only for one coordinate pair. Per the decision rules, an open High finding precludes Acceptance and Conditional Acceptance. The remediation surface is small and precisely specified (§16); the board expects a clean re-review after one remediation pass.

---

## Quality scores

| Area | Score | Explanation |
|------|-------|-------------|
| Requirements completeness | 8 | All approved scope present; concurrent prevention + mandatory reason (DRB guidance) outstanding |
| Correctness | 7 | Reproduced dedup race (001) and fp-fragile boundary (002) |
| Architecture alignment | 9 | — |
| Code quality | 9 | — |
| Data integrity | 7 | Proximity rule not concurrency-safe (001); immutable-column write-back (006); name uniqueness + audit solid |
| API quality | 8 | Contract-accurate overall; `archived` exposure (004) + param coercion (007) |
| Test quality | 8 | Meaningful, deterministic, well-asserted; missing concurrency/transition/reason tests |
| Security | 9 | — |
| Observability | 9 | — |
| Reliability | 6 | Uncontrolled-duplication path reproduced (001); otherwise atomic and clean |
| Developer experience | 9 | — |
| Documentation | 7 | One materially stale contract row (005); registry/DR-01 handling exemplary |

Scores below 8 are explained inline. Per board rules, the High finding governs the decision regardless of score averages.
