# ForecastIQ — WP-15 Dashboard Query APIs: Implementation Report

**Version**: 1.0
**Implementation date**: 2026-07-24
**Work package**: WP-15 — Dashboard Query APIs
**Authority**: `docs/planning/05-implementation-work-packages.md` §WP-15; `docs/api/01-screen-api-contracts.md` §1–§6; `docs/api/02-response-conventions.md`; `docs/api/08-caching-and-partial-results.md`; `docs/domain/03-metric-methodology.md` §4–§7
**Branch**: `feature/wp15-dashboard-query-apis` (base: `main` `dd4c919`)
**Status**: **Implementation Complete — Not Accepted** (Delivery Review Board transition only)

> Scope discipline: this package adds the **public read surface** over pre-computed catalog + analysis rows — envelope conventions, LRU/ETag caching, `/rankings` (+observation_context), `/rankings/methodology`, `/accuracy/summary` (both modes + collection_window), `/accuracy` trends, `/locations`, `/providers` (+adapter_version/collecting_since), partial-result assembly, OpenAPI + drift gate. It adds **no writes** and no auth wiring (RequireAuth/Role middleware is WP-19), no FvA endpoint (WP-16), no admin/health (WP-18).

---

## 1. Executive summary

Delivered across five slices, each an independently green commit:

- **Slice 1 (`681a602`)** — read-path foundation: envelope `provenance` block + `methodology_version`/`weights_version` metadata; server-computed `freshness` helper (four states, clock-skew clamped); presentation-layer rounding by value kind (methodology §5 / conventions §7); an in-process TTL-bounded **LRU (256)** + strong content-based **ETag** middleware keyed by route + canonical-sorted params + auth class (`If-None-Match` → 304; per-class `Cache-Control`; errors never cached); `lru_cache_hits_total`/`lru_cache_misses_total`. Wired on `/locations`+`/providers` (catalog class, 300 s).
- **Slice 2 (`94dd680`)** — `GET /rankings` (S-01) and `GET /rankings/methodology` (S-06): new `analysis.ReadService` + `analysispg` read repository; ranked→provisional→unranked ordering with CI-overlap tie grouping reusing `domain.RankOrder`; per-component breakdown; `coverage_penalty_applied`; `observation_context` ground-truth line (NP-01 provenance record); methodology document single-sourced from the engine constants.
- **Slice 3 (`6acd88b`)** — `GET /accuracy/summary` (S-02 location mode / S-03 provider mode with `collection_window`) and `GET /accuracy` trends (S-04): tz-aware echo, daily/weekly/monthly aggregation over the stored period spans, hollow points (every bucket carries `sample_count`), 365-day range bound, limit cap.
- **Slice 4 (`1833808`)** — catalog amendments: `/providers` + `GET /providers/{id}` expose `adapter_version` (latest successful collection) + `collecting_since`; `/locations` accepts the reserved `bbox` param (documented no-op).
- **Slice 5 (`dcc703b`)** — partial-result assembly (active providers absent from a derived payload → `provider_unavailable` warnings, omitted from data; `partial_result` true) and response-size governance assertions.

**Deferred (documented follow-on, honest scoping)**: on-demand **cross-horizon profile composites** (`short_term`/`daily_planning`) and **custom-weight** serving. WP-14 stores per-horizon `uniform` rankings; `/rankings` serves those, defaults to +24h when `horizon_minutes` is omitted, accepts the profile enum, and returns an empty (freshness-unavailable) cohort for the unstored profiles rather than inventing a composite. Custom `weights` returns 422 in this release. These are WP-15 §6 "computed on demand, no write" concerns that can layer on without schema change.

## 2. Authorization + selection

| Check | Evidence | Result |
|-------|----------|--------|
| WP-14 Accepted + merged | registry line 14; PR #12 merged `dd4c919` | ✅ |
| WP-03 Accepted (identity use cases) | registry line 3 | ✅ |
| Dependencies (WP-14 `provider_rankings` + `accuracy_metrics`; WP-03) | in place | ✅ |

## 3. Scope reconstruction (§WP-15)

