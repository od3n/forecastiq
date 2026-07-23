# ForecastIQ — WP-05 Provider Adapter Framework Hardening: Traceability Matrices

**Version**: 1.0
**Review date**: 2026-07-23
**Work package**: WP-05 — Provider Adapter Framework Hardening
**Reviewed commit**: `469560b` (on accepted WP-04 tip `15b8faa` via `9a6023f`)
**Authority**: `docs/planning/05-implementation-work-packages.md` §WP-05; Phase 1 architecture; relevant ADRs
**Companion documents**: `WP-05-delivery-review.md` (report), `WP-05-findings.md` (finding cards)

---

## 1. Scope traceability (approved WP-05 definition → evidence)

| # | Approved scope item | Implementation evidence | Result |
|---|---------------------|-------------------------|--------|
| S1 | `ForecastProviderAdapter` port | `internal/collection/ports/adapter.go` — `Slug`/`SchemaVersion`/`AdapterVersion`/`Capabilities`/`FetchForecast`; `ReplayDecoder.DecodeStored` | ✅ Implemented |
| S2 | Schema validation helper | `openmeteo/decompose.go` (JSON/shape validation, >50% invalid → `schema_drift`) + `domain.ForecastSnapshot.Validate()` (row physical-range) | ✅ Implemented (adapter-owned decode + shared domain row validation) |
| S3 | Checksum + gzip payload store (scheme-prefixed keys) | `adapters/payloadstore/filesystem.go`; `ports.Checksum`/`VerifyChecksum` (SHA-256); `ports.BuildPayloadKey` → `file://{slug}/yyyy/mm/dd/{id}.json.gz` (ADR-011/019) | ✅ Implemented |
| S4 | Condition taxonomy mapper (v1) | `openmeteo/condition_map.go` (WMO→canonical); `domain.ConditionTaxonomyVersion`; unmapped → `ConditionUnknown` + FC-15 tally | ✅ Implemented |
| S5 | Collection use case (tx: collection + snapshots + event) | `internal/collection/collect.go` — single bounded tx (ADR-027); post-commit events/metrics | ✅ Implemented (prototype retained + integrated) |
| S6 | Dedup rules (snapshot + collection-level) | `collect.go` `FindDedup` + `ErrDuplicateCollection` race handling; `SnapshotRepository.InsertBatch` conflict dedup | ✅ Implemented |
| S7 | `error_code` classification (FC-13) | `ports/errors.go` closed `ErrorCode` set + `ProviderError`; `providerhttp` `classify()` | ✅ Implemented |
| S8 | Circuit breaker (persistent, catalog-owned) | `catalog.CircuitService`/`CircuitState`; `domain/circuit.go` state machine (threshold 5) | ✅ Implemented (catalog-owned; consumed by collector) |

**Scope result: 8/8 implemented.** Hardening additions (commit `469560b`): shared `providerhttp` transport; `ProviderError` taxonomy; `Capabilities()`; explicit `Registry` with `provider.registered` startup log; `ReplayDecoder.DecodeStored`; Open-Meteo refactor behind the framework; authoring guide. No DB/API/migration change (`git diff --stat 15b8faa..469560b` touches only Go source under `internal/collection`, `adapters/forecastproviders`, `cmd/forecastiq/app.go`, one integration-test line, and docs). No scope expansion; no WP-06 work.

---

## 2. Required-test traceability (approved WP-05 test list → evidence)

| # | Required test | Evidence | Result |
|---|---------------|----------|--------|
| T1 | Pipeline — success (stub adapter) | `db_test.go` `TestCollectionIdempotency` (real PG16); adapter `TestFetchForecast_Success` | ✅ Covered |
| T2 | Pipeline — partial | `openmeteo_test.go` partial-result case (adapter boundary) | ✅ Covered (adapter) |
| T3 | Pipeline — schema drift | `openmeteo_test.go` >50%-invalid → `schema_drift` | ✅ Covered (adapter) |
| T4 | Pipeline — dedup | `db_test.go` `TestSnapshotDedupOnConflict` + `TestCollectionIdempotency` (real PG16) | ✅ Covered (pipeline, real DB) |
| T5 | Pipeline — replay | `openmeteo_test.go` `DecodeStored` determinism (checksum + snapshot-count stability) | ✅ Covered (adapter) |
| T6 | Checksum verification | `filesystem_test.go` round-trip (`Checksum` indirectly); `VerifyChecksum` **no direct test** | ⚠️ Partial (DRB-WP05-003) |
| T7 | Circuit transitions (5→open→half-open→closed) | `circuit_test.go` domain state machine | ✅ Covered |
| T8 | Condition unmapped counter (FC-15) | `openmeteo_test.go` unmapped-code → `ConditionUnknown` + tally | ✅ Covered |
| T9 | FC-13 classification (429/401/403/5xx/4xx) | `client_test.go` classification table; `errors_test.go` outcome mapping | ✅ Covered |
| T10 | Retry disposition (FC-08) | `client_test.go` retry-then-success (call count) + non-retryable-stops | ✅ Covered |
| T11 | Rate-limit header normalization | `client_test.go` header parsing (values asserted); `openmeteo_test.go` rate-limited case | ✅ Covered |
| T12 | Transport hardening (bounded body, redirect cap) | `client_test.go` bounded-body fail-closed + redirect-cap fail-closed | ✅ Covered |
| T13 | Error redaction (log-safety) | `errors_test.go` `TestProviderError_ErrorIsLogSafe`; `client_test.go` `TestGet_ErrorMessageRedaction` | ✅ Covered |
| T14 | Pipeline-level non-success classification (partial/failed/timeout/rate_limited through `Collect`) | Adapter boundary only; **not driven through `CollectService.Collect`** | ⚠️ Partial (DRB-WP05-002) |
| T15 | Registry validation (duplicate slug / empty identity / replay-capability) | `registry_test.go` rejection cases + descriptor sorting/copy semantics | ✅ Covered |

