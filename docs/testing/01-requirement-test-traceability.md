# ForecastIQ — Requirement ↔ Test Traceability

**Version**: 1.0 (UI ↔ Backend Reconciliation Board)
**Status**: Authoritative — every critical business rule and every mandated UI state maps to at least one test category
**Inputs**: NFR-M01/M02; methodology §10–11 (test vectors + property invariants); `docs/ui/06-ui-state-contracts.md`; `docs/requirements/04-acceptance-criteria.md`

Test categories: **Unit** (U), **Formula verification** (F — methodology test vectors), **Property-based** (P — fuzzed invariants), **Adapter contract** (AC — recorded provider fixtures), **API integration** (I), **Authorization** (Auth), **Frontend component** (FE), **Accessibility** (A11y), **End-to-end** (E2E), **Partial-data** (PD), **Stale-data** (SD), **Provider-failure** (PF), **Export** (EX), **Performance** (Perf).

---

## 1. Business Rule → Test Traceability

| Requirement/rule | U | F | P | AC | I | Auth | FE | A11y | E2E | PD | SD | PF | EX | Perf | Notes |
|------------------|---|---|---|----|----|------|----|----|-----|----|----|----|----|----|-------|
| BR-01 Immutability (forecasts/observations) | ✓ trigger tests | | | | ✓ update/delete rejected | | | | | | | | | | DB trigger + API attempts |
| BR-02 Separate observation storage | ✓ | | | | ✓ | | | | | | | | | | Schema test |
| BR-03 Accuracy requires both sides | ✓ | ✓ TV-3 pattern | ✓ prop 8 | | ✓ | | | | | ✓ | | | | | Null-pair exclusion |
| BR-04 Attribution displayed | | | | | ✓ payload field | | ✓ footer renders | ✓ | ✓ | | | | | | Per-provider config, not hardcoded |
| BR-05 Key scopes + rate limits | | | | | ✓ | ✓ | | | | | | | | ✓ 429 path | |
| BR-06 UTC storage | ✓ | | | ✓ adapter tz conversion | ✓ | | | | | | | | | | BR-PROV-01 adapter tests |
| BR-07 Backoff + retry | ✓ | | | | ✓ | | | | | | | ✓ | | | Sequence timing tests |
| BR-08 Credentials never exposed | | | | | ✓ no field in any response | ✓ | ✓ status-only UI | | | | | | | | Serializer-level |
| BR-09 Retention | | | | | ✓ partition drop + payload job | | | | | | | | | | Scheduled-job tests |
| BR-RANK-01 Ranking transparency fields | | | | | ✓ all fields present | | ✓ renders | | | | | | | | API schema test |
| BR-RANK-02 Thresholds 30/10 | ✓ | ✓ | | | ✓ status transitions | | ✓ badge copy | | ✓ | | | | | | Boundary: 9/10/29/30/31 |
| BR-RANK-03 Slow-horizon period extension | ✓ | | | | ✓ 90d auto | | | | | | | | | | +3d/+7d cells |
| BR-RANK-04 Coverage penalty + outranking rule | ✓ | ✓ worked example | ✓ prop 9 | | ✓ | | ✓ penalty display | | | | | | | | Penalty-on/off pairs |
| BR-RANK-05 Tie groups (CI overlap) | ✓ | ✓ | | | ✓ significant_vs_next | | ✓ tie annotation | ✓ | | | | | | | Overlapping/non-overlapping CIs |
| BR-RANK-06 Breakdown one interaction away | | | | | ✓ components in payload | | ✓ expand works | ✓ keyboard | ✓ | | | | | | |
| BR-RANK-07 Version changes → new rows | ✓ | | | | ✓ recompute creates new rows | ✓ admin | | | | | | | | | Old rows intact |
| BR-RANK-08 Null metric weight redistribution | ✓ | ✓ TV-3 | ✓ | | ✓ | | | | | ✓ | | | | | Dry-period scenario |
| BR-RANK-09 7-day minimum age | ✓ | | | | ✓ | | ✓ copy | | | | | | | | New-location scenario |
| BR-MATCH-01..06 Matching rules | ✓ | ✓ | ✓ prop 7 | | ✓ | | | | ✓ golden path | | | | | | Exact-hour UTC; ±15min sub-hourly; provenance rank; corrected preference |
| BR-OBS-01..04 Provenance | ✓ | | | ✓ type assignment | ✓ fields | | ✓ badge | ✓ | | | | | | | |
| BR-PROV-01 Adapter tz normalization | | | | ✓ per adapter | | | | | | | | | | | Contract fixtures |
| BR-LOC-01 Dedup 0.05° | ✓ distance calc | | | | ✓ 409 + override | ✓ admin | ✓ warning UI | | ✓ | | | | | | Boundary: 0.049°/0.051° |
| BR-LOC-03 Disable = stop future only | | | | | ✓ historical queryable | ✓ | | | ✓ | | | | | | |
| BR-ATTR-01 | (see BR-04) | | | | | | | | | | | | | | |
| BR-FRESH-01/02 Freshness | ✓ threshold calc | | | | ✓ server-computed field | | ✓ banner/badge | ✓ labels | ✓ | | ✓ | | | | Threshold boundary tests |
| BR-TZ-01..05 | ✓ bucketing | | | | ✓ tz echo | | ✓ zone labels | | | | | | | | BR-TZ-05 DST-edge buckets |
| BR-INV-01..03 Correction recompute | ✓ | | | | ✓ new rows + superseded_by | | | | ✓ | | | | | | Corrected-observation E2E |
| AUTH-01..09 | | | | | ✓ | ✓ full matrix | ✓ auth pages | ✓ | ✓ register→verify→login | | | | | | Incl. disable/delete propagation |
| FC-01..15 Collection | ✓ | | | ✓ schema drift, dedup, invalid rows | ✓ | | | | ✓ | ✓ partial | | ✓ circuit, rate limit | | ✓ cycle time | |
| OC-01..06 Observations | ✓ | | | ✓ | ✓ | | | | ✓ | | | ✓ source failure | | | |
| CE-01..11 Engine | ✓ | ✓ all TVs | ✓ all 11 props | | ✓ | | | | ✓ | ✓ missing skip | | | | ✓ batch < 10min | NFR-P06 |
| API-01..11 Conventions | | | | | ✓ pagination, ETag, idempotency, request-id, CORS | ✓ | | | | | | | | ✓ p50/p95/p99 | |
| DB-01..09 Dashboard | | | | | | | ✓ per screen | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ paint < 2s, CLS | |
| ADMIN-01..06 | | | | | ✓ | ✓ | ✓ | | ✓ | | | | | | |
| NFR-D06 Backup/restore | | | | | | | | | ✓ runbook drill | | | | | | Monthly automated restore test |
| NFR-SEC05/06 Input validation/SQLi | | | | | ✓ fuzz | | | | | | | | | | OWASP suite |

