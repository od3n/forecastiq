# ForecastIQ — WP-18 Collection-Health API and Admin Operations: Implementation Report

**Version**: 1.0
**Implementation date**: 2026-07-24
**Work package**: WP-18 — Collection-Health API and Admin Operations
**Authority**: `docs/planning/05-implementation-work-packages.md` §WP-18; `docs/api/01-screen-api-contracts.md` §9/§11/§13/§14; `docs/operations/01-ui-operational-signals.md` §4; `docs/data/01-query-and-index-requirements.md`
**Branch**: `feature/wp18-collection-health-admin` (base: `main` `e974d10`)
**Status**: **Implementation Complete — Not Accepted** (Delivery Review Board transition only)

> Scope decision (operator-confirmed): this session delivers the **core** admin surface — `GET /admin/health`, provider/config admin, `GET /admin/audit-events`, and `POST /admin/recompute`. **User management** (`/admin/users` disable/delete) and **GDPR export** are deferred to **WP-19**, where the Supabase-Auth admin propagation and the `export_jobs` table land — building stubs now would be throwaway. No migration/schema change.

---

## 1. Executive summary

Delivered across four commits, each an independently green slice:

- **Slice 1 (`4c3d036`)** — `GET /admin/health` (S-10): a new `admin` operations module assembles the operator triage view **purely from application tables + statfs + the backup status file** (operations doc §4 binding rule — never logs/metrics): per provider×location **cell** (last success, last status, next scheduled slot, freshness), provider **circuits**, the **observation collector** (per-location last observation, `suspect_count_24h`, `locations_covered`), and the **system** section (payload volume via statfs, `engine_lag_seconds` = now − max(`accuracy_metrics.calculated_at`), backup/restore status). `adminpg` runs the aggregates; the payload store gains a build-tagged `Usage()`; a `backupstatus` file reader (missing file → section omitted); `FIQ_BACKUP_STATUS_FILE` config.
- **Slice 2 (`3350ff6`)** — provider/config admin (S-11): `PATCH /admin/providers/{id}/status` (active|disabled; archived reserved) and `PATCH /admin/provider-configurations/{id}` (status, minute offset, adapter version, validation state) — each a bounded tx + audit. The config DTO exposes `has_credential` (boolean) and **never** the credential reference (BR-08).
- **Slice 3 (`c5e2da9`)** — `GET /admin/audit-events` (S-14): keyset-paginated audit trail over the existing `audit.Reader`, filterable by action/resource_type/user_id; sanitized details.
- **Slice 4 (`c5e2da9`)** — `POST /admin/recompute` (S-13): runs the analysis pipeline on demand (match → aggregate → rank) via `RecomputeService`, recording an `analysis.recompute` audit event.

All admin endpoints are behind `RequireAdmin` and set `Cache-Control: no-store` (conventions §6: admin operational data is never cached; the 60 s S-10 polling hits cheap assembly directly).

## 2. Authorization + selection

| Check | Evidence | Result |
|-------|----------|--------|
| WP-08 Accepted (scheduler/collection, circuits, schedules) | registry line 8 | ✅ |
| WP-03 Accepted (identity/audit) | registry line 3 | ✅ |
| WP-17 Accepted + merged (base) | PR #15 merged `eca9093` (→ `e974d10`) | ✅ |

## 3. Scope reconstruction (§WP-18)

| # | Approved scope item | This package | Result |
|---|---------------------|--------------|--------|
| S1 | `GET /admin/health` (cells + circuits + next_scheduled_at + observation_collector + system statfs/backup) | full assembly from application tables + statfs + status file | ✅ |
| S2 | Provider status/config endpoints (credential never echoed) | PATCH status + config; `has_credential` boolean only | ✅ |
| S3 | `GET /admin/audit-events` | keyset trail over audit.Reader | ✅ |
| S4 | Recompute endpoint | `POST /admin/recompute` (match→aggregate→rank) + audit | ✅ |
| S5 | User management (lockout guards, Supabase propagation) | **Deferred to WP-19** (Supabase admin API + auth wiring land there) | ⏸ documented |
| Acc | Health assembly < 200 ms (cheap DB aggregates); credential-absence; admin-guard | integration + unit; `has_credential` proves no ref echoed | ✅ (user self-lockout / Supabase-502 paths move with S5 to WP-19) |

## 4. Architecture + key decisions

- **New `admin` operations module** holds the cross-cutting read (HealthService) + the recompute orchestration (RecomputeService); it owns no writes — provider/config mutations live in the **catalog** module (their owner) and are invoked by the API layer. Correct dependency direction; the health repo (`adminpg`) reads application tables only.
- **Operations §4 binding rule honored**: `/admin/health` never queries logs or Prometheus — cells/circuits/observations/engine-lag come from the DB, volume from `statfs`, backup/restore from the status file. Optional volume/backup sections **degrade to omitted** on error rather than failing the view.
- **Credential safety (BR-08)**: the config admin never accepts or returns the credential reference; the DTO exposes only `has_credential`. Secret rotation stays env-side.
- **Recompute** reuses the exact scheduled pipeline (`AnalysisDispatcher.Recompute` shares `run()` with `Dispatch`) so an on-demand recompute is byte-identical to a scheduled batch; the trigger is audited with `records_affected`.
- **Cross-platform statfs**: `diskUsage` is build-tagged (Linux int64 / Darwin uint64 block fields) to keep the shared code conversion-clean on both.

## 5. Tests

| Layer | Test | Proves |
|-------|------|--------|
| Unit | `internal/admin/health_test.go` | assembly of all sections; optional volume/backup degrade-to-omitted on error; repo error fails the view |
| Integration (real PG16) | `test/integration/admin_health_test.go` | admin-guard 401; full-view assembly after a collection+observation (cell + freshness, circuit, observation collector, system); freshness `status` filter |
| Integration | `test/integration/admin_provider_test.go` | provider enable/disable + 401 + archived-422; config update of mutable fields + `has_credential` (no ref) + minute-offset-422 |
| Integration | `test/integration/admin_ops_test.go` | audit-events 401 + reflects a `provider.set_status` mutation + action filter; recompute 401 + runs clean + emits an `analysis.recompute` audit event |

Full `go test -race ./internal/... ./adapters/...` green; `gofmt`/`go vet`/`golangci-lint` clean; `go vet -tags integration ./test/integration/...` compiles (Docker unavailable locally → real-PG runs in CI). `make docs` valid (20 paths).

## 6. Database / API / security

**No migration, no schema change.** New admin GET/PATCH/POST endpoints, all behind `RequireAdmin` and `no-store`. Provider/config mutations are audited transactions; reads are parameterized. The credential reference is never accepted/returned. `FIQ_BACKUP_STATUS_FILE` is the only new config (empty → backup section omitted).

## 7. CI evidence

_To be captured on the pushed branch (six mandatory jobs on the exact code+test SHA); recorded here and in the registry once green._

## 8. Deviations

```text
User management (/admin/users list/disable/delete) and GDPR export
(POST /admin/users/{id}/export) are deferred to WP-19 (operator-confirmed):
they require Supabase-Auth admin propagation and the export_jobs table, both
of which land with the WP-19 auth/GDPR wiring. Building them now would be
throwaway stubs. The self-lockout-409 and Supabase-failure-502 acceptance
paths move with them.
```

## 9. Work-package transition

```text
WP-18 — Collection-Health API and Admin Operations
Previous State: Selected — Not Started
New State: Implementation Complete (core scope; user mgmt + export → WP-19)
Acceptance State: Not Accepted (pending pushed-branch CI + Delivery Review Board)
```

## 10. Recommended next action

```text
Push feature/wp18-collection-health-admin and capture the six mandatory CI
jobs on the exact code+test SHA, then convene the Delivery Review Board.
```
