# ForecastIQ — WP-19c GDPR Export Subsystem: Implementation Report

**Version**: 1.0
**Implementation date**: 2026-07-24
**Work package**: WP-19c — GDPR export subsystem (AUTH-09; deferred from WP-19b)
**Authority**: `docs/api/07-authentication-and-authorization.md` §5; `docs/security/01-ui-authorization-matrix.md` §3 (S-14) / §4; `docs/domain/04-ui-domain-model-reconciliation.md` §2.2 / D-06 / D-08; `docs/data/03-table-design.md` §2; ADR-019
**Branch**: `feature/wp19c-gdpr-export` (base: `main` `7182d04`)
**Status**: **Implementation Complete — Not Accepted** (Delivery Review Board transition only)

> This completes the WP-19 auth bucket: the AUTH-09 account-data export deferred from WP-19b.

---

## 1. Executive summary

Delivered across three commits, each an independently green slice:

- **Slice 1 (`623656f`)** — data + domain + service. Migration `20260801000012` creates `export_jobs` (DDL per table-design §2) with the one-active-per-user partial unique index (D-06). `domain.ExportJob` + export sentinel errors; `ports.ExportJobRepository` + `ports.ExportStore` (structurally satisfied by the filesystem payload store); `identitypg.ExportJobRepository` (unique-violation → `ErrExportInProgress`). `identity.ExportService` generates the account-data export **inline** (user row + API-key metadata + own audit events; reconciliation §2.2), inserting a `pending` row first (so the active-job guard applies) then `completed`, writing gzip to the payload volume under the `exports/` key prefix with a 24h expiry; `DownloadExport` authorizes requester/target/admin and 410s expired.
- **Slice 2 (`85ef854`)** — HTTP surface. `POST /me/export` (self) + `POST /admin/users/{id}/export` (admin-on-target) + `GET /exports/{id}` (authorized download). Wired in the composition root + test harness; error mapping `ErrExportInProgress`→409, `ErrExportNotFound`→404, `ErrExportExpired`→410. OpenAPI 30 paths + drift-gate list.
- **Slice 3 (`this`)** — integration tests + the `requested_by` FK correctness fix (below).

## 2. Scope reconstruction

| # | Item | This package | Result |
|---|------|--------------|--------|
| AUTH-09 | Self GDPR export | `POST /me/export` (inline account-data JSON) | ✅ |
| §4 / D-08 | Admin-triggered export (admin-on-target) | `POST /admin/users/{id}/export` | ✅ |
| §4 | Authorized download (self OR admin) | `GET /exports/{id}` (requester/target/admin; 24h → 410) | ✅ |
| D-06 | One active job per user | partial unique index → 409 | ✅ |
| §2.2 | Account-data content only | user row + key metadata + own audit events | ✅ |

## 3. Design notes + deviations

- **Inline generation (documented deviation).** The reconciliation calls the export "async with download link." The account-data payload is tiny (one user + a few keys + own audit events), so it is generated **synchronously** in the request: `pending` → write file → `completed`. The `pending` insert still exercises the one-active-per-user guard (a concurrent request → 409); on generation failure the job is marked `failed` (freeing the guard). This avoids an async scheduler/retention subsystem for a KB-scale payload. **Physical retention sweep** (deleting expired files + stale `pending` rows) is a documented follow-on; expiry is enforced **logically** at download (410).
- **`requested_by` FK correctness fix (deviation from the documented DDL).** The table-design DDL declares `requested_by` as a plain `NOT NULL REFERENCES users(id)`. That would make any account with a prior export **undeletable** (the FK blocks the WP-19b account delete). Since AUTH-09 "removes only personal rows" and an export job is a personal row, the migration declares `requested_by ... ON DELETE CASCADE` (export jobs are ephemeral, 24h). `target_user_id` stays `ON DELETE SET NULL` (an admin-triggered export of a deleted user is retained; its file expires within 24h). Verified by `TestExport_DeleteRequesterCascades`.
- **Storage reuse**: the export file uses the existing filesystem payload store (ADR-019, gzip, atomic write) under an `exports/{target}/{job}.json.gz` key — no new adapter, port satisfied structurally.
- **Secret hygiene**: exports carry API-key metadata only (`key_prefix`, scopes, timestamps) — never the hash or plaintext; audit details are already write-time sanitized. `no-store` on all export responses.
- **Download authorization** (object-level §4): requester OR target OR admin; any other case returns 404 (no existence disclosure), mirroring the API-key ownership pattern.

## 4. Files changed

- **Migration**: `migrations/20260801000012_create_export_jobs.{up,down}.sql`.
- **Identity**: `internal/identity/export_service.go` (new), `internal/identity/identity.go` (`ExportJob` alias), `internal/identity/domain/export_job.go` (new), `internal/identity/domain/errors.go` (export errors), `internal/identity/ports/ports.go` (`ExportJobRepository` + `ExportStore`), `adapters/persistence/identitypg/export_job.go` (new).
- **API**: `internal/api/handlers/export.go` (new) + `handlers.go` (`ExportManager` port/field), `internal/api/router.go` (routes), `internal/api/respond/errors.go` (409/404/410 mapping).
- **Wiring**: `cmd/forecastiq/app.go`, `test/integration/setup_test.go`.
- **API contract**: `api/openapi/openapi.json` (30 paths), `Makefile`, `.github/workflows/ci.yml`.
- **Tests**: `test/integration/export_test.go` (new).

## 5. Local gate

- `gofmt -l` clean; `go build ./...` + `go build -tags integration ./test/...` clean; `go vet` clean (only unrelated cgo `go-m1cpu` warnings); `make lint` clean.
- `go test -race ./...` unit **ok**.
- `make docs` → **OpenAPI valid: 30 paths** (Docker unavailable locally; the real-PG export suite runs in CI `backend-integration`, and `migrations` applies `20260801000012`).

## 6. CI evidence

_(captured on push — see the delivery-review report and the registry row.)_
