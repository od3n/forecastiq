# ForecastIQ — WP-14 Provider Ranking: Implementation Report

**Version**: 1.0
**Implementation date**: 2026-07-23
**Work package**: WP-14 — Provider Ranking
**Authority**: `docs/planning/05-implementation-work-packages.md` §WP-14; `docs/domain/03-metric-methodology.md` §6–§8/§11; `docs/workflows/04-evaluation-and-ranking.md` §4/§6; `docs/data/03-table-design.md` §4; ADR-010; BR-RANK-01..09
**Branch**: `feature/wp14-provider-ranking` (base: `main` `a09778c`)
**Status**: **Implementation Complete — Not Accepted** (Delivery Review Board transition only)

> Scope discipline: this package ranks providers from `accuracy_metrics` into `provider_rankings` (cohort normalization, weights, coverage penalty, statuses, CI, ties, supersede, atomic publication). It adds **no public API** (the read endpoints, custom-weight-on-demand, and cross-horizon profile serving are WP-15). It completes the Analysis phase.

---

## 1. Executive summary

- **Objective**: `ProviderRanking` rows per methodology §6–7.
- **Implemented**:
  - **Migration `20260801000010`** — the `ranking_status` enum (deferred from migration 1) + `provider_rankings` (per §4 DDL), a partial live-row index (publication surface), and a **supersede-only immutability trigger** (BR-RANK-07).
  - **Pure ranking engine** (`internal/analysis/domain/ranking.go`): `RankCohort` — ratio-to-cohort-best normalization with **ε guard** and **null-component weight redistribution** (§6.2), weighted composite (w-2026.1), **coverage penalty** (§7.3), **status** (ranked/provisional/unranked per §7.2 + BR-RANK-02/04/09), first-order **composite CI**, and `component_scores`. Plus `RankOrder` (CI-overlap **tie grouping**, BR-RANK-05) and horizon-profile constants. Deterministic + permutation-invariant.
  - **`RankService`** (`internal/analysis/rank.go`) + `analysispg` ranking repository: reads a cell's live `accuracy_metrics` across the cohort, maps to component inputs (|bias| from signed bias; coverage = min across variables), ranks, and writes rows in an **atomic per-cell publication tx** that supersedes the previous live rows.
  - **Pipeline**: the `analysis_batch` dispatcher now runs **match → aggregate → rank** (workflow §1); `ranking_rows_published_total` metric; composition-root wiring.
- **DR-06**: discovered and corrected a §8 arithmetic slip — the doc's OM composite (0.940) is inconsistent with its own normalized values × w-2026.1 (correct **0.957**; OW 0.777, PX 0.617). Formula/weights/order/statuses unchanged; methodology §8 corrected with a DR note.
- **Deferred**: read APIs, custom weights on demand, and cross-horizon profile serving (WP-15). Stored rows are per-horizon `uniform`; the profile-weight helpers are provided for WP-15's on-demand composite (§6 "computed on demand, no write").
- **Final status**: Implementation Complete; awaiting pushed-branch CI + Delivery Review Board.

## 2. Authorization + selection

| Check | Evidence | Result |
|-------|----------|--------|
| WP-13 Accepted + merged | registry line 13; PR #11 merged `a09778c` | ✅ |
| WP-14 dependency (WP-13) | `accuracy_metrics` in place | ✅ |

## 3. Scope reconstruction (§WP-14)

| # | Approved scope item | This package | Result |
|---|---------------------|--------------|--------|
| S1 | Cohort normalization (ε guard, null redistribution) | `RankCohort` (§6.2) | ✅ |
| S2 | Weights (default + custom hash) | default w-2026.1; custom-hash weights are a WP-15 on-demand concern (§6) | ✅ (default) |
| S3 | Coverage penalty + outranking rule | ×(cov/0.8) for [0.5,0.8); cov<0.5 → unranked (BR-RANK-04) | ✅ |
| S4 | Statuses (30/10/7-day) | §7.2 + BR-RANK-02/09 | ✅ |
| S5 | CI propagation | first-order composite CI (documented approximation) | ✅ |
| S6 | Tie grouping | `RankOrder` CI-overlap groups (BR-RANK-05) | ✅ |
| S7 | Horizon profiles | `uniform` stored per-horizon; profile weights provided for WP-15 on-demand | ✅ (stored uniform) |
| S8 | Atomic publication tx | per-cell insert-then-supersede tx (workflow §4) | ✅ |
| Acc | §8 worked example reproduced exactly | integration + unit tests (order + statuses exact; composites per corrected §8) | ✅ |

## 4. DR-06 — methodology §8 arithmetic correction

