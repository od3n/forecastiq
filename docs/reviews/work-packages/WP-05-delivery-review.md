# ForecastIQ — WP-05 Provider Adapter Framework Hardening: Delivery Review Board Report

**Version**: 1.0
**Review date**: 2026-07-23
**Work package**: WP-05 — Provider Adapter Framework (hardening pass)
**Reviewed commit**: `469560b` feat(collection): harden provider adapter framework (WP-05), on the DRB-accepted WP-04 tip `15b8faa` (via `9a6023f`)
**Companion documents**: `WP-05-findings.md` (finding cards), `WP-05-traceability.md` (matrices)
**Dependency gate**: WP-04 — **Accepted** (confirmatory re-review 2026-07-23, CI run 29952834878 on `15b8faa`). Dependency satisfied; review continued.

> Scope discipline: this is a **full first review** of WP-05. The board implemented nothing, remediated nothing, started no WP-06 work, and introduced no requirements outside the approved WP-05 definition. Only this review artifact set was written.

---

## 1. Executive summary

WP-05 hardens the previously prototyped collection pipeline (`ce88618`) into a **provider-agnostic adapter framework**. The delivered surface is clean and matches the approved WP-05 definition: a shared, hardened HTTP transport (`providerhttp`) centralising User-Agent, capped redirects, bounded body reads, FC-08 retry/backoff with jitter, FC-13 classification, and rate-limit-header normalization; a structured `ports.ProviderError` taxonomy over the closed FC-13 code set; a static `Capabilities()` declaration; an explicit `collection.Registry` that validates identity/versions, rejects duplicate slugs, and enforces the replay-capability contract at startup; a `ReplayDecoder` for deterministic decode from stored bytes; and the Open-Meteo adapter refactored to sit entirely behind the framework. A new authoring guide documents the extension contract.

The board verified every claim against the repository and **executed the full quality gate locally with Docker present**: `go build`, `go vet`, `golangci-lint` (zero findings), `go test -race` (all packages green), and — decisively — the **testcontainers integration suite against real PostgreSQL 16 (green, 6.4 s)**, which includes `TestCollectionIdempotency` proving the WP-05 acceptance criterion ("full collection pipeline green with a fake provider; idempotent re-execution proven"). Module boundaries, dependency direction, the single-transaction collection command (ADR-027), payload lineage (SHA-256 + `file://` scheme-prefixed gzip keys), and log-safe error rendering are all confirmed.

No Critical or High finding exists. The implementation is substantively complete and correct. The single gate preventing a clean **ACCEPTED** is **evidence**: the WP-05 commit `469560b` is **not pushed** (remote branch tip is `15b8faa`), so no pushed-branch CI run exists for the WP-05 SHA. Per the board's standing rule — established and applied at the WP-04 re-review — local checks do not substitute for pushed-branch CI when it is a mandatory gate. Two Low findings (untested pipeline-level classification of non-success outcomes; untested `VerifyChecksum`) are non-blocking.

**Decision: CONDITIONALLY ACCEPTED** — accepted on the merits, contingent on tracked condition **TC-05-01** (push `469560b` and capture the six mandatory CI jobs green on that exact SHA).

## 2. Scope verification

WP-05 approved scope (`docs/planning/05-implementation-work-packages.md` §WP-05) vs. repository evidence:

