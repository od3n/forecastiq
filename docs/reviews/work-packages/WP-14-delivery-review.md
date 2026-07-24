# ForecastIQ — WP-14 Provider Ranking: Delivery Review Board Report

**Version**: 1.0
**Review date**: 2026-07-23
**Work package**: WP-14 — Provider Ranking
**PR**: #12 (`feature/wp14-provider-ranking` → `main`)
**Reviewed SHA**: `f2d84461efe788c1b0608f87f683aee553ec670f` (`f2d8446`); code+test tip `04695ede2d6ea10bf86761e8be64455591599c6f` (`04695ed`)
**Authority**: `docs/planning/05-implementation-work-packages.md` §WP-14; `docs/domain/03-metric-methodology.md` §6–§8/§11; `docs/workflows/04-evaluation-and-ranking.md` §4/§6; `docs/data/03-table-design.md` §4; ADR-010; BR-RANK-01..09
**Decision**: **ACCEPTED** — no Critical/High/Medium/Low finding.

---

## 1. Scope of this review

First independent Delivery Review Board review of WP-14 (the analysis-phase capstone). Dependency gate satisfied: WP-13 **Accepted** and merged. The board independently re-verified commit identity, CI evidence, scope reconstruction, and ran an adversarial read of the diff `a09778c..f2d8446` — tracing the §8 worked-example arithmetic, the DB CHECK constraints, the normalization/penalty/status math, and the atomic supersede mechanics.

## 2. Commit identity + CI evidence (independently verified)

| Check | Evidence | Result |
|-------|----------|--------|
| Local HEAD == remote tip | `git rev-parse HEAD` == `git ls-remote origin …` == `f2d8446` | ✅ |
| Code+test lineage | code/test commits `808747a`/`b24320e`/`9d42cfe`/`ccb5438`/`04695ed`; commit since (`f2d8446`) is **docs-only** (`git diff --stat 04695ed..HEAD` → WP-14 report) | ✅ |
| CI on the code+test tip | run **30055978366** (`pull_request`, head `04695ed`) **success**, all six mandatory jobs green (none skipped/cancelled) | ✅ |
| `migrations` job | applied `20260801000010` (ranking_status enum + provider_rankings) | ✅ |
| `backend-integration` job | ran the §8 worked-example + supersede ranking tests against real PostgreSQL 16 | ✅ |
| PR mergeable | `MERGEABLE` | ✅ |
| `ci.yml` unchanged | not in diff | ✅ |