| # | Approved scope item | This package | Result |
|---|---------------------|--------------|--------|
| S1 | Envelope assembly (metadata/freshness/provenance/attribution/warnings/pagination) | `respond` envelope extended; freshness + rounding helpers | ✅ |
| S2 | LRU + ETag middleware | in-process TTL LRU (256) + strong ETag/304; per-class Cache-Control | ✅ |
| S3 | `/rankings` (+observation_context) | S-01 endpoint; ordering + ties + components + observation context | ✅ |
| S4 | `/rankings/methodology` | S-06 static doc from engine constants | ✅ |
| S5 | `/accuracy/summary` (both modes + collection_window) | location + provider modes; C-08 window | ✅ |
| S6 | `/accuracy` (trends) | tz echo, daily/weekly/monthly buckets, hollow points, 365-d bound | ✅ (baseline; DST re-bucketing is WP-17) |
| S7 | `/locations`, `/providers` (+adapter_version/collecting_since) | lineage fields; `/providers/{id}`; bbox reserved | ✅ |
| S8 | Cursor pagination | `/locations` keyset (pre-existing); trends bounded-by-range + limit | ✅ (trends cursor deferred to WP-17) |
| S9 | Rounding | `respond.RoundMetric` by (variable, metric_type); scores 4dp | ✅ |
| S10 | OpenAPI generation + drift CI | 14 paths + Provenance schema; `make docs` + `api-contract` required-path list extended | ✅ |
| S11 | Partial-result assembly | `provider_unavailable` warnings; `partial_result` flag; all-absent ≠ partial | ✅ |
| Acc | Screen contracts satisfied; drift gate green; response-size assertions | integration byte-size assertions (16/40/80 KB) | ✅ |

## 4. Architecture + key decisions

- **Read model in the analysis module**: `analysis.ReadService` owns no writes; ordering + tie grouping reuse `domain.RankOrder` so the served rank matches the methodology exactly. The `analysispg.ReadRepository` implements the read port; all queries hit the live (`superseded_by IS NULL`) surface and are parameterized.
- **Cache correctness**: the middleware fully buffers the handler response so a content-based ETag can be computed and a 200-vs-304 decision made before any bytes flush; only 200s are stored; non-200/errors get `no-store`. Keyed by route + sorted params + auth class (public), never per-user (caching §1). The cached body's `metadata.request_id` reflects the request that populated the entry; the per-request correlation id remains authoritative via the `X-Request-Id` **header** (set per request) — an accepted property of body caching.
- **Methodology single-sourcing**: `domain.Methodology()` reads the same weights/thresholds/penalty constants the ranking engine uses, so S-06 can never drift from the composite it explains (asserted by a unit test).
- **collection_window ownership**: `adapter_version`/`collecting_since` and snapshot MIN/MAX come from `forecast_collections`/`forecast_snapshots`, read through the **collection** module (`ProviderLineages` on `ForecastReader`) — the module that owns those tables — not by the analysis or catalog layer reaching across.
- **Aggregation spans**: `accuracy_metrics` has no period-kind column; daily/weekly/monthly are isolated by a validated constant `period_end − period_start` predicate (never user input).
- **Partial-result honesty**: active providers absent from a cohort are surfaced as `provider_unavailable` (omitted from data); an all-absent response is **not** partial (§4.2 rule 6) — `freshness=unavailable` communicates that instead.

## 5. Tests

| Layer | Test | Proves |
|-------|------|--------|
| Unit | `internal/api/cache_test.go` | miss→store→hit (handler runs once), If-None-Match→304, key includes sorted params, TTL expiry, non-200 never cached, LRU eviction |
| Unit | `internal/api/respond/{freshness,round}_test.go` | four freshness states + skew clamp; rounding by value kind; nil preserved |
| Unit | `internal/analysis/read_test.go` | rankings ordering (ranked→provisional→unranked) + CI-overlap ties + no-rows; location/provider summary assembly; trend grouping + hollow-point retention; methodology-vs-engine consistency (weights sum 1.0) |
| Unit | `internal/api/handlers/partial_test.go` | absent-provider warnings; all-present → none; nothing-servable → none |
| Integration (real PG16) | `test/integration/rankings_api_test.go` | §8 worked-example ordering + observation_context + versioning + ETag/304 + 16 KB bound; 422 validation; partial-result warning; methodology endpoint |
| Integration | `test/integration/accuracy_api_test.go` | both summary modes + collection_window + 40 KB bound; trends bucketing + hollow point + 80 KB bound; validation (missing filters, bad aggregation, >365 d) |
| Integration | `test/integration/providers_api_test.go` | lineage absent pre-collection then present on list+detail; 404 unknown provider; bbox accepted |

