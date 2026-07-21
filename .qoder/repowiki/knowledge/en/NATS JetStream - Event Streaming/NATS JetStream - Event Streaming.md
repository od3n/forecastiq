---
kind: external_dependency
name: NATS JetStream - Event Streaming
slug: nats-jetstream
category: external_dependency
category_hints:
    - vendor_identity
scope:
    - '**'
---

Lightweight messaging system using JetStream for persistent, exactly-once capable event streaming. Connects collectors to comparison engine via events like 'forecast.collected', 'observation.collected', and 'accuracy.calculated'. Enables decoupled service communication within the microservices architecture.