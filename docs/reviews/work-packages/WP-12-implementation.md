# ForecastIQ — WP-12 Pair-Level Evaluation: Implementation Report

**Version**: 1.0
**Implementation date**: 2026-07-23
**Work package**: WP-12 — Pair-Level Evaluation
**Authority**: `docs/planning/05-implementation-work-packages.md` §WP-12; `docs/domain/03-metric-methodology.md` §3/§4/§5/§6.4/§10/§11; ADR-010
**Branch**: `feature/wp12-pair-evaluation` (base: `main` `e1acf02`)
**Status**: **Implementation Complete — Not Accepted** (Delivery Review Board transition only)

> Scope discipline: this package is a **pure, in-memory evaluation kernel** — the statistical functions that turn matched pairs into per-variable metrics. It has **no** DB, migration, API, scheduler, or wiring. Aggregation into `accuracy_metrics` rows (with CIs, coverage/reliability, null-weight redistribution, supersession) is WP-13; composite scoring/ranking is WP-14.

---

## 1. Executive summary

- **Objective**: in-memory pair computations — continuous errors, categorical occurrence, Brier, observation-quality weighting — with all 5 methodology test vectors exact.
- **Implemented this package** (new `internal/analysis/eval`):
  - **Continuous** (`continuous.go`): weighted `MAE`/`RMSE`/`Bias` accumulator (methodology §4.1/§6.4). Weighted forms reduce to the unweighted definitions when all weights are 1.
  - **Categorical** (`categorical.go`): weighted precipitation-occurrence `Confusion` matrix → `Recall`/`Precision`/`F1`/`FalseAlarmRate`/`ThreatScore`/`OccurrenceAgreement` with the §5 null rules (zero denominator → `nil`, never 0/NaN); pairs contribute fractionally by weight (§6.4).
  - **Probabilistic** (`brier.go`): weighted `Brier` score (§4.3).
  - **Shared** (`eval.go`): `ProvenanceWeight` (§6.4), rain-event definitions `ForecastRain`/`ObservedRain` (§4.2), per-variable `Eligible` (§3), nullable results as `*float64`.
- **Test vectors**: TV-1..TV-5 all exact; classification boundaries (prob ≥ 0.5, amount > 0, mm ≥ 0.1); properties 1–8 fuzzed.
- **Deferred**: aggregation/CIs/coverage/reliability/supersede (WP-13); composite/normalization/ranking (WP-14); properties 9–11 (composite/recompute) belong there.
- **Final status**: Implementation Complete; awaiting pushed-branch CI + Delivery Review Board.

## 2. Authorization + selection

| Check | Evidence | Result |
|-------|----------|--------|
| WP-11 Accepted + merged | registry line 11; PR #9 merged `e1acf02` | ✅ |
| WP-12 dependency (WP-11) | matching engine + `matched_evaluations` in place | ✅ |

## 3. Scope reconstruction (§WP-12)

| # | Approved scope item | This package | Result |
|---|---------------------|--------------|--------|
| S1 | Evaluation kernel (pure functions) | `internal/analysis/eval` — accumulators + free functions, no I/O | ✅ |
| S2 | Observation-quality weighting | `ProvenanceWeight` (§6.4); weighted continuous + fractional categorical + weighted Brier | ✅ |
| S3 | Eligibility per variable | `Eligible` (both non-null, not suspect; §3) | ✅ |
| S4 | All 5 test vectors exact | TV-1..TV-5 unit tests pass to 1e-9/1e-4 | ✅ |

Exclusions respected: no persistence, aggregation, CI-interval math, coverage/reliability, or composite scoring (WP-13/14).

## 4. Architecture + key decisions

- **Pure kernel, correct placement**: lives in the analysis module (`internal/analysis/eval`), zero dependencies beyond the standard library. Accumulator pattern (`Add` then read) is composable by the WP-13 aggregation batch over chunked pairs and is inherently permutation-invariant (property 7).
- **Nullable via `*float64`**: the methodology's `null` (zero denominator, §5) is modelled as a `nil` pointer — never `0`, `NaN`, or `±Inf` (property 8). Callers (WP-13) map `nil` to a SQL `NULL` column.
- **Weighting keys off `observation_type`**: `corrected` observations retain their underlying type, so no special case; `suspect` is filtered by `Eligible` before it reaches an accumulator (§5/§6.4).
- **Weighted forms are the general case**: MAE/RMSE/Bias/Brier all use `Σ wᵢ·x / Σ wᵢ`; with all weights 1 they equal the unweighted definitions, so TV-1/TV-4 (unweighted) and TV-5 (weighted) share one implementation.

## 5. Tests

| Layer | Test | Proves |
|-------|------|--------|
| Test vectors | `eval_test.go` — `TestTV1..TV5` | continuous (1.375/1.75/0.875), categorical (0.8889/0.8/0.8421/0.1818/0.7273/0.85), zero-denominator nulls (TV-3), Brier (0.1925), weighted MAE (1.6667) |
| Boundaries | `TestEventBoundaries`, `TestEligible`, `TestProvenanceWeight` | prob ≥ 0.5 inclusive; amount > 0 strict; mm ≥ 0.1 inclusive; eligibility; provenance weights |
| Null safety | `TestEmptyAccumulators_Null` | empty accumulators return nil (property 8) |
| Properties (fuzz) | `properties_test.go` | P1 (MAE≥0, =0 iff zero), P2 (RMSE≥MAE, equality iff equal \|e\|), P3 (\|Bias\|≤RMSE), P4 (ratios & BS ∈ [0,1]), P5 (F1=2PR/(P+R); null iff P,R null), P6 (MAE stability), P7 (permutation invariance), P8 (no NaN/Inf) |

All 14 tests green under `-race`; `gofmt`/`go vet`/`golangci-lint` clean.

## 6. Database / API / security

None. No migration, no endpoint, no external call, no credential. Standard-library-only package.

## 7. CI evidence

Branch pushed; PR #10 → `main` triggered CI run **30041647144** (event `pull_request`) **success** on head SHA `6452c4936b5cb2b581128eebe924b5b9798bdbb7` (`6452c49`) with all six mandatory jobs green (`backend-checks`, `backend-integration`, `migrations`, `api-contract`, `security`, `image`), none skipped/cancelled; local == remote tip == CI head SHA. This WP adds no migration or integration test; the kernel is exercised by `backend-checks` (unit + `-race`), and `migrations`/`backend-integration` ran unchanged.

## 8. Files changed

- **Kernel**: `internal/analysis/eval/{eval.go,continuous.go,categorical.go,brier.go}`
- **Tests**: `internal/analysis/eval/{eval_test.go,properties_test.go}`
- **Docs**: this report; `docs/planning/06-work-package-status-registry.md`

## 9. Deviations

```text
No approved-scope deviations.
```

## 10. Work-package transition

```text
WP-12 — Pair-Level Evaluation
Previous State: Selected — Not Started
New State: Implementation Complete
Acceptance State: Not Accepted (pending pushed-branch CI + Delivery Review Board)
```

## 11. Recommended next action

```text
Convene the Delivery Review Board for WP-12. CI evidence is captured (§7):
run 30041647144 on head SHA 6452c49, six mandatory jobs green.
```