Full `go test -race ./internal/... ./adapters/...` green; `gofmt`/`go vet`/`golangci-lint` clean; `go vet -tags integration ./test/integration/...` compiles (Docker unavailable locally → real-PG runs in CI). `make docs` valid (14 paths).

## 6. Database / API / security

**No migration, no schema change.** New public GET endpoints only: `/rankings`, `/rankings/methodology`, `/accuracy/summary`, `/accuracy`, `/providers/{id}` (and lineage fields on `/providers`, bbox note on `/locations`). All reads are parameterized and live-row-scoped. No credentials/external calls. No auth middleware wiring (WP-19); endpoints are public per AUTH-08. Caching is public-class only (never admin/mutations).

## 7. CI evidence

Branch pushed; PR #13 → `main` triggered CI run **30059195672** (event `pull_request`) **success** on head SHA `20beb5958daf4787ec273ba6b07b0e7272514adc` (`20beb59`) with all six mandatory jobs green (`backend-checks`, `backend-integration`, `migrations`, `api-contract`, `security`, `image`), none skipped/cancelled; local == `git ls-remote origin` == CI head SHA. `backend-integration` ran the `/rankings`, `/accuracy/summary`, `/accuracy`, and `/providers` API tests against real PostgreSQL 16 (worked-example ordering, observation_context, ETag/304, collection_window, trend hollow points, partial-result warning, size bounds). One earlier run (30058982218 on `330a98e`) failed a **test-only** assertion (§8 ProviderX seeded as lowercase `providerx`); fixed in `20beb59` with no product-code change.

## 8. Files changed (by area)

- **respond**: `envelope.go` (provenance + version metadata + WeatherUnits), `freshness.go`, `round.go` (+ tests)
- **api**: `cache.go` (LRU + ETag middleware), `router.go` (routes + cache classes), `handlers/{rankings,accuracy,provider,location,partial,handlers}.go` (+ tests)
- **analysis**: `read.go`, `ports/read.go`, `domain/methodology.go` (+ `read_test.go`)
- **collection**: `collection.go`, `reader.go`, `ports/repositories.go` (ProviderLineages)
- **persistence**: `analysispg/readpg.go`, `collectionpg/collection.go`
- **platform**: `metrics/metrics.go` (cache counters)
- **composition + tests**: `cmd/forecastiq/app.go`, `test/integration/{setup,rankings_api,accuracy_api,providers_api}_test.go`
- **contract**: `api/openapi/openapi.json`, `Makefile`, `.github/workflows/ci.yml` (drift-gate required-path list)
- **docs**: this report; `docs/planning/06-work-package-status-registry.md`

## 9. Deviations

```text
On-demand cross-horizon profile composites (short_term/daily_planning) and
custom-weight serving are deferred as a documented follow-on: WP-14 stores
per-horizon uniform rankings; /rankings serves those (default +24h), accepts the
profile enum, and returns an empty freshness-unavailable cohort for unstored
profiles rather than inventing a composite; custom weights → 422 this release.
/accuracy trend cursor pagination is deferred to WP-17 (responses are bounded by
the 365-day range + limit cap); WP-17 also adds DST-aware date_trunc re-bucketing.
```

## 10. Work-package transition

```text
WP-15 — Dashboard Query APIs
Previous State: In Progress (selected 2026-07-24)
New State: Implementation Complete
Acceptance State: Not Accepted (pending pushed-branch CI + Delivery Review Board)
```

## 11. Recommended next action

```text
Push feature/wp15-dashboard-query-apis and capture the six mandatory CI jobs on
the exact code+test SHA, then convene the Delivery Review Board for WP-15.
```
