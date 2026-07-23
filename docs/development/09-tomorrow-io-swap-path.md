# ForecastIQ — Provider Swap Path: OpenWeather → Tomorrow.io

**Version**: 1.0
**Status**: Approved — WP-07 (Second Forecast Provider)
**Authority**: ADR-002 (provider scope + documented fallback); dependency D-05 (OpenWeather ToS gate); `docs/development/08-provider-adapter-authoring-guide.md`

ADR-002 selected **Open-Meteo + OpenWeather** for the MVP, *conditioned on the
OpenWeather Terms-of-Service review (D-05)* and with a **documented fallback**:
if the ToS review fails, swap OpenWeather → **Tomorrow.io** as the second
provider. The adapter interface bounds that change. This document is the
concrete swap runbook so the fallback is a mechanical, low-risk operation rather
than a redesign.

The swap is **code-bounded to one new adapter package plus composition-root
wiring and one seed row.** No domain, service, scheduler, persistence, API, or
migration change is required — the collection pipeline is provider-agnostic
(WP-05 framework).

---

## 1. When to trigger

Per ADR-002 §"Migration trigger", add/replace the second provider when **any**
holds:

- The OpenWeather ToS review (D-05) fails or lapses before public launch; or
- OpenWeather's free-tier / attribution / redistribution terms become
  incompatible with the product; or
- Evidenced user demand for Tomorrow.io (≥ 20 requests) at the Level-3 gate.

Until a trigger fires, OpenWeather remains wired but its operational
configuration ships **disabled** (`cmd/forecastiq/seed.go`), so the scheduler
generates no OpenWeather slots.

## 2. What stays unchanged (the bounded blast radius)

Everything below is provider-agnostic and is **not** touched by the swap:

- `internal/collection/ports` — the `ForecastProviderAdapter` / `ReplayDecoder`
  contracts, `ForecastResult`, FC-13 `ProviderError` taxonomy.
- `internal/collection` — the collection use case, dedup, circuit breaker,
  registry, replay.
- `adapters/forecastproviders/providerhttp` — shared transport (User-Agent,
  capped redirects, bounded reads, FC-08 retry/backoff, rate-limit
  normalization).
- `internal/scheduler`, `internal/platform/*`, persistence, API, migrations.
- The canonical condition taxonomy v1 and the physical validation ranges.

## 3. Swap steps

### 3.1 Add the adapter package

Create `adapters/forecastproviders/tomorrowio/` following the authoring guide
(§1–§9) and using `adapters/forecastproviders/openweather` as the closest
reference (both are keyed, hourly, budget-sensitive providers):

- `tomorrowio.go` — identity (`ProviderSlug = "tomorrow-io"`,
  `SchemaVersion = "tomorrowio-v1"`, `AdapterVersion = "1.0.0"`),
  `Capabilities` (hourly; `RequiresCredential: true`; `SupportsReplay: true`;
  `MaxForecastHorizon` per the chosen Tomorrow.io tier), `FetchForecast`,
  `DecodeStored`, and `buildURL`. Tomorrow.io's Timelines API takes the key in
  the `apikey` query parameter (adjust `buildURL` accordingly; never log it).
- `decompose.go` — parse the Timelines `intervals[]` array; `startTime` is
  RFC3339 UTC (normalize to UTC, no offset math); map fields to the canonical
  snapshot (temperature, `temperatureApparent`, `humidity`, `windSpeed`,
  `windDirection`, `pressureSurfaceLevel`, `cloudCover`,
  `precipitationProbability` → `[0,1]`, `rainAccumulation`/`snowAccumulation` →
  mm). Reuse the same drift/partial classification (> 50 % invalid → failed).
- `condition_map.go` — map Tomorrow.io `weatherCode` values to canonical
  taxonomy v1; tally unmapped codes into `result.UnmappedConditions` (FC-15).
  Never guess a mapping.
- **Rate budget:** Tomorrow.io free tier enforces per-second / per-hour / per-day
  limits. Reuse the WP-07 daily-budget guard pattern
  (`openweather/budget.go`) — a UTC-day counter with 429 → pause — parameterized
  for Tomorrow.io's daily allowance, plus the shared per-minute
  `ratelimit.Limiter` for the finer windows.

### 3.2 Wire it in the composition root

In `cmd/forecastiq/app.go`, build the adapter (timeout, limiter, daily budget
from config) and `registry.Register(tioAdapter)`. The registry validates
identity/versions and the replay contract at startup.

### 3.3 Seed the operational configuration + attribution

- Add `TomorrowIoProviderID` / `TomorrowIoConfigID` to
  `internal/catalog/domain/seed.go`.
- In `cmd/forecastiq/seed.go`, upsert the Tomorrow.io provider row (mandatory
  `attribution_text` / `attribution_url`, BR-ATTR-01) and its
  `ProviderConfiguration` (`CredentialRef: "FIQ_PROVIDER_TOMORROWIO_API_KEY"`,
  staggered `MinuteOffset`). If Tomorrow.io **replaces** OpenWeather, disable the
  OpenWeather config in the same change; if it is **added**, leave both.
- Add `FIQ_PROVIDER_TOMORROWIO_API_KEY` and
  `FIQ_PROVIDER_TOMORROWIO_DAILY_BUDGET` to `.env.example` and the credential
  map in `internal/platform/config`.

### 3.4 Contract test matrix

Add `tomorrowio_test.go` + committed fixtures under
`test/fixtures/tomorrow-io/` covering the full §1.2 matrix
(`docs/testing/03-contract-testing.md`) — success, edge nulls, partial invalid,
schema drift (structural + majority), 429 (rate limit metadata), 401 (auth,
no retry), unmapped condition, replay determinism — plus the daily-budget
enforcement tests (budget exhausted → no upstream call; 429 → pause → resume),
mirroring `openweather_test.go`. Add a `capture-tomorrowio-fixture.sh` script.

## 4. Acceptance for the swap

- Registry logs the new provider at startup (`provider.registered`).
- Contract matrix green under `-race`; budget tests green.
- One live collection into a local DB verified (manual smoke, key required).
- If replacing: OpenWeather config disabled; no scheduler slots generated for it.
- ADR-002 updated (status note) and the work-package registry records the swap.

## 5. Cross-reference

- Provider scope + fallback decision: `docs/adr/ADR-002-provider-scope.md`.
- Adapter authoring: `docs/development/08-provider-adapter-authoring-guide.md`.
- Contract matrix: `docs/testing/03-contract-testing.md` §1.2.
- Reference implementation: `adapters/forecastproviders/openweather`.
