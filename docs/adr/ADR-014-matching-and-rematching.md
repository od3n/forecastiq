# ADR-014: Matching and Rematching Strategy

**Status**: Accepted (Phase 1)
**Date**: 2026-07-22

## Context
BR-MATCH-01..06 and methodology §3.1 define matching rules; Phase 1 must fix the algorithm shape, storage, and rematching mechanics.

## Options considered
1. **Deterministic batch matching over unmatched snapshots with total-order candidate selection; pairs immutable; rematch = new pair rows.**
2. Windowed re-evaluation (recompute all matches every batch) — wasteful and destabilizing (pair churn breaks metric reproducibility).
3. Eager matching at observation arrival — couples collection and analysis transactions; complicates late arrivals from both sides.

## Decision
Option 1, per `docs/workflows/03-matching.md`: exact-hour UTC; candidate order = corrected preference → provenance rank → |Δ to top-of-hour| → id; uniqueness `(forecast_snapshot_id, observation_id)`; corrections add pairs (never edit).

## Rationale
- Total order removes all ambiguity → same inputs always produce same matches (property-testable, reproducible — methodology principle).
- Append-only rematch preserves lineage: the original pair remains evidence of what was known then (BR-INV-03).
- Batch decoupling keeps collection transactions short and failure-isolated.

## Consequences
- (+) Idempotent by constraint; permutation-invariant by construction.
- (+) Correction cascade is mechanical (event → rematch scope → recompute).
- (−) Matches lag observations by up to one batch (30 min) — acceptable (metrics are batch products anyway).

## Risks
R-18 (wrong pairing) — mitigated by stored match_rule/time_delta + lineage + determinism tests.

## Migration trigger
Sub-hourly observation sources (NOAA at Level 3) activate the ±15 min rule behind the existing source-capability flag — no redesign.

## Review date
At NOAA adapter introduction (Level 3) or matching-rule methodology change.
