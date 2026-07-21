# ADR-002: Provider Scope — Open-Meteo + OpenWeather (ToS-Gated)

**Status**: Accepted (Phase 0 Amendment)
**Date**: 2026-07-22

## Context
Phase 0 committed to 4 providers (OpenWeather, Tomorrow.io, Visual Crossing,
Open-Meteo) at MVP. The ARB recommended cutting to 2. The amendment mandate required
evaluating developer access, free-tier limits, attribution, storage rights, Malaysian
coverage, and redistribution restrictions — and explicitly forbade assuming OpenWeather.

## Options considered
1. 4 providers (Phase 0 baseline) — 4× adapter, ToS, and rate-limit surface.
2. **Open-Meteo + OpenWeather** — open/permissive primary + high-recognition second.
3. Open-Meteo + Tomorrow.io — alternative second with clean hourly timelines.
4. Open-Meteo only — minimum possible.

## Decision
Option 2, **conditioned on ToS validation** (dependency D-05). Full evaluation table in
`docs/product/03-mvp-scope.md` §2.1. Documented fallback: swap OpenWeather →
Tomorrow.io (option 3) if the ToS review fails; the adapter interface bounds the change.
Open-Meteo alone (option 4) rejected: single-provider rankings are not a product.

## Rationale
- One call per provider per hour yields all horizons (array responses) — 2 providers
  fit free tiers with 4× headroom at 5–10 locations.
- Open-Meteo removes key/licensing friction; OpenWeather adds model-lineage diversity
  (distinct NWP) needed for meaningful comparison.
- ToS gate converts the ARB's legal concern (Risk R-02) into a launch-blocking task
  rather than an assumption.

## Consequences
- (+) ~50% less adapter work; 2 ToS reviews instead of 4.
- (+) Swap path keeps the decision reversible.
- (−) Fewer providers may show smaller accuracy spread (mitigation: A-03 pilot check).
- (−) Tomorrow.io/Visual Crossing users must wait for Level 3.

## Migration trigger
Add provider 3 when: MVP validated (Level 3 gate) OR OpenWeather ToS fails (immediate
swap) OR users request a specific provider with evidence (≥ 20 requests).

## Review date
At Level 3 gate; ToS gate reviewed before public launch.
