# ForecastIQ — WP-19 Authentication and Authorization Integration: Implementation Report

**Version**: 1.0
**Implementation date**: 2026-07-24
**Work package**: WP-19 — Authentication and Authorization Integration
**Authority**: `docs/planning/05-implementation-work-packages.md` §WP-19; `docs/api/07-authentication-and-authorization.md`; `docs/security/01-ui-authorization-matrix.md`; ADR-008; ADR-017
**Branch**: `feature/wp19-auth-integration` (base: `main` `9cb01dc`)
**Status**: **Implementation Complete — Not Accepted** (Delivery Review Board transition only)

> **Scope decision (operator-confirmed, thinnest option):** this session wires **real** authentication/authorization to the existing route surface and adds the self-service endpoints + the full authorization-matrix tests. **Deferred to WP-19b:** Supabase Admin API propagation (ban/delete), GDPR export (`export_jobs` migration + `POST /me/export` + `DELETE /me`), `/admin/users`, and the signed Supabase auth-webhook receiver. No migration/schema change in this package.

---

## 1. Executive summary

Delivered across three commits, each an independently green slice:

- **Slice 1 (`535c2e8`)** — the middleware chain becomes real. `RequireAuth` resolves a `Bearer` JWT (`identity.UserService.Authenticate`, provisioning on first use) or an `X-API-Key` (`identity.APIKeyService.AuthenticateAPIKey`) to a **database-backed** principal; `RequireRole` reads the role **per request** (ADR-017 — admin disable is immediately effective); `RequireScope` gates API-key scopes (a JWT session carries full user rights). The static dev-admin-token seam is removed from routing. `respond.Principal` is enriched (`WorkspaceID`/`Email`/`Method`/`Scopes`) with a `HasScope` helper. The router now guards the admin group with `RequireAuth + RequireRole("admin")`, gates raw `/forecasts/latest` behind `read:data`, and adds the `/me` + `/api-keys` self-service group (all `no-store`). A **bootstrap-admin** seed (`FIQ_BOOTSTRAP_ADMIN_SUBJECT`/`_EMAIL`) promotes the first admin so the operator surface is reachable. Identity domain errors map to 401/404. `app.go` and the integration harness construct identity **before** the router; the harness seeds the admin backing the shared test token.
- **Slice 2 (`42793b7`)** — OpenAPI: three new paths (`/me` GET+PATCH, `/api-keys` GET+POST, `/api-keys/{id}` DELETE) with bearer/apiKey security; a new `apiKeyAuth` (`X-API-Key`) security scheme; `/forecasts/latest` marked as gated raw data. Drift-gate required-path list extended in the Makefile + CI (**23 paths**).
- **Slice 3 (`d886780`)** — authorization-matrix integration tests: public/user/admin × endpoints, API-key scope gating, object-level key ownership, and disabled-user immediacy.

## 2. Authorization + selection

| Check | Evidence | Result |
|-------|----------|--------|
| WP-03 Accepted (identity: verifiers, `Authenticate`, `AuthenticateAPIKey`, audit) | registry line 3 | ✅ |
| WP-15/18 route surface Accepted (routes to gate) | registry lines 15, 18 | ✅ |
| WP-18 Accepted + merged (base) | PR #16 merged `9cb01dc` | ✅ |

## 3. Scope reconstruction (§WP-19, operator-confirmed subset)

| # | Approved scope item | This package | Result |
|---|---------------------|--------------|--------|
| S1 | `RequireAuth`/`RequireRole`/`RequireScope` on all routes (matrix-driven) | real middleware wired to identity; admin group + raw-data + self-service rewired | ✅ |
| S5 | Auth audit events | reuses identity/audit (`user.provisioned`, `apikey.create/revoke`, `user.update_profile`) written in the module txns | ✅ |
| — | `/me` + `/api-keys` self-service (route surface for the matrix) | GET/PATCH `/me`, GET/POST `/api-keys`, DELETE `/api-keys/{id}` | ✅ |
| S2 | Supabase project hardening config (documented + applied) | **deferred to WP-19b** (vendor-side config; documented) | ⤳ |
| S3 | Auth webhook receiver (signed) | **deferred to WP-19b** (see §6 discrepancy) | ⤳ |
| S4 | `POST /me/export` + `DELETE /me` (Supabase admin propagation) | **deferred to WP-19b** (`export_jobs` migration + admin API) | ⤳ |

## 4. Design notes

