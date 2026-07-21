# ADR-010: Composite Scoring Methodology

**Status**: Accepted (Phase 0 Amendment)
**Date**: 2026-07-22

## Context
Blocker 1: the "composite accuracy score" — the product's headline output — was never
defined in Phase 0. The amendment mandated a full methodology: metric-level results,
normalization, weights, minimum samples, missing-data treatment, horizon and
observation-quality weighting, coverage penalty, statistical confidence, versioning,
worked examples, and ranking statuses.

## Options considered
1. **Weighted ratio-to-cohort-best normalization** with coverage penalty, sample
   thresholds (30/10), CI-based ties, and full versioning — as specified in
   `docs/domain/03-metric-methodology.md`.
2. Percentile-rank normalization across all history — more stable but less
   interpretable ("what is my percentile of?") and harder to explain per period.
3. Elo-style pairwise rating — interesting but opaque for a transparency-first
   product and heavy to validate.
4. Publish only per-metric tables, no composite — maximum honesty, but abdicates the
   core product question ("which provider is best?").

## Decision
Option 1, frozen as methodology_version `2026.1`, weights `w-2026.1`
(temp 0.30 / precip-F1 0.25 / rain-MAE 0.15 / wind 0.15 / |bias| 0.05 / coverage 0.05 /
reliability 0.05). Full normative specification lives in the methodology document; the
worked example (3 providers, Johor Bahru +24h) is reproduced as an integration test.

## Rationale
- Ratio-to-best is directly explainable in one sentence per component ("score relative
  to the best provider observed for this location, horizon, and period") — matching
  the transparency principle and the amendment's "always expose methodology" mandate.
- Thresholds + statuses + coverage penalty + CI ties collectively prevent every
  misleading-ranking failure mode the ARB listed (small samples, coverage gaming,
  false precision).
- Versioning makes every published number reproducible and lets the methodology
  evolve without destroying history.

## Consequences
- (+) Rankings are defensible, inspectable, and testable (11 property invariants +
  vectors).
- (+) Custom weights via API without forking stored data (weights echoed + versioned).
- (−) Cohort-relative normalization can inflate scores in weak cohorts — mitigated by
  thresholds, penalty, and always-visible raw metrics; documented limitation.
- (−) Methodology changes require communication (version display, changelog on the
  methodology page).

## Migration trigger
Methodology revisions (new version, not replacement of this ADR) when: real data
reveals weight misalignment (user-trust studies), Brier Score graduates into ranking
(Level 3), or per-variable normalization replaces ratio-to-best (needs its own review).

## Review date
After first 90 days of production data (weight calibration review).
