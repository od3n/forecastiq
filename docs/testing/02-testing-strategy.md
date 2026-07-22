# ForecastIQ — Testing Strategy (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: NFR-M01/M02/M09; `docs/testing/01-requirement-test-traceability.md` (reconciliation, binding traceability); methodology §10–11

---

## 1. Test Layers and Ownership

| Layer | Scope | Tooling | Runs | Gate |
|-------|-------|---------|------|------|
| Unit | Domain rules, formulas, normalization, matching selection, validation | Go testing + testify + gopter (property) | Every push (< 2 min) | Blocking |
| Adapter contract | Provider response parsing vs. recorded fixtures | Go testing + fixture files | Every push | Blocking |
| DB integration | Constraints, triggers, partitioning, claiming, migrations | testcontainers-go (PostgreSQL 16) | Every push (< 8 min) | Blocking |
| API integration | Contracts, authorization, pagination, partials, errors | httptest + testcontainers | Every push | Blocking |
| End-to-end (golden path) | Location → collection → observation → match → metric → rank → API | docker-compose environment + scripted scenario | Every merge to main | Blocking for main |
| Reliability | Timeout/rate-limit/malformed/duplicate/late/restart/reconnect | Fault-injection fakes + chaos-style scenarios | Weekly + pre-release | Blocking for release |
| Performance | Ingestion throughput, key queries, dashboard endpoints | k6 + synthetic data seeder | Weekly + pre-release | Threshold gates (NFR) |
| Frontend | Component + screen state contracts; axe-core a11y | Vitest/Testing Library + Playwright (critical flows) + axe | Every push | Blocking (a11y from first screen) |

## 2. Unit Test Requirements (NFR-M01: ≥ 80% on analysis/domain; 100% formulas)

### 2.1 Methodology formulas — complete coverage

- **All 5 test vectors** (methodology §10: TV-1..TV-5) as table-driven tests with exact expected values.
- **All 11 property invariants** (methodology §11) via gopter fuzzing (≥ 1,000 cases each in CI; 10,000 nightly):
  1. MAE ≥ 0; = 0 iff all errors 0
  2. RMSE ≥ MAE; equality iff |errors| equal
  3. |Bias| ≤ RMSE
  4. All ratios ∈ [0,1]
  5. F1 identity; null iff P and R null
  6. MAE stability under mean-error pair addition
  7. Permutation invariance
  8. No NaN/±Inf from null denominators
  9. Coverage penalty monotonicity
  10. Composite ∈ [0,1]
  11. Byte-identical recomputation
- **Worked example** (methodology §8, three providers) as integration-level test: fixture inputs → exact published ranking table (ADR-010 mandate).

### 2.2 Domain rules

- Matching candidate selection: provenance rank, corrected preference, top-of-hour tiebreak, determinism (property: shuffled candidate arrival → same chosen).
- Freshness state machine (thresholds per BR-FRESH table).
- Ranking eligibility/statuses (30/10 thresholds, 7-day minimum, coverage gates, tie grouping).
- Null/weight-redistribution behavior (BR-RANK-08).
- Location dedup (0.05° haversine; override flag).
- Condition mapping (unmapped → unknown + counter).
- Idempotency rules (snapshot uniqueness semantics; collection dedup).

## 3. Integration Test Requirements (NFR-M02)

**Golden path** (blocking): seed location + providers → fake provider serves fixture → collection → observation fixture → matching → metrics → rankings → `GET /rankings` returns expected envelope with provenance/freshness/attribution.

**Per-endpoint contract tests** (every endpoint in catalog):
- Happy path shape (OpenAPI-validated response)
- Auth matrix cases (public/user/admin × endpoint — authorization matrix as test source)
- Validation failures (422 + errors[] shape)
- Pagination (cursor stability, has_more, limit bounds)
- Partial results (fixture: one provider circuit-open → warnings[] + omission)
- Error envelope (request_id present, retryable flag, no leak assertions)
- Idempotency (same key+body → replay; same key different body → 409)
- Rate limiting (budget exhaustion → 429 + headers)
- ETag/304 behavior on cacheable endpoints

**DB integration specifics:**
- Immutability triggers raise on UPDATE/DELETE (all protected tables)
- Snapshot/observation dedup ON CONFLICT behavior
- SKIP LOCKED claiming under 2 concurrent claimers (no double claim)
- Lease expiry re-claim
- Partition creation + pruning mechanics
- Migration up/down (reversible ones) + idempotent re-run
- Export-job partial-unique 409 guard
- Self-lockout 409 guards

## 4. Security Tests (threat-model mapped)

| Threat | Test |
|--------|------|
| BOLA | Every object-scoped endpoint: access with other user's principal → 404/403 |
| Enumeration | 404 vs 403 indistinguishability assertions |
| Credential leakage | Response body grep for credential fixtures across all endpoints |
| Injection | Fuzz params with quote/semicolon payloads (validation rejects) |
| CORS | Non-allowlisted origin rejected |
| Rate limiting | Per-key and per-IP budget tests |
| Audit emission | Every admin mutation produces audit row (registry coverage) |
| Sanitization | Error responses contain no provider bodies/stack traces |

## 5. Quality Gates (minimum test gate per phase completion)

A work package is **not complete** without:
1. Unit coverage ≥ 80% on touched packages (formulas: 100% + properties).
2. Contract tests for any adapter touched (old + new fixtures).
3. Integration tests for any endpoint/migration touched.
4. Zero golangci-lint warnings (NFR-M03).
5. OpenAPI diff clean (no unintended breaking changes).
6. Golden path green.
7. No skipped tests without tracked issue reference.

**Release gate (launch):** all of the above + reliability suite green + performance thresholds met (NFR targets) + axe-core zero critical on all screens + manual keyboard/screen-reader pass on chart screens (accessibility doc mandate) + worked-example integration test reproducing methodology §8.

## 6. Test Environments

| Environment | Use | Data |
|-------------|-----|------|
| Unit | Pure logic | Synthetic in-memory |
| testcontainers (CI) | DB/API integration | Seeded synthetic fixtures (never production data) |
| docker-compose (local + e2e CI) | Golden path, reliability | Fixtures + fake providers |
| k6 (CI runner) | Performance | Synthetic seeder (100K snapshots) |

Production data never enters test environments (data classification rule).

## 7. Cross-Reference

- Traceability (rule → test): `docs/testing/01-requirement-test-traceability.md`
- Contract testing detail: `docs/testing/03-contract-testing.md`
- Performance: `docs/testing/04-performance-testing.md`
- Fixtures: `docs/testing/05-test-data-and-fixtures.md`
