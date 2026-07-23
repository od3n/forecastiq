# ForecastIQ — WP-03 Identity and Workspace Foundation: Implementation Report

**Version**: 1.0
**Implementation date**: 2026-07-23
**Work package**: WP-03 — Identity and Workspace Foundation
**Authority**: `docs/planning/05-implementation-work-packages.md` §WP-03; ADR-008 (Supabase Auth / JWKS); ADR-009 (single-workspace MVP); `docs/data/03-table-design.md` §2; `docs/security/01-ui-authorization-matrix.md`
**Branch**: `feature/wp03-identity-workspace` (base: accepted WP-08 tip `9620f1d`)
**Status**: **Implementation Complete — Not Accepted** (Delivery Review Board transition only)

> Scope discipline: this package delivers the identity application layer, the JWKS verifier adapter, API-key management, the audit reader, and the identity schema. **No HTTP routes** are added — handlers land in WP-15 and route/middleware wiring in WP-19 (per the WP-03 definition). No `export_jobs`/GDPR work (WP-19).

---

## 1. Executive summary

- **Objective**: user provisioning, JWT verification (JWKS), API keys, roles, and the audit reader.
- **Delivered**:
  1. **Migration** `20260801000006_create_identity` — `user_role` enum, `users` and `api_keys` tables, and the `audit_events.user_id` FK that WP-02 explicitly deferred (see DR-03).
  2. **identity module** — domain (`User`, `APIKey`, `Role`), ports (`TokenVerifier`, repositories), `UserService` (verify → **provision-on-first-use** → login), `APIKeyService` (**argon2id**, plaintext shown once, hash never returned, owner-only revoke, key authentication).
  3. **JWKS verifier adapter** (`adapters/auth/jwks`) — verifies **RS256 + ES256** using only stdlib crypto (no third-party JWT dependency), with a cached, **rotation-tolerant** key set (unknown `kid` triggers a rate-limited refresh).
  4. **Dev-mode verifier** (`adapters/auth/devauth`) — local-only token verifier, **build-tag excluded from release** (`-tags release` compiles a fail-closed stub).
  5. **Audit reader** — `audit.Reader`/`ReaderService` + `auditpg.Store.List` (keyset, newest-first, filterable). The recorder pre-existed.
- **Security posture**: the authoritative **role is read from the database, never from a token claim**; API-key hashes never leave the DB except on the authentication path; failures on key auth return a single non-oracle error.

## 2. Authorization and selection

| Check | Evidence | Result |
|-------|----------|--------|
| WP-08 Accepted; WP-03 selected | Registry (Selection Board, 2026-07-23) | ✅ |
| Hard dependency WP-02 Accepted | Registry | ✅ |
| WP-03 definition located | `05-implementation-work-packages.md` §WP-03 | ✅ |
| Schema authority | `docs/data/03-table-design.md` §2 + audit migration header | ✅ |

## 3. Recorded discrepancy (DR-03)

The WP-03 summary says **"DB changes: None (schema from WP-02)"**, but WP-02 never created `users`/`api_keys`, and the audit migration header explicitly defers the `users` table + `user_id` FK to WP-03. **Resolution**: the table-design doc + migration header are authoritative; WP-03 adds the identity migration. `export_jobs` remains deferred to WP-19. Recorded in the status registry (Recorded discrepancies, DR-03).

## 4. Architecture

Dependency direction preserved. `domain/` is stdlib-only (argon2 lives in the application layer, not domain). The `TokenVerifier` is an identity **port**; the JWKS + dev verifiers are **adapters** implementing it; the composition root selects one by config. Persistence adapters implement the identity repository ports. No internal package imports adapters (depguard clean). No new module dependency; argon2id uses the already-present `golang.org/x/crypto` (now a direct require after `go mod tidy`).

## 5. Requirement → test traceability

