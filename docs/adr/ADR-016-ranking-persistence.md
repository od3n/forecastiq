# ADR-016: Ranking Persistence Strategy — Stored Immutable Projections

**Status**: Accepted (Phase 1)
**Date**: 2026-07-22

## Context
Rankings are the product's headline output. Phase 1 must decide: computed dynamically per request, materialized views, or stored rows.

## Options considered
1. Dynamic computation per request — cohort normalization needs all providers' metrics per request; latency and consistency both suffer; custom-weights requests would recompute everything.
2. Materialized views (PostgreSQL) — refresh-state complexity; no supersede history; refresh locks; versioning awkward.
3. **Stored immutable ProviderRanking rows written by the batch (and on-demand recompute); latest-per-cell serves the API; supersede links retain history.**

## Decision
Option 3, per `docs/workflows/04-evaluation-and-ranking.md` §4 and `docs/data/03-table-design.md`.

## Rationale
- Read path is a bounded indexed scan (≤ 10 providers per cell) — p50 < 50 ms guaranteed regardless of input volume.
- Immutability + versioning (methodology_version, weights_version) makes every published ranking reproducible and attributable (PC-02, BR-RANK-07).
- Batch-stable publication means a provider outage never reshuffles ranks mid-batch — statistical stability the partial-result contract relies on.
- Custom-weights requests compute over stored metrics without touching stored rankings.

## Consequences
- (+) Ranking history is an audit asset (what was published, when, under which methodology).
- (+) Ties/statuses computed once, consistent everywhere.
- (−) Storage grows with recompute frequency — managed by supersede-purge policy (data doc 06 §4: 2 y full history, monthly snapshots indefinite).
- (−) Freshness bounded by batch cadence (honest labeling per BR-FRESH).

## Risks
Superseded-row accumulation (R-20) — purge policy + monthly volume review.

## Migration trigger
None anticipated for MVP shape; per-workspace ranking customization (Level 3) adds rows per workspace, same mechanics.

## Review date
With methodology review (90 days post-launch, ADR-010 schedule).
