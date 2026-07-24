# ForecastIQ — WP-19c GDPR Export Subsystem: Delivery Review Board

**Review date**: 2026-07-24
**Work package**: WP-19c — GDPR export subsystem (AUTH-09; deferred from WP-19b)
**Reviewed SHA**: `2dccab7a95423aec799a3dcf5fd730daf56edfc0` (`2dccab7`)
**Decision**: **ACCEPTED**

---

## 1. Verification of evidence

| Check | Result |
|-------|--------|
| Commit identity: local HEAD == `git ls-remote origin` == CI head | ✅ all `2dccab7` |
| CI run **30068633471** (`pull_request`, head `2dccab7`) | ✅ **success** (first run) |
| Six mandatory jobs green, none skipped/cancelled | ✅ `backend-checks`, `backend-integration`, `migrations`, `api-contract`, `security`, `image` |
| Migration `20260801000012` applied by the `migrations` job | ✅ |
| Dependency gate: WP-19b + WP-03 Accepted | ✅ (registry lines 19b, 3) |

## 2. Scope review

- **`POST /me/export`** (self) + **`POST /admin/users/{id}/export`** (admin-on-target) create an account-data export; **`GET /exports/{id}`** streams it (authorized: requester/target/admin; 24h → 410). One-active-per-user (partial unique index → 409, D-06). Content = user row + API-key metadata + own audit events (reconciliation §2.2). All `no-store`.
- Migration `20260801000012` matches the table-design DDL, with the `requested_by` FK correctness deviation below.

## 3. Architecture + security assessment

- **Object-level download authz** (§4): requester OR target OR admin; any other case → 404 (no existence disclosure), mirroring the API-key ownership pattern.
- **Secret hygiene**: exports carry API-key metadata only (prefix/scopes/timestamps) — never hash/plaintext; audit details are write-time sanitized; the download body never contains `key_hash` (test-asserted).
- **Storage**: reuses the filesystem payload store (ADR-019, gzip, atomic write) via a structurally-satisfied `ExportStore` port + an `exports/` key prefix — no new adapter, correct dependency direction.
- **Correctness fix (deviation, justified)**: `requested_by ON DELETE CASCADE` — the documented plain-FK DDL would make any account with a prior export undeletable (blocking the WP-19b AUTH-09 delete). Export jobs are ephemeral personal rows; CASCADE is consistent with "deletion removes only personal rows" (reconciliation §5). `target_user_id` remains `ON DELETE SET NULL`. Verified by `TestExport_DeleteRequesterCascades`.

## 4. Adversarial checks (no blocking defect)

- **One-active guard**: a pending row → next request 409 (verified via a seeded pending job; the partial unique index is the enforcement point).
- **Download authorization**: non-owner → 404; owner + admin → 200; unknown id → 404. Verified.
- **Admin-triggered**: non-admin → 403 (role gate); admin export of a target returns the target's data. Verified.
- **Expiry**: a completed export past `expires_at` → 410 (logical expiry at download). Verified.
- **Delete interaction**: a user with a prior export is deletable and the export row cascades away. Verified.

## 5. Findings

**DRB-WP19c-001 (Low, informational, non-blocking)**: the export is generated **inline** (not via an async scheduler pipeline) and expiry is enforced **logically** at download; the **physical retention sweep** (deleting expired files + reaping stale `pending` rows) is a documented follow-on. For an admin-triggered export of a subsequently deleted user, the file lingers on disk until its 24h expiry (`target_user_id` SET NULL retains the row). No exposure beyond the authorized/expiring download.

No Critical/High/Medium finding.

## 6. Decision

**ACCEPTED.** WP-19c delivers the AUTH-09 GDPR export subsystem — `export_jobs` + `POST /me/export` + admin-triggered export + authorized `GET /exports/{id}` — CI-verified green on the exact code+test SHA `2dccab7` including the migration, the `api-contract` drift gate, and the real-PG `backend-integration` job. The `requested_by` FK fix closes a latent AUTH-09 deletion block.

**Accepted Implementation SHA `2dccab7`.** PR #19 ready to merge to `main`. **The WP-19 auth bucket (19 + 19b + 19c) is complete.** **WP-20 (Frontend Foundation)** remains eligible.