The §8 worked example's published composite for Open-Meteo (**0.940**) is arithmetically inconsistent with its own normalized values and the w-2026.1 weights, which sum to **0.9568**. OW (0.780→0.777) and PX (0.899→0.897 pre-penalty; 0.618→0.617 final) were also off by rounding. The **formula, weights, normalized values, ranking order (OM > OW > PX), and statuses (ranked/ranked/provisional) are unchanged** — only the arithmetic totals. `docs/domain/03-metric-methodology.md` §8 corrected with an inline DR-06 note; the WP-14 tests assert the corrected values.

## 5. Architecture + key decisions

- **Pure engine in `domain`**: all ranking math (`RankCohort`, `RankOrder`) is I/O-free → unit + property tested; `RankService` orchestrates I/O; the scheduler depends on a small `BatchRanker` interface; wiring only in the composition root.
- **Composite ∈ [0,1] by construction**: each normalized component ∈ [0,1] and redistributed weights sum to 1, so the composite ≤ 1; the coverage penalty only reduces it — the DB `CHECK (composite_score BETWEEN 0 AND 1)` and `CHECK ((composite_score IS NULL) = (unranked))` can never be violated (unranked → NULL, else non-null).
- **Atomic publication**: a cell's cohort rows are inserted (live) then the previous live rows superseded within one tx — readers never see a half-updated cohort (workflow §4). The non-unique live index tolerates the in-tx two-live-rows window; the supersede-only trigger permits exactly the `superseded_by` NULL→value mutation.
- **Coverage component** = min across the cell's per-variable coverage rows (§6.1); **|bias|** derived from the signed `bias` metric.

## 6. Tests

| Layer | Test | Proves |
|-------|------|--------|
| Unit | `domain/ranking_test.go` | **§8 worked example** (normalized values 3dp; composites 0.9568/0.7772/0.6169; statuses; order); **BR-RANK-04** (cov<0.5 unranked, excluded from order); **property 9** (penalty monotonic); **property 10** (composite ∈ [0,1], fuzz); **tie grouping** (CI overlap) |
| Integration (real PG16) | `test/integration/ranking_test.go` | **§8 end-to-end via DB** (OM 0.9568 ranked / OW 0.7772 ranked / PX 0.6169 provisional; order); **supersede-on-recompute** (byte-identical; one live, one superseded) |

Full `go test -race ./internal/... ./adapters/...` green; `gofmt`/`go vet`/`golangci-lint` clean; `go build -tags integration ./test/integration/...` compiles.

## 7. Database / API / security

Adds migration `20260801000010` (up + down): `ranking_status` enum, `provider_rankings`, live index, supersede-only trigger. No API change. No external calls/credentials. All SQL parameterized (the 17-column insert builds only `$n` placeholders); `component_scores` marshaled as JSON.

## 8. CI evidence

Branch pushed; PR #12 → `main` triggered CI run **30055978366** (event `pull_request`) **success** on head SHA `04695ede2d6ea10bf86761e8be64455591599c6f` (`04695ed`) with all six mandatory jobs green (`backend-checks`, `backend-integration`, `migrations`, `api-contract`, `security`, `image`), none skipped/cancelled; local == remote tip == CI head SHA. The `migrations` job applied `20260801000010`; `backend-integration` ran the §8 worked-example + supersede ranking tests against real PostgreSQL 16. One earlier integration failure was a test-harness-only FK gap (seedCatalog seeds only Open-Meteo) fixed in `04695ed`; product ranking code unchanged.

## 9. Files changed

- **Migration**: `migrations/20260801000010_create_provider_rankings.{up,down}.sql`
- **Domain**: `internal/analysis/domain/ranking.go`
- **Service/ports/persistence**: `internal/analysis/rank.go`, `internal/analysis/ports/ports.go`, `adapters/persistence/analysispg/rankingpg.go`
- **Scheduler/metrics/wiring**: `internal/scheduler/analysis_dispatcher.go`, `internal/platform/metrics/metrics.go`, `cmd/forecastiq/app.go`
- **Tests**: `internal/analysis/domain/ranking_test.go`, `test/integration/ranking_test.go`
- **Docs**: this report; `docs/domain/03-metric-methodology.md` (§8 DR-06 correction); `docs/planning/06-work-package-status-registry.md`

## 10. Deviations

```text
DR-06: methodology §8 composite totals corrected (arithmetic slip). Custom-hash
weights and cross-horizon profile serving are computed on demand at the API
(WP-15, §6 "no write"); WP-14 stores per-horizon uniform rankings and provides
the profile-weight helpers. Composite CI uses a first-order propagation
(documented approximation; §7.4).
```

## 11. Work-package transition

```text
WP-14 — Provider Ranking
Previous State: Selected — Not Started
New State: Implementation Complete
Acceptance State: Not Accepted (pending pushed-branch CI + Delivery Review Board)
```

## 12. Recommended next action

```text
Convene the Delivery Review Board for WP-14. CI evidence is captured (§8):
run 30055978366 on head SHA 04695ed, six mandatory jobs green.
```
