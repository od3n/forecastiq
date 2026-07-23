# ForecastIQ — WP-07 Second Forecast Provider (OpenWeather): Implementation Report

**Version**: 1.0
**Implementation date**: 2026-07-23
**Work package**: WP-07 — Second Forecast Provider (OpenWeather)
**Authority**: `docs/planning/05-implementation-work-packages.md` §WP-07 (definition); `docs/testing/03-contract-testing.md` §1.2 (contract matrix); ADR-002 (provider scope + Tomorrow.io fallback); `docs/product/05-business-rules.md` BR-PROV-01/BR-ATTR-01
**Branch**: `feature/wp07-openweather-provider` (base: accepted WP-06 tip on `main` `927106d`)
**Status**: **Implementation Complete — Not Accepted** (Delivery Review Board transition only)

> Scope discipline: this package implemented the OpenWeather One Call 3.0 adapter behind the WP-05 framework, the daily rate-budget guard (429 → pause), the 401/429 contract fixtures, the full contract matrix, and the Tomorrow.io swap-path documentation. No WP-08 (scheduler — already Accepted), WP-09 (observation adapter), or WP-10 (observation collection) work was started. No domain, service, persistence, API, or migration change was made; the adapter is additive behind the existing provider-agnostic pipeline. The OpenWeather operational configuration is seeded **disabled** (ToS gate D-05).

---

## 1. Executive summary

- **Title / objective**: WP-07 — OpenWeather One Call 3.0 adapter (48-period hourly) behind the `ForecastProviderAdapter` framework, with daily rate-budget enforcement (429 → pause), 401/429 handling, the full contract matrix, and Tomorrow.io swap-path documentation.
- **Implemented this package**:
  - New adapter package `adapters/forecastproviders/openweather/` (`openweather.go`, `decompose.go`, `condition_map.go`) implementing `ports.ForecastProviderAdapter` + `ports.ReplayDecoder` over the shared `providerhttp` transport.
  - Daily rate-budget guard `budget.go` — a UTC-day counter that refuses collection pre-emptively once the day's budget is spent and **pauses on a 429** (honoring `Retry-After`, else resting until the next UTC day). Thread-safe; clock-injected for determinism.
  - Contract-matrix + budget tests (`openweather_test.go`, `budget_test.go`) — 22 tests green under `-race`; 8 committed fixtures.
  - Composition-root wiring (`cmd/forecastiq/app.go`): registered the adapter with a per-minute limiter and the configured daily budget.
  - Config `FIQ_PROVIDER_OPENWEATHER_DAILY_BUDGET` (default 1000) + `.env.example`.
  - Seeded OpenWeather operational configuration (`cmd/forecastiq/seed.go`, new `OpenWeatherConfigID`) — **disabled** pending ToS (D-05); staggered `:02` minute offset (WP-08 stagger note).
  - Tomorrow.io swap-path runbook (`docs/development/09-tomorrow-io-swap-path.md`) + capture script.
- **Reused (WP-05 framework)**: `providerhttp` transport (User-Agent, capped redirects, bounded reads, FC-08 retry/backoff, FC-13 classification, rate-limit normalization), `collection.Registry`, `ports` contracts, the canonical condition taxonomy, physical validation ranges, and the payload/collection pipeline — all unchanged.
- **Deferred**: OpenWeather ToS review (D-05) is a public-launch gate, not a code blocker — recorded, config seeded disabled. Live smoke against the real API requires an API key (manual, documented §10).
- **Final status**: Implementation Complete; awaiting CI on the pushed branch and Delivery Review Board.

## 2. Authorization and selection

| Check | Evidence | Result |
|-------|----------|--------|
| WP-06 Accepted | `06-work-package-status-registry.md` line 20 — "WP-07 may now be selected" | ✅ |
| WP-07 Selected | Registry line 21 — Programme Work-Package Selection Board, 2026-07-23 | ✅ |
| Hard dependency (WP-06) Accepted; pattern proven | Registry line 20 | ✅ |
| WP-07 definition found | `05-implementation-work-packages.md` §WP-07 | ✅ |
| Implementation authorized | User instruction "authorize implementation of WP-07" (2026-07-23) | ✅ |

