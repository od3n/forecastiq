---
kind: external_dependency
name: Tomorrow.io - Weather Intelligence Platform
slug: tomorrow-io
category: external_dependency
category_hints:
    - vendor_identity
scope:
    - '**'
---

External weather intelligence API provider integrated via adapter pattern. One of four initial forecast sources requiring API key authentication. System must implement circuit breaker protection (opens after 5 consecutive failures, half-open after 60s) and exponential backoff retry logic for handling provider outages.