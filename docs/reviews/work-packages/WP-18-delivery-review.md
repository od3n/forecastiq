# ForecastIQ — WP-18 Collection-Health API and Admin Operations: Delivery Review Board

**Review date**: 2026-07-24
**Work package**: WP-18 — Collection-Health API and Admin Operations (core scope)
**Reviewed SHA**: `b18a8410401ce28b3efabe72a422490a9267d718` (`b18a841`)
**Decision**: **ACCEPTED**

---

## 1. Verification of evidence

| Check | Result |
|-------|--------|
| Commit identity: local HEAD == `git ls-remote origin` == CI head | ✅ all `b18a841` |
| CI run **30063273864** (`pull_request`, head `b18a841`) | ✅ **success** |
| Six mandatory jobs green, none skipped/cancelled | ✅ `backend-checks`, `backend-integration`, `migrations`, `api-contract`, `security`, `image` |
| Dependency gate: WP-08 + WP-03 Accepted | ✅ (registry lines 8, 3) |
| Prior red run (30062939998 on `41b2817`) | two defects (Linux unconvert; requested_at-vs-completed_at freshness) fixed in `b18a841`; diff scoped to `usage.go` + `adminpg` + the test |

## 2. Scope review (operator-confirmed core: S1–S4)

- **`GET /admin/health`**: cells (last success from **completed_at**, last status, next scheduled slot, freshness), circuits, observation collector (last obs + `suspect_count_24h` + `locations_covered`), and system (payload statfs, `engine_lag_seconds`, backup/restore) — all from application tables + statfs + status file (operations §4). Admin-guarded; `no-store`. Filters (provider/location/status) verified.
- **Provider/config admin**: enable/disable + config edit, each a tx + audit; `has_credential` boolean, credential reference never accepted/returned (BR-08); archived-422; minute-offset-422.
- **`GET /admin/audit-events`**: keyset trail over the existing reader; reflects a `provider.set_status` mutation; action filter.
- **`POST /admin/recompute`**: reuses the scheduled pipeline (`AnalysisDispatcher.Recompute` shares `run()` with `Dispatch`); emits an `analysis.recompute` audit event.
- **S5 user management + GDPR export**: deferred to WP-19 (operator-confirmed) — Supabase-Auth admin propagation + `export_jobs` land there. Recorded as a deviation, not a gap.

## 3. Architecture + security assessment

- New `admin` operations module owns the cross-cutting read + recompute orchestration; provider/config mutations live in their owning **catalog** module. Correct dependency direction; `adminpg` reads application tables only (operations §4 binding rule honored — no log/metric queries).
- Credential safety (BR-08): the config surface never accepts or returns the credential reference; only `has_credential`.
- Admin mutations are audited transactions (ADR-027); reads parameterized. All admin routes behind `RequireAdmin` + `no-store`. No migration/schema change.
- **Correctness fix** (found in review-cycle CI): health `last_success` now reflects `completed_at` (when we collected), not the forecast issuance `requested_at` — so a just-collected cell reads fresh and freshness is stable across the hour.

## 4. Adversarial checks (no defect found post-fix)

- **Cell derivation**: `forecast_collections ∪ scheduled slots` — manual-trigger and scheduled cells both appear (empty otherwise).
- **Open circuit → cell unavailable**: verified; closed/half-open unaffected.
- **Optional sections degrade**: statfs/backup errors omit the section, never fail the view; missing backup file → omitted (not an error).
- **statfs portability**: single `usage.go` compiles conversion-clean on Linux (Bsize int64) and Darwin (Bsize uint32); Blocks/Bavail uint64 on both.
- **Freshness stability**: `completed_at`-based, minute-of-hour independent.

## 5. Findings

No Critical/High/Medium/Low finding. The two review-cycle CI issues (Linux unconvert; freshness source) were remediated in `b18a841` and re-verified green.

## 6. Decision

**ACCEPTED.** WP-18 delivers the core Collection-Health + admin surface — `GET /admin/health` (S-10), provider/config admin (S-11), `GET /admin/audit-events` (S-14), and `POST /admin/recompute` (S-13) — CI-verified green on the exact code+test SHA `b18a841` including the `api-contract` drift gate and the real-PG `backend-integration` job. User management and GDPR export are deferred to WP-19 (operator-confirmed; they require Supabase-Auth admin propagation + the `export_jobs` table).

**Accepted Implementation SHA `b18a841`.** PR #16 ready to merge to `main`. **WP-19 (Authentication and Authorization Integration) becomes eligible** — it wires production auth (RequireAuth/RequireRole/RequireScope middleware, Supabase config, webhooks) and the GDPR/user-management flows deferred here; it depends on WP-03 + the route surface from WP-15/18 (all Accepted).
