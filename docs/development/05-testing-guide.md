# Testing Guide

Testing follows the eight-layer strategy in `docs/testing/02-testing-strategy.md`.
The first slice implements unit, adapter-contract, DB-integration, and
API-integration layers.

## Running tests

```bash
make test                 # unit tests (race detector)
make test-integration     # DB + API integration (testcontainers; needs Docker)
make test-all             # both
make test-coverage        # unit coverage report
```

Integration tests carry the `integration` build tag and are excluded from
`make test`. CI runs them in a dedicated job.

## Test locations

| Kind | Location |
|------|----------|
| Unit | `*_test.go` alongside code (domain, config, ratelimit, payloadstore) |
| Adapter contract | `adapters/forecastproviders/openmeteo/openmeteo_test.go` + `test/fixtures/openmeteo/` |
| DB + API integration | `test/integration/` (build tag `integration`) |
| Fixtures | `test/fixtures/openmeteo/*.json` |

## What the slice covers

**Unit**
- Location validation + BR-LOC-01 haversine dedup boundary (exactly 0.05° permitted).
- Circuit breaker (5 failures → open; half-open probe after 60 s; success closes).
- Schedule slot-time enumeration (hourly + minute offset).
- Snapshot physical-range + temporal validation.
- Config fail-fast validation; token-bucket rate limiter; payload store round-trip + checksum.

**Adapter contract** (fixture-driven, no network)
- Happy path (exact field values, UTC normalization, probability ÷100, condition mapping, horizons).
- Edge nulls (nullable fields preserved).
- Partial invalid (out-of-range row rejected → `partial`).
- Schema drift (renamed `time` field → `failed` + `schema_drift`).
- Unmapped condition code → `unknown` + unmapped tally.
- 429 → `rate_limited`; 401 → `auth_failed`; 5xx → `failed`.
- Replay determinism (identical checksum + counts).

**DB integration** (testcontainers PostgreSQL 16)
- Migrations apply; partitioned `forecast_snapshots` exists.
- Collection idempotency (second collect → `deduplicated`, zero duplicate snapshots).
- Snapshot `ON CONFLICT DO NOTHING` dedup.
- Immutability triggers (completed collection + snapshots reject UPDATE/DELETE).
- `FOR UPDATE SKIP LOCKED` claiming (two concurrent claimers, no double-claim).

**API integration**
- Health/readiness probes.
- Auth gating (401 without token).
- Create + list location; trigger collection; latest forecast (attribution + freshness);
  collection lineage; OpenAPI served; RFC 7807 validation shape.

## Fixtures

Fixtures are recorded from real Open-Meteo responses and sanitized (Open-Meteo is
keyless). Re-capture with `deploy/scripts/capture-fixture.sh`. When a production
schema-drift alert fires, the offending payload becomes a new fixture (the suite
accumulates the provider's evolution — contract testing doc §1.3).

## Quality gates (per work package)

Unit coverage ≥ 80% on touched packages · contract tests for any adapter touched ·
integration tests for any endpoint/migration touched · zero golangci-lint warnings ·
OpenAPI valid · golden path green · no skipped tests without an issue reference.
