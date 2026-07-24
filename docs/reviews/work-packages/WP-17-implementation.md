# ForecastIQ — WP-17 Accuracy Analytics API: Implementation Report

**Version**: 1.0
**Implementation date**: 2026-07-24
**Work package**: WP-17 — Accuracy Analytics API
**Authority**: `docs/planning/05-implementation-work-packages.md` §WP-17; `docs/api/01-screen-api-contracts.md` §4; `docs/data/01-query-and-index-requirements.md` §bucketing; BR-TZ-02..05
**Branch**: `feature/wp17-accuracy-analytics-api` (base: `main` `e974d10`)
**Status**: **Implementation Complete — Not Accepted** (Delivery Review Board transition only)

> Scope discipline: this package completes the S-04 `GET /accuracy` trend endpoint with **tz-aware bucketing** over the WP-15 baseline. It changes no schema, adds no new endpoint, and reuses the WP-15 envelope/cache layer. The stored metric values are unchanged (UTC-computed per methodology §3.1); the refinement is presentation-layer timezone alignment.

---

## 1. Executive summary

- **Objective**: tz-aware trend bucketing (`date_trunc` over `period_start AT TIME ZONE tz`, post-scan) + hollow points + 365-day bound; provider grid mode completion.
- **Implemented**:
  - **tz-aware re-bucketing** in `analysis.ReadService.Trends`: the stored span-rows are grouped into the requested timezone's local **day/week/month** boundaries (Monday-based weeks) — the post-scan equivalent of `date_trunc(granularity, period_start AT TIME ZONE tz)` on ≤ 365 rows (query doc §bucketing). DST and variable month length are handled by `time.AddDate` normalization (a spring-forward day is 23 h, fall-back 25 h). Bucket boundaries are emitted as UTC instants of the tz-local midnights.
  - **Value semantics**: the common daily case is a **1:1 relabel** that preserves the stored value + CI; where a tz bucket spans multiple stored rows the values combine by **sample-count-weighted mean** (CI dropped, not recoverable from row values). **Hollow points** (sample_count 0, value nil) are preserved.
  - **Provider grid mode**: `/accuracy` continues to serve one series per provider (all providers, or a single `provider_id`) — the S-03/S-04 grid cell trends; ordering is deterministic.
  - **Bounds**: the 365-day range rejection + `limit` cap (from WP-15) are retained and re-tested.
- **No migration, no new endpoint, no new dependency.** OpenAPI `/accuracy` description updated; 15 paths.

## 2. Authorization + selection

| Check | Evidence | Result |
|-------|----------|--------|
| WP-13 Accepted (accuracy_metrics) | registry line 13 | ✅ |
| WP-15 Accepted (envelope + /accuracy baseline) | registry line 15; PR #13 merged `9c20c8c` (→ `e974d10`) | ✅ |

## 3. Scope reconstruction (§WP-17)

| # | Approved scope item | This package | Result |
|---|---------------------|--------------|--------|
| S1 | Trend bucketing (tz-aware, `date_trunc` post-scan) | `truncToBucket` + `trendGrouping` in the service | ✅ |
| S2 | Hollow points (sample_count per bucket) | preserved (null value + sample_count 0) | ✅ |
| S3 | 365-d bound enforcement | retained from WP-15 (422 tested) | ✅ |
| S4 | Provider grid mode completion | per-provider series (all / single provider_id), deterministic | ✅ |
| Acc | Bucketing across a DST boundary (tz echo); bound rejection; hollow points | unit (DST 23h/25h) + integration (tz-aligned buckets) | ✅ |

## 4. Architecture + key decisions

- **Post-scan bucketing in Go** (query doc §bucketing: "≤ 365 rows, trivial"): pure, deterministic, and DST-testable without a database. The repository query is unchanged (span-filtered stored rows in range).
- **UTC statistics, tz labels**: metric values remain the UTC-period aggregates the engine computed (methodology §3.1 matches in UTC — no local-time logic in the engine); the timezone only aligns bucket boundaries for display (BR-TZ-02..05). This is documented, not silent.
- **Combination rule**: multi-row tz buckets use a sample-count-weighted mean and drop CI (CIs are not reconstructable from stored per-row values); the dominant daily case is a 1:1 relabel that keeps value + CI intact — so no accuracy is lost in normal use.

## 5. Tests

| Layer | Test | Proves |
|-------|------|--------|
| Unit | `internal/analysis/read_test.go` | `truncToBucket` tz alignment (UTC+8 local-midnight window); DST edges (spring-forward 23 h, fall-back 25 h); monthly + Monday-week alignment; `Trends` keeps distinct tz days and relabels to local midnights preserving value; hollow-point retention |
| Integration (real PG16) | `test/integration/accuracy_api_test.go` | daily rows queried with `tz=Asia/Kuala_Lumpur` → 3 tz-aligned buckets (period_start at KL midnight = 16:00Z), tz echoed in `data.tz` + `metadata.timezone`; existing UTC bucketing + hollow point + 80 KB bound + validation (missing filters, bad aggregation, > 365 d) retained |

Full `go test -race ./internal/... ./adapters/...` green; `gofmt`/`go vet`/`golangci-lint` clean; `go vet -tags integration ./test/integration/...` compiles (Docker unavailable locally → real-PG runs in CI). `make docs` valid (15 paths).

## 6. Database / API / security

**No migration, no schema change, no new endpoint.** `/accuracy` behavior refined only. Reads unchanged (parameterized, live-row-scoped). No credentials/external calls. Public per AUTH-08; caching public-class only.

## 7. CI evidence

Branch pushed; PR #15 → `main` triggered CI run **30061310556** (event `pull_request`) **success** on head SHA `9531e56eea0a6c860c1b0c2e43d0f17cf698111a` (`9531e56`) with all six mandatory jobs green (`backend-checks`, `backend-integration`, `migrations`, `api-contract`, `security`, `image`), none skipped/cancelled; local == `git ls-remote origin` == CI head SHA. `backend-integration` ran the `/accuracy` tz-bucketing tests against real PostgreSQL 16 (tz-aligned buckets, UTC bucketing, hollow points, bound rejection). Green on the first run.

## 8. Deviations

```text
Cursor pagination on /accuracy remains omitted (not in the §WP-17 scope; responses
are bounded by the 365-day range + limit cap). Multi-row tz buckets combine by
sample-count-weighted mean with CI dropped; the dominant daily 1:1 case preserves
value + CI. Metric values are UTC-computed (methodology §3.1) with tz applied only
to bucket boundaries (BR-TZ display-layer).
```

## 9. Work-package transition

```text
WP-17 — Accuracy Analytics API
Previous State: Selected — Not Started
New State: Implementation Complete
Acceptance State: Not Accepted (pending pushed-branch CI + Delivery Review Board)
```

## 10. Recommended next action

```text
Push feature/wp17-accuracy-analytics-api and capture the six mandatory CI jobs on
the exact code+test SHA, then convene the Delivery Review Board for WP-17.
```