| Scope item | Evidence | Result |
|------------|----------|--------|
| `ForecastProviderAdapter` port | `internal/collection/ports/adapter.go` (Slug/SchemaVersion/AdapterVersion/Capabilities/FetchForecast) | ✅ Present |
| Schema validation helper | `openmeteo/decompose.go` (JSON/shape validation, >50% invalid → drift) + `domain.ForecastSnapshot.Validate()` (row physical-range) | ✅ Present (adapter-owned decode + shared domain row validation, per authoring guide §4) |
| Checksum + gzip payload store (scheme-prefixed keys) | `adapters/payloadstore/filesystem.go`; `ports.Checksum`/`VerifyChecksum` (SHA-256); `ports.BuildPayloadKey` (`file://{slug}/yyyy/mm/dd/{id}.json.gz`) | ✅ Present |
| Condition taxonomy mapper (v1) | `openmeteo/condition_map.go` (WMO→canonical), `domain.ConditionTaxonomyVersion`; unmapped → `ConditionUnknown` + FC-15 tally | ✅ Present |
| Collection use case (tx: collection + snapshots + event) | `internal/collection/collect.go` (single bounded tx; post-commit events) | ✅ Present (prototype, retained + integrated) |
| Dedup rules (snapshot + collection-level) | `collect.go` `FindDedup` + `ErrDuplicateCollection` race handling; `SnapshotRepository.InsertBatch` conflict dedup | ✅ Present |
| `error_code` classification (FC-13) | `ports/errors.go` closed `ErrorCode` set + `ProviderError`; transport `classify()` | ✅ Present |
| Circuit breaker (persistent, catalog-owned) | `catalog.CircuitService`/`CircuitState`; `domain` state machine (`circuit.go`) | ✅ Present (catalog-owned; consumed by collector) |

**Hardening additions (WP-05 commit `469560b`):** shared `providerhttp` transport; `ProviderError` taxonomy; `Capabilities()`; explicit `Registry` with startup inspection log (`provider.registered`); `ReplayDecoder.DecodeStored`; Open-Meteo refactor behind the framework; authoring guide. **No DB/API/migration change** (diff touches only Go source under `internal/collection`, `adapters/forecastproviders`, `cmd/forecastiq/app.go`, one integration-test line, and docs — confirmed by `git diff --stat`). No scope expansion beyond WP-05; no WP-06 work.

## 3. Architecture review

- **Module boundaries / dependency direction**: The port (`internal/collection/ports`) is imported by adapters (`adapters/forecastproviders/*`), never the reverse; `providerhttp` depends on `ports` + `platform/ratelimit` only. The registry lives in the collection application package and is wired **only** in the composition root (`cmd/forecastiq/app.go`). Handlers/domain do not import adapters. Consistent with module architecture §3 and the binding rule; `golangci-lint` depguard is green.
- **Provider abstraction integrity**: `Capabilities` + identity (`Slug`/`SchemaVersion`/`AdapterVersion`) let the composition root and operators reason about a provider with no provider-specific knowledge. The transport is genuinely provider-agnostic (no Open-Meteo specifics leak into `providerhttp`); Open-Meteo owns only request shaping, schema, normalization, and condition mapping.
- **Canonical contract implementation**: `ForecastRequest`/`ForecastResult` are the canonical I/O; adapters return a classified `*ForecastResult` with a **nil Go error** for all classified outcomes (documented contract), reserving Go errors for programmer faults. Confirmed in `openmeteo.FetchForecast`.
- **Adapter registration**: `Registry.Register` validates non-empty identity, rejects duplicate slugs, and — critically — rejects an adapter that declares `SupportsReplay` but does not implement `ReplayDecoder` (fail-fast at startup, not first collection). Startup logs `provider.registered` per adapter.
- **Replay design**: `DecodeStored` re-derives a result from stored bytes with no network call and no HTTP metadata; determinism is asserted (checksum + snapshot-count stability).
- **Raw-response strategy**: Best-effort `Body`/`StatusCode`/`LatencyMS` are populated even on failure so the raw error payload can still be persisted (ADR-011); checksum computed on raw bytes before parsing (lineage integrity).
- **Transaction architecture**: Collection + snapshots + circuit outcome commit in one bounded transaction (ADR-027); partitions ensured (idempotent DDL) before insert; dedup race resolved by treating a concurrent same-key commit as deduplicated.

No architectural drift; no ADR-required change was introduced. The one interface extension (`Capabilities()` on the port) is additive and propagated to the integration `fakeAdapter` (3-line change), keeping the suite compiling.

## 4. Implementation traceability

See `WP-05-traceability.md` for the full requirement/test matrices. Summary: 8/8 scope items implemented; the WP-05 required-test list is covered as follows — success ✅ (adapter + pipeline), partial ✅ (adapter), drift ✅ (adapter), dedup ✅ (pipeline, real DB), replay ✅ (adapter), rate-limited ✅ (adapter + transport, metadata asserted), auth-failed ✅, server-error ✅, unmapped-condition ✅, circuit transitions ✅ (domain: 5→open→half-open→closed), bounded body ✅, redirect cap ✅, error redaction ✅. Gap: non-success **pipeline-level** classification (partial/failed/timeout/rate_limited through `CollectService.Collect`) is not exercised (DRB-WP05-002, Low).

