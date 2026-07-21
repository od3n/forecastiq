---
kind: external_dependency
name: NOAA/NWS - US Weather Observations
slug: noaa-nws
category: external_dependency
category_hints:
    - vendor_identity
scope:
    - '**'
---

Primary US weather observation source providing hourly station data. Used as ground-truth data for forecast accuracy comparison. Coverage gaps expected for non-US locations, requiring Open-Meteo as global fallback. Data availability and quality assumptions require validation before Phase 6.