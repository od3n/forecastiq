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