## 5. Security review

- **Payloads never logged**: `ProviderError.Error()` renders code + status only; wrapped cause is never printed (`TestProviderError_ErrorIsLogSafe`, `TestGet_ErrorMessageRedaction`). Collector logs `error_code`, counts, and IDs — never bodies, URLs, or query strings.
- **Credential handling**: The adapter reads only `req.Credential`; the service resolves `credential_ref` via `resolveCred` (env), never the adapter directly. No credential is logged; auth header is set but never rendered.
- **SSRF**: Base URLs are seeded provider configuration, not user input; the transport caps redirects (default 5) and bounds body reads (default 10 MB, fail-closed). No user-supplied-URL fetch path.
- **Transport hardening**: stable User-Agent, `Accept` default, bounded reads, capped redirects — all tested.
- No injection surface introduced (no SQL change; base URLs constant). No new authn/authz path (trigger/replay endpoints are WP-08 scope and untouched).

## 6. Database review

**No schema, migration, trigger, or index change** in WP-05 (`git diff --stat … -- migrations/` empty). The collection command uses the existing WP-02 schema; snapshot dedup relies on the existing uniqueness constraint (verified live by `TestSnapshotDedupOnConflict`: re-insert of identical snapshots stores 0). Immutability of completed collections is enforced by an existing trigger (`TestImmutabilityTriggers`: `UPDATE` on a completed collection errors). Partition existence is ensured idempotently before insert. No data-integrity regression; the idempotency invariant (one collection row + 3 snapshots after two identical collections) is proven against real PostgreSQL 16.

## 7. API review

**No API or OpenAPI change** in WP-05 (`git diff --stat … -- api/` empty; `openapi.json` untouched). The framework is internal to the collection module and composition root; no route, request/response contract, or error envelope was added or altered. API compatibility is therefore preserved by construction. The `api-contract` CI job validates the committed spec (unchanged).

## 8. Testing review

- **Executed by the board** (this environment has Docker, unlike the WP-04 first review):
  - `go build ./...` ✅ · `go vet ./...` ✅ · `golangci-lint run ./...` ✅ (zero findings)
  - `go test -race -count=1 ./...` ✅ — all packages pass, no races (collection 4.19 s, ports 4.70 s, catalog/domain 3.42 s, openmeteo 2.22 s, providerhttp 1.73 s, …)
  - `go test -tags integration -count=1 ./test/integration/...` ✅ — **green, 6.4 s** against testcontainers PostgreSQL 16
  - `go vet -tags integration ./test/...` ✅ (suite compiles; only third-party CGO warnings)
- **Quality**: framework tests assert meaningful outcomes — transport classification table (429/401/403/5xx/4xx), retry-then-success (call count), non-retryable stops immediately, rate-limit header parsing (values asserted), bounded-body fail-closed, redirect-cap fail-closed, error redaction; registry duplicate/empty-identity/replay-capability rejection and descriptor sorting/copy semantics; `ProviderError` outcome mapping + log-safety; adapter contract matrix (success/edge-null/partial/drift/unmapped/rate-limited/auth-failed/5xx) and replay determinism (`DecodeStored` + double-fetch). The pipeline acceptance test (`TestCollectionIdempotency`) drives the real `CollectService` with a fake provider and proves idempotent re-execution.
- **Gaps** (Low): (a) non-success outcomes are not driven through `CollectService.Collect` — only success + dedup are (DRB-WP05-002); (b) `ports.VerifyChecksum` has no direct unit test (DRB-WP05-003).
- **Confidence**: High for the transport, registry, taxonomy, capabilities, replay, and adapter contract; High for the success/dedup pipeline path (real-DB); Medium for pipeline-level non-success status persistence (logic is a thin deterministic mapper, adapter-level classification is tested).

## 9. Operational readiness

