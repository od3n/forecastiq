# ADR-003: Observation Source Strategy — Open-Meteo Historical, Provenance-Typed

**Status**: Accepted (Phase 0 Amendment)
**Date**: 2026-07-22

## Context
Phase 0 named NOAA/NWS primary with Open-Meteo fallback, but the launch location is
Johor Bahru, Malaysia — outside NOAA coverage. The ARB escalated observation-quality
risk (R-03). METMalaysia has no confirmed public API. The amendment required
distinguishing station, interpolated, reanalysis, and provider-estimated sources.

## Options considered
1. NOAA primary + Open-Meteo fallback (Phase 0) — unusable at the launch location.
2. **Open-Meteo Historical primary**, with mandatory provenance typing and a
   pre-launch quality spike — global coverage including JB.
3. Provider "current weather" endpoints as truth — circular (judging providers using
   providers' own estimates).
4. Wait for METMalaysia API — unbounded delay, no commitment exists.

## Decision
Option 2. All observations carry `observation_type` and `quality_flag`; types drive
quality weighting (methodology §6.4); provenance is always displayed. NOAA/NWS remains
the documented US-expansion adapter (Level 3). Provider current-weather is allowed only
as `provider_estimated` (weight 0.5), never primary. A quality spike against available
METMalaysia station bulletins/literature gates ranking publication (D-06).

## Rationale
- The product cannot launch without a JB-capable truth source; option 2 is the only
  legally usable, global, machine-accessible choice today.
- Provenance typing + weighting + display converts the weakness into disclosed,
  managed uncertainty (product contract PC-04) instead of hidden error.
- The spike makes the assumption testable before users see rankings.

## Consequences
- (+) Launch unblocked; global expansion works with the same source.
- (+) Honest uncertainty model differentiates the product.
- (−) Tropical reanalysis has real error; rankings carry that noise (weighted, disclosed).
- (−) "Ground truth" marketing claims must be avoided (copy rules).

## Migration trigger
Add station source when: METMalaysia publishes an API, or US launch adds NOAA, or the
spike shows reanalysis error > acceptable bounds for rain occurrence (then: priority
mix change + heavier weighting of station data).

## Review date
At spike completion (pre-launch) and at Level 3 gate.