**Required-test result: 13/15 fully covered; 2 partial (T6, T14) — both Low, non-blocking.**

---

## 3. Acceptance-criterion traceability

| Approved acceptance criterion | Evidence | Result |
|-------------------------------|----------|--------|
| "Full collection pipeline green with a fake provider; idempotent re-execution proven." | `db_test.go` `TestCollectionIdempotency` — drives real `CollectService.Collect` with `fakeAdapter`; two identical collections → exactly one collection row + 3 snapshots. **Executed live against real PostgreSQL 16 (green, 6.4 s).** | ✅ **MET** |

---

## 4. Architecture / ADR traceability

| Reference | Requirement | Evidence | Result |
|-----------|-------------|----------|--------|
| Module architecture §3 | Dependency direction (adapters → ports, never reverse; registry wired only in composition root) | `providerhttp` imports `ports` + `platform/ratelimit` only; `cmd/forecastiq/app.go` wires registry; depguard green | ✅ Compliant |
| ADR-011 | Raw-payload retention (checksum on raw bytes before parse) | `providerhttp` populates `Body`/`StatusCode`/`LatencyMS` even on failure; checksum on raw bytes | ✅ Compliant |
| ADR-019 | Object-storage use (`file://` scheme-prefixed keys) | `ports.BuildPayloadKey` → `file://{slug}/…` | ✅ Compliant |
| ADR-027 | Single-transaction collection (collection + snapshots + circuit) | `collect.go` single bounded tx | ✅ Compliant |
| FC-08 | Bounded retry/backoff with jitter | `providerhttp` backoff 1,2,4,8,16 s ±20% jitter, cap 5 attempts | ✅ Compliant |
| FC-13 | Closed provider-error taxonomy | `ports/errors.go` closed `ErrorCode` set | ✅ Compliant |
| FC-15 | Unmapped-condition metric | `condition_map.go` + `condition_unmapped` metric (slug, code) | ✅ Compliant |

---

## 5. Non-regression / boundary confirmation

| Surface | Change in WP-05? | Evidence |
|---------|------------------|----------|
| Migrations / schema / triggers / indexes | **No** | `git diff --stat … -- migrations/` empty; `TestImmutabilityTriggers` / `TestSnapshotDedupOnConflict` green on existing schema |
| API routes / OpenAPI | **No** | `git diff --stat … -- api/` empty; `openapi.json` untouched; `api-contract` job validates unchanged spec |
| `go.mod` / `Dockerfile` | **No** | not in diff; dependency-scan / image surfaces unchanged from accepted `15b8faa` |
| WP-06 (first provider full matrix) | **No** | not started by the board; out of WP-05 scope |

---

## Outstanding condition

| ID | Condition | Blocking Accepted? |
|----|-----------|--------------------|
| TC-05-01 | Push `469560b` (and `9a6023f`); capture all six mandatory CI jobs green on that exact SHA | **Yes** — **SATISFIED 2026-07-23** |
| TC-05-02 | Pipeline-level non-success classification test through `CollectService.Collect` (DRB-WP05-002) | No |
| TC-05-03 | Direct `VerifyChecksum` unit test (DRB-WP05-003) | No |

## TC-05-01 CI evidence (2026-07-23, closure team)

| Item | Value | Result |
|------|-------|--------|
| Remote branch | `review/wp05-ci-evidence` | ✅ pushed |
| Remote tip / local HEAD / CI headSha | `469560b3fe9eed8bfecf25b190f93d53cb136069` | ✅ all equal |
| `9a6023f` (parent) in branch history | contained | ✅ |
| Workflow / run | CI / `29978249699` (event `pull_request`, PR #2) | ✅ success |
| `backend-checks` | headSha `469560b` | ✅ success |
| `backend-integration` | headSha `469560b` | ✅ success |
| `migrations` | headSha `469560b` | ✅ success |
| `api-contract` | headSha `469560b` | ✅ success |
| `security` | headSha `469560b` | ✅ success |
| `image` | headSha `469560b` | ✅ success |
| Framework-code change | none | ✅ reviewed SHA intact |

**TC-05-01: SATISFIED.** WP-05 → **READY FOR CONFIRMATORY RE-REVIEW** (not Accepted; DRB transition only).

**Confirmatory re-review 2026-07-23:** evidence independently re-verified (remote tip `469560b`, CI run 29978249699 headSha `469560b`, all six mandatory jobs green, no post-review framework change). **Decision: ACCEPTED.** TC-05-01 → **Closed — Satisfied**; WP-05 → **Accepted**.
