# ForecastIQ — WP-19b User Lifecycle + Auth Webhook: Delivery Review Board

**Review date**: 2026-07-24
**Work package**: WP-19b — deferred auth items (user lifecycle + Supabase propagation + signed webhook)
**Reviewed SHA**: `13c1fd9dee1a62b2e668e5291cfe2fd0633d5b63` (`13c1fd9`)
**Decision**: **ACCEPTED**

---

## 1. Verification of evidence

| Check | Result |
|-------|--------|
| Commit identity: local HEAD == `git ls-remote origin` == CI head | ✅ all `13c1fd9` |
| CI run **30067494937** (`pull_request`, head `13c1fd9`) | ✅ **success** (first run) |
| Six mandatory jobs green, none skipped/cancelled | ✅ `backend-checks`, `backend-integration`, `migrations`, `api-contract`, `security`, `image` |
| Migration `20260801000011` applied by the `migrations` job | ✅ |
| Dependency gate: WP-19 + WP-03 Accepted | ✅ (registry lines 19, 3) |

## 2. Scope review (operator-confirmed)

- **User lifecycle (S-14 / AUTH-09)**: `GET /admin/users`, `PATCH /admin/users/{id}/status` (disable/enable), `DELETE /admin/users/{id}`, `DELETE /me` — each admin-guarded (or self), audited, `no-store`.
- **Supabase Admin API propagation (§6)**: `SupabaseAdmin` port + HTTP adapter + dev no-op; ban before local disable (fail → 502, local unchanged), delete after local commit (best-effort).
- **Signed webhook (§5)**: `POST /auth/webhook` HMAC-gated, non-blocking, `auth.*` audit mapping; mounted only with a secret.
- **GDPR audit anonymization (§4)**: audit FK `ON DELETE SET NULL` + a session-GUC exemption on `raise_immutable()`.
- **Deferred to WP-19c** (operator-confirmed): the GDPR export subsystem (`export_jobs` + `/me/export` + download + admin-triggered export). Recorded as a deviation (AUTH-09 export gap), not a silent omission.

## 3. Architecture + security assessment

- **Immutability preserved, exemption controlled.** `raise_immutable()` still forbids UPDATE/DELETE by default; the new `app.allow_immutable_write` GUC is set only via `SET LOCAL` inside the account-delete transaction, so the FK-driven anonymization is the only path that mutates `audit_events`, and only for that transaction. Down migration restores the unconditional trigger + plain FK.
- **Correct propagation ordering** per docs/api/07 §6, verified to differ by operation (ban-first for disable; local-first for delete). Local table stays authoritative; a disabled user is already denied by role/status-from-DB independent of the ban.
- **Credential/secret hygiene**: service-role key is env-only; the webhook secret gates the route; `authSubject` is URL-path-escaped before the admin call (no path injection). Audit details carry subject references only (never email; sanitization §3).
- **Correct dependency direction**: `SupabaseAdmin`/webhook are ports in `internal/identity`; adapters live under `adapters/auth`. No cycle. Migration is the only schema change.

## 4. Adversarial checks (no blocking defect)

- **Self-lockout**: admin self-target on status + delete → 409; `DELETE /me` self-service allowed. Verified.
- **Disable immediacy**: a disabled user's next call → 401 (role/status from DB). Verified.
- **Delete cascade + anonymization**: `api_keys` cascade to zero; the provisioning audit row is preserved with `user_id` NULL; the `admin.user_deleted` row keeps the admin actor. Verified.
- **Webhook**: valid HMAC → 204 + `auth.login` audit row; bad/missing signature → 401 with no audit row; unrecognized event type → 204 with no audit row. Verified (timing-safe compare via `hmac.Equal`; fail-closed on empty secret/header).

## 5. Findings

**DRB-WP19b-001 (Low, informational, non-blocking)**: the `app.allow_immutable_write` exemption is global to every table guarded by `raise_immutable()`, not scoped to `audit_events`. Mitigated by the transaction-local (`SET LOCAL`) scope and the single call site (account delete). A future refinement could scope the GUC per-table; no current exposure.

No Critical/High/Medium finding.

## 6. Decision

**ACCEPTED.** WP-19b delivers the deferred user-lifecycle surface (`/admin/users` + `DELETE /me`), Supabase Admin API propagation, and the signed auth-webhook receiver — CI-verified green on the exact code+test SHA `13c1fd9` including the migration, the `api-contract` drift gate, and the real-PG `backend-integration` job.

**Accepted Implementation SHA `13c1fd9`.** PR #18 ready to merge to `main`. **WP-19c** (GDPR export subsystem) and **WP-20 (Frontend Foundation)** remain eligible.
