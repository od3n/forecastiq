# ForecastIQ — WP-05 Provider Adapter Framework Hardening: Delivery Review Findings

**Version**: 1.0
**Review date**: 2026-07-23
**Work package**: WP-05 — Provider Adapter Framework Hardening
**Reviewed commit**: `469560b` feat(collection): harden provider adapter framework (WP-05), on the DRB-accepted WP-04 tip `15b8faa` (via `9a6023f`)
**Board decision**: CONDITIONALLY ACCEPTED (no Critical/High; one Medium evidence gate + two Low coverage gaps) — evidence gate DRB-WP05-001 / **TC-05-01 SATISFIED** 2026-07-23 (CI run 29978249699, six jobs green on `469560b`); WP-05 **READY FOR CONFIRMATORY RE-REVIEW**. **Confirmatory re-review 2026-07-23: ACCEPTED** — TC-05-01 **Closed — Satisfied**; DRB-WP05-001 **Closed**; WP-05 → **Accepted** (report §17).
**Authority**: Delivery Review Board prompt; `docs/planning/05-implementation-work-packages.md` §WP-05

Finding ID scheme: `DRB-WP05-<NNN>`. Statuses: Open | Resolved (verified re-review) | Accepted Risk | Deferred by Approved Decision | Resolved During Review | Not Reproducible.

---

## DRB-WP05-001 — WP-05 commit `469560b` unpushed; no pushed-branch CI on the WP-05 SHA (mandatory gate)

| Attribute | Value |
|-----------|-------|
| Severity | **Medium** (evidence gate — blocks clean Accepted, not the merits) |
| Discipline | SRE / Release Engineering / QA |
| Affected requirement | Mandatory CI evidence gate (board standing rule, established at the WP-04 re-review: "local checks do not substitute for pushed-branch CI when it is a mandatory gate") |
| Affected files | none (repository state / release process) |
| Status | **Closed** (confirmatory re-review 2026-07-23; TC-05-01 Satisfied) — evidence independently re-verified: remote tip `469560b`, CI run 29978249699 headSha `469560b`, all six mandatory jobs green |
| Owner | Eng |

**Evidence.** `git ls-remote --heads origin` shows `refs/heads/fix/wp04-final-review` at the accepted WP-04 tip `15b8faa`. Local HEAD is **2 commits ahead**: `9a6023f` (WP-04 confirmatory doc) and `469560b` (WP-05 implementation). No CI run exists on the WP-05 SHA `469560b`. The remote is reachable in this environment (`git ls-remote` succeeds), unlike the WP-04 re-review environment.

**Expected behaviour.** The six mandatory CI jobs (`backend-checks`, `backend-integration`, `migrations`, `api-contract`, `security`, `image`) must be green on the exact SHA under review, on the pushed branch, before a package moves to Accepted — as applied for WP-04 (`15b8faa`, run 29952834878).

**Actual behaviour.** The WP-05 commit is local-only; the board can (and did) reproduce five of the six gates locally, but no pushed-branch CI run exists for `469560b`.

**Impact.** Acceptance evidence is incomplete. The merits are satisfied and the diff is source-only on an accepted base (no `go.mod`, `Dockerfile`, OpenAPI, or migration change), so the govulncheck / gitleaks / migration / api-contract / image surfaces are unchanged from `15b8faa`; residual risk is low but the mandatory gate is unmet.

**Recommended remediation.** Push `469560b` (and `9a6023f`) to `origin`; capture all six mandatory CI jobs green on that exact SHA.

**Required tests.** None new — existing CI jobs must execute on the pushed SHA.

**Acceptance condition (TC-05-01).** All six mandatory CI jobs green on `469560b` (headSha verified == remote SHA == local HEAD). On satisfaction, this converts to a clean **ACCEPTED** with no framework code change required.

