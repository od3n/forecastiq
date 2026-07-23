# ForecastIQ — WP-12 Pair-Level Evaluation: Delivery Review Board Report

**Version**: 1.0
**Review date**: 2026-07-23
**Work package**: WP-12 — Pair-Level Evaluation
**PR**: #10 (`feature/wp12-pair-evaluation` → `main`)
**Reviewed SHA**: `7a8d52b2fee3a8d2da236d3839e6b061a61b0534` (`7a8d52b`); code+test tip `6452c4936b5cb2b581128eebe924b5b9798bdbb7` (`6452c49`)
**Authority**: `docs/planning/05-implementation-work-packages.md` §WP-12; `docs/domain/03-metric-methodology.md` §3/§4/§5/§6.4/§10/§11; ADR-010
**Decision**: **ACCEPTED** — no Critical/High/Medium/Low finding (one Informational, non-blocking).

---

## 1. Scope of this review

First independent Delivery Review Board review of WP-12. Dependency gate satisfied: WP-11 **Accepted** and merged. The board independently re-verified commit identity, CI evidence, scope reconstruction, and ran an adversarial read of the diff `e1acf02..7a8d52b` — tracing the arithmetic of all five test vectors, every null guard, event boundary, weighted formula, and the property tests.

## 2. Commit identity + CI evidence (independently verified)

| Check | Evidence | Result |
|-------|----------|--------|
| Local HEAD == remote tip | `git rev-parse HEAD` == `git ls-remote origin …` == `7a8d52b` | ✅ |
| Code+test lineage | code/test commits `577c129`/`0c87c89`/`cbb5bfc`; commit since (`7a8d52b`) is **docs-only** (`git diff --stat 6452c49..HEAD` → WP-12 report) | ✅ |
| CI on the code+test tip | run **30041647144** (`pull_request`, head `6452c49`) **success**, all six mandatory jobs green (none skipped/cancelled) | ✅ |
| PR mergeable | `MERGEABLE` (mergeState `UNSTABLE` only because the docs-commit run was queued at review time; the authoritative code+test-tip run is green) | ✅ |
| `ci.yml` unchanged | not in diff | ✅ |

Local gate re-run on the reviewed tree: `gofmt -l` clean; `go vet` clean; `go test -race ./internal/analysis/eval/...` green (14 tests); `golangci-lint run` clean. This WP adds no migration or integration test, so the kernel is exercised by `backend-checks` (unit + `-race`); `migrations` and `backend-integration` ran unchanged.

## 3. Scope reconstruction (§WP-12)

| # | Approved scope item | Verified | Result |
|---|---------------------|----------|--------|
| S1 | Evaluation kernel (pure functions) | `internal/analysis/eval` — stdlib-only accumulators + free functions, no I/O | ✅ |
| S2 | Observation-quality weighting | `ProvenanceWeight` (§6.4); weighted continuous + fractional categorical + weighted Brier | ✅ |
| S3 | Eligibility per variable | `Eligible` (both non-null, not suspect; §3) | ✅ |
| S4 | All 5 test vectors exact | TV-1..TV-5 verified by arithmetic trace + unit tests | ✅ |

Exclusions respected: no persistence, aggregation, CI-interval math, coverage/reliability, or composite scoring (WP-13/14); properties 9–11 correctly deferred.

## 4. Adversarial review — verified correct

The board's adversarial pass traced and confirmed:

- **TV arithmetic (exact)**: TV-1 MAE 1.375 / RMSE 1.75 / Bias 0.875; TV-2 0.8889/0.8/0.8421/0.1818/0.7273/0.85; TV-3 nulls + FAR 0.0 + agreement 1.0; TV-4 Brier 0.1925; TV-5 weighted MAE 1.6667.
- **Null semantics (§5)**: every ratio (`MAE`/`RMSE`/`Bias`/`Brier`/`Recall`/`Precision`/`F1`/`FAR`/`ThreatScore`/`OccurrenceAgreement`) guards its denominator with an explicit `== 0` → `nil`; the all-zero-weight case (`sumWeight==0`, `n>0`) returns nil. No 0/NaN/±Inf escapes.
- **Event boundaries (§4.2)**: `ForecastRain` prob `>= 0.5` (inclusive) OR amount `> 0` (strict); `ObservedRain` mm `>= 0.1` (inclusive); nil fields handled.
- **Weighted formulas (§6.4)**: RMSE = `√(Σw·e²/Σw)` (sqrt wraps the division), Bias = `Σw·e/Σw`, fractional confusion counts; `2*c.TP/d` = `(2·TP)/d` (correct precedence). `corrected` keyed off `observation_type` (documented, correct); unknown type → 0 (defensive).
- **Confusion `Add`**: all four `(forecastRain, observedRain)` combinations route correctly.
- **Determinism**: additive accumulators; permutation-invariant within ε (property 7, real shuffle).

## 5. Findings

**DRB-WP12-001 (Informational, non-blocking).** In `properties_test.go`, the property-2 equality branch (`RMSE = MAE` iff all `|e|` equal) and the property-5 null-iff assertion (`F1` null ⇔ `Precision` and `Recall` both null) rarely fire under random float inputs (identical `|e|`, or all-zero `TP/FP/FN`, are near-impossible from the RNG). Both invariants are algebraically correct by inspection and their conditions are separately covered by explicit vectors — TV-1's non-equal errors exercise `RMSE > MAE`, and TV-3 explicitly verifies the null `F1`/`Precision`/`Recall` path. Recommendation (optional, future): add two deterministic cases (all-equal `|e|`; a confusion with `TP=FP=FN=0`) to stress those exact branches. Non-blocking — does not affect correctness or acceptance.

No Critical/High/Medium/Low finding.

## 6. Regression assessment

- Additive-only: a brand-new leaf package (`internal/analysis/eval`) with no importers yet; no existing code paths touched.
- No migration/API/scheduler/config/CI-control change (`ci.yml` untouched).

## 7. Completion-gate assessment

| Gate | Result |
|------|--------|
| Exact WP-12 definition located | ✅ |
| Dependency (WP-11) Accepted | ✅ |
| Scope reconstruction complete; exclusions respected | ✅ |
| Methodology fidelity (formulas, null rules, weights, boundaries) | ✅ |
| All 5 test vectors exact | ✅ |
| Properties 1–8 covered | ✅ (one Informational test-strengthening note) |
| SHA identity (local == remote == CI head) | ✅ |
| No Critical/High/Medium finding | ✅ |

## 8. Decision

**ACCEPTED.** WP-12 delivers a faithful, pure evaluation kernel: weighted continuous errors, categorical occurrence with the methodology's null rules, Brier, observation-quality weighting, and per-variable eligibility — all five test vectors exact and properties 1–8 fuzzed. CI-verified green on the exact code+test SHA `6452c49`. The adversarial review found no defect; the single Informational note concerns test-branch stimulation only.

**Accepted Implementation SHA `7a8d52b`** (code+test lineage `6452c49`). PR #10 ready to merge to `main`. **WP-13 (Aggregated Metrics)** becomes eligible — it feeds matched pairs into this kernel to produce `accuracy_metrics` rows (CIs, coverage/reliability, null-weight redistribution, supersede-on-recompute).

## 9. Tracked conditions

None (DRB-WP12-001 is Informational, non-blocking).
