# ForecastIQ — WP-07 Second Forecast Provider (OpenWeather): Delivery Review Board Report

**Version**: 1.0
**Review date**: 2026-07-23
**Work package**: WP-07 — Second Forecast Provider (OpenWeather)
**PR**: #6 (`feature/wp07-openweather-provider` → `main`)
**Reviewed SHA**: `3b8760110523fd79e0525021da1f6da122d55040` (`3b87601`)
**Authority**: `docs/planning/05-implementation-work-packages.md` §WP-07; `docs/testing/03-contract-testing.md` §1.2; ADR-002 (provider scope + Tomorrow.io fallback); implementation report `WP-07-implementation.md`
**Decision**: **CONDITIONALLY ACCEPTED** — accepted on technical/architectural merits (no Critical/High); one blocking condition (**TC-07-01**) on the rate-budget/retry interaction, to be closed before the OpenWeather provider is activated.

---

## 1. Scope of this review

First independent Delivery Review Board review of WP-07. Dependency gate satisfied: WP-06 **Accepted** (registry line 20), the adapter pattern proven. The board independently re-verified commit identity, CI evidence, the approved-scope reconstruction, architecture boundaries, security, and the contract-test matrix, and performed an adversarial read of the diff `927106d..3b87601`.

## 2. Commit identity + CI evidence (independently verified)

| Check | Evidence | Result |
|-------|----------|--------|
| Local HEAD == remote tip | `git rev-parse HEAD` == `git ls-remote origin refs/heads/feature/wp07-openweather-provider` == `3b87601` | ✅ |
| CI ran on the exact reviewed SHA | CI run **30012298442** (event `pull_request`, PR #6) headSha == `3b8760110523fd79e0525021da1f6da122d55040` | ✅ |
| CI conclusion | **success** | ✅ |
| Six mandatory jobs green | `backend-checks`, `backend-integration`, `migrations`, `api-contract`, `security`, `image` — all `success`, none skipped/cancelled | ✅ |
| No `continue-on-error` | `.github/workflows/ci.yml` unchanged (not in diff) | ✅ |
| Prior implementation SHA also green | run **30011969157** on `1211878` (six jobs green) — closure diff `1211878..3b87601` is docs-only (registry + report) | ✅ |

The board re-ran the local gate on the reviewed tree: `gofmt -l` clean; `go vet` clean; `go test -race ./adapters/forecastproviders/openweather/...` green. **CI evidence is authoritative on the exact reviewed SHA `3b87601`** — no docs-descendant caveat required.

## 3. Scope reconstruction (§WP-07)

| # | Approved scope item | Verified | Result |
|---|---------------------|----------|--------|
| S1 | Adapter (48-period hourly) | `openweather` package; One Call 3.0 hourly; `exclude`/`units=metric`; UTC epoch `dt` → exact UTC; `appid` credential; `ReplayDecoder` | ✅ |
| S2 | Rate-budget enforcement (daily counter) | `budget.go` UTC-day counter; pre-emptive refuse when spent; **429 → pause** (Retry-After / next UTC day); clock-injected, mutex-guarded | ✅ (see DRB-WP07-001) |
| S3 | 401/429 handling fixtures | `onecall_401.json`, `onecall_429.json`; auth-failure-no-retry + rate-limit-metadata tests | ✅ |
| S4 | Swap-path documentation (Tomorrow.io) | `docs/development/09-tomorrow-io-swap-path.md` (bounded blast radius, steps, acceptance) | ✅ |
| S5 | Contract matrix | Full §1.2 matrix + budget tests (§6) | ✅ |
| Acc | Contract suite green; 429 → pause | `TestFetch_RateLimit_EngagesPause`, `TestFetch_BudgetExhausted_NoUpstreamCall` green under `-race` | ✅ |

Exclusions respected: **no** WP-09 (observation adapter) or WP-10 (observation collection) work; no analysis-layer code; no scheduler change (WP-08 already Accepted). No domain/service/persistence/API/migration change; `ci.yml` untouched.

## 4. Architecture + security assessment

- **Dependency direction correct**: the adapter imports `internal/collection/ports` + `internal/platform/*`; it is wired only in the composition root (`cmd/forecastiq/app.go`). No handler/domain leakage. The budget guard is provider-specific policy correctly co-located in the adapter package and depends only on `internal/platform/clock` for determinism.
- **Framework reuse, not fork**: transport hardening, FC-08 retry, FC-13 classification, and rate-limit normalization remain in the shared `providerhttp` helper — unchanged.
- **Security (verified correct)**: the API key flows only into the `appid` query parameter; `providerhttp` logs no URLs/bodies; `ProviderError.Error()` renders code+status only; committed fixtures contain no key; the capture script warns on sanitization. No SSRF surface (seeded base URL). `gitleaks` (security job) green.
- **ToS gate honored**: the OpenWeather operational configuration is seeded **disabled** (`ListActive` filters `status='active'`), so the scheduler generates no OpenWeather slots until an operator activates it post-ToS (D-05). This is the correct, conservative posture and is the reason DRB-WP07-001 has **zero current production impact**.

## 5. Contract-matrix verification (§1.2)

The board confirmed all §1.2 rows are covered (`TestFetch_Success`, `..._RequestShape` [BR-PROV-01 UTC + units], `..._AttributionFields`, `..._EdgeNulls`, `..._PartialInvalid`, `..._SchemaDrift`, `..._SchemaDrift_MajorityInvalid`, `..._RateLimited`, `..._AuthFailed`, `..._AuthFailed_NoRetry`, `..._UnmappedCondition`, replay determinism) plus the WP-07 budget tests and `budget_test.go` unit tests. Independent `go test -list` reports **21** discovered top-level tests (see DRB-WP07-003).

## 6. Findings

| ID | Severity | Summary | Disposition |
|----|----------|---------|-------------|
| DRB-WP07-001 | **Medium** | Daily budget under-counts actual upstream calls; a daily-quota 429 is retried by the shared transport before the pause engages | **TC-07-01 (blocking)** |
| DRB-WP07-002 | Low | `budget.pause()` uses a timestamp captured before the transport call; after retry backoff the Retry-After window resumes early | Fold into TC-07-01 |
| DRB-WP07-003 | Low (docs) | Implementation report states "22 tests"; actual discovered top-level tests = 21 | Correct the count |

### DRB-WP07-001 (Medium) — budget/retry interaction

`Adapter.FetchForecast` calls `budget.reserve()` **once** per invocation, before the transport call, but the wired transport retries retryable failures up to `MaxAttempts=5` (429 is classified `Retryable: true` in `providerhttp/client.go`). Consequences:

1. **Budget accuracy**: one `FetchForecast` can produce up to 5 real upstream HTTP requests (during a 5xx/timeout/429 storm) while the counter records 1 — so the configured daily ceiling (default 1000) can be exceeded by up to ~5× in the worst case. This weakens the guarantee that is the package's headline.
2. **Pointless 429 retries**: an OpenWeather free-tier 429 means the daily allowance is spent; retrying it 4 more times with exponential backoff (~15 s) burns wall-clock and hammers the provider before the `pause()` engages (the pause *does* correctly engage after the FetchForecast returns, so the over-call is bounded per paused cycle, not a runaway).

**Recommended fix**: do not retry a daily-quota 429 for this adapter — either cap `MaxRetries` to 1 in the OpenWeather wiring (losing FC-08 transient-5xx retry, acceptable given the circuit breaker + per-minute limiter), or (preferred, longer-term) add a retry-decision hook to `providerhttp.Config` so an adapter can opt a status out of retry while keeping 5xx/timeout retry. Counting actual attempts against the budget is an acceptable alternative.

**Why Medium, not High**: no data-corruption or security impact; the config is seeded **disabled**, so there is no live traffic today; the stated acceptance criteria (429 → pause; budget-exhaustion → pre-emptive refuse) are demonstrably met. It is nonetheless a real correctness gap in the core deliverable, so it is a **blocking condition to close before the provider is activated**.

### DRB-WP07-002 (Low) — stale pause timestamp

`now` is captured before the (potentially multi-second, retrying) transport call and reused for `budget.pause(now, retryAfter)`, so the pause window is anchored to a stale instant and can resume slightly early. Fix by reading `a.clock.Now()` at the pause site. Naturally resolved alongside DRB-WP07-001 (which removes the long retry delay).

### DRB-WP07-003 (Low, documentation)

`WP-07-implementation.md` §1 and §13 say "22 tests"; `go test -list` on the package reports 21 top-level tests. Correct to the verified count (mirrors WP-06 DRB-WP06-002).

## 7. Regression assessment

- WP-05 framework / `providerhttp` / registry: untouched (adapter additive).
- WP-06 Open-Meteo adapter: untouched.
- WP-08 scheduler: untouched; the disabled OpenWeather config generates no slots.
- Mandatory CI controls (`ci.yml`): unchanged. `migrations` seed×2 job green — the new idempotent (disabled) OpenWeather seed row is exercised.

## 8. Completion-gate assessment

| Gate | Result | Evidence |
|------|--------|----------|
| Exact WP-07 definition located | ✅ | §3 |
| Dependency (WP-06) Accepted | ✅ | registry line 20 |
| Scope reconstruction complete | ✅ | §3 |
| All approved requirements implemented | ✅ | §3, §5 |
| Exclusions respected (no WP-09/10) | ✅ | §3 |
| Architecture boundaries preserved | ✅ | §4 |
| Security (no secret/body/URL logging) | ✅ | §4 |
| Tests map to the contract matrix | ✅ | §5 |
| Local + CI validation green | ✅ | §2 |
| SHA identity (local == remote == CI head) | ✅ | §2 — `3b87601` |
| No Critical/High finding | ✅ | §6 |

## 9. Decision

**CONDITIONALLY ACCEPTED.** WP-07 is accepted on technical and architectural merits: 5/5 approved scope items implemented, the acceptance criteria proven (429 → pause; budget-exhaustion → pre-emptive refuse), a clean provider-agnostic implementation, secrets never logged, and all six mandatory CI jobs green on the exact reviewed SHA `3b87601`. No Critical or High finding.

**Blocking condition — TC-07-01**: remediate DRB-WP07-001 (do not retry a daily-quota 429 / make the daily budget reflect actual upstream calls) and, opportunistically, DRB-WP07-002. Correct DRB-WP07-003. Because the OpenWeather configuration ships **disabled**, TC-07-01 must be closed before the provider is activated (operator enablement post-ToS); it does not block merge of the disabled wiring.

## 10. Tracked conditions

| Condition | Severity | Requirement | Blocking |
|-----------|----------|-------------|----------|
| TC-07-01 | Medium | Fix the 429-retry/budget interaction (DRB-WP07-001) + stale pause timestamp (DRB-WP07-002); add/adjust a test proving a daily-quota 429 triggers exactly one upstream request and the budget counts actual calls | Yes (before provider activation) |
| TC-07-02 | Low | Correct the test count (22 → 21) in `WP-07-implementation.md` (DRB-WP07-003) | No |

## 11. Recommended next action

```text
Implementation team: run a WP-07 remediation pass for TC-07-01 (+ TC-07-02),
push to the branch, capture the six mandatory CI jobs green on the new head SHA,
then re-convene the board for a short confirmatory re-review. Only the Delivery
Review Board may transition WP-07 to Accepted.
```
