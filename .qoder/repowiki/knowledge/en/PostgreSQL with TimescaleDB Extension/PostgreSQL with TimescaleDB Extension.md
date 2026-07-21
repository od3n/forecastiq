---
kind: external_dependency
name: PostgreSQL with TimescaleDB Extension
slug: postgresql-timescaledb
category: external_dependency
category_hints:
    - vendor_identity
scope:
    - '**'
---

Primary time-series database combining PostgreSQL relational capabilities with TimescaleDB's time-series optimizations. Chosen for purpose-built time-series workload handling, partitioning support, and retention policies. Assumption that it handles projected write throughput at target scale requires load testing validation.