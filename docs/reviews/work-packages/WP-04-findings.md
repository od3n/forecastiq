# ForecastIQ — WP-04 Location Management: Delivery Review Findings

**Version**: 1.0
**Review date**: 2026-07-22
**Work package**: WP-04 — Location Management
**Reviewed commits**: `19035f9..5445435` (on top of accepted baseline `b78a748`)
**Board decision**: CHANGES REQUIRED (one High finding open)
**Re-review (2026-07-23)**: DRB-WP04-001..005 verified **RESOLVED**; decision **BLOCKED** on TC-04 (pushed-branch CI unverifiable) + red integration gate. See `WP-04-delivery-re-review.md`.
**Authority**: Delivery Review Board prompt; `docs/planning/05-implementation-work-packages.md` §WP-04

Finding ID scheme: `DRB-WP04-<NNN>` (original), `DRB-WP04-RR-<NNN>` (re-review). Statuses: Open | Resolved (verified re-review) | Accepted Risk | Deferred by Approved Decision | Resolved During Review | Not Reproducible.

---

## DRB-WP04-001 — BR-LOC-01 dedup check is not concurrency-safe (TOCTOU)

| Attribute | Value |
|-----------|-------|
| Severity | **High** |
| Discipline | Principal Backend Engineer / SRE / QA |
| Affected requirement | BR-LOC-01; DRB WP-04 guidance "concurrent duplicate prevention" |
| Affected files | `internal/catalog/location_service.go` (CreateLocation tx), `adapters/persistence/catalogpg/location.go` |
| Status | **Resolved (verified re-review 2026-07-23)** — `AcquireDedupLock` (`pg_advisory_xact_lock`) serializes the create tx; `TestAPI_ConcurrentDuplicateCreates` executed against real PostgreSQL: 6 concurrent → 1×201, 5×409, 1 row. |
| Owner | Eng |

**Evidence.** `CreateLocation` performs `ListActive` → haversine scan → `Insert` inside a single READ COMMITTED transaction. No database-level constraint enforces proximity, and nothing serializes concurrent creates. Reproduced live against the real binary + PostgreSQL: 6 concurrent `POST /api/v1/locations` at identical coordinates (30.001, 60.001) with distinct names produced **two 201 responses and two rows** (`Storm Loc 3`, `Storm Loc 5`); the other four requests correctly returned 409. The two near-duplicate rows coexist in `locations`, violating BR-LOC-01.

**Expected behaviour.** BR-LOC-01 ("a new location within 0.05° of an existing active location is rejected unless `allow_near_duplicate` is set") must hold under concurrent requests. The DRB WP-04 test list explicitly requires a "duplicate concurrent requests" test; none exists (unit or integration).

**Actual behaviour.** Concurrent creates can both pass the proximity check before either commits; both insert. The partial unique index `locations_active_name_uidx` only catches identical names, not proximity.

**Impact.** Business-rule violation writable through the public admin API; duplicate active locations then generate duplicate collection slots and skew coverage/reliability denominators downstream. Low probability for a single-operator MVP, but the path is uncontrolled and untested.

**Reproduction.**
```bash
for i in 1..6; do curl -X POST .../api/v1/locations -H "Authorization: Bearer <token>" \
  -d '{"name":"Storm Loc '$i'","latitude":30.001,"longitude":60.001,"country_code":"IR","timezone":"Asia/Tehran"}' & done; wait
# observe: 2× HTTP 201, 4× HTTP 409; SELECT count(*) FROM locations WHERE name LIKE 'Storm Loc%'; → 2
```

**Recommended remediation.** Serialize the dedup window: take `pg_advisory_xact_lock(<fixed key>)` at the start of the create transaction (adequate for MVP admin traffic), or run the create transaction at SERIALIZABLE with bounded retry on `40001`. Do not rely on application-level check-then-insert alone.

**Required tests.** Integration test: N (≥5) concurrent near-duplicate creates at the same coordinates → exactly one 201, rest 409, exactly one row persisted. Unit test of the retry path if SERIALIZABLE is chosen.