- **Observability**: `provider.registered` startup log (slug/schema/adapter/horizon/credential/replay); `collection.started`/`completed`/`deduplicated`/`circuit_open` structured events; metrics for collection attempts/duration, provider latency, snapshots stored, records rejected (reason), circuit-state gauge, rate-limit hits, and `condition_unmapped` (slug, code). No secrets in any log field.
- **Failure handling**: FC-08 bounded retry with jitter (caps at 5 attempts); FC-13 closed-set classification drives circuit outcome (success/partial close; failed/timeout advance; rate_limited/auth_failed leave the breaker unchanged); circuit pre-check short-circuits with a typed `CircuitOpenError`. Graceful degradation on payload-write failure (`payload_write_failed` recorded, pipeline continues).
- **Config handling**: transport timeouts/limits/retries are config-driven with safe defaults; provider limiter (6 req/min) wired in the composition root. Authoring guide + checklist document the extension path for future providers.

## 10. Documentation review

`docs/development/08-provider-adapter-authoring-guide.md` (new) accurately describes the port, shared transport usage, FC-13 taxonomy, schema validation/normalization rules, capabilities/replay contract, registration, security rules, and the contract-test matrix. It matches the code (verified against `openmeteo`, `providerhttp`, `registry`). The status-registry entry for WP-05 accurately describes the delivered hardening and states "no DB/API change" — consistent with the diff. No stale or contradictory documentation found.

## 11. CI review

CI (`.github/workflows/ci.yml`) defines six mandatory jobs on `push` and `pull_request`: `backend-checks` (gofmt, golangci-lint incl. depguard, govulncheck@v1.6.0, unit `-race` + coverage), `backend-integration` (testcontainers PG16), `migrations` (build + migrate up + verify schema + seed ×2), `api-contract` (OpenAPI validation), `security` (gitleaks), `image` (distroless build). All six were proven green on the accepted base `15b8faa` (WP-04 confirmatory run 29952834878). WP-05 changes only Go source + docs (no `go.mod`, `Dockerfile`, OpenAPI, or migration change), so the dependency-scan / migration / api-contract / image surfaces are unchanged.

**Evidence gap (governing):** the WP-05 commit `469560b` is **not pushed** — `git ls-remote --heads origin` shows `refs/heads/fix/wp04-final-review` at `15b8faa`; local HEAD is 2 commits ahead (`9a6023f`, `469560b`). **No CI run exists on the WP-05 SHA.** The remote is reachable (unlike the WP-04 re-review environment), and all six gates are reproducible locally (five run by the board, green; the remaining govulncheck/gitleaks/image are unaffected by a source-only diff on an accepted base), but per the board's standing rule local checks do not satisfy the mandatory pushed-branch CI gate. Recorded as **DRB-WP05-001 (Medium, evidence)** / **TC-05-01**.

## 12. Findings

Ordered by severity; full cards in `WP-05-findings.md`.

| ID | Severity | Title | Status |
|----|----------|-------|--------|
| DRB-WP05-001 | **Medium** | WP-05 commit `469560b` unpushed; no pushed-branch CI on the WP-05 SHA (mandatory gate) | Open (evidence) |
| DRB-WP05-002 | Low | Pipeline-level classification of non-success outcomes (partial/failed/timeout/rate_limited) not exercised through `CollectService.Collect` | Open |
| DRB-WP05-003 | Low | `ports.VerifyChecksum` has no direct unit test | Open |

No Critical or High finding. No architectural drift, security weakness, DB/API regression, or scope expansion.

## 13. Traceability matrix

Full matrices in `WP-05-traceability.md`. Headline: **scope 8/8 implemented**; **required tests 13/14 covered** (one item — pipeline-level non-success classification — partially covered at the adapter boundary only); **acceptance criterion met** (idempotent pipeline with fake provider, proven live against real PostgreSQL).

## 14. Quality score

