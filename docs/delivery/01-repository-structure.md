# ForecastIQ — Repository Structure (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: ADR-001 (modular monolith); `docs/architecture/03-module-architecture.md`; prompt §30 direction (adapted to idiomatic Go)

**Decision: monorepo** (single repository for backend, dashboard, migrations, IaC, docs). Rationale: one team, one deployable backend + one static frontend, atomic cross-cutting changes (API contract + dashboard together), simplest CI. Multi-repo promotion trigger: none anticipated before Level 3.

---

## 1. Layout

```text
forecastiq/
├── cmd/
│   └── forecastiq/            # main package: flags (serve|worker|migrate|seed), wiring
├── internal/                  # private application code (Go visibility)
│   ├── platform/              # cross-cutting: config, logging, httpserver, middleware,
│   │                          #   lru cache, ratelimit, eventbus, health, metrics registry
│   ├── identity/              # module: domain/, application/, persistence/, (no http here)
│   ├── catalog/               # module: workspaces, providers, configurations, locations, circuits
│   ├── collection/            # module: forecast + observation collection, adapters port
│   ├── analysis/              # module: matching, metrics, ranking, methodology registry
│   ├── scheduler/             # module: slots, claims, dispatch, run history
│   ├── operations/            # module: health assembly
│   ├── audit/                 # module: recorder + reader
│   └── api/                   # HTTP layer: handlers, envelopes, validation, openapi gen
│       ├── handlers/
│       └── dto/               # request/response structs (serializer-level exclusions)
├── adapters/                  # infrastructure adapters (importable by wiring only)
│   ├── forecastproviders/
│   │   ├── openmeteo/         # adapter + schema version + condition map
│   │   └── openweather/
│   ├── observationsources/
│   │   └── openmeteo/
│   ├── persistence/           # pgx repositories per module (packages: identitypg, catalogpg, ...)
│   ├── authn/                 # JWKS verifier, supabase admin client
│   ├── payloadstore/          # filesystem impl (scheme-prefixed keys)
│   └── notify/                # (reserved: webhooks L3)
├── migrations/                # NNNN_*.up.sql / .down.sql (golang-migrate)
├── api/
│   └── openapi/               # committed generated spec (CI drift-checked)
├── web/                       # Next.js dashboard (own package.json)
│   ├── app/                   # routes per approved IA (S-01..S-15)
│   ├── components/
│   ├── lib/                   # api client (generated from OpenAPI), auth, csv export
│   └── test/
├── deploy/
│   ├── bootstrap.sh           # VPS provisioning (idempotent)
│   ├── caddy/Caddyfile
│   ├── systemd/forecastiq.service
│   ├── grafana/               # dashboards JSON + alerts yaml
│   └── scripts/               # backup.sh, restore-test.sh, capture-fixture.sh
├── terraform/                 # cloudflare DNS + neon project (state: remote)
├── docs/                      # this documentation set (authoritative)
├── scripts/                   # dev helpers (dev-up.sh, seed-local.sh)
├── test/
│   ├── fixtures/              # provider response fixtures
│   ├── e2e/                   # golden-path scenario
│   └── perf/                  # k6 + seeder
├── .github/workflows/         # CI/CD
├── docker-compose.yml         # local dev (app + postgres + volume)
├── Dockerfile                 # distroless production image
├── go.mod / go.sum
└── Makefile                   # build, test, lint, migrate, gen targets
```

## 2. Package Rules (binding)

| Rule | Enforcement |
|------|-------------|
| `internal/` modules never import another module's `persistence` package | golangci-lint `depguard` config (CI gate) |
| Cross-module calls via module service interfaces (application layer) | Review + depguard |
| Domain packages (module `domain/`) import only stdlib + shared domain kernel | depguard |
| `adapters/` imports module ports, never handlers | depguard |
| `cmd/` is the only composition root (wires adapters to ports) | Convention + review |
| No `pkg/` public API surface at MVP (everything internal; public SDK is Level 3) | Layout |
| Handlers contain no business logic (call use cases; assemble envelopes) | Review + handler test pattern |

## 3. Generated Files

| Artifact | Generator | Location | CI check |
|----------|-----------|----------|----------|
| OpenAPI spec | oapi-codegen/swag from handler annotations | `api/openapi/openapi.json` | Generated == committed (drift gate) |
| API client (dashboard) | openapi-typescript + fetch wrapper | `web/lib/api/generated.ts` | Regenerated in CI; committed |
| DB models (if codegen adopted) | sqlc from queries | `adapters/persistence/*/models.go` | Generated == committed |

## 4. Test Locations

| Kind | Location |
|------|----------|
| Unit | `*_test.go` alongside code (Go convention) |
| Module integration | `internal/<module>/integration_test.go` (build-tagged `integration`) |
| API integration | `internal/api/*_integration_test.go` |
| E2E | `test/e2e/` |
| Performance | `test/perf/` |
| Fixtures | `test/fixtures/` |
| Frontend | `web/**/__tests__/` + `web/e2e/` (Playwright) |

## 5. Migration Ownership

- `migrations/` owned collectively; every PR touching schema lists migration numbers in description.
- Numbering: timestamp-prefix sequence (`20260801120000_create_catalog.up.sql`) to avoid merge collisions.
- Seed data (system workspace, providers) in migration `000001_seed` (idempotent ON CONFLICT).

## 6. Documentation Ownership

- `docs/` is the authoritative product/architecture record (this package); changes via PR with the same review rigor as code.
- ADRs immutable once Accepted (supersede via new ADR).
- Code-level READMEs: minimal (point to docs/); no parallel documentation that can drift.

## 7. Cross-Reference

- Module specs: `docs/architecture/03-module-architecture.md`
- CI/CD: `docs/delivery/02-ci-cd.md`
- Implementation sequence: `docs/delivery/05-implementation-sequence.md`