## 3. Package identity

- **ID / title**: WP-07 — Second Forecast Provider (OpenWeather)
- **Authoritative source**: `docs/planning/05-implementation-work-packages.md` §WP-07
- **Previous state**: Selected — Not Started (registry line 21)
- **Resulting state**: Implementation Complete (Not Accepted)
- **Hard dependencies**: WP-06 (Accepted); platform WP-05/WP-04/WP-02 (Accepted)
- **Branch**: `feature/wp07-openweather-provider`

## 4. Scope reconstruction

| # | Approved scope item (§WP-07) | This package | Result |
|---|------------------------------|--------------|--------|
| S1 | Adapter (48-period hourly) | New `openweather` package; One Call 3.0 hourly, `exclude=current,minutely,daily,alerts`, `units=metric`; UTC epoch `dt` → UTC instant; `appid` credential | ✅ |
| S2 | Rate-budget enforcement (daily counter) | `budget.go` UTC-day counter; pre-emptive refuse when spent; 429 → pause (Retry-After or next UTC day); wired via `FIQ_PROVIDER_OPENWEATHER_DAILY_BUDGET` | ✅ |
| S3 | 401/429 handling fixtures | `onecall_401.json`, `onecall_429.json`; contract tests for auth-failed (no retry) and rate-limited (metadata) | ✅ |
| S4 | Swap-path documentation (Tomorrow.io slot) | `docs/development/09-tomorrow-io-swap-path.md` + capture script guidance | ✅ |
| S5 | Contract matrix | Full §1.2 matrix + budget tests (§7) | ✅ |
| Acc | Contract suite green; budget enforcement (429 → pause) | 22 tests green `-race`; dedicated pause + budget-exhausted tests | ✅ (live smoke: manual, key-gated) |

## 5. Implementation plan and completion (commit slices)

Per §WP-07 slices: (1) adapter; (2) budget + error fixtures; (3) contract suite.

| Slice | Planned outcome | Actual result | Status |
|-------|-----------------|---------------|--------|
| 1 | Adapter + happy path | `openweather.go`/`decompose.go`/`condition_map.go`; success decomposition green | ✅ |
| 2 | Budget + 401/429 error fixtures | `budget.go`; `onecall_401`/`onecall_429` + budget tests (429 → pause, exhaustion) | ✅ |
| 3 | Full contract suite + wiring + swap doc | matrix complete; app.go wiring; seed (disabled); config + `.env.example`; Tomorrow.io swap doc; this report | ✅ |

## 6. Architecture implementation

The adapter implements `ports.ForecastProviderAdapter` + `ports.ReplayDecoder`; dependency direction is `adapter → ports/platform`, wired only in the composition root (`cmd/forecastiq/app.go`). No new module, service, or persistence surface. The daily-budget guard lives inside the adapter package (provider-specific policy) and depends only on `internal/platform/clock` for determinism. Transport hardening, retry, and classification remain in the shared `providerhttp` helper — unchanged.

## 7. Contract-matrix traceability (`docs/testing/03-contract-testing.md` §1.2)

| # | §1.2 matrix row | Test | Fixture |
|---|-----------------|------|---------|
| T1 | Happy path decomposition | `TestFetch_Success` | `onecall_success_v3` |
| T2 | Timezone conversion (BR-PROV-01) | `TestFetch_RequestShape` (epoch `dt` → exact UTC; `units=metric`) | `onecall_success_v3` |
| T3 | Attribution fields | `TestFetch_AttributionFields` (`X-Request-Id`; no fabricated model-run time) | `onecall_success_v3` + header |
| T4 | Edge nulls | `TestFetch_EdgeNulls` | `onecall_edge_nulls` |
| T5 | Partial invalid (<50%) | `TestFetch_PartialInvalid` | `onecall_partial_invalid` |
| T6 | Schema drift — structural | `TestFetch_SchemaDrift` | `onecall_schema_drift` |
| T7 | Schema drift — >50% invalid → failed | `TestFetch_SchemaDrift_MajorityInvalid` | `onecall_majority_invalid` |
| T8 | Rate limit response (429) | `TestFetch_RateLimited` | `onecall_429` + `Retry-After` |
| T9 | Auth failure (401) | `TestFetch_AuthFailed` | `onecall_401` |
| T10 | Auth failure — no retry | `TestFetch_AuthFailed_NoRetry` | `onecall_401` + call counter |
| T11 | Condition unmapped | `TestFetch_UnmappedCondition` | `onecall_unmapped_condition` (781 tornado) |
| T12 | Replay determinism | `TestReplayDeterminism` / `TestDecodeStored_ReplayDeterminism` | `onecall_success_v3` (twice) |
| WP-07-A | Daily budget exhausted → no upstream call | `TestFetch_BudgetExhausted_NoUpstreamCall` | inline success + budget=2 |
| WP-07-B | 429 → pause → resume | `TestFetch_RateLimit_EngagesPause` | `onecall_429` → success |
| WP-07-C | Budget unit behaviour | `budget_test.go` (consume/reset/pause-resume/rest-until-next-day) | — |