**Acceptance condition.** Concurrency test green in CI; live re-verification shows a single row for the reproduction above.

---

## DRB-WP04-002 — 0.05° dedup boundary is floating-point fragile ("exactly 0.05° permitted" not robustly proven)

| Attribute | Value |
|-----------|-------|
| Severity | **Medium** |
| Discipline | Principal Backend Engineer / QA |
| Affected requirement | WP-04 test list "Dedup boundary (exactly 0.05°)"; testing doc boundary 0.049°/0.051° |
| Affected files | `internal/catalog/domain/location.go` (`IsNearDuplicate`, `HaversineDegrees`), `internal/catalog/location_service_test.go` |
| Status | **Resolved (verified re-review 2026-07-23)** — `dedupTolerance = 1e-9` guard band; `TestIsNearDuplicate_BoundaryTable` (equator/mid/high lat, meridional+zonal, DRB pair permitted, 0.049 rejected, 0.051 permitted) executed and passed. |
| Owner | Eng |

**Evidence.** The rule is `distanceDegrees < 0.05` (strict), documented as "exactly 0.05° is permitted". Live check: a point mathematically exactly 0.05° from an existing location (20.001 → 20.051, same meridian) was **rejected** with computed `distance_degrees: 0.04999999999999716` — IEEE-754 error landed below the threshold. The passing unit test (`TestCreateLocation_DuplicateBoundaryPermitted`, base latitude 1.4927) succeeds only because that particular sum rounds above the threshold.

**Expected behaviour.** The documented boundary semantics ("exactly 0.05° permitted") should hold for coordinate pairs at exact 0.05° separation regardless of representation luck, within a stated precision.

**Impact.** Operators creating grid-style locations at exact 0.05° spacing get inconsistent 409/201 outcomes depending on coordinates; acceptance criterion is proven only for one favorable pair.

**Recommended remediation.** Compare with a documented tolerance, e.g. duplicate iff `dist < DedupThresholdDegrees - 1e-9`, or round `dist` to a defined precision (1e-6 degrees ≈ 0.11 m) before comparing. Document the precision in the domain comment.

**Required tests.** Boundary table test: for several base latitudes (equator, mid-latitude, high latitude) and both meridional and zonal offsets, exact-0.05° separation is permitted and 0.049° is rejected. Include the (20.001 → 20.051) pair from this finding.

**Acceptance condition.** Boundary table test green; live re-check of the reproduced pair returns 201.

---

## DRB-WP04-003 — Override reason audited but not mandatory when `allow_near_duplicate` is set

| Attribute | Value |
|-----------|-------|
| Severity | **Medium** |
| Discipline | Business Analyst / Security Engineer |
| Affected requirement | DRB WP-04 guidance "mandatory override reason"; WP-04 accountability for BR-LOC-01 override |
| Affected files | `internal/catalog/location_service.go`, `internal/api/handlers/handlers.go` (`CreateLocationRequest`), `api/openapi/openapi.json` |
| Status | **Resolved (verified re-review 2026-07-23)** — empty/whitespace `override_reason` with `allow_near_duplicate` → 422; unit (`OverrideWithoutReason`/`WithWhitespaceReason`) + integration (`TestAPI_OverrideWithoutReason422`) executed and passed; OpenAPI note added. |
| Owner | Eng |

**Evidence.** Live: `POST /locations` with `allow_near_duplicate: true` and **no** `override_reason` → 201 Created; the audit row records `"override_reason": ""`. The field exists, is described in OpenAPI ("Operator justification… audited"), and the comment in `catalog.go` says "audited when AllowNearDuplicate is set; WP-04 accountability" — but nothing enforces non-emptiness.

**Expected behaviour.** When the override flag is set, a non-empty reason must be supplied (422 otherwise), so every dedup bypass in the audit trail is accountable.

**Impact.** Audit trail can record overrides with no justification, weakening the accountability control the package introduced.