| Area | Score (1–10) | Explanation |
|------|--------------|-------------|
| Scope completeness | 9 | All 8 scope items present; hardening additions match the definition |
| Architecture alignment | 9 | Clean boundaries, provider-agnostic transport, additive port extension |
| Provider abstraction integrity | 9 | Capabilities + identity + registry enforce a knowledge-free composition root |
| Canonical contract | 9 | `ForecastRequest`/`Result` + nil-error classified-outcome contract, consistent |
| HTTP client hardening | 9 | UA, redirect cap, bounded reads, retry/backoff+jitter, all tested |
| Retry / rate-limit / taxonomy | 9 | FC-08 retry disposition + FC-13 closed set + normalized rate-limit metadata |
| Replay | 9 | Deterministic `DecodeStored`, no network, no HTTP metadata, asserted |
| Security | 9 | Log-safe errors, no credential/body/URL leakage, no SSRF surface |
| Database integrity | 9 | No schema change; idempotency + immutability proven on real PG16 |
| API compatibility | 10 | No API/OpenAPI change; compatibility preserved by construction |
| Test quality | 8 | Strong framework + adapter + real-DB pipeline tests; two Low coverage gaps |
| Documentation | 9 | Accurate, complete authoring guide; registry consistent with diff |
| CI confidence | 4 | **No pushed-branch CI on the WP-05 SHA (DRB-WP05-001).** Primary reason acceptance is conditional |
| Operational readiness | 9 | Events, metrics (incl. unmapped/circuit), config-driven, graceful degradation |

A high average does not override a mandatory evidence gate: TC-05-01 governs the transition to Accepted.

## 15. Final decision

### CONDITIONALLY ACCEPTED

**Rationale.** WP-05 satisfies the approved implementation requirements: the provider-agnostic framework (transport hardening, FC-13 taxonomy, capabilities, explicit registry with startup validation, deterministic replay, Open-Meteo refactor) is architecturally clean, security-sound, and free of DB/API drift, and the acceptance criterion — full collection pipeline green with a fake provider, idempotent re-execution proven — was **executed live and passed** against real PostgreSQL 16, with the entire unit suite green under `-race` and lint clean. There is **no Critical or High finding**. The package is not marked **Accepted** solely because the mandatory **pushed-branch CI evidence for the WP-05 commit `469560b` does not yet exist** (the commit is local-only; remote tip is `15b8faa`), and — per the board's standing rule applied at the WP-04 re-review — local checks do not substitute for pushed-branch CI. Two Low findings are non-blocking. On satisfaction of TC-05-01 (push the commit and capture the six mandatory CI jobs green on that exact SHA), this converts to a clean **ACCEPTED** with no code change required to the framework.

**Tracked conditions**

| ID | Condition | Owner | Blocking Accepted? |
|----|-----------|-------|--------------------|
| TC-05-01 | Push `469560b` (and the WP-04 confirmatory doc `9a6023f`); capture all six mandatory CI jobs green on that SHA (backend-checks, backend-integration, migrations, api-contract, security, image) | Eng | **Yes** |
| TC-05-02 | (Opportunistic) Add a pipeline-level test driving `CollectService.Collect` for partial/failed/timeout/rate_limited outcomes (DRB-WP05-002) | Eng | No |
| TC-05-03 | (Opportunistic) Add a direct `VerifyChecksum` unit test (DRB-WP05-003) | Eng | No |

**Recommended next action.** Run a WP-05 remediation pass for the identified findings (satisfy TC-05-01; optionally close the two Lows), then re-convene the board for a short confirmatory re-review. Do **not** begin WP-06 until WP-05 is Accepted.

---

## 16. TC-05-01 evidence addendum (2026-07-23, closure team)

> Recorded by the WP-05 Conditional-Acceptance Closure Team. **No framework-code change** was made (diff of `469560b` unchanged; the reviewed SHA is intact). This addendum records only the pushed-branch CI evidence that satisfies the blocking evidence gate DRB-WP05-001 / **TC-05-01**. The board's decision remains **CONDITIONALLY ACCEPTED**; only the Delivery Review Board may convert it to **ACCEPTED** at the confirmatory re-review.

**Commit relationship (verified).** `9a6023f` is the direct parent of `469560b` (`git merge-base --is-ancestor 9a6023f 469560b` → true; `git rev-parse 469560b^` == `9a6023f45dc4a966bc0e40caca5208db57927a01`). `469560b` is the final reviewed descendant carrying the full WP-05 change set; no unreviewed commit follows it.