Adjacent coverage: `TestFetch_ServerError` (provider_5xx), `TestCapabilities`, `TestFetch_RequestShape` (appid passed as query, never logged).

## 8. Database changes

```text
No schema/migration changes required.
```

One seed row added (idempotent `Upsert`): the OpenWeather operational `ProviderConfiguration` (`OpenWeatherConfigID`), status **disabled**, `credential_ref = FIQ_PROVIDER_OPENWEATHER_API_KEY`. The OpenWeather provider row itself was already seeded in WP-02. Because the config is disabled, `ListActiveConfigurations` excludes it and the scheduler generates no OpenWeather slots — the ToS gate (D-05) is honored without disabling code paths.

## 9. API changes

```text
No public API changes required.
```

`api/openapi/openapi.json` untouched; the `api-contract` CI gate validates the unchanged spec.

## 10. Reliability + live-collection acceptance (manual smoke, documented)

Retry/backoff, timeout, and rate-limit-header handling are owned by the shared `providerhttp` transport; the daily-budget guard adds outbound-budget protection (429 → pause). All are contract-tested (§7).

**Real-API smoke (key-gated) — manual procedure:**

1. Obtain an OpenWeather One Call 3.0 API key; export `FIQ_PROVIDER_OPENWEATHER_API_KEY`.
2. `make dev-up`; `./bin/forecastiq migrate up && ./bin/forecastiq seed`.
3. Activate the OpenWeather configuration (operator action once ToS D-05 is cleared) — it is seeded disabled.
4. Trigger one collection for the `openweather` provider and verify a `forecast_collections` row with `status=success`, ~48 `forecast_snapshots`, and a gzip payload under `data/payloads/openweather/{yyyy}/{mm}/{dd}/{collection_id}.json.gz` with a matching SHA-256 checksum.

Live provider calls are **not** run in CI (contract-testing doc §4); the fixture suite substitutes. The committed fixtures are hand-authored to the documented One Call 3.0 shape (no account key available in the build environment); `deploy/scripts/capture-openweather-fixture.sh` records real (sanitized) responses when a key is available.

## 11. Security

- Credential handling: the API key is resolved from `credential_ref` by the service (`Config.ResolveCredential`) and passed to the adapter as `req.Credential`; the adapter places it in the `appid` query parameter only. `providerhttp` does not log URLs/query strings, and `ProviderError.Error()` renders code + status only. The key is never logged and never committed (fixtures carry no key; capture script warns).
- SSRF: base URL is trusted seeded configuration, not user input (`providerhttp` package doc). Unchanged.
- Payload log-safety: payloads are never logged; error-message redaction is covered by the shared transport tests (WP-05).
- Scans: `govulncheck` (backend-checks) and `gitleaks` (security) run in CI on the pushed SHA.

## 12. Observability

