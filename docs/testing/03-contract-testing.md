# ForecastIQ — Contract Testing (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: NFR-M02/M09; R-01 (schema drift mitigation); FC-11; reconciliation testing readiness §12

Two distinct contract surfaces: **provider adapter contracts** (upstream) and **API contracts** (downstream). Consumer-driven Pact is deferred to Level 3 (constraints §3) — at MVP, fixture-based adapter tests + OpenAPI diff checks deliver the needed protection.

---

## 1. Provider Adapter Contract Tests

### 1.1 Fixture strategy

```text
test/fixtures/
├── openmeteo/
│   ├── forecast_success_v1.json          # recorded real response (sanitized)
│   ├── forecast_success_v1_edge.json     # nulls, missing optional fields
│   ├── forecast_partial_invalid.json     # some rows out of range
│   ├── forecast_schema_drift.json        # renamed fields (drift simulation)
│   ├── historical_success_v1.json
│   └── historical_corrected.json         # value change vs. prior fixture
└── openweather/
    ├── onecall_success_v3.json
    ├── onecall_429.json
    ├── onecall_401.json
    └── onecall_schema_drift.json
```

- Fixtures recorded from **real provider responses** (manual capture script; committed with attribution note); sanitized of any account identifiers.
- Each fixture pinned to `schema_version` + `adapter_version` in its metadata header comment.

### 1.2 Test matrix (per adapter)

| Test | Fixture | Asserts |
|------|---------|---------|
| Happy path decomposition | success | Snapshot count, field values (exact), issued_at UTC normalization, horizon derivation, condition mapping |
| Timezone conversion (BR-PROV-01) | success (local-time provider) | UTC conversion exact; contract-tested per adapter |
| Attribution fields | success | Provider request id / model run time captured when exposed |
| Edge nulls | edge | Nullable fields stored NULL; no crash; required-missing → invalid row |
| Partial invalid | partial_invalid | Status partial; counts exact; error_message content |
| Schema drift | drift | > 50% invalid → failed + error_code schema_drift; < 50% → partial |
| Rate limit response | 429 | Status rate_limited; Retry-After honored in bucket |
| Auth failure | 401 | Status failed; error_code invalid_credentials; no retry |
| Condition unmapped | fixture with unknown code | canonical = unknown; counter incremented |
| Replay determinism | success (twice) | Second run → deduplicated collection; zero new snapshots |

### 1.3 Drift drill (operational linkage)

When production schema_drift alert fires (runbook `docs/operations/06-provider-failure-runbook.md` §3), the captured current payload becomes a new fixture permanently — the contract suite accumulates the provider's evolution history.

### 1.4 Fake provider server (integration/e2e)

An in-test HTTP server serving fixtures with controllable behavior (latency injection, 429 sequences, mid-response failures) drives collection integration tests without real provider calls. Deterministic, CI-safe, no key required.

## 2. API Contract Tests (OpenAPI governance, NFR-M09)

### 2.1 Spec-as-contract

- OpenAPI 3.1 generated from code annotations (single source: the code).
- **CI gate 1:** generated spec vs. committed spec must match (drift check).
- **CI gate 2:** `openapi-diff` against main — breaking changes (removed endpoints/fields, type changes, new required params) fail the build.
- Additive changes pass (new optional fields, new endpoints) — v1 governance (API §7).

### 2.2 Response validation tests

Every API integration test validates responses against the generated OpenAPI schema (kin-openapi or equivalent middleware in test harness) — the spec and behavior cannot diverge silently.

### 2.3 Envelope contract tests

Binding conventions asserted explicitly:
- Envelope field applicability (no null placeholders — conventions §1)
- Freshness block presence + server-computed state on all time-sensitive payloads
- Provenance/attribution presence on derived payloads
- Warnings[] shape + closed enum codes
- Error taxonomy: every endpoint's documented error states producible in tests
- Rounding rules (4/2/1/2 dp per class)

### 2.4 Deprecation contract

Deprecated endpoints (future) must emit `Deprecation` + `Sunset` headers — test helper asserts when any endpoint marked deprecated.

## 3. Internal Interface Contracts (module seams)

- Module service interfaces (Go interfaces) have contract tests at the boundary: fake implementations verify consumer expectations (e.g., scheduler ↔ collector contract: CollectNow returns within timeout or error; circuit state consulted before call).
- Event seam payloads: schema_version field asserted; payload shape frozen tests (ADR-006 compatibility rule — any change requires version bump + test update).

## 4. What Is NOT Contract-Tested at MVP

- Consumer-driven negotiation (Pact) — single consumer (our dashboard), single team; OpenAPI + e2e covers it. Promotion trigger: external API consumers with independent release cycles (Level 3).
- Live provider smoke tests in CI (rate-limit unfriendly; replaced by fixture suite + manual weekly check script with one real call per provider).

## 5. Cross-Reference

- Strategy context: `docs/testing/02-testing-strategy.md`
- Fixtures: `docs/testing/05-test-data-and-fixtures.md`
- Drift runbook: `docs/operations/06-provider-failure-runbook.md` §3
- API governance: `docs/api/04-api-architecture.md` §6
