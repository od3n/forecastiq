# ADR-025: Observation Collection Model — Direct Rows, Polled with Backfill Window

**Status**: Accepted (Phase 1) — implements ADR-003
**Date**: 2026-07-22

## Context
The prompt requires deciding whether observations need a parent collection entity like ForecastCollection, and the collection mechanics (polled/batched/on-demand/backfilled).

## Options considered
1. ObservationCollection parent entity mirroring forecasts — symmetry for its own sake; reconciliation board evaluated and rejected (doc 04 §1): no replay need (source re-queryable), no payload storage, health derivable from observation aggregates.
2. **Direct observation rows, polled hourly at :05 with a 2-hour backfill window per call; dedup by (source, location, observed_at); corrections as new rows + supersession pointer.**
3. On-demand observation fetch at match time — couples analysis latency to source availability; no caching benefit; rejected.
4. Batch import from downloaded datasets — ERA5-direct style; post-MVP enhancement path (ADR-003), not MVP mechanics.

## Decision
Option 2, per `docs/workflows/02-observation-collection.md`.

## Rationale
- The 2 h window makes late publication self-healing: every call re-covers the recent hours; uniqueness dedups the overlap — backfill is the normal path, not a special mode.
- Correction detection falls out of the same mechanism (value diff on re-fetch beyond ε → corrected row).
- One fewer entity = one fewer lifecycle, one fewer failure mode, smaller schema — the simplicity the amendment mandates.

## Consequences
- (+) Collector health = MAX(observed_at) per location (no run-table needed for MVP; schedule_runs still records job executions).
- (+) Source outage recovery is automatic within the window; beyond it, honest gaps.
- (−) No per-call payload audit for observations — acceptable: source is the truth reference and re-queryable; provenance fields on rows carry the needed lineage.
- (−) Correction detection limited to the 2 h window — older silent corrections undetectable (documented; methodology recomputation handles detected ones).

## Risks
R-17 (source quality) — unchanged by mechanics; addressed by D-06 spike + weighting.

## Migration trigger
NOAA/NWS addition (Level 3): same row model, new adapter; if a source requires bulk historical import, a one-off import job writes rows with proper provenance (no schema change).

## Review date
At NOAA adapter introduction.