**Recommended remediation.** In `CreateLocation` validation: if `AllowNearDuplicate && strings.TrimSpace(OverrideReason) == ""` → `ValidationError{override_reason: "required when allow_near_duplicate is true"}`. Note the conditional requirement in the OpenAPI schema description.

**Required tests.** Unit: override without reason → 422; override with whitespace-only reason → 422. Integration: API-level 422 and audit assertion for the accepted case (already exists).

**Acceptance condition.** Both tests green; live override-without-reason returns 422.

---

## DRB-WP04-004 — Status lifecycle over-permissive: reserved `archived` reachable via API; no transition validation

| Attribute | Value |
|-----------|-------|
| Severity | **Medium** |
| Discipline | API Architect / Domain review |
| Affected requirement | Domain architecture §2.3 lifecycle (`active → disabled`, `archived` reserved); API requirements §4.1 PATCH "enable/disable" |
| Affected files | `internal/catalog/location_service.go` (SetLocationStatus), `api/openapi/openapi.json` (`SetLocationStatusRequest` enum), `internal/api/handlers/location.go` |
| Status | **Resolved (verified re-review 2026-07-23)** — `Status.Settable()` restricts to `active|disabled`; no-op → `StatusTransitionError`→409; enum reduced to `[active,disabled]`; unit + integration (`ArchivedRejected` 422, `NoOpRejected` 409) executed and passed. |
| Owner | Eng |

**Evidence.** Live: `PATCH /locations/{id}/status` with `{"status":"archived"}` → 200; subsequent `archived → active` → 200. `SetLocationStatus` validates only that the value is a known status — any→any transition is accepted, including into the reserved `archived` state, and no-op transitions (e.g. `disabled → disabled`) are persisted and audited as changes. The OpenAPI request enum advertises `archived`.

**Expected behaviour.** Approved lifecycle is `active ↔ disabled` (enable/disable per API doc and UI spec S-12); `archived` is reserved and must not be settable until a future package defines its semantics. Invalid or no-op transitions should be rejected (409/422), not silently audited.

**Impact.** Public contract exposes an undefined state; future packages cannot assume `archived` is unused. No-op transitions pollute the audit trail.

**Recommended remediation.** Restrict the status endpoint to `active|disabled` (service-level whitelist + OpenAPI enum), reject same-state transitions with 409 `conflict` (or 422), or record an ADR/domain amendment explicitly approving `archived` now. Add transition-matrix tests (valid: active→disabled, disabled→active; invalid: to/from archived, same→same).

**Acceptance condition.** Transition tests green; `archived` no longer settable via API (or ADR recorded).

---

## DRB-WP04-005 — Stale documentation: endpoint catalog still lists `timezone` as PUT-mutable

| Attribute | Value |
|-----------|-------|
| Severity | **Medium** |
| Discipline | Developer Experience / Documentation |
| Affected requirement | Documentation consistency (DR-01 resolution); API doc accuracy |
| Affected files | `docs/api/05-endpoint-catalog.md` (PUT `/locations/{id}` row: body `{name?, timezone?}`; POST row missing `override_reason`), `README.md` (endpoint list omits PUT/PATCH) |
| Status | **Resolved (verified re-review 2026-07-23)** — endpoint catalog PUT row → `{name}` + immutability note; POST row adds `override_reason? (required when allow_near_duplicate)`; PATCH row → `{active|disabled}` + 409; README endpoint list updated. No doc claims timezone mutable. |
| Owner | Eng |

**Evidence.** The WP-04 changeset correctly fixed `docs/api/00-api-requirements.md` (PUT = name only, per DR-01), but `05-endpoint-catalog.md` line 32 still documents the PUT body as `{name?, timezone?}` — directly contradicting the implemented and approved immutability rule. The POST row omits `override_reason`. README's "What's implemented" endpoint list predates the new PUT/PATCH routes.

**Impact.** A client developer reading the endpoint catalog would send `timezone` on PUT and misread the immutability contract. Materially misleading on exactly the point DR-01 resolved.