**Resolution (2026-07-23, closure team).** Branch `review/wp05-ci-evidence` pushed at exactly `469560b`; remote tip == local HEAD == CI headSha == `469560b3fe9eed8bfecf25b190f93d53cb136069`. CI run **29978249699** (event `pull_request`, PR #2) completed **success** with all six mandatory jobs green on `469560b` (`backend-checks`, `backend-integration`, `migrations`, `api-contract`, `security`, `image` — none skipped/neutral/cancelled). No framework-code change. **TC-05-01 SATISFIED**; WP-05 set to **READY FOR CONFIRMATORY RE-REVIEW** (only the DRB may mark Accepted).

---

## DRB-WP05-002 — Pipeline-level classification of non-success outcomes not exercised through `CollectService.Collect`

| Attribute | Value |
|-----------|-------|
| Severity | **Low** |
| Discipline | QA / Test coverage |
| Affected requirement | WP-05 test list: pipeline with stub adapter across success/partial/drift/dedup/replay; FC-13 `error_code` persistence |
| Affected files | `internal/collection/collect.go`, `test/integration/db_test.go` (`setup_test.go` `fakeAdapter`) |
| Status | **Open** (non-blocking) — TC-05-02 |
| Owner | Eng |

**Evidence.** The integration pipeline tests drive `CollectService.Collect` for the **success** path (`TestCollectionIdempotency`) and the **dedup** path (`TestSnapshotDedupOnConflict`), against real PostgreSQL 16. The adapter-level tests exercise the full FC-13 classification matrix (partial / drift / rate-limited / auth-failed / 5xx) at the `openmeteo` and `providerhttp` boundary. What is **not** exercised is the deterministic mapping of a non-success `ForecastResult` (partial / failed / timeout / rate_limited) **through the pipeline** into the persisted `collection.status` / `error_code` and the corresponding circuit outcome.

**Expected behaviour.** A stub adapter returning each non-success classified outcome should be driven through `Collect` and the persisted `collection` row status + `error_code` (and circuit-state effect) asserted.

**Impact.** The pipeline mapper is a thin deterministic switch and the classification it consumes is fully tested at the adapter boundary; residual risk is a mislabelled status/`error_code` on the non-success persistence path. Low.

**Recommended remediation.** Add integration cases driving `CollectService.Collect` with a stub adapter returning partial / failed / timeout / rate_limited; assert persisted status, `error_code`, and circuit effect.

**Acceptance condition.** New pipeline-level non-success cases green.

---

## DRB-WP05-003 — `ports.VerifyChecksum` has no direct unit test

| Attribute | Value |
|-----------|-------|
| Severity | **Low** |
| Discipline | QA / Test coverage |
| Affected requirement | WP-05 test list: "checksum verification" |
| Affected files | `internal/collection/ports/*` (`Checksum`/`VerifyChecksum`), `adapters/payloadstore/filesystem_test.go` |
| Status | **Open** (non-blocking) — TC-05-03 |
| Owner | Eng |

**Evidence.** `ports.Checksum` (SHA-256) is exercised indirectly via the payload-store round-trip and replay determinism assertions, but `ports.VerifyChecksum` (the match/mismatch predicate used to detect payload corruption) has no dedicated unit test asserting both the matching and the tampered-bytes cases.

**Expected behaviour.** A direct unit test: `VerifyChecksum(bytes, Checksum(bytes)) == true`; `VerifyChecksum(tampered, checksum) == false`.

**Impact.** Corruption-detection predicate is simple and its inputs are tested elsewhere; residual risk is a silent mismatch-detection regression. Low.

**Recommended remediation.** Add the two-case unit test alongside `filesystem_test.go` or in `ports`.

**Acceptance condition.** Direct `VerifyChecksum` match/mismatch test green.

---

## Review-environment note

Unlike the WP-04 first review (no Docker), this environment **has Docker**. The board executed the full local quality gate — `go build`, `go vet`, `golangci-lint` (zero findings), `go test -race` (all packages green), and the testcontainers integration suite against real PostgreSQL 16 (green, 6.4 s, including `TestCollectionIdempotency` which proves the WP-05 acceptance criterion). The sole outstanding evidence item is the pushed-branch CI run for the WP-05 SHA (DRB-WP05-001 / TC-05-01).
