# ForecastIQ — WP-15 Dashboard Query APIs: Delivery Review Board

**Review date**: 2026-07-24
**Work package**: WP-15 — Dashboard Query APIs
**Reviewed SHA**: `20beb5958daf4787ec273ba6b07b0e7272514adc` (`20beb59`)
**Decision**: **ACCEPTED**

---

## 1. Verification of evidence

| Check | Result |
|-------|--------|
| Commit identity: local HEAD == `git ls-remote origin` == CI head | ✅ all `20beb59` |
| CI run **30059195672** (`pull_request`, head `20beb59`) | ✅ **success** |
| Six mandatory jobs green, none skipped/cancelled | ✅ `backend-checks`, `backend-integration`, `migrations`, `api-contract`, `security`, `image` |
| No `continue-on-error` / conditional gates | ✅ (all unconditional in `ci.yml`) |
| Dependency gate: WP-14 + WP-03 Accepted | ✅ (registry lines 14, 3) |
| Prior red run (30058982218 on `330a98e`) | test-only assertion (ProviderX name case); fixed in `20beb59`, no product-code change — verified via diff (`330a98e..20beb59` touches only `rankings_api_test.go` + report) |

## 2. Scope review (11/11 approved items)

S1–S11 + acceptance all implemented (implementation report §3). Independently spot-checked:

- **Envelope + caching**: content-based strong ETag; `If-None-Match`→304; per-class `Cache-Control` (`max-age=300` catalog, `60` analysis); errors `no-store`; TTL LRU with lazy expiry + LRU eviction. The buffering writer defers all flushes so the ETag decision precedes any byte write — verified by `TestCache_*` and the live `/rankings` ETag/304 integration test.
- **/rankings**: ranked→provisional→unranked ordering with CI-overlap tie groups via `domain.RankOrder`; the §8 worked example reproduces OM > OW > PX end-to-end vs real PG16; `observation_context` present; `coverage_penalty_applied` correct for PX (cov 0.55).
- **/rankings/methodology**: single-sourced from the engine constants (unit test asserts weights sum to 1.0 and thresholds match).
- **/accuracy/summary**: both modes; `collection_window` from snapshot MIN/MAX + ranking coverage/reliability (C-08).
- **/accuracy**: daily/weekly/monthly span isolation via a validated constant predicate; hollow points retained (null value + sample_count 0); 365-day bound + limit cap enforced (422 tested).
- **/providers**: `adapter_version`/`collecting_since` absent pre-collection, present after; `/providers/{id}` 404 on unknown; `/locations` bbox accepted (no-op).
- **Partial results**: active-but-absent provider → single `provider_unavailable` warning + `partial_result: true`; all-absent is not partial (freshness unavailable).
- **OpenAPI drift**: 14 paths; `make docs` + `api-contract` required-path list extended.

## 3. Architecture + security assessment

- Correct dependency direction: read model in `analysis` (no writes), read port implemented in `analysispg`; `collection_window`/lineage read through the **collection** module that owns `forecast_collections`/`forecast_snapshots` — no cross-module table reaching.
- All reads parameterized + live-row-scoped (`superseded_by IS NULL`); the only interpolated SQL is the validated aggregation-span constant and an integer `LIMIT`. No injection surface.
- No credentials, no external calls, no migration/schema change. Public endpoints only (AUTH-08); caching is public-class only (never admin/mutations).
- Ordering/ties reuse the domain engine, so the served rank cannot diverge from the composite it reports.

## 4. Adversarial checks (no defect found)

- **Cache vs middleware ordering**: rate-limit aborts before the cache middleware wraps the writer (429 unaffected); the global metrics/log middleware read the restored writer's real status after flush — verified correct for hit (200), conditional (304), and error (422/no-store) paths.
- **Cache key**: route + canonically-sorted params + auth-class; param permutations produce distinct keys; public endpoints share one entry (never per-user).
- **All-providers-absent**: correctly NOT partial (§4.2 rule 6).
- **Empty cohort / unknown location**: 200 with empty data + freshness unavailable (no panic; nil-safe).
- **Aggregation span**: monthly uses `+ interval '1 month'` (variable month length safe); daily/weekly exact.

## 5. Findings

| Finding | Severity | Summary | Disposition |
|---------|----------|---------|-------------|
| DRB-WP15-001 | Low (informational) | A cache **hit** returns the stored body, whose `metadata.request_id` and `metadata.generated_at` reflect the request that populated the entry (≤ TTL old). The authoritative per-request correlation id is the `X-Request-Id` **response header**, set per request. | Non-blocking; inherent to body caching (caching §1). Documented in the implementation report §4. Revisit if per-request body correlation becomes a requirement. |

No Critical/High/Medium finding.

## 6. Tracked / deferred (accepted, documented)

- On-demand cross-horizon **profile composites** (`short_term`/`daily_planning`) and **custom-weight** serving are a documented follow-on (WP-14 stores per-horizon `uniform`; `/rankings` serves those and rejects unsupported params honestly). Layers on without schema change.
- `/accuracy` **cursor pagination** + DST-aware `date_trunc` re-bucketing are WP-17 (responses already bounded by the 365-day range + limit cap).

## 7. Decision

**ACCEPTED.** WP-15 delivers the public dashboard read surface — envelope conventions, an in-process LRU + strong-ETag caching layer, `/rankings` (+observation_context) and `/rankings/methodology`, `/accuracy/summary` (both modes + collection_window) and `/accuracy` trends, `/providers` lineage + `/providers/{id}`, partial-result assembly, and response-size governance — CI-verified green on the exact code+test SHA `20beb59` including the `api-contract` drift gate and the real-PG `backend-integration` job. The adversarial review found no defect; one Low informational note (DRB-WP15-001) is non-blocking.

**Accepted Implementation SHA `20beb59`.** PR #13 ready to merge to `main`. **WP-16 (Forecast Evolution API) becomes eligible** — it depends on WP-08/WP-10 (data) + WP-15 (envelope), all Accepted.
