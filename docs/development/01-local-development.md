# Local Development Guide

One-command startup:

```bash
make setup     # install pinned Go tooling (golangci-lint, swag, goimports) + create .env.local
make dev-up    # docker compose up: PostgreSQL 16 + app (auto-migrate + auto-seed, air hot reload)
```

The app listens on `http://localhost:8080` (API) and `http://localhost:9090/metrics`.

## Prerequisites

- Go 1.23+ (the module declares `go 1.23.4`; older toolchains auto-download it via `GOTOOLCHAIN`)
- Docker + Docker Compose
- `make`

## Everyday commands

| Command | Purpose |
|---------|---------|
| `make dev-up` | Start the stack (build + migrate + seed + hot reload) |
| `make dev-down` | Stop the stack (keeps data volumes) |
| `make dev-reset` | Destroy volumes + restart clean |
| `make dev-logs` | Tail app logs |
| `make migrate` | Apply pending migrations |
| `make seed` | Seed reference data (idempotent) |
| `make test` | Unit tests (race detector) |
| `make test-integration` | Integration tests (testcontainers; needs Docker) |
| `make lint` | golangci-lint (incl. depguard module boundaries) |
| `make fmt` | gofmt + goimports |
| `make build` | Compile the single binary into `bin/` |
| `make docs` | Regenerate the OpenAPI spec |
| `make obs-up` | Start the local observability stack (Grafana :3000, Prometheus :9091, Loki, Promtail) |
| `make obs-down` | Stop the observability stack |
| `make obs-reset` | Destroy observability volumes and restart clean |

## Frontend (dashboard)

The compose stack includes a `frontend` service (Next.js dev server with hot
reload, host port **3001**). It reads `web/.env.local`:

```bash
NEXT_PUBLIC_API_BASE_URL=http://localhost:8080/api/v1
# Optional — set both to sign in through a real Supabase project:
NEXT_PUBLIC_SUPABASE_URL=
NEXT_PUBLIC_SUPABASE_ANON_KEY=
```

Sign-in always goes through `/auth/signin`; no bearer token is injected from
the environment. Without Supabase config the page shows a **dev-token form**
riding the ADR-008 dev-mode verifier: enter `admin` (the token is verified
against `GET /me` before being stored client-side). `make seed` promotes the
`dev|admin` subject to role=admin, so the Admin screens work locally without a
Supabase project. With Supabase configured, the email/password form is shown
instead and the SDK session token is attached to API calls automatically.

## Local observability (obs profile)

`make obs-up` starts Prometheus + Loki + Promtail + Grafana under the compose
`obs` profile (config in `deploy/observability/`). Grafana at
http://localhost:3000 (anonymous admin in dev) is pre-provisioned with both
datasources and the "ForecastIQ Operations" dashboard. This is the local
counterpart of the production grafana-agent → Grafana Cloud pipeline
(operations doc 03).

## Running without Docker

Point `FIQ_DATABASE_URL` at any PostgreSQL 16, then:

```bash
go run ./cmd/forecastiq migrate up
go run ./cmd/forecastiq seed
go run ./cmd/forecastiq serve --mode=all   # api | worker | all
```

## Trying the slice

```bash
# Liveness / readiness
curl localhost:8080/healthz
curl localhost:8080/readyz

# Seeded locations + providers
curl localhost:8080/api/v1/locations
curl localhost:8080/api/v1/providers

# Trigger a collection (dev admin token from .env.local)
curl -X POST localhost:8080/api/v1/admin/collections/trigger \
  -H "Authorization: Bearer dev-admin-token" -H "Content-Type: application/json" \
  -d '{"provider_id":"00000000-0000-0000-0000-000000000010","location_id":"00000000-0000-0000-0000-000000000030"}'

# Latest forecast + collection lineage
curl "localhost:8080/api/v1/forecasts/latest?provider_id=00000000-0000-0000-0000-000000000010&location_id=00000000-0000-0000-0000-000000000030"
curl -H "Authorization: Bearer dev-admin-token" localhost:8080/api/v1/forecast-collections
```

The worker also collects automatically every hour while `--mode=all` (or `worker`) runs.

## Authentication (dev seam)

Admin endpoints require `Authorization: Bearer <FIQ_DEV_ADMIN_TOKEN>`. This is a
**development-only** seam; production auth (Supabase JWKS) lands in a later work
package (WP-03/19). `FIQ_DEV_ADMIN_TOKEN` must be unset in production (config
validation enforces this).