| Requirement | Implementation | Test | Level |
|-------------|----------------|------|-------|
| JWKS verify: valid | `jwks.Verify` | `TestVerify_ValidRS256`, `TestVerify_ValidES256` | unit |
| JWKS verify: expired | expiry + leeway check | `TestVerify_Expired` | unit |
| JWKS verify: wrong issuer | issuer check | `TestVerify_WrongIssuer` | unit |
| JWKS verify: wrong audience | audience check | `TestVerify_WrongAudience` | unit |
| JWKS verify: unknown kid | refresh then reject | `TestVerify_UnknownKID` | unit |
| JWKS verify: bad signature / malformed | signature verify | `TestVerify_BadSignature`, `TestVerify_Malformed` | unit |
| JWKS key rotation | refresh on unknown kid | `TestVerify_KeyRotation` | unit |
| argon2id hash/verify, no plaintext leak | `hashKey`/`verifyKey` | `TestHashVerifyRoundTrip`, `TestHashSaltIsRandom` | unit |
| Provisioning idempotency | `resolveOrProvision` | `TestProvisioningIdempotency` | integration |
| Role/status from DB (disabled denied) | `Authenticate` | `TestDisabledUserDenied` | integration |
| Key create (plaintext once) / list (no hash) / key auth / owner-only revoke | `APIKeyService` | `TestAPIKeyLifecycle` | integration |
| Audit emission per action | recorder in tx | `TestProvisioningIdempotency`, `TestAPIKeyLifecycle` | integration |
| Audit reader (filter, newest-first) | `audit.ReaderService` + `auditpg.List` | `TestAuditReader` | integration |

## 6. Database changes

Migration `20260801000006_create_identity` (up/down). Up: `user_role` enum; `users` (+updated_at trigger); `api_keys` (+user index); `audit_events` `user_id` FK. Down: drop FK → api_keys → users → enum. Seed unaffected (no user/key seed; provisioning is on-first-use). Admin promotion of a provisioned user is a WP-18 user-management concern; locally the existing dev-admin-token seam grants admin at the HTTP layer.

## 7. API changes

```text
No HTTP routes added.
```

Handlers for `GET/PATCH /me` and `/api-keys` CRUD land in WP-15; production middleware/route wiring lands in WP-19. This package delivers and tests the use cases and the verifier the API layer will consume.

## 8. Security

- Role/status authoritative from the database (`Authenticate` loads the user; a disabled user is denied).
- API-key secrets are argon2id-hashed (PHC-encoded, per-key salt); the hash is excluded from list/get and from the creation result; only `GetByPrefix` (auth path) returns it.
- Key authentication uses constant-time comparison and returns a single `ErrInvalidCredential` for unknown-prefix / wrong-secret (no oracle).
- Dev-mode auth is compiled out of release builds and fails closed there; production config forbids dev-mode and requires a JWKS URL.
- No credentials/tokens are logged; JWT verification never surfaces cryptographic internals.

## 9. Validation results

Docker unavailable in this environment — the integration suite runs in CI (mirroring prior WP reviews).

| Command | Result |
|---------|--------|
| `gofmt` / `make fmt-check` | ✅ clean |
| `go vet ./...` (+ integration tag) | ✅ clean |
| `go build ./...` and `go build -tags release ./...` | ✅ both |
| `go test -race ./...` (unit) | ✅ all pass (jwks matrix + argon2) |
| `golangci-lint run ./...` | ✅ zero findings |
| `make docs` | ✅ 9 paths |
| `go test -tags integration ./...` | ⏳ **CI to confirm** (no local Docker) |

## 10. Files changed

- **Migration**: `migrations/20260801000006_create_identity.{up,down}.sql`
- **Identity module**: `internal/identity/{identity.go,credential.go,user_service.go,apikey_service.go}`, `internal/identity/domain/{user.go,apikey.go,errors.go}`, `internal/identity/ports/ports.go`
- **Adapters**: `adapters/auth/jwks/jwks.go`, `adapters/auth/devauth/{devauth.go,devauth_release.go}`, `adapters/persistence/identitypg/{user.go,apikey.go}`
- **Audit reader**: `internal/audit/reader.go`, `adapters/persistence/auditpg/reader.go`
- **Composition / config**: `cmd/forecastiq/app.go`, `internal/platform/config/config.go`, `.env.example`, `go.mod`/`go.sum`
- **Tests**: `internal/identity/credential_test.go`, `adapters/auth/jwks/jwks_test.go`, `test/integration/{setup_test.go,identity_test.go}`
- **Docs**: this report; `docs/planning/06-work-package-status-registry.md` (state + DR-03)

## 11. Deviations

DR-03 (migration added despite the "DB changes: None" summary) — resolved and recorded. No other approved-scope deviations.

## 12. Known limitations

- No HTTP surface yet (by design: WP-15/WP-19).
- Admin role assignment is manual/WP-18; provisioning always creates role=`user`.
- Only RS256/ES256 are accepted (Supabase asymmetric algorithms); HS256 is intentionally unsupported (no shared-secret verification).

## 13. Work-package transition

```text
WP-03 — Identity and Workspace Foundation
Previous State: Selected — Not Started
New State: Implementation Complete
Acceptance State: Not Accepted
```

## 14. Recommended next action

```text
Push the branch to capture CI evidence, then convene the Delivery Review Board for WP-03.
```
