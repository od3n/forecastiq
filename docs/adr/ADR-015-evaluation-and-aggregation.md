# ADR-015: Evaluation and Aggregation Strategy — 30-Minute Batch, Persisted Metrics

**Status**: Accepted (Phase 1)
**Date**: 2026-07-22

## Context
CE-09 requires batch every 30 min + on-demand. Phase 1 must decide where pair-level results live and how aggregation executes.

## Options considered
1. **Batch every 30 min: pair-level evaluation in memory per cell; only AccuracyMetric rows persisted; rolling recomputation of trailing periods.**
2. Persist pair-level evaluation rows (a table per pair per variable) — 5–10× match volume; zero MVP query surface for it.
3. Continuous/incremental aggregation (update metric rows as pairs arrive) — breaks immutability, complicates corrections, risks drift.
4. On-demand aggregation at query time — p95 < 200 ms impossible over 30 d × cells under cold cache; DB load per view.

## Decision
Option 1, per `docs/workflows/04-evaluation-and-ranking.md`. Day metrics for S-05 are the single query-time computation (≤ 48 pairs, in-memory, reusing the evaluation kernel).

## Rationale
- Persisted immutable metric rows make every screen read an indexed scan (NFR-P02 trivially met) and make staleness/freshness honest (row timestamps).
- Rolling recomputation absorbs late/corrected observations automatically — no incremental bookkeeping, and supersede links keep history reproducible.
- In-memory pair evaluation avoids a table nothing queries (reconciliation verdict: no EvaluationResult-per-variable entity).

## Consequences
- (+) One computation path (batch kernel reused by recompute + day metrics) — formula risk centralized and tested.
- (+) Correction handling = recompute scope, no special-case incremental logic.
- (−) Up to 30-min metric lag (freshness labels communicate; rankings threshold 2 h — comfortable).
- (−) Rolling recompute recomputes some stable periods (cost trivial at MVP: ≤ 700 cells × ≤ 2,160 pairs).

## Risks
Batch duration growth with locations/horizons — PT-4 gate at 100K pairs; chunked transactions bound memory.

## Migration trigger
Recompute cost > 50% of batch budget sustained → incremental invalidation scopes (still batch-executed); NATS promotion is separate (constraints §4).

## Review date
Quarterly with performance baselines.
