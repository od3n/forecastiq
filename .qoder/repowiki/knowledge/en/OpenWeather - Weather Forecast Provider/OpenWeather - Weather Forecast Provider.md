---
kind: external_dependency
name: OpenWeather - Weather Forecast Provider
slug: openweather
category: external_dependency
category_hints:
    - vendor_identity
scope:
    - '**'
---

External weather forecast API provider integrated via adapter pattern. One of four initial providers (alongside Tomorrow.io, Visual Crossing, Open-Meteo) for collecting immutable forecast snapshots. Requires API key authentication with rate limiting compliance. The platform must respect provider rate limits and handle failures gracefully with circuit breaker patterns.