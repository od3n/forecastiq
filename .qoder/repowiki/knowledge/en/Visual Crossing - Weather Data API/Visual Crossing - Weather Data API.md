---
kind: external_dependency
name: Visual Crossing - Weather Data API
slug: visual-crossing
category: external_dependency
category_hints:
    - vendor_identity
scope:
    - '**'
---

External weather data API provider integrated via adapter pattern. One of four initial forecast sources requiring API key authentication. Must handle HTTP 500 errors with exponential backoff (1s, 2s, 4s, 8s, 16s) and support provider enable/disable functionality through admin portal.