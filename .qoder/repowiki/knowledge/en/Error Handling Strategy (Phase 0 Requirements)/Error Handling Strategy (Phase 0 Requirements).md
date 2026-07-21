---
kind: error_handling
name: Error Handling Strategy (Phase 0 Requirements)
category: error_handling
scope:
    - '**'
source_files:
    - docs/phase-0-business-analysis/04-functional-requirements.md
    - docs/phase-0-business-analysis/05-non-functional-requirements.md
    - docs/phase-0-business-analysis/06-domain-model.md
---

This repository contains only Phase 0 business analysis documentation — no implementation code exists yet. The error handling strategy is therefore defined purely as requirements and design specifications across the functional and non-functional requirement documents.

**API Error Format (RFC 7807)**
The REST API must return RFC 7807 Problem Details for all errors, with a consistent JSON envelope containing `type`, `title`, `status`, `detail`, `instance`, and an optional `errors` array for field-level validation failures. Example type URIs follow the pattern `https://forecastiq.com/errors/<category>`.

**Provider Resilience Patterns**
External provider calls are governed by explicit retry/backoff rules: exponential backoff starting at 1s (1, 2, 4, 8, 16s) for timeouts; respecting `Retry-After` headers on 429 responses; up to 5 retries for 5xx errors before alerting; and a circuit breaker that opens after 5 consecutive failures and transitions to half-open after 60s. Invalid response schemas are logged and the raw payload is persisted to S3 for later inspection while skipping processing.

**Observability Integration**
Errors are surfaced through structured JSON logging (Loki + Promtail), Prometheus metrics using the RED method (Rate, Errors, Duration), distributed tracing via OpenTelemetry/Jaeger with trace IDs correlated into logs, and Grafana Alerting rules tuned to error rate thresholds.

**Domain-Level Error Semantics**
The domain model documents service methods returning `error` explicitly (e.g., `Save(snapshot) → error`, `Update(provider) → error`), indicating Go-style error returns rather than panics. Suspect observations outside valid ranges are stored with a `suspect` flag rather than rejected outright.

No source code implementing these patterns exists in this branch — this is a requirements-only snapshot.