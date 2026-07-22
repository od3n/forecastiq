# Environment Setup

Configuration is 12-factor: everything arrives via `FIQ_`-prefixed environment
variables. The app validates configuration at startup and **fails fast** on any
invalid value (`internal/platform/config`). Copy `.env.example` to `.env.local`
for local development (`.env.local` is gitignored).

## Configuration reference

| Variable | Default | Description |
|----------|---------|-------------|
| `FIQ_ENV` | `development` | `development` \| `test` \| `production` |
| `FIQ_MODE` | `all` | `api` \| `worker` \| `all` (overridable via `serve --mode`) |
| `FIQ_HTTP_ADDR` | `0.0.0.0:8080` | API listen address |
| `FIQ_METRICS_ADDR` | `127.0.0.1:9090` | Prometheus metrics address (localhost-bound) |
| `FIQ_HTTP_READ_TIMEOUT` | `10s` | HTTP read timeout |
| `FIQ_HTTP_WRITE_TIMEOUT` | `30s` | HTTP write timeout |
| `FIQ_HTTP_SHUTDOWN_TIMEOUT` | `30s` | Graceful shutdown drain |
| `FIQ_LOG_LEVEL` | `info` | `debug` \| `info` \| `warn` \| `error` |
| `FIQ_LOG_FORMAT` | `json` | `json` \| `text` |
| `FIQ_DATABASE_URL` | *(required)* | PostgreSQL DSN (`postgres://…`) |
| `FIQ_DB_MAX_CONNS` | `20` | Pool max connections |
| `FIQ_DB_MIN_CONNS` | `2` | Pool min connections |
| `FIQ_DB_MAX_CONN_LIFETIME` | `1h` | Max connection lifetime |
| `FIQ_AUTO_MIGRATE` | `false` | Apply migrations on boot |
| `FIQ_AUTO_SEED` | `false` | Seed reference data on boot |
| `FIQ_PAYLOAD_STORE_DIR` | `./data/payloads` | Raw payload volume root |
| `FIQ_PROVIDER_TIMEOUT` | `10s` | Provider HTTP timeout |
| `FIQ_PROVIDER_MAX_RESPONSE_BYTES` | `10485760` | Provider response size cap (10 MB) |
| `FIQ_SCHEDULER_INTERVAL` | `15s` | Scheduler tick interval |
| `FIQ_SLOT_LEASE_DURATION` | `5m` | Slot claim lease |
| `FIQ_WORKER_MAX_CONCURRENT` | `8` | Worker goroutine pool size |
| `FIQ_DEV_ADMIN_TOKEN` | *(unset)* | Dev-only admin token (must be unset in production) |
| `FIQ_RATE_LIMIT_PER_IP_PER_MIN` | `120` | Per-IP API rate limit |
| `FIQ_CORS_ALLOW_ORIGINS` | `http://localhost:3000` | Comma-separated CORS allowlist |

## Secrets

Secrets are environment-only and referenced indirectly (BR-08): the database
stores a `credential_ref` (the *name* of an environment variable), never the
secret. The app resolves it at call time:

| Variable | Purpose |
|----------|---------|
| `FIQ_DATABASE_URL` | Database DSN (secret) |
| `FIQ_PROVIDER_OPENMETEO_API_KEY` | Open-Meteo key (keyless at MVP) |
| `FIQ_PROVIDER_OPENWEATHER_API_KEY` | OpenWeather key (deferred provider) |
| `FIQ_DEV_ADMIN_TOKEN` | Dev admin token (never in production) |

`.env`, `.env.local`, keys, and payloads are gitignored; gitleaks scans commits
in CI.

## Environments

| Environment | Notes |
|-------------|-------|
| local | `docker compose` (PostgreSQL 16 + app); `FIQ_LOG_FORMAT=text` for readability |
| ci | ephemeral; integration tests use testcontainers; migrations job uses a PG16 service |
| production | Hetzner VPS + Caddy + managed Neon PostgreSQL; `FIQ_ENV=production`, no dev token |

There is intentionally **no staging** environment (ADR-031).
