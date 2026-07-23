# ForecastIQ — WP-09 Observation Source Adapter: Implementation Report

**Version**: 1.0
**Implementation date**: 2026-07-23
**Work package**: WP-09 — Observation Source Adapter
**Authority**: `docs/planning/05-implementation-work-packages.md` §WP-09 (definition); `docs/workflows/02-observation-collection.md`; ADR-003 (observation source strategy), ADR-025 (collection model); OC-04 range rules; domain architecture §2.7; decision log A-4 (reanalysis default)
**Branch**: `feature/wp09-observation-adapter` (base: `main` `275114e`)
**Status**: **Implementation Complete — Not Accepted** (Delivery Review Board transition only)

> Scope discipline: this package implemented the Open-Meteo Historical **observation source adapter** plus its domain model, port, validation (OC-04→suspect), provenance typing (reanalysis default), and correction-detection primitive, behind the WP-05 shared transport. It did **not** implement observation storage, scheduling, supersession writes, freshness gauges, or events — those are **WP-10** (Observation Collection). No composition-root wiring (no consumer until WP-10). No DB/API/migration change.

---

## 1. Executive summary

- **Objective**: Open-Meteo Historical observation adapter with provenance typing, mirroring the WP-05/06 forecast-adapter pattern.
- **Implemented this package**:
  - Observation domain model `internal/collection/domain/observation.go`: `Observation` root; `ObservationType` (station_observation / interpolated / reanalysis / provider_estimated) + `QualityFlag` (valid / suspect / corrected) enums; `RangeReasons()` (OC-04 physical ranges, reusing the shared snapshot range constants); `DiffersFrom()` correction primitive with per-variable ε thresholds (workflow §4).
  - Observation source port `internal/collection/ports/observation.go`: `ObservationSourceAdapter`, `ObservationRequest` (location + 2 h UTC window), `ObservationResult` (rows + counts + FC-13 classification, reusing `Outcome`/`RateLimit`).
  - Adapter `adapters/observationsources/openmeteo/` (`openmeteo.go`, `decompose.go`, `condition_map.go`): historical request (2 h window via `start_hour`/`end_hour`, `timezone=UTC`), schema `openmeteo-historical-v1`, UTC normalization, reanalysis-default provenance (configurable), OC-04 range→suspect flagging (rows kept, never dropped), WMO→canonical condition mapping, `observed_at ≤ window-end` invariant. Uses the shared hardened `providerhttp` transport (retry/rate-limit/FC-13).
  - Contract tests + 5 fixtures; domain unit tests (ranges, ε correction, provenance).
  - Capture script `deploy/scripts/capture-observation-fixture.sh`.
- **Deferred to WP-10**: storage tx + `ON CONFLICT` dedup, correction supersession UPDATE, `observation.collected`/`observation.corrected` event emission, freshness gauge, scheduler `:05` slots, composition-root wiring, and the `observations`/`observation_type`/`quality_flag` migration.
- **Final status**: Implementation Complete; awaiting pushed-branch CI + Delivery Review Board.

## 2. Authorization and selection

| Check | Evidence | Result |
|-------|----------|--------|
| WP-07 Accepted + merged | registry line 07 (Accepted); PR #6 merged `7101eea` | ✅ |
| WP-09 Selected | registry line 09 (`docs(planning): select WP-09`, `275114e`) | ✅ |
| Hard dependency (WP-05) Accepted | registry line 05 | ✅ |
| WP-09 definition found | `05-implementation-work-packages.md` §WP-09 | ✅ |
| Implementation authorized | User instruction (2026-07-23) | ✅ |

## 3. Scope reconstruction (§WP-09)

| # | Approved scope item | This package | Result |
|---|---------------------|--------------|--------|
| S1 | Adapter (2 h window) | `openmeteo` observation adapter; `start_hour`/`end_hour` UTC window; hourly measured variables | ✅ |
| S2 | observation_type resolution (API provenance, reanalysis default) | `DefaultObservationType` config, defaulting to `reanalysis` (ADR-003 / A-4); overridable for future station/interpolated sources | ✅ |
| S3 | Range validation (OC-04) | `Observation.RangeReasons()`; violations → `quality_flag=suspect`, row kept + counted (`SuspectCount`) | ✅ |
| S4 | Correction detection (value-diff beyond ε) | `Observation.DiffersFrom()` with per-variable ε (temp 0.1 °C, precip 0.05 mm, …); presence-mismatch differs | ✅ |
| S5 | Fixtures | 5 fixtures under `test/fixtures/openmeteo-historical/` | ✅ |
| Tests | Provenance tagging; suspect flagging; correction detection; dedup | contract matrix §5 | ✅ |

Exclusions respected: no storage/pipeline/scheduler/events/migration (WP-10); no analysis/matching (WP-11+); no composition-root wiring (no consumer yet).

## 4. Architecture