## 2. UI State → Test Traceability (every mandated state)

| UI state (state-contracts §1) | FE | I | PD | SD | PF | E2E | A11y | Trigger fixture |
|-------------------------------|----|----|----|----|----|-----|------|-----------------|
| Loading (skeleton, debounce, dim refetch) | ✓ | | | | | | ✓ aria-busy | Mocked latency > 100ms |
| No locations | ✓ | ✓ empty 200 | | | | ✓ | ✓ | Empty DB seed |
| No data for location | ✓ | ✓ | | | | ✓ | ✓ | Location w/o collections |
| Insufficient / provisional | ✓ | ✓ statuses | ✓ | | | ✓ | ✓ copy | 9/15/29-sample fixtures |
| Partial provider failure | ✓ | ✓ warnings[] | ✓ | | ✓ | ✓ | ✓ badge + banner | One provider circuit-open |
| Observation unavailable | ✓ | ✓ flag | ✓ | | ✓ obs source down | ✓ | ✓ | Observation gap fixture |
| Delayed / stale | ✓ | ✓ freshness | | ✓ | | ✓ | ✓ banner text | Aged last_updated fixtures |
| Unavailable | ✓ | ✓ | | ✓ | ✓ | | ✓ | 24h gap fixture |
| 401 / 403 | ✓ | ✓ | | | | ✓ | ✓ | Token scenarios |
| Validation failure | ✓ | ✓ errors[] | | | | | ✓ summary | Bad payloads |
| Conflict/duplicate | ✓ | ✓ 409 | | | | ✓ | | Near-duplicate location |
| Rate limited | ✓ | ✓ 429 + headers | | | | | | Exhausted bucket |
| Provider dependency failure (admin actions) | ✓ | ✓ 502 | | | ✓ | | | Circuit-open trigger attempt |
| Full failure + cached dim + 3-retry cap | ✓ | ✓ 5xx | | | ✓ | | ✓ role=alert | Mocked 500 |
| Offline + recovery | ✓ | | | | ✓ | | ✓ assertive banner | navigator.onLine mock |
| Timeout | ✓ | ✓ | | | ✓ | | | Abort deadline |

