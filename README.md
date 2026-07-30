# ForecastIQ

ForecastIQ is a weather forecast **accuracy** comparison platform. It collects
forecasts from multiple providers, compares them against observations, and ranks
providers by measured accuracy. **ForecastIQ measures forecast accuracy — it
does not deliver weather forecasts.**

This repository contains the Phase 1 architecture (modular Go monolith + static
Next.js dashboard + PostgreSQL) implemented end to end: multi-provider forecast
collection, observation ingestion, forecast-vs-actual analysis, accuracy metrics
and provider rankings, a public + admin dashboard, authentication/identity with
GDPR self-service, and an image-based production deployment.

> **Status:** Work packages through WP-26 delivered (WP-26 accepted at scaffold
> scope; the remainder is tracked as WP-26b in
> `docs/planning/05-implementation-work-packages.md`). The platform is deployed
> to production per ADR-033 (image-based Docker deploy).
> See `docs/reviews/` for the architecture and DRB acceptance reports.

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

The app seeds a system workspace, the Open-Meteo + OpenWeather providers,
provider configurations, and the Johor Bahru location. The worker collects
forecasts and observations on schedule; the trigger endpoint runs an immediate
collection. The `dev-admin-token` example works only with the dev auth adapter
(local builds); production uses JWT (JWKS) auth via Supabase.

## What's implemented

- **Forecast collection**: Open-Meteo and OpenWeather adapters (real HTTP,
  retry, rate-limit/call-budget awareness, schema validation, condition
  taxonomy) → raw payload (gzip + SHA-256) → normalized snapshots → single
  bounded transaction → collection- and snapshot-level deduplication →
  structured logs + metrics + audit events. Admin replay of stored payloads.
- **Observation ingestion**: Open-Meteo observation source feeding the
  ground-truth side of the comparison.
- **Analysis engine**: forecast↔observation matching, per-variable accuracy
  evaluation, aggregation into accuracy metrics, and provider rankings with a
  documented methodology (`/rankings/methodology`). Admin-triggered recompute.
- **Worker**: in-process scheduler with `FOR UPDATE SKIP LOCKED` slot claims,
  leases, retry, and run history — dispatching forecast, observation, and
  analysis jobs.
- **Auth & identity**: JWKS JWT verification (Supabase in production, dev-token
  adapter locally), roles + scopes, API keys, auth-provider webhook, GDPR
  self-service (`/me` view/update/delete, data export) and admin user
  management.
- **Database**: 13 migration pairs — catalog, collection (partitioned
  `forecast_snapshots`, immutability triggers), scheduler, audit (GDPR-aware),
  identity, observations, analysis, accuracy metrics, provider rankings,
  export jobs. Idempotent seed.
- **Dashboard** (`web/`): Next.js 15 + React 19 static export. Public pages:
  overview, trends, forecast vs actual, methodology, locations, providers,
  settings. Admin area: dashboard, forecasts, health, locations, providers,
  schedules, users. Deployed manually to Cloudflare Pages via wrangler.
- **Observability**: Prometheus metrics (separate localhost-bound `/metrics`
  server), engine + backup Prometheus collectors, and a local Grafana +
  Prometheus + Loki stack (`make obs-up`, Grafana at `http://localhost:3000`).
- **Deployment**: distroless image pushed to GHCR (`make deploy-release`,
  ADR-033), docker-compose production topology (`deploy/compose/`,
  `deploy/cotenant/`), smoke tests (`make deploy-smoke`).

## API surface

All under `/api/v1` (RFC 7807 errors, standard envelope, rate-limited);
spec served at `/api/v1/openapi.json`.

- **Operational**: `/healthz`, `/readyz` (unversioned), localhost `/metrics`.
- **Public catalog** (cached 300 s): `GET /locations`, `/locations/{id}`,
  `/providers`, `/providers/{id}`.
- **Public analysis** (cached 60 s): `GET /rankings`, `/rankings/methodology`,
  `/accuracy`, `/accuracy/summary`, `/forecast-comparison`.
- **Authenticated data** (`read:data` scope): `GET /forecasts/latest`.
- **Self-service** (any authenticated user): `GET|PATCH|DELETE /me`,
  `POST /me/export`, `GET /exports/{id}`, `GET|POST /api-keys`,
  `DELETE /api-keys/{id}`.
- **Admin** (role `admin`): location CRUD/status, `POST
  /admin/collections/trigger`, `POST /admin/collections/{id}/replay`,
  `GET /forecast-collections[/{id}[/snapshots]]`, `GET /admin/health`,
  `PATCH /admin/providers/{id}/status`, provider-configuration list/update,
  `GET /admin/audit-events`, `POST /admin/recompute`, user management
  (`GET /admin/users`, status/role updates, delete, export).
- **Webhook**: `POST /auth/webhook` (HMAC-gated, mounted when configured).

## What remains (WP-26b)

Performance & reliability completion: PT-3/PT-4/PT-7/PT-8 scenarios,
fault-injection reliability suite, functional perf seeder DB writes, the
baseline register, and wiring the weekly k6 + reliability job into scheduled
CI. See `docs/planning/05-implementation-work-packages.md`.

## Documentation

- `docs/development/` — local dev, architecture overview, repository guide,
  environment setup, testing, migrations, contributing.
- `docs/architecture/`, `docs/domain/`, `docs/data/`, `docs/api/`,
  `docs/workflows/` — the authoritative Phase 1 design.
- `docs/operations/`, `docs/security/`, `docs/testing/`, `docs/delivery/` —
  runbooks, security architecture, test strategy, deployment.
- `docs/adr/` — architecture decision records (ADR-001..033).

## License

Private — all rights reserved.
