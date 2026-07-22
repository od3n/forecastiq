# ForecastIQ — WP-04 Location Management: Delivery Review Findings

**Version**: 1.0
**Review date**: 2026-07-22
**Work package**: WP-04 — Location Management
**Reviewed commits**: `19035f9..5445435` (on top of accepted baseline `b78a748`)
**Board decision**: CHANGES REQUIRED (one High finding open)
**Authority**: Delivery Review Board prompt; `docs/planning/05-implementation-work-packages.md` §WP-04

Finding ID scheme: `DRB-WP04-<NNN>`. Statuses: Open | Accepted Risk | Deferred by Approved Decision | Resolved During Review | Not Reproducible.

---

## DRB-WP04-001 — BR-LOC-01 dedup check is not concurrency-safe (TOCTOU)

| Attribute | Value |
|-----------|-------|
| Severity | **High** |
| Discipline | Principal Backend Engineer / SRE / QA |
| Affected requirement | BR-LOC-01; DRB WP-04 guidance "concurrent duplicate prevention" |
| Affected files | `internal/catalog/location_service.go` (CreateLocation tx), `adapters/persistence/catalogpg/location.go` |
| Status | Open |
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
| Status | Open |
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
| Status | Open |
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
| Status | Open |
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
| Status | Open |
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
| Status | Open |
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
| Status | Open |
| Owner | Eng |

**Evidence.** `b, _ := strconv.ParseBool(v)` — `GET /locations?active=banana` silently behaves as `active=false` (returns only non-active locations) instead of 422. Similarly `limit=abc` silently falls back to the default (acceptable for limit, but the boolean changes filter semantics).

**Recommended remediation.** Return a 422 `validation` problem for non-boolean `active` values.

**Acceptance condition.** Malformed `active` → 422; test added.

---

## Review-environment limitation (not a finding)

The testcontainers integration suite (`make test-integration`) could not be executed in the review environment (Docker unavailable). Mitigations applied: the suite compiles under `-tags integration` (`go vet`); every WP-04 behaviour it asserts was re-verified live against the real binary and a real PostgreSQL (14; production target is 16 — no 16-specific DDL identified); unit suite green with `-race`. The WP-04 commits are local-only; CI has not run on them. **CI green on the pushed branch is required before the state moves to Accepted** (tracked in the delivery review §17).