- **Role/status from the database, never the token** (ADR-017 / security §7): the verifier asserts only subject+email; the principal's role and active status come from `users`, read on every request. Verified by the disabled-user test (immediate 401).
- **Dev vs production auth** is unchanged from WP-03: `AuthDevMode` selects the dev verifier (compiled out of release builds) or the Supabase JWKS verifier. In dev the bootstrap-admin subject is the dev token prefixed `dev|`.
- **API-key scopes** (AUTH-05): a JWT session has full user rights (`HasScope` short-circuits true); an API key must carry the scope, with `read:public` implicit. `read:data` gates the raw `/forecasts/latest` surface.
- **Object-level ownership** (BOLA, threat model §3): `RevokeKey` returns `ErrKeyNotFound` (→ 404) for a non-owned/unknown key — no existence disclosure.
- **Credential safety**: `/api-keys` responses never include `key_hash`; the plaintext is returned exactly once at creation (`key`), never re-derivable.
- **Bootstrap admin**: the `DevAdminToken` config field is retained only as a production safety guard (must be unset in production); it no longer grants access on its own.

## 5. Files changed

- **Middleware/principal**: `internal/api/middleware.go` (Auth + RequireAuth/RequireRole/RequireScope), `internal/api/respond/context.go` (enriched Principal + HasScope), `internal/api/respond/errors.go` (identity error mapping).
- **Router/handlers**: `internal/api/router.go` (rewiring), `internal/api/handlers/handlers.go` (UserProfile/APIKeyManager ports + fields), `internal/api/handlers/identity.go` (new — `/me` + `/api-keys` handlers).
- **Wiring/config/seed**: `cmd/forecastiq/app.go` (identity before router; Auth), `cmd/forecastiq/seed.go` + `serve.go` (`seedBootstrapAdmin`), `internal/platform/config/config.go` (`AuthBootstrapAdminSubject/Email`), `.env.example`, `.env.local`.
- **API contract**: `api/openapi/openapi.json` (3 paths + apiKeyAuth + gated `/forecasts/latest`), `Makefile`, `.github/workflows/ci.yml` (drift-gate list → 23 paths).
- **Tests**: `test/integration/setup_test.go` (Auth wiring + `seedTestAdmin`), `test/integration/authz_matrix_test.go` (new), plus `api_test.go`/`location_test.go` (authenticate the now-gated `/forecasts/latest` calls).

## 6. Recorded discrepancy (webhook)

`docs/security/05-audit-requirements.md` §56 states the signed Supabase auth-webhook receiver is implemented in MVP (HMAC, timing-safe, non-blocking); `docs/security/02-threat-model.md` §7 (SSRF) states "no webhook receivers in MVP". **Resolution:** the threat-model line scopes to the *provider-collection* pipeline (no outbound user-supplied URL is fetched); the auth webhook is an *inbound, signature-verified* receiver — a distinct concern that is not an SSRF vector. The receiver is deferred to WP-19b together with the GDPR flows (operator-confirmed), and §05 remains the governing spec for it.

## 7. Local gate

- `gofmt -l` clean; `go build ./...` clean; `go vet ./...` clean (only the unrelated cgo `go-m1cpu` warnings); `make lint` (`golangci-lint run ./...`, depguard boundaries) clean.
- `go test -race ./...` unit **ok**.
- `go build -tags integration ./test/...` + `go vet -tags integration ./test/integration/` clean (Docker unavailable locally, so the real-PG matrix runs in CI's `backend-integration`).
- `make docs` → **OpenAPI valid: 23 paths**.

## 8. CI evidence

**CI run 30065631322** (`pull_request`, head `7be85e02fbf01054d68d7dacb9a955a81320cdb6` / `7be85e0`) — **success**, all six mandatory jobs green (none skipped/cancelled): `backend-checks`, `backend-integration` (real PG16 ran the authorization-matrix suite + the existing identity tests), `migrations`, `api-contract` (23-path drift gate), `security`, `image`. Commit identity verified: **local == `git ls-remote origin` == CI head == `7be85e0`**.

Two earlier runs surfaced **test-harness-only** defects (no product-code change), fixed and re-verified:
- Run 30065209115 (`8b48e9e`): `seedTestAdmin` ran inside `newTestEnv` **before** `seedCatalog`, so the `users_workspace_id` FK had no `workspaces` row (SQLSTATE 23503). Fixed in `31afb39` by upserting the system workspace first.
- Run 30065436279 (`31afb39`): `TestProvisioningIdempotency` counted **all** users (the seeded bootstrap admin made it 2). Fixed in `7be85e0` by scoping the count to alice's subject (the property under test).