Existing pipeline metrics apply unchanged: `provider_rate_limit_hits_total` increments on the rate-limited outcome (including the budget guard's pre-emptive refusal), `condition_unmapped_total` tallies FC-15 unmapped codes, and `provider_latency_seconds` / `collection_attempts_total` carry the `openweather` label. The `provider.registered` startup log now inspects the OpenWeather descriptor (48h horizon, requires_credential=true, supports_replay=true).

## 13. Tests

- **Unit / contract (adapter)**: `adapters/forecastproviders/openweather/openweather_test.go` + `budget_test.go` — 22 tests, all green under `-race`.
- **Regression**: full `go test ./...` green locally (all pre-existing suites unchanged and passing).
- **Lint / fmt / vet**: `gofmt -l` clean; `go vet ./...` clean; `golangci-lint run` clean on the changed packages.
- **Security**: `govulncheck`, `gitleaks` in CI.

## 14. Validation results (local)

- `gofmt -l` on all changed packages: no output (clean).
- `go build ./...`: success.
- `go vet ./...`: clean.
- `golangci-lint run ./adapters/forecastproviders/openweather/... ./cmd/... ./internal/platform/config/... ./internal/catalog/domain/...`: clean.
- `go test ./...`: all packages `ok` (openweather 22 tests green).

## 15. CI evidence

```text
PENDING — branch not yet pushed. The mandatory six-job CI run on the pushed
feature/wp07-openweather-provider tip must be captured before Delivery Review
Board acceptance (per the WP-05/WP-06/WP-08 evidence protocol): backend-checks,
backend-integration, migrations, api-contract, security, image — all green on
the exact head SHA, none skipped/cancelled. To be recorded here on push.
```

## 16. Files changed

- **New code**: `adapters/forecastproviders/openweather/{openweather.go,budget.go,decompose.go,condition_map.go}`
- **New tests**: `adapters/forecastproviders/openweather/{openweather_test.go,budget_test.go}`
- **New fixtures**: `test/fixtures/openweather/{onecall_success_v3,onecall_edge_nulls,onecall_partial_invalid,onecall_schema_drift,onecall_majority_invalid,onecall_unmapped_condition,onecall_401,onecall_429}.json`
- **Wiring / config / seed**: `cmd/forecastiq/app.go`, `cmd/forecastiq/seed.go`, `internal/catalog/domain/seed.go`, `internal/platform/config/config.go`, `.env.example`
- **Scripts**: `deploy/scripts/capture-openweather-fixture.sh`
- **Documentation**: `docs/development/09-tomorrow-io-swap-path.md`; this report; `docs/planning/06-work-package-status-registry.md`

## 17. ADRs

```text
No new architecture decisions required.
```

ADR-002 already records the provider scope and the OpenWeather → Tomorrow.io fallback; §16's swap-path doc operationalizes it.

## 18. Deviations

```text
No approved-scope deviations.
```

**Design note (recorded):** the OpenWeather operational configuration is seeded **disabled** rather than active. Rationale: §WP-07 lists ToS review (D-05) as a public-launch gate and an API key is required; seeding it active would generate hourly scheduler slots that fail 401 without a key. Disabled keeps the adapter fully wired and manually triggerable while respecting the gate and avoiding unattended-collection noise (WP-08 48h soak). Operator activates it once D-05 clears and a key is set.

## 19. Known limitations

- Live smoke is manual and key-gated (no key in the build environment); fixtures are authored to the documented One Call 3.0 contract and refreshed via the capture script.
- OpenWeather auto-collection is inactive until an operator enables the seeded configuration post-ToS.

## 20. Regression assessment

- WP-05 framework / `providerhttp` / registry: untouched (adapter is additive).
- WP-06 Open-Meteo adapter: untouched.
- WP-08 scheduler: untouched; the disabled OpenWeather config generates no slots.
- Mandatory CI controls (`ci.yml`): unchanged.

## 21. Work-package transition

```text
WP-07 — Second Forecast Provider (OpenWeather)

Previous State:
Selected — Not Started

New State:
Implementation Complete

Acceptance State:
Not Accepted (pending pushed-branch CI + Delivery Review Board)
```

## 22. Recommended next action

```text
Push feature/wp07-openweather-provider, capture the six mandatory CI jobs green
on the head SHA (§15), then convene the Delivery Review Board for WP-07.
```

## 23. Final status

```text
IMPLEMENTATION COMPLETE — READY FOR REVIEW (contingent on §15 CI evidence)
```