Local gate re-run on the reviewed tree: `gofmt -l` clean; `go vet` clean; `go test -race ./internal/... ./adapters/...` green (incl. the §8 unit + property 9/10 + tie tests); `go build -tags integration ./test/integration/...` compiles; `golangci-lint run` clean. (Docker unavailable in the review environment → the integration suite is validated via CI's green `backend-integration` on the exact SHA.)

**CI-history note:** one earlier `backend-integration` failure (`30055757944` on `ed7ae80`) was a **test-harness-only** FK gap (`seedCatalog` seeds only Open-Meteo; the §8 cohort needs OpenWeather + ProviderX rows), fixed in `04695ed`. No product code changed.

## 3. Scope reconstruction (§WP-14)

| # | Approved scope item | Verified | Result |
|---|---------------------|----------|--------|
| S1 | Cohort normalization (ε guard, null redistribution) | `RankCohort` §6.2 | ✅ |
| S2 | Weights (default + custom hash) | default w-2026.1; custom-hash weights are WP-15 on-demand (§6) | ✅ (default) |
| S3 | Coverage penalty + outranking rule | ×(cov/0.8) [0.5,0.8); cov<0.5 → unranked (BR-RANK-04) | ✅ |
| S4 | Statuses (30/10/7-day) | §7.2 + BR-RANK-02/09 | ✅ |
| S5 | CI propagation | first-order composite CI (documented approximation) | ✅ |
| S6 | Tie grouping | `RankOrder` CI-overlap groups (BR-RANK-05) | ✅ |
| S7 | Horizon profiles | `uniform` stored per-horizon; profile helpers for WP-15 on-demand | ✅ (stored uniform) |
| S8 | Atomic publication tx | per-cell insert-then-supersede tx (workflow §4) | ✅ |
| Acc | §8 worked example reproduced exactly | unit + real-PG integration (order + statuses exact; composites per corrected §8) | ✅ |

## 4. DR-06 assessment (methodology §8 correction)

The board **independently reproduced** the §8 arithmetic and confirms the published OM composite **0.940** is inconsistent with the doc's own normalized values × w-2026.1; the correct value is **0.9568** (OW 0.7772; PX pre-penalty 0.8974 → ×0.6875 = 0.6169). The DR-06 correction to `docs/domain/03-metric-methodology.md` §8 (totals only; formula/weights/normalized values/order/statuses unchanged) is **correct and warranted**. The ranking OUTCOME (OM > OW > PX; ranked/ranked/provisional) matches §8 exactly — the substantive acceptance is satisfied. This is a valuable catch (a latent methodology arithmetic error) analogous to DR-05.

## 5. Adversarial review — verified correct

The board's adversarial pass found **no** defects and independently confirmed:

1. **§8 arithmetic** — OM 0.9568, OW 0.7772, PX 0.6169 (traced normalization + penalty); matches the code + tests.
2. **DB CHECK safety** — `(composite IS NULL) = (unranked)` holds (unranked early-returns nil; else `clamp01`-set non-nil); `composite ∈ [0,1]` by construction + clamp; coverage/reliability bounds are an upstream-guaranteed safety net.
3. **Weight redistribution** — division by `activeWeight` is only reached for active components (so `activeWeight > 0`); no divide-by-zero.
4. **ε guard** — `best=0` → norm `ε/(value+ε)` ∈ (0,1]; no divide-by-zero.
5. **Status precedence** — ≥30 pairs but cov∈[0.5,0.8) → provisional; cov<0.5 → unranked; days<7 → provisional.
6. **Coverage penalty** — applied once, only to non-unranked, only cov<0.8; monotonic (property 9).
7. **Composite ∈ [0,1]** — each normalized ∈ [0,1], redistributed weights sum to 1, penalty ≤ 1; no NaN (property 10).
8. **Atomic supersede** — per-cell tx inserts live rows then supersedes prior (`id <> new`, `superseded_by IS NULL`, logical key incl. profile); non-unique index tolerates the in-tx window; trigger permits only the `superseded_by` transition; whole cohort in one tx.
9. **buildCohort** — |bias| = abs(signed bias); coverage = min across variables; sample counts from the right rows; missing component → excluded (redistribution); deterministic.
10. **SQL** — parameterized 17-col insert; `component_scores` JSON → jsonb; exact-period match.
11. **Tie grouping** — nil composites excluded; stable sort; leader-overlap grouping (valid §7.4 interpretation).
12. **Determinism/permutation-invariance** — best/active detection order-independent; component order fixed by the `Components` slice.

## 6. Findings

None. No Critical/High/Medium/Low finding.

## 7. Regression assessment

- WP-11/12/13 analysis code untouched except the additive `analysis_batch` extension (match → aggregate → **rank**); existing suites remain green.
- No API change; mandatory CI controls (`ci.yml`) unchanged.

## 8. Completion-gate assessment

| Gate | Result |
|------|--------|
| Exact WP-14 definition located | ✅ |
| Dependency (WP-13) Accepted | ✅ |
| Scope reconstruction complete; exclusions respected | ✅ |
| Methodology fidelity (§6–8 normalization/penalty/status/CI/ties) | ✅ |
| Acceptance (§8 reproduced) proven vs real PG16 | ✅ |
| SHA identity (local == remote == CI head) | ✅ |
| No Critical/High/Medium finding | ✅ |

## 9. Decision

**ACCEPTED.** WP-14 delivers the provider-ranking engine — cohort ratio-to-best normalization with ε guard and null redistribution, weighted composite (w-2026.1), coverage penalty, statuses, CI, tie grouping, and atomic supersede-on-recompute — CI-verified green on the exact code+test SHA `04695ed` including the `migrations` and real-PG `backend-integration` jobs that reproduce methodology §8 end-to-end. The adversarial review found no defect and confirmed both DB CHECK constraints are unviolable. **DR-06** (a latent §8 arithmetic error) was correctly discovered and remediated. This **completes the Analysis phase** (matching → evaluation → aggregation → ranking).

**Accepted Implementation SHA `f2d8446`** (code+test lineage `04695ed`). PR #12 ready to merge to `main`. **WP-15 (Dashboard Query APIs)** becomes eligible — public read endpoints over the catalog + analysis rows (envelope conventions, LRU/ETag, `/rankings` + methodology + `/accuracy` + `/locations` + `/providers`, cursor pagination, OpenAPI drift gate), depending on WP-14 + WP-03.

## 10. Tracked conditions

None.
