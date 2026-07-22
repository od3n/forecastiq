# Repository Guide

Monorepo layout (per `docs/delivery/01-repository-structure.md`, adapted to
idiomatic Go). The dependency direction is enforced by golangci-lint `depguard`.

```text
forecastiq/
├── cmd/forecastiq/          # composition root: serve|worker|migrate|seed, --mode flag
├── internal/                # private application code
│   ├── platform/            # cross-cutting (config, logging, db, dbtx, events,
│   │                        #   metrics, health, ratelimit, clock, ids, buildinfo)
│   ├── catalog/             # module: domain/ + ports/ + application services
│   ├── collection/          # module: domain/ + ports/ + collect/reader use cases
│   ├── scheduler/           # module: slots, claims, dispatch, run history
│   ├── audit/               # module: append-only recorder
│   └── api/                 # HTTP layer: router, middleware, handlers/, respond/
├── adapters/                # infrastructure adapters (wired only in cmd/)
│   ├── forecastproviders/openmeteo/   # Open-Meteo adapter + WMO condition map
│   ├── persistence/         # catalogpg, collectionpg, schedulerpg, auditpg (pgx)
│   └── payloadstore/        # filesystem gzip payload store (file:// scheme)
├── migrations/              # NNNN_*.up.sql / .down.sql (embedded; golang-migrate)
├── api/openapi/             # committed OpenAPI 3.1 spec (served at /api/v1/openapi.json)
├── test/
│   ├── fixtures/openmeteo/  # recorded provider responses (contract tests)
│   └── integration/         # testcontainers DB + API integration tests
├── scripts/                 # dev helpers (dev-up.sh, seed-local.sh)
├── deploy/                  # bootstrap.sh, caddy/, systemd/, grafana/, scripts/
├── docs/                    # authoritative product + architecture documentation
├── .github/workflows/       # CI/CD
├── docker-compose.yml       # local dev (postgres + app + volumes)
├── Dockerfile               # multi-stage: dev (air) + prod (distroless)
├── Makefile                 # build/test/lint/migrate/seed/docs targets
└── go.mod                   # module github.com/forecastiq/forecastiq
```

## Package rules (binding, depguard-enforced)

| Rule | Effect |
|------|--------|
| `internal/**` may not import `adapters/**` | only `cmd/` wires adapters to ports |
| `internal/**/domain/**` may not import infra (gin, pgx, prometheus, net/http, …) | domain stays pure |
| `adapters/**` may not import `internal/api` | adapters depend on ports, never handlers |

## Module internal structure

Each business module follows `domain/ → ports/ → application services`:

- `domain/` — entities, value objects, invariants, domain errors (stdlib + uuid only).
- `ports/` — repository + adapter interfaces the module needs.
- application services (module root) — use cases; one bounded transaction per command.

`catalog` exposes `LocationManager`, `ProviderCatalog`, `ConfigurationManager`,
`CircuitState`; `collection` exposes `ForecastCollector`, `ForecastReader`.
Other modules consume these interfaces (never another module's tables).

## Where things live

| Concern | Location |
|---------|----------|
| Add a provider adapter | `adapters/forecastproviders/<slug>/` implementing `collection/ports.ForecastProviderAdapter` |
| Add a persistence repo | `adapters/persistence/<module>pg/` implementing the module's repository port |
| Add an endpoint | `internal/api/handlers/` + route in `internal/api/router.go` + OpenAPI |
| Add a migration | `migrations/<timestamp>_name.{up,down}.sql` |
| Add a metric | `internal/platform/metrics/` |
