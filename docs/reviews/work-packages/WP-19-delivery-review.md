# ForecastIQ — WP-19 Authentication and Authorization Integration: Delivery Review Board

**Review date**: 2026-07-24
**Work package**: WP-19 — Authentication and Authorization Integration (core scope)
**Reviewed SHA**: `7be85e02fbf01054d68d7dacb9a955a81320cdb6` (`7be85e0`)
**Decision**: **ACCEPTED**

---

## 1. Verification of evidence

| Check | Result |
|-------|--------|
| Commit identity: local HEAD == `git ls-remote origin` == CI head | ✅ all `7be85e0` |
| CI run **30065631322** (`pull_request`, head `7be85e0`) | ✅ **success** |
| Six mandatory jobs green, none skipped/cancelled | ✅ `backend-checks`, `backend-integration`, `migrations`, `api-contract`, `security`, `image` |
| Dependency gate: WP-03 + WP-15/18 route surface Accepted | ✅ (registry lines 3, 15, 18) |
| Prior red runs (23503 FK; total-user count) | test-harness-only; fixed in `31afb39`/`7be85e0`, product code unchanged |

## 2. Scope review (operator-confirmed core: real middleware + self-service + matrix tests)

- **`RequireAuth`/`RequireRole`/`RequireScope`** wired to the identity module (JWT via `UserService.Authenticate`, API key via `APIKeyService.AuthenticateAPIKey`); the static dev-admin-token seam is gone. Role/status are read from the database per request (ADR-017).
- **Route rewiring**: admin group behind `RequireAuth + RequireRole("admin")`; raw `/forecasts/latest` behind `read:data`; `/me` (GET/PATCH) + `/api-keys` (GET/POST/DELETE) self-service (`no-store`). Public set unchanged (AUTH-08).
- **Bootstrap admin** seed makes the operator surface reachable ("first account seeded admin").
- **OpenAPI**: `/me`, `/api-keys`, `/api-keys/{id}` with bearer/apiKey security; `apiKeyAuth` scheme; `/forecasts/latest` marked gated; drift gate at 23 paths.
- **Matrix tests** (real PG): public reachable tokenless; admin 401/403/200 across none/user/admin; self-service; API-key scope gating; object-level key ownership (non-owner revoke → 404); disabled-user immediate 401.
- **Deferred to WP-19b** (operator-confirmed): Supabase Admin API propagation, GDPR export (`export_jobs` + `/me/export` + `DELETE /me`), `/admin/users`, signed auth-webhook. Recorded as a deviation, not a gap.

## 3. Architecture + security assessment

- **Fail-closed**: every auth failure (invalid/expired token, unusable key, disabled user, missing credential) returns a uniform 401 with no oracle. The dev verifier is compiled out of release builds; production config forbids dev-mode and requires JWKS; the JWKS verifier never fails open (JWKS-down → 401).
- **Authoritative role from DB** (ADR-017 / security §7): the token asserts only subject+email; role/status come from `users` on every request — proven by the disabled-user immediacy test.
- **BOLA** (threat model §3): `RevokeKey` returns 404 for non-owned/unknown keys (no existence disclosure).
- **Credential safety**: API-key hash never serialized; plaintext returned once, never re-derivable.
- **Correct dependency direction**: `internal/api` depends inward on `internal/identity`; no cycle. No migration/schema change.

## 4. Adversarial checks (no blocking defect)

- **Public/gated boundary**: the router's public set matches AUTH-08 §2.3 exactly; raw `/forecasts/latest` moved to user+`read:data`; all mutations + admin gated. No unguarded protected route found.
- **Scope semantics**: JWT session → full rights (all scopes); API key limited to granted scopes + implicit `read:public`; `read:data` gate proven both ways (403 without, 200 with) plus revoked-key → 401.
- **Role gate**: user-role principal → 403 on admin; admin → 200; tokenless → 401.
- **Provisioning idempotency + audit**: unchanged WP-03 behavior; the seeded admin does not perturb subject-scoped assertions or audit counts.

## 5. Findings

**DRB-WP19-001 (Low, informational, non-blocking)**: the `FIQ_DEV_ADMIN_TOKEN` config field is now dead in routing — retained only as a production safety guard (it must remain unset in production). A future cleanup may remove it once no runbook references it. No security impact (it grants nothing on its own).

No Critical/High/Medium finding.

## 6. Decision

**ACCEPTED.** WP-19 (core) delivers real identity-backed authentication + authorization on the full route surface, the `/me` + `/api-keys` self-service endpoints, and the complete authorization-matrix integration suite — CI-verified green on the exact code+test SHA `7be85e0` including the `api-contract` drift gate and the real-PG `backend-integration` job.

**Accepted Implementation SHA `7be85e0`.** PR #17 ready to merge to `main`. **WP-19b** (Supabase admin propagation + GDPR export `export_jobs`/`/me/export`/`DELETE /me` + `/admin/users` + signed auth-webhook) and **WP-20 (Frontend Foundation)** become eligible.
