# ForecastIQ — Work-Package Status Registry

**Version**: 1.0
**Status**: Living document — updated at each work-package completion
**Authority**: `docs/planning/05-implementation-work-packages.md` (definitions); delivery reports (evidence)

State model: Not Started → Prototype Exists → Partially Implemented → Implementation Complete → Review Findings Open → Accepted. Side states: Blocked, Deferred.

---

## Registry

| WP | Name | State | Last updated | Evidence / notes |
|----|------|-------|--------------|------------------|
| 01 | Repository + dev env | Accepted (bootstrap) | 2026-07-22 | Repository Bootstrap final report; `make dev-up`, CI green |
| 02 | DB foundation | Accepted (bootstrap) | 2026-07-22 | Migrations 20260801000001..05; integration suite |
| 03 | Identity + workspace | Not Started | 2026-07-22 | Audit recorder seam exists (used by WP-04); JWKS/API keys pending |
| 04 | Location management | Ready for confirmatory re-review | 2026-07-23 | DRB-WP04-001..005 verified **RESOLVED** at re-review (advisory-lock dedup proven vs real PostgreSQL; fp-tolerant boundary; mandatory override reason; restricted status lifecycle; doc corrections). RR-001/RR-002 resolved and **green in CI**. Two deferred mandatory CI gates now **RESOLVED** (CI-WP04-001 backend-checks, CI-WP04-002 security): **all six mandatory CI jobs green on `b277fba`** (run 29952013546). Awaiting board confirmatory re-review; **not yet Accepted**. Report: `docs/reviews/work-packages/WP-04-delivery-re-review.md` (§A, §B). |
| 05 | Adapter framework | Prototype Exists | 2026-07-22 | First-slice collection pipeline + Open-Meteo adapter; hardening pending |
| 06 | First provider (Open-Meteo) | Prototype Exists | 2026-07-22 | Adapter + fixtures exist; full contract matrix pending |
| 07 | Second provider (OpenWeather) | Not Started | 2026-07-22 | |
| 08 | Scheduler + collection ops | Prototype Exists | 2026-07-22 | Scheduler loop, slots, runs, trigger endpoint exist; hardening pending |
| 09–27 | (remaining) | Not Started | 2026-07-22 | |

---

## Recorded discrepancies

Documentation-vs-documentation conflicts discovered during implementation, with resolutions (master prompt: "record the discrepancy and resolve it before materially extending the affected behaviour").

| # | Discrepancy | Resolution | Affected docs | Packages impacted |
|---|-------------|------------|---------------|-------------------|
| DR-01 | Location timezone mutability: domain architecture §2.3 lists timezone as **immutable** (Mutable: name, status, updated_at); `docs/api/00-api-requirements.md` §4.1 listed PUT as "(name, timezone, status)"; UI design spec edit form includes timezone; security matrix lists "name/timezone/status" | Domain architecture authoritative (Phase 1 aggregate design; timezone derives from immutable coordinates; mutation would re-bucket historical display data). Implementation: PUT accepts name only. API requirements doc corrected 2026-07-22. | `docs/api/00-api-requirements.md` (corrected); `docs/ui/02-ui-design-specification.md` §S-12 (edit form should render timezone read-only — flagged for WP-21); `docs/security/01-ui-authorization-matrix.md` S-12 row (flagged for WP-19 review) | WP-21 (UI form), WP-19 (matrix) |

---

## WP-04 Delivery Review Board outcome (2026-07-22)

Decision: **CHANGES REQUIRED** (report: `docs/reviews/work-packages/WP-04-delivery-review.md`).