**Recommended remediation.** Correct the PUT row to `{name}` with an immutability note; add `override_reason?` to the POST row; extend the README endpoint list with `PUT /locations/{id}` and `PATCH /locations/{id}/status`.

**Acceptance condition.** No doc describes timezone as mutable; README lists the WP-04 routes.

---

## DRB-WP04-006 — Repository `Update` SQL writes the immutable `timezone` column (prototype leftover)

| Attribute | Value |
|-----------|-------|
| Severity | **Low** |
| Discipline | Database Architect / Code quality |
| Affected requirement | Domain architecture §2.3 immutability (application-enforced per table-design doc) |
| Affected files | `adapters/persistence/catalogpg/location.go` (`Update`) |
| Status | Open (non-blocking) — re-review 2026-07-23: **not fixed**; statement still `SET name = $2, timezone = $3`. Out of the 001..005 remediation scope; remains a Low hygiene item. |
| Owner | Eng |

**Evidence.** `UPDATE locations SET name = $2, timezone = $3 WHERE id = $1`. The service never mutates `loc.Timezone` (the prototype's `Timezone *string` input was removed in this package), so the statement writes back the loaded value — no behavioural deviation today, and `TestUpdateLocation_NameOnly` asserts immutability. The SQL nonetheless invites a future invariant breach and contradicts the documented rule.

**Recommended remediation.** `UPDATE locations SET name = $2 WHERE id = $1`.

**Acceptance condition.** Statement updated; existing tests green.

---

## DRB-WP04-007 — `ListLocations` silently coerces malformed `active` query parameter

| Attribute | Value |
|-----------|-------|
| Severity | **Low** |
| Discipline | API Architect |
| Affected requirement | Input validation conventions (docs/api/02-response-conventions.md — malformed input → 422) |
| Affected files | `internal/api/handlers/location.go` (ListLocations) |
| Status | Open (non-blocking) — re-review 2026-07-23: **not fixed**; still `b, _ := strconv.ParseBool(v)`. Out of the 001..005 remediation scope; remains a Low item. |
| Owner | Eng |

**Evidence.** `b, _ := strconv.ParseBool(v)` — `GET /locations?active=banana` silently behaves as `active=false` (returns only non-active locations) instead of 422. Similarly `limit=abc` silently falls back to the default (acceptable for limit, but the boolean changes filter semantics).

**Recommended remediation.** Return a 422 `validation` problem for non-boolean `active` values.

**Acceptance condition.** Malformed `active` → 422; test added.

---

## Re-review findings (2026-07-23) — `DRB-WP04-RR-*`

Discovered during re-review when the integration suite was executed (Docker was available this time; the original review could not run it). **Both are pre-existing, not caused by the remediation.**

### DRB-WP04-RR-001 — `TestAPI_DuplicateOverrideWithReason` asserts the wrong audit type (string vs boolean)

| Attribute | Value |
|-----------|-------|
| Severity | **Medium** |
| Discipline | QA / Test quality |
| Affected files | `test/integration/location_test.go:68` |
| Origin | Original WP-04 test commit `c412fee` (pre-remediation) |
| Status | **Resolved (final remediation 2026-07-23, branch `fix/wp04-final-review`)** — assertion now type-checks a JSON boolean (`bool` true), not the string `"true"`. Focused test + full suite green. |

**Evidence.** `assert.Equal(t, "true", details["allow_near_duplicate"])` expects the string `"true"`, but the audit `details` JSON stores a boolean, so `details["allow_near_duplicate"]` unmarshals to Go `bool(true)`. The test fails deterministically (isolated and in-suite). **Production behaviour is correct** (audit records a boolean); the assertion is wrong. Latent until now because the first review could not execute the integration suite.

**Remediation applied (test-only).** `test/integration/location_test.go` now does a type assertion `allow, ok := details["allow_near_duplicate"].(bool)`, `require.Truef(ok, …%T…)`, and `assert.True(allow, …)`. This verifies the value is a **JSON boolean** and is **true** — the test now fails if the field is missing, `false`, or a string. Production audit payload is unchanged (`in.AllowNearDuplicate` remains a Go `bool` in `location_service.go`).

**Test evidence.** `go test -tags integration -run TestAPI_DuplicateOverrideWithReason ./test/integration/` → **PASS** (0.17 s); full `make test-integration` → **PASS** (see RR-002 repeated-run table).

**Acceptance condition.** `TestAPI_DuplicateOverrideWithReason` green; full integration suite green. **MET.**

### DRB-WP04-RR-002 — Integration suite is flaky under container contention

| Attribute | Value |
|-----------|-------|
| Severity | Low |
| Discipline | Test infrastructure |
| Affected files | `test/integration/*` (per-test PostgreSQL containers) |
| Origin | Test-infra design (pre-existing) |
| Status | **Resolved (final remediation 2026-07-23, branch `fix/wp04-final-review`)** — single package-wide container via `TestMain`; per-test isolated database. 5 consecutive full-suite runs green; no leaked containers. |

**Evidence (reproduction & root cause).** The board observed two full-suite runs failing on *different* unrelated tests (`TestAPI_ValidationErrorShape`, then `TestSkipLockedClaim`); both pass in isolation. Root cause: the suite started **one PostgreSQL container per test (~28 per run)** and `t.Cleanup` fired `container.Terminate` asynchronously with a background context, so one container's teardown overlapped the next container's startup. Under constrained CI (ubuntu-latest, 2 vCPU / ~7 GB) plus Ryuk-reaper churn, that overlap produced intermittent readiness timeouts / connection resets on unrelated tests. On a well-resourced local machine the failures did not reproduce in 4 baseline runs (≈40 s each), consistent with a resource-pressure race rather than a code defect. No `t.Parallel()`, shared global clients, or fixed host ports were involved (testcontainers assigns random ports; each test builds its own pool).

**Remediation applied.** `test/integration/setup_test.go`: a `TestMain` owns a single PostgreSQL 16 container for the whole package (started once, terminated once, synchronously). `startPostgres` now `CREATE DATABASE`s a uniquely-named database (`it_<pid>_<counter>`) inside that shared container, migrates and seeds it independently, and `DROP DATABASE … WITH (FORCE)` on `t.Cleanup`. Every test therefore still gets a pristine, fully-isolated database; container churn (and its teardown/startup overlap) is eliminated. All 28 call sites are unchanged. Parallelism was **not** disabled (there was none to disable); no retries, sleeps, skips, or weakened assertions were added.

**Repeated-run evidence (single container, per-test DB).**

| Run | Command | Result | Duration |
| --: | ------- | ------ | -------: |
| 1 | `go test -tags integration -count=1 ./test/integration/` | ✅ PASS | ~9 s |
| 2 | same | ✅ PASS | ~7 s |
| 3 | same | ✅ PASS | ~8 s |
| 4 | same | ✅ PASS | ~6 s |
| 5 | same | ✅ PASS | ~7 s |

No unrelated test failed across the five runs; no leaked `postgres:16-alpine` test containers remained afterward.

**Acceptance condition.** Suite deterministically green across repeated runs; isolation preserved. **MET.**

---

## Review-environment limitation (original review — superseded by re-review)

The original review could not run the testcontainers integration suite (Docker unavailable). **Re-review update (2026-07-23):** Docker was available; the WP-04 remediation integration tests (concurrency, override-422, status lifecycle) were **executed against real PostgreSQL 16 and passed**. The full suite is red only due to DRB-WP04-RR-001 (pre-existing test-assertion bug) and DRB-WP04-RR-002 (flakiness). **TC-04 (CI green on the pushed branch) remains unsatisfied** — the remote rejects authentication and CI results are not inspectable in this environment; state cannot move to Accepted until real pushed-branch CI evidence exists (delivery re-review §11).