**Push & remote verification.** Branch `review/wp05-ci-evidence` created at exactly `469560b` and pushed to `origin`. `git ls-remote --heads origin review/wp05-ci-evidence` → `469560b3fe9eed8bfecf25b190f93d53cb136069`. Local HEAD == remote tip == `469560b3fe9eed8bfecf25b190f93d53cb136069`. Both `9a6023f` and `469560b` are contained in the remote branch (`git branch -r --contains`).

**CI evidence.** Workflow **CI**, run **29978249699** (`https://github.com/od3n/forecastiq/actions/runs/29978249699`), event `pull_request` (PR #2 → `main`; `main` is an ancestor of `469560b`, so the merge ref equals head), **headSha `469560b3fe9eed8bfecf25b190f93d53cb136069`**, overall conclusion **success**.

| # | Mandatory job | Status | Conclusion | Tested headSha | Skipped? |
|--:|---------------|--------|-----------|----------------|----------|
| 1 | `backend-checks` | completed | ✅ success | `469560b` | No |
| 2 | `backend-integration` | completed | ✅ success | `469560b` | No |
| 3 | `migrations` | completed | ✅ success | `469560b` | No |
| 4 | `api-contract` | completed | ✅ success | `469560b` | No |
| 5 | `security` | completed | ✅ success | `469560b` | No |
| 6 | `image` | completed | ✅ success | `469560b` | No |

All six mandatory jobs ran, completed, and passed on the exact reviewed SHA; none was neutral, skipped, or cancelled. The closure team also reproduced the full gate locally (gofmt, `golangci-lint` zero findings, `govulncheck` 0 called, `go test -race` all green, testcontainers integration green, distroless image build green).

**TC-05-01: SATISFIED.** **DRB-WP05-001: Resolved (evidence captured).** WP-05 state set to **READY FOR CONFIRMATORY RE-REVIEW** — **not** Accepted. TC-05-02 / TC-05-03 (Low, non-blocking) remain Open.

---

## 17. Confirmatory re-review — DRB decision (2026-07-23)

> Independent confirmatory re-review of the single tracked condition **TC-05-01**. The board re-verified commit identity, ancestry, the real remote, the CI run, and all six mandatory jobs directly against the repository and GitHub Actions API. It did not reopen the merits, rescore, or alter historical decisions.

**Verification performed (all independently confirmed):**

- **Commit identity.** `9a6023f45dc4a966bc0e40caca5208db57927a01` and `469560b3fe9eed8bfecf25b190f93d53cb136069` both exist; `git merge-base --is-ancestor 9a6023f 469560b` → true (`9a6023f` is the direct parent of `469560b`). `469560b` carries the full reviewed WP-05 change set.
- **Remote.** `git ls-remote origin` → `refs/heads/review/wp05-ci-evidence` = `469560b3fe9eed8bfecf25b190f93d53cb136069`; `refs/pull/2/head` = same. Local `review/wp05-ci-evidence` == remote tip == `469560b`. History preserved (no force-rewrite).
- **CI run.** Workflow **CI**, run **29978249699**, event `pull_request`, branch `review/wp05-ci-evidence`, **headSha `469560b3fe9eed8bfecf25b190f93d53cb136069`** (exact reviewed SHA, not a synthetic merge commit), overall conclusion **success**.
- **Six mandatory jobs.** `backend-checks`, `backend-integration`, `migrations`, `api-contract`, `security`, `image` — all `completed` / `success`; none skipped, cancelled, or neutral. No `continue-on-error` exists in `ci.yml`.
- **No post-review framework change.** The only commit after `469560b` (`311a4a7`, on `fix/wp04-final-review`) is documentation-only (status registry + WP-05 review artifacts) and is not on the reviewed branch. The CI-tested framework commit remains exactly `469560b`.
- **Documentation.** Consistent; no document prematurely marked WP-05 Accepted prior to this decision.

**Decision: ACCEPTED.** TC-05-01 → **Closed — Satisfied**. DRB-WP05-001 → **Closed**. WP-05 package state → **Accepted**. TC-05-02 / TC-05-03 (Low, non-blocking) remain Open as opportunistic follow-ups. WP-06 may be selected in a separate action. No new Critical or High finding was revealed by the final evidence.
