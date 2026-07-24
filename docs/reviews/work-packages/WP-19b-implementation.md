# ForecastIQ — WP-19b User Lifecycle + Auth Webhook: Implementation Report

**Version**: 1.0
**Implementation date**: 2026-07-24
**Work package**: WP-19b — deferred auth items (user lifecycle + Supabase propagation + signed webhook)
**Authority**: `docs/api/07-authentication-and-authorization.md` §5/§6; `docs/security/01-ui-authorization-matrix.md` §3 (S-14) / §7; `docs/security/05-audit-requirements.md` §4/§5; ADR-008; ADR-017
**Branch**: `feature/wp19b-user-lifecycle-webhook` (base: `main` `08b80b6`)
**Status**: **Implementation Complete — Not Accepted** (Delivery Review Board transition only)

> **Scope decision (operator-confirmed):** this slice delivers **user lifecycle** (`/admin/users` + `DELETE /me`), **Supabase Admin API propagation**, and the **signed auth-webhook receiver**. The **entire GDPR export subsystem** (`export_jobs` migration + `POST /me/export` + authorized download) is **deferred to a later slice (WP-19c)** — a recorded AUTH-09 export gap.

---

## 1. Executive summary

Delivered across three commits, each an independently green slice:

- **Slice 1 (`62cf078`)** — the S-14 user-lifecycle surface (AUTH-09 / ADMIN-05): `GET /admin/users` (keyset list), `PATCH /admin/users/{id}/status` (disable/enable, 409 self-lockout), `DELETE /admin/users/{id}` and `DELETE /me` (account delete). A new `AdminUserService` orchestrates each as an audited operation with **Supabase Admin API propagation** (`SupabaseAdmin` port + HTTP adapter + dev no-op): ban/unban runs **before** the local disable (a propagation failure → **502**, local unchanged; docs/api/07 §6), and the provider delete runs **after** the local delete commits (best-effort; the local table is authoritative). **Migration `20260801000011`** re-creates `audit_events.user_id` FK as `ON DELETE SET NULL` and adds a **session-GUC exemption** (`app.allow_immutable_write`) to `raise_immutable()` — the documented retention-purge GUC mechanism — so the FK-driven anonymization is permitted only for a transaction that opts in (the delete tx sets it via `SET LOCAL`). Repo gains `List`/`UpdateStatus`/`Delete`; errors `ErrSelfLockout` → 409 and `ErrUpstreamAuth` → 502.
- **Slice 2 (`cac5b39`)** — `POST /auth/webhook`: an HMAC-SHA256 signature over the raw body (timing-safe), mounted only when `FIQ_AUTH_WEBHOOK_SECRET` is set. Recognized events map to the `auth.*` audit registry; ingestion is **non-blocking** (valid signature → 204 even if the audit write fails; invalid/missing → 401), so the auth flow never depends on our audit availability (audit-requirements §5).
- **Slice 3 (`this`)** — integration tests + OpenAPI (27 paths) + drift-gate extension.

## 2. Authorization + selection

| Check | Evidence | Result |
|-------|----------|--------|
| WP-19 Accepted + merged (base) | PR #17 merged `08b80b6` | ✅ |
| WP-03 identity (repos, audit, provisioning) Accepted | registry line 3 | ✅ |

## 3. Scope reconstruction

| # | Item | This package | Result |
|---|------|--------------|--------|
| S-14 | List / disable / enable / delete users | `/admin/users` (+ 409 self-lockout) | ✅ |
| AUTH-09 | Self account delete | `DELETE /me` | ✅ |
| §6 | Supabase ban/unban + delete propagation | port + HTTP adapter + dev no-op | ✅ |
| §5 | Signed auth-webhook receiver | `POST /auth/webhook` (HMAC, non-blocking) | ✅ |
| §4 | GDPR audit anonymization on delete | FK `ON DELETE SET NULL` + GUC-exempt trigger | ✅ |
| — | GDPR export (`export_jobs`, `/me/export`, download) | **deferred to WP-19c** | ⤳ |
| — | Admin-triggered user export | deferred with GDPR export | ⤳ |

