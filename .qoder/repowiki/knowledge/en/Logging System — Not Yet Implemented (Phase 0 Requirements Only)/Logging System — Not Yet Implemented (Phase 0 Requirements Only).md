---
kind: logging_system
name: Logging System — Not Yet Implemented (Phase 0 Requirements Only)
category: logging_system
scope:
    - '**'
source_files:
    - docs/phase-0-business-analysis/05-non-functional-requirements.md
    - docs/phase-0-business-analysis/03-software-requirements-spec.md
---

This repository contains only Phase 0 business analysis documentation for ForecastIQ and does not include any implementation code. There is no logging system present in the codebase.

The requirements documents specify a structured JSON logging strategy as part of the observability architecture:
- **NFR-OBS01**: Structured JSON logging via Loki + Promtail across all services
- **NFR-OBS06**: Log correlation using Trace ID propagated from OpenTelemetry context
- **NFR-SEC11**: Audit logging for all authentication and admin actions, with structured audit log format
- **AUTH-06**: All authentication events must be logged as an audit trail
- **NFR-D04**: Audit log retention policy of 1 year

These are design requirements documented in `docs/phase-0-business-analysis/05-non-functional-requirements.md` and `03-software-requirements-spec.md`, but no corresponding logging framework initialization, configuration, or application code exists in this repository snapshot.