# ForecastIQ

ForecastIQ is a weather forecast **accuracy** comparison platform. It collects
forecasts from multiple providers, compares them against observations, and ranks
providers by measured accuracy. **ForecastIQ measures forecast accuracy — it
does not deliver weather forecasts.**

This repository contains the approved Phase 1 architecture (modular Go monolith
+ static Next.js dashboard + managed PostgreSQL) and the **first vertical
implementation slice**: repository foundation, database foundation, and a single
end-to-end forecast-collection proof (one location — Johor Bahru — one provider
— Open-Meteo).

> **Status:** Phase 1 architecture approved; first implementation work package
> (WP-01 + WP-02 vertical foundation slice) implemented. Later work packages are
> stubbed/deferred by design. See `docs/reviews/05-phase-1-architecture-report.md`.

## Quick start

```bash
make setup        # install Go tooling + create .env.local
make dev-up       # start PostgreSQL + app (auto-migrate + auto-seed, hot reload)

curl http://localhost:8080/healthz
curl http://localhost:8080/api/v1/locations
curl -X POST http://localhost:8080/api/v1/admin/collections/trigger \
  -H "Authorization: Bearer dev-admin-token" -H "Content-Type: application/json" \
  -d '{"provider_id":"00000000-0000-0000-0000-000000000010","location_id":"00000000-0000-0000-0000-000000000030"}'
```

The app seeds a system workspace, the Open-Meteo + OpenWeather providers, an
Open-Meteo configuration, and the Johor Bahru location. The worker collects
hourly; the trigger endpoint runs an immediate collection.

## What's implemented (first slice)

- **Repository foundation**: monorepo layout, Makefile, docker-compose, distroless
  Dockerfile, golangci-lint with depguard module-boundary rules, GitHub Actions CI.
- **Database foundation**: migrations for the slice tables (workspaces, providers,
  provider_configurations, provider_circuits, locations, forecast_collections,
  partitioned forecast_snapshots, collection_schedules, schedule_runs, audit_events),
  immutability triggers, monthly partitions, idempotent seed.
- **Idempotent forecast collection**: Open-Meteo adapter (real HTTP, retry,
  rate-limit awareness, schema validation, condition taxonomy) → raw payload
  (gzip + SHA-256) → normalized snapshots → single bounded transaction →
  collection-level + snapshot-level deduplication → structured logs + metrics + events.
- **Worker**: in-process scheduler with `FOR UPDATE SKIP LOCKED` slot claims,
  leases, retry, and run history.
- **API**: `/healthz`, `/readyz`, `/metrics`, `/api/v1/locations`,
  `/api/v1/locations/{id}` (GET/PUT), `/api/v1/locations/{id}/status` (PATCH),
  `/api/v1/providers`, `/api/v1/forecasts/latest`,
  `/api/v1/admin/collections/trigger`, `/api/v1/forecast-collections`,
  `/api/v1/openapi.json` — RFC 7807 errors, standard envelope, dev-token auth seam.

## What's explicitly deferred

Provider comparison, rankings, accuracy metrics, observation collection, matching,
analytics, the dashboard, full Supabase auth, multi-provider support, and the
remaining 25 work packages. Each has a documented promotion path.

## Documentation

- `docs/development/` — local dev, architecture overview, repository guide,
  environment setup, testing, migrations, contributing.
- `docs/architecture/`, `docs/domain/`, `docs/data/`, `docs/api/`, `docs/workflows/`
  — the authoritative Phase 1 design.
- `docs/adr/` — architecture decision records (ADR-001..032).

## License

Private — all rights reserved.
