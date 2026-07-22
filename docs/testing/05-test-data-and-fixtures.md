# ForecastIQ — Test Data and Fixtures (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: Data classification rule (no production data in test envs); `docs/testing/02-testing-strategy.md`

---

## 1. Data Categories

| Category | Source | Location | Use |
|----------|--------|----------|-----|
| Provider response fixtures | Recorded real responses (sanitized) + handcrafted edge cases | `test/fixtures/{provider}/` | Adapter contract tests |
| API scenario fixtures | Synthetic (builder helpers in Go) | `internal/testutil/builders` | Integration tests |
| Golden-path seed | Deterministic generator | `test/e2e/seed` | E2E environment |
| Performance data | Synthetic seeder (fixed seed) | `test/perf/seeder` | k6 scenarios |
| Methodology vectors | Hand-computed (methodology §10) | Unit test tables | Formula exactness |
| UI state fixtures | Synthetic API responses (mock server) | `web/test/fixtures` | Screen state tests (all 19 states) |

**Binding rule:** production data (DB dumps, payloads, logs with real values) never enters the repository or CI environments. Synthetic only. Provider fixtures are provider responses (not user data) — sanitized of account identifiers, committed with attribution.

## 2. Canonical Test Location

Reference location for all fixtures: **Johor Bahru** (1.4927° N, 103.7414° E, tz `Asia/Kuala_Lumpur`, country `MY`) — matches the product's launch context and the methodology worked example. Secondary fixtures: a temperate location (e.g., Denver, for snow/sleet condition codes and DST display tests) and an arid location (zero-rain denominator behavior).

## 3. Weather Value Generators (synthetic plausibility)

| Variable | Tropical range (JB) | Notes |
|----------|--------------------|-------|
| temperature_c | 23–35 | Diurnal sine + noise |
| humidity_pct | 55–98 | Inverse-correlated with temp |
| precipitation_probability | 0–1 | Afternoon convective peak pattern |
| precipitation_amount_mm | 0–40/h (heavy tail) | ~15% of hours wet |
| wind_speed_ms | 0.5–12 | |
| pressure_hpa | 1005–1018 | |
| cloud_cover_pct | 0–100 | Correlated with precip |

Generators produce **forecast ≠ observation** with realistic error structure (provider A better at temp, provider B better at rain — so rankings have meaningful spread in tests).

## 4. Scenario Fixture Library (state contracts coverage)

Every UI state (state contracts doc, 19 states) has a fixture:

| State | Fixture |
|-------|---------|
| Fresh/normal | Standard seed |
| Empty (no data) | Location with zero collections |
| No location | Zero active locations |
| Insufficient data | 5 pairs (below provisional threshold) |
| Partial failure | One provider circuit open |
| Stale | Last collection 4 h ago |
| Unavailable | No success in 24 h |
| Loading / error / timeout / offline | Mock server behaviors |
| Permission denied | User token on admin endpoint |
| Conflict / validation / rate-limited | 409/422/429 responses |
| Cached-data label | 304 flow fixture |

## 5. Methodology Fixture Set

- TV-1..TV-5 exact vectors (methodology §10) — committed as test tables with expected values to 4 dp.
- Worked example (§8): full 3-provider input set + expected ranking table — integration test (ADR-010).
- Zero-denominator set (TV-3 pattern): no-rain period → null metrics + weight redistribution assertion.
- Tie scenario: two providers with overlapping CIs → same rank number.
- Coverage penalty boundary: 0.79 vs 0.80 vs 0.50 coverage cases.

## 6. Fixture Governance

| Rule | Detail |
|------|--------|
| Recording | Capture script (`scripts/capture-fixture.sh`) fetches real response → sanitizes (jq filter for account fields) → writes with metadata header |
| Versioning | Fixture filename carries schema version; drift additions never overwrite originals (append new file) |
| Review | Fixture changes require adapter-owner review (they encode provider behavior assumptions) |
| Size | Individual fixtures < 1 MB; large arrays trimmed to representative subset (first/last/edge entries) with `"_truncated": true` marker |
| Secrets | CI pre-commit check: no API keys/tokens in fixture files (pattern scan) |

## 7. Cross-Reference

- Contract tests consuming fixtures: `docs/testing/03-contract-testing.md`
- State contracts (UI): `docs/ui/06-ui-state-contracts.md`
- Methodology vectors: `docs/domain/03-metric-methodology.md` §10