The adapter implements `ports.ObservationSourceAdapter`; dependency direction `adapter → ports/domain/platform`. Observations live in the **collection module** (table-design §3 groups them there). The adapter reuses the shared `providerhttp` transport unchanged (no framework change this package). Correction detection is a pure domain method (`DiffersFrom`) that the WP-10 pipeline will call against already-stored rows; the adapter itself performs no DB access and stores no raw payload (ADR-025).

## 5. Contract-matrix traceability

| # | Behaviour | Test | Fixture |
|---|-----------|------|---------|
| T1 | Happy path decomposition (values, UTC observed_at, condition, reanalysis provenance) | `TestFetch_Success` | `historical_success_v1` |
| T2 | Edge nulls (nullable fields; null weather_code → no condition) | `TestFetch_EdgeNulls` | `historical_edge_nulls` |
| T3 | OC-04 range violation → suspect (kept, counted) | `TestFetch_Suspect` | `historical_suspect` |
| T4 | Schema drift (missing hourly) → failed/schema_drift | `TestFetch_SchemaDrift` | `historical_schema_drift` |
| T5 | Server error → failed/provider_5xx | `TestFetch_ServerError` | inline 500 |
| T6 | Provenance default reanalysis | `TestFetch_ProvenanceDefault` | `historical_success_v1` |
| T7 | Provenance override (config) | `TestFetch_ProvenanceOverride` | `historical_success_v1` |
| T8 | Request shape (UTC, 2 h start/end, hourly vars) | `TestFetch_RequestShape` | `historical_success_v1` |
| T9 | observed_at ≤ window-end invariant | `TestFetch_FutureObservedAtRejected` | `historical_success_v1` |
| T10 | Correction detection + dedup (value-diff ε) | `TestCorrectionDetection` | `historical_success_v1` + `historical_corrected` |
| T11 | Non-retryable 4xx → exactly one request | `TestFetch_ClientError_NoRetry` | inline 400 |
| D1–D4 | Domain: type validity, ranges, ε correction, mutation invariant | `internal/collection/domain/observation_test.go` | — |

## 6. Database / API changes

```text
No schema/migration or API change in WP-09.
```

The `observations` table + `observation_type`/`quality_flag` enums (table-design §3; enums deferred by migration `20260801000001`) are created by **WP-10** (storage), matching the Go domain constants defined here.

## 7. Security

- Open-Meteo Historical is keyless; no credential handling.
- Base URL is seeded configuration, not user input (no SSRF surface; `providerhttp` doc).
- No raw payloads or URLs logged; observations store no payload (ADR-025).

## 8. Tests / validation (local)

- `internal/collection/domain/observation_test.go` (4 tests) + `adapters/observationsources/openmeteo/openmeteo_test.go` (11 tests) green under `-race`.
- Full `go test -race ./...` green (no regression). `gofmt -l` clean; `go vet` clean; `golangci-lint run` clean.

## 9. CI evidence

Branch pushed; PR #7 → `main` triggered CI run **30023733403** (event `pull_request`) **success** on head SHA `ffc47876c1199b87226c2a962c6ecd676050a121` (`ffc4787`) with all six mandatory jobs green (`backend-checks`, `backend-integration`, `migrations`, `api-contract`, `security`, `image`), none skipped/cancelled; local == remote tip == CI head SHA. The subsequent docs-only CI-evidence commit is a descendant of the CI-verified SHA (no code/CI change), so run `30023733403` on `ffc4787` remains the authoritative implementation CI evidence.

## 10. Files changed

- **New code**: `internal/collection/domain/observation.go`, `internal/collection/ports/observation.go`, `adapters/observationsources/openmeteo/{openmeteo.go,decompose.go,condition_map.go}`
- **New tests**: `internal/collection/domain/observation_test.go`, `adapters/observationsources/openmeteo/openmeteo_test.go`
- **New fixtures**: `test/fixtures/openmeteo-historical/{historical_success_v1,historical_edge_nulls,historical_suspect,historical_corrected,historical_schema_drift}.json`
- **Scripts**: `deploy/scripts/capture-observation-fixture.sh`
- **Documentation**: this report; `docs/testing/03-contract-testing.md` (§1.1 fixture tree); `docs/planning/06-work-package-status-registry.md`

## 11. Deviations / recorded discrepancies

- **DR-04 (documentation-only):** contract-testing doc §1.1 originally listed the observation fixtures (`historical_*`) under `test/fixtures/openmeteo/`. They are committed under a dedicated `test/fixtures/openmeteo-historical/` directory to keep forecast and observation fixtures separate (distinct adapters/schemas). §1.1 updated to match the committed layout.

## 12. Work-package transition

```text
WP-09 — Observation Source Adapter
Previous State: Selected — Not Started
New State: Implementation Complete
Acceptance State: Not Accepted (pending pushed-branch CI + Delivery Review Board)
```

## 13. Recommended next action

```text
Convene the Delivery Review Board for WP-09. CI evidence is captured (§9):
run 30023733403 on head SHA ffc4787, six mandatory jobs green.
```
