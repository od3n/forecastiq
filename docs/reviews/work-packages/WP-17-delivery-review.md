# ForecastIQ — WP-17 Accuracy Analytics API: Delivery Review Board

**Review date**: 2026-07-24
**Work package**: WP-17 — Accuracy Analytics API
**Reviewed SHA**: `9531e56eea0a6c860c1b0c2e43d0f17cf698111a` (`9531e56`)
**Decision**: **ACCEPTED**

---

## 1. Verification of evidence

| Check | Result |
|-------|--------|
| Commit identity: local HEAD == `git ls-remote origin` == CI head | ✅ all `9531e56` |
| CI run **30061310556** (`pull_request`, head `9531e56`) | ✅ **success** |
| Six mandatory jobs green, none skipped/cancelled | ✅ `backend-checks`, `backend-integration`, `migrations`, `api-contract`, `security`, `image` |
| Dependency gate: WP-13 + WP-15 Accepted | ✅ (registry lines 13, 15) |
| Docs-only lineage since code+test tip `5dba04c` | ✅ (`9531e56` = report + registry) |

## 2. Scope review (4/4 + acceptance)

- **tz-aware bucketing**: `truncToBucket` aligns stored span-rows to tz-local day/week/month boundaries (Monday weeks) — the post-scan `date_trunc(granularity, period_start AT TIME ZONE tz)` mandated by the query doc. Verified by unit (UTC+8 local-midnight = 16:00Z window) and the real-PG integration test (KL buckets at 16:00Z, tz echoed in `data.tz` + `metadata.timezone`).
- **DST correctness**: spring-forward day = 23 h, fall-back = 25 h via `time.AddDate` normalization (unit-proven for `America/New_York` 2026-03-08 / 2026-11-01).
- **Hollow points**: null value + sample_count 0 preserved.
- **Bounds**: 365-day range rejection + `limit` cap retained (422 tested).
- **Provider grid mode**: per-provider series (all / single `provider_id`), deterministic order.

## 3. Architecture + security assessment

- Post-scan bucketing lives in the analysis `ReadService` (pure, DST-testable without a DB); the repository query is unchanged. Correct dependency direction; no new module edge.
- **Statistical honesty**: metric values stay the UTC-computed aggregates (methodology §3.1 matches in UTC); the timezone aligns only bucket boundaries (BR-TZ display-layer). The daily case is a value-preserving 1:1 relabel; multi-row tz buckets combine by a documented sample-count-weighted mean with CI dropped (CIs not reconstructable from stored values). This is documented, not silent.
- No migration, no new endpoint, no credentials/external calls. Reads parameterized + live-row-scoped. Public per AUTH-08.

## 4. Adversarial checks (no defect found)

- **DST edges**: 23 h / 25 h days produce correct bucket instants (not double-counted or dropped).
- **Bucket collapse**: consecutive UTC daily rows in a UTC±N zone remain distinct tz days (not merged); the map key is the tz-local midnight instant.
- **Week/month alignment**: Monday-week start and first-of-month verified; `AddDate` handles variable month length.
- **tz=UTC**: identity relabel — WP-15 baseline behavior preserved (existing UTC bucketing test still green).
- **Ordering**: buckets sorted by start instant; series ordered by provider first-seen.

## 5. Findings

| Finding | Severity | Summary | Disposition |
|---------|----------|---------|-------------|
| DRB-WP17-001 | Low (informational) | Multi-row tz buckets combine by sample-count-weighted mean with CI dropped; exact re-aggregation would require the underlying pairs (not at the API). | Non-blocking; the dominant daily case is a 1:1 value+CI-preserving relabel. Documented. |

No Critical/High/Medium finding.

## 6. Decision

**ACCEPTED.** WP-17 completes the S-04 `GET /accuracy` endpoint with timezone-aware `date_trunc` bucketing (post-scan), correct DST handling, preserved hollow points, and the retained 365-day bound — CI-verified green on the exact code+test SHA `9531e56` including the `api-contract` drift gate and the real-PG `backend-integration` job. The adversarial review found no defect; one Low informational note (DRB-WP17-001, multi-row CI drop) is non-blocking.

**Accepted Implementation SHA `9531e56`.** PR #15 ready to merge to `main`. **WP-18 (Collection-Health API and Admin Operations) becomes eligible** — it depends on WP-08 + WP-03 (both Accepted).