| Finding | Severity | Summary | Remediation target |
|---------|----------|---------|--------------------|
| DRB-WP04-001 | High | BR-LOC-01 dedup race: concurrent creates bypass proximity check (reproduced: 6 parallel POSTs → 2 rows); no concurrency test | Serialize create tx (advisory lock or SERIALIZABLE+retry) + concurrency integration test |
| DRB-WP04-002 | Medium | Exact-0.05° boundary fp-fragile (live pair rejected at 0.04999999999999716°) | Epsilon/precision-tolerant comparison + multi-coordinate boundary tests |
| DRB-WP04-003 | Medium | `override_reason` not mandatory when `allow_near_duplicate` set | 422 on empty reason + tests |
| DRB-WP04-004 | Medium | Reserved `archived` settable via PATCH; no transition validation | Restrict to active\|disabled (or ADR) + transition tests |
| DRB-WP04-005 | Medium | `docs/api/05-endpoint-catalog.md` still lists PUT timezone; README endpoint list stale | Doc corrections |
| DRB-WP04-006 | Low | Repository UPDATE writes immutable timezone column | Remove column from statement |
| DRB-WP04-007 | Low | Malformed `active` query param silently coerced | 422 on invalid boolean |

Tracked conditions: TC-01 Idempotency-Key (→ WP-15, this registry's deferral accepted); TC-02 optimistic concurrency (deferred); TC-03 dev-token seam replacement (WP-03/19) + DR-01 UI/matrix follow-ups (WP-21/19); TC-04 CI green on pushed branch + testcontainers run (review env had no Docker).

## WP-04 Delivery Review Board **Re-Review** outcome (2026-07-23)

Decision: **BLOCKED** (report: `docs/reviews/work-packages/WP-04-delivery-re-review.md`). All five original findings independently verified **RESOLVED** (High concurrency finding proven fixed against real PostgreSQL 16; four Medium findings closed with executed unit + integration tests and matching OpenAPI/docs). No remediation-caused Critical/High regression. Blocked purely on evidence:

- **TC-04 NOT SATISFIED / BLOCKED BY EXTERNAL ACCESS** — `git ls-remote origin` fails authentication; no distinct remediation branch (all on `main`); local `origin/main` moved to HEAD unverifiably; GitHub Actions results not inspectable. Local checks do not substitute for pushed-branch CI.
- **`make test-integration` red** — DRB-WP04-RR-001 (Medium, **pre-existing** test-assertion bug: string `"true"` vs boolean `true` at `location_test.go:68`; production correct) + DRB-WP04-RR-002 (Low, flaky per-test containers).

Next action (implementation team): push the branch with working credentials and capture CI for `fc72f08`; fix RR-001 and stabilise the suite; then re-convene for a short confirmatory re-review. **WP-05 must not be selected until WP-04 is Accepted.**

## WP-04 team final remediation (2026-07-23)

Status: **PARTIALLY COMPLETE** (report addendum: `docs/reviews/work-packages/WP-04-delivery-re-review.md` §A1–A6). The implementation team addressed the two evidence blockers within strict scope (test-only + test-harness; no production, migration, OpenAPI, or CI changes):

- **DRB-WP04-RR-001 — RESOLVED (test-only).** `test/integration/location_test.go` now type-asserts `allow_near_duplicate` as a JSON boolean (`bool` + `true`) instead of the string `"true"`. Production audit payload unchanged. Commit `bf694a0`.
- **DRB-WP04-RR-002 — RESOLVED (test-harness).** Single package-wide PostgreSQL container via `TestMain`; each test gets a fresh migrated database (`it_<pid>_<counter>`) dropped `WITH (FORCE)`. Per-test container churn eliminated; isolation preserved; no retries/skips/weakened assertions. 5 consecutive local green runs (~40 s → ~7 s). Commit `b7f2479`.
- **TC-04 — PARTIALLY SATISFIED.** Branch `fix/wp04-final-review` (base `33ad0ab`, tip `1fa9105`) pushed via SSH; local == remote SHA verified; PR #1 triggered CI run `29945014559` (`pull_request`, headSha `1fa9105`). **`backend-integration` went red→green** (failure on `main` `fc72f08` → success on `1fa9105`), verifying RR-001+RR-002 in CI. `api-contract`, `migrations`, `image` also green.
- **Out of scope (deferred, separate work item).** Two mandatory but **pre-existing, unrelated** CI jobs remain red on the branch (they fail on `main` independently of WP-04): `backend-checks` (`govulncheck` flags Go 1.23.x stdlib CVEs GO-2025-4007/4008 + `golang.org/x/net@v0.25.0` GO-2025-3595) and `security` (`gitleaks` HTTP 403 `Resource not accessible by integration` — a `pull_request`-event token-permission quirk, not a detected secret). By explicit scope decision these were left unchanged; their remediation (Go toolchain/dependency bump; CI `permissions:` block) is a separate task.

Next action: raise the two pre-existing CI failures as a separate maintenance task, then re-convene the board for a short confirmatory re-review. **Only the Delivery Review Board may mark WP-04 Accepted. WP-05 must not be selected until WP-04 is Accepted.**

## WP-04 mandatory CI gate remediation (2026-07-23)

Status: **READY FOR CONFIRMATORY RE-REVIEW** (report addendum: `docs/reviews/work-packages/WP-04-delivery-re-review.md` §B). The two mandatory but pre-existing/unrelated CI jobs deferred by the team final remediation were fixed within strict scope (build/dependency + CI config only; no WP-04 production, migration, or OpenAPI changes):

- **CI-WP04-001 — `backend-checks` RESOLVED (code/dependency defect).** `govulncheck` flagged real Go 1.23.x stdlib CVEs plus outdated deps. Fixed by moving the module to `go 1.25.0` + `toolchain go1.25.12`, upgrading `pgx/v5 v5.6.0→v5.9.2`, `x/text v0.15.0→v0.39.0`, `x/net v0.25.0→v0.56.0` (`go mod tidy`), bumping the `Dockerfile` base to `golang:1.25-alpine`, and updating `ci.yml` (`go-version 1.25.12`; `govulncheck@v1.6.0`; `golangci-lint install-mode: goinstall` for v1.64.8-on-Go-1.25). No jobs disabled, no suppressions. One **uncalled** advisory (`GO-2026-5932`, `x/crypto@v0.53.0`, no fix) remains informational and does not fail the gate. Commit `542c808`.
- **CI-WP04-002 — `security` RESOLVED (CI-config defect).** `gitleaks-action@v2` needs `pull-requests: read` to list PR commits on `pull_request` events; the default token (`contents: read` only) caused HTTP 403 `Resource not accessible by integration` — not a detected secret. Fixed with a least-privilege job-scoped `permissions:` block (`contents: read` + `pull-requests: read`). Detection unchanged. Commit `b277fba`.
- **CI evidence.** Run `29952013546` (`pull_request`, headSha `b277fba`): **all six mandatory jobs green** (`backend-checks`, `security`, `backend-integration`, `migrations`, `api-contract`, `image`). Prior baseline `701a0ed` run `29946041618` had `backend-checks`+`security` red. <https://github.com/od3n/forecastiq/actions/runs/29952013546>

Next action: re-convene the Delivery Review Board for the short confirmatory re-review. **Only the board may mark WP-04 Accepted. WP-05 must not be selected until WP-04 is Accepted.**

## Deferred items recorded during WP-04

| Item | Rationale | Revisit |
|------|-----------|---------|
| Idempotency-Key infrastructure for POST /locations | No `idempotency_keys` table in the approved 18-table schema; cross-cutting concern spanning all mutable POSTs; WP-04 acceptance (BR-LOC-01..03) does not require it; snapshot/collection dedup makes re-execution harmless for the pipeline | WP-15 or a dedicated cross-cutting package |
| Optimistic concurrency on location update | Master prompt conditions it on "if approved"; no ADR or domain-architecture section approves it; single-operator MVP with DB unique constraints suffices | If multi-operator admin emerges |