## 3. Screen → Required Test Set (MVP)

| Screen | Component tests | Integration touchpoints | E2E critical path | A11y pass |
|--------|----------------|------------------------|-------------------|-----------|
| S-01 | RankingTable, breakdown, badges, context line, states | `/rankings` contract (incl. observation_context, warnings) | Location switch → ranking render → breakdown expand | ✓ (table + keyboard) |
| S-02 | MetricTable sections, null handling, warning text | `/accuracy/summary` contract (incl. collection_window) | Horizon change → table update | ✓ |
| S-03 | Grid cells, tooltips, reliability/coverage explainer | provider-mode summary contract | Cell click → S-02 navigation | ✓ |
| S-04 | TrendChart, hollow points, aggregation control | `/accuracy` bucketing + tz echo | Point click → S-05 with date | ✓ (chart SR table) |
| S-05 | OverlayChart, gaps, legend, day table | `/forecast-comparison` contract | Date/variable change → chart + gaps | ✓ (chart SR table) |
| S-06 | Anchor navigation | methodology payload | Metric link from S-01 → anchor | ✓ |
| S-07 | Onboarding overlay, dismissal | `PATCH /me` default location | First-login → onboarding → dismiss → re-open | ✓ |
| S-08 | Forms, inline errors | Supabase SDK flows (mocked) | Register → verify → login | ✓ |
| S-09 | Key dialog (one-time display), danger zone | key CRUD, export job, delete | Create key → copy → revoke | ✓ |
| S-10 | HealthGrid, expanded actions, system panel | `/admin/health` contract; trigger 409/429 | Anomaly → re-collect → recovery | ✓ |
| S-11 | Edit dialog, credential status-only | status/config endpoints | Disable provider → scheduler skip | ✓ |
| S-12 | Form validation, dedup warning | POST 409 + override | Add location → first collection | ✓ |
| S-13 | Run rows, replay gating, recompute dialog | collections list, replay, recompute | Replay stored payload → new collection | ✓ |
| S-14 | User actions, lockout guard, audit filters | admin user endpoints | Disable user → login refused | ✓ |
| S-15 | Error pages, request_id display | status-code routing | — | ✓ |

## 4. Formula and Property Test Requirements (methodology §10–11, binding)

- All five test vectors (TV-1..TV-5) as golden-value tests.
- All eleven property invariants fuzzed (permutation invariance, RMSE ≥ MAE, null-safety, composite ∈ [0,1], byte-identical recomputation, etc.).
- Coverage: 100% of methodology formulas (NFR-M01).
- Adapter contract tests against recorded fixtures per provider (Open-Meteo, OpenWeather): schema drift detection, condition mapping, tz conversion, dedup behavior.

## 5. Performance Test Requirements

| Scenario | Target | Category |
|----------|--------|----------|
| API p50/p95/p99 under 100 req/s sustained | NFR-P01..P03, P05 | Perf (load test at Level 1 exit) |
| Comparison batch 100K pairs | < 10 min (NFR-P06) | Perf |
| Collection cycle all providers/locations | < 5 min (NFR-P07) | Perf |
| DB query p95 | < 100ms (NFR-P08) | Perf (pg_stat_statements) |
| Dashboard meaningful paint / CLS | < 2s / < 0.1 (NFR-P04, AC-6.3) | Perf (Lighthouse CI) |
| 2× MVP volume schema validation | Extrapolation to 100M rows documented (NFR-S01) | Perf (design review artifact) |

## 6. Gap Verdict

Every critical business rule, every mandated UI state, and every MVP screen maps to at least one test category above. **No untested critical surface remains.** Partial-data, stale-data, and provider-failure categories — historically the most neglected — have explicit fixture-driven coverage for every data-bearing screen per §2.