## 4. Design notes

- **Immutability GUC.** `raise_immutable()` previously raised unconditionally, so an FK-driven `SET NULL` on the immutable `audit_events` would have blocked account deletion. The migration adds a session-scoped exemption; only a transaction that runs `SET LOCAL app.allow_immutable_write = 'on'` (the delete path) can drive the anonymization — all other UPDATE/DELETE stay forbidden. This is also the mechanism the future retention purge will use.
- **Propagation ordering** follows docs/api/07 §6 exactly and differs by operation: disable bans first (fail → 502, no local change); delete deletes locally first, then propagates best-effort.
- **Self-lockout** (SEC-10): the admin surface refuses a self-target for status-change and delete (409); `DELETE /me` is the self-service path and is always allowed.
- **Audit actor anonymization**: on self-delete the deletion audit row's `user_id` is nulled by the same cascade (actor anonymized); details carry `target_user_id` so the record remains meaningful. Admin-delete keeps the admin actor (not deleted).
- **Webhook** is public but signature-gated; unrecognized event types are acknowledged (204) without an audit row; the subject is resolved to a user id best-effort; details carry the subject reference only (never email; sanitization §3).
- **Dev/prod selection**: the Supabase adapter is the no-op in dev-mode or when unconfigured, the real HTTP client otherwise; the webhook route is mounted only when a secret is set. `authSubject` is URL-path-escaped before the admin call (defensive).

## 5. Files changed

- **Migration**: `migrations/20260801000011_alter_audit_gdpr.{up,down}.sql`.
- **Identity**: `internal/identity/admin_user_service.go` (new), `internal/identity/webhook_service.go` (new), `internal/identity/ports/ports.go` (SupabaseAdmin + repo methods), `internal/identity/domain/errors.go` (ErrSelfLockout/ErrUpstreamAuth), `adapters/persistence/identitypg/user.go` (List/UpdateStatus/Delete).
- **Adapters**: `adapters/auth/supabaseadmin/{supabaseadmin.go,noop.go}` (new).
- **API**: `internal/api/handlers/{admin_users.go,webhook.go}` (new) + `handlers.go` (ports/fields), `internal/api/router.go` (routes), `internal/api/respond/errors.go` (409/502 mapping).
- **Wiring/config**: `cmd/forecastiq/app.go`, `internal/platform/config/config.go` (`FIQ_SUPABASE_URL`/`FIQ_SUPABASE_SERVICE_ROLE_KEY`/`FIQ_AUTH_WEBHOOK_SECRET`).
- **API contract**: `api/openapi/openapi.json` (27 paths + `DELETE /me`), `Makefile`, `.github/workflows/ci.yml`.
- **Tests**: `test/integration/user_lifecycle_test.go` (new), `test/integration/setup_test.go` (AdminUserService + WebhookService wiring + `testWebhookSecret`).

## 6. Local gate

- `gofmt -l` clean; `go build ./...` + `go build -tags integration ./test/...` clean; `go vet` clean (only unrelated cgo `go-m1cpu` warnings); `make lint` clean.
- `go test -race ./...` unit **ok**.
- `make docs` → **OpenAPI valid: 27 paths** (Docker unavailable locally; the real-PG lifecycle + webhook tests run in CI `backend-integration`, and `migrations` applies `20260801000011`).

## 7. CI evidence

**CI run 30067494937** (`pull_request`, head `13c1fd9dee1a62b2e668e5291cfe2fd0633d5b63` / `13c1fd9`) — **success on the first run**, all six mandatory jobs green (none skipped/cancelled): `backend-checks`, `backend-integration` (real PG16 ran the user-lifecycle + webhook suite), `migrations` (applied `20260801000011`), `api-contract` (27-path drift gate), `security`, `image`. Commit identity verified: **local == `git ls-remote origin` == CI head == `13c1fd9`**.
