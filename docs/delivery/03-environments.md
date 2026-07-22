# ForecastIQ — Environments (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: `docs/architecture/06-deployment-architecture.md` §2; scaling doc §1.9 (staging promotion)

---

## 1. Environment Set (three)

| Environment | Purpose | Lifecycle | Parity with prod |
|-------------|---------|-----------|------------------|
| **local** | Development, debugging, fast iteration | Per developer; docker-compose up | High (same PG16, same binary flags; fake providers optional) |
| **ci** | Automated verification | Ephemeral per job (GitHub runners + service containers) | High for tests; no persistence |
| **production** | Public service | Permanent | — |

**Staging: intentionally absent.** Rationale: single operator; preview deploys (Pages) cover dashboard review; migration dry-runs against prod-schema copies cover schema risk; cost/ops of a standing staging env buys nothing at this team size. Promotion trigger (scaling doc §1.9): second engineer, customer SLAs, or migration complexity beyond dry-run confidence.

## 2. Local Environment

```yaml
# docker-compose.yml (committed)
services:
  postgres:
    image: postgres:16
    environment: { POSTGRES_DB: forecastiq, ... }
    volumes: [pgdata:/var/lib/postgresql/data]
    ports: ["5432:5432"]
  app:
    build: { context: ., target: dev }   # air hot-reload
    environment_file: .env.local          # gitignored
    volumes:
      - .:/src
      - payloads:/var/lib/forecastiq/payloads
    depends_on: [postgres]
    command: ["air"]                       # or: go run ./cmd/forecastiq serve --mode=all
volumes: { pgdata: {}, payloads: {} }
```

| Aspect | Specification |
|--------|---------------|
| Bootstrap | `make dev-up`: compose up + auto-migrate + seed (system workspace, providers, admin user via `seed-local` with dev-only flag) |
| Secrets | `.env.local` (gitignored); `.env.example` committed with placeholders |
| Providers | Fake provider server (`make dev-fake-providers`) serving fixtures on local ports; real provider calls opt-in via env flag (rate-limit conscious) |
| Auth | Supabase local dev (supabase CLI) OR dev-mode JWT signer (test-only, flag-gated, never in prod binary path) |
| Reset | `make dev-reset`: volume down -v + re-migrate |
| Observability | Logs to stdout; optional local Grafana via compose profile (not default) |

## 3. CI Environment

| Aspect | Specification |
|--------|---------------|
| Runners | ubuntu-latest (GitHub-hosted); self-hosted only if cost/speed demands (promotion-style decision) |
| DB | postgres:16 service container (fresh per job; migrations applied by test harness) |
| Caching | Go build cache, module cache, npm cache (actions/cache) |
| Secrets | GitHub Actions secrets (deploy key, API tokens); never in logs |
| Artifacts | Binary + spec + checksums; 90-day retention; cosign-signed |
| Parallelism | Job-level (checks/integration/contract/image parallel); test-level via Go's native parallelism |

## 4. Production Environment

Per deployment architecture: VPS (systemd + Caddy + volume) + managed PostgreSQL + Cloudflare (DNS/Pages) + Supabase Auth + Grafana Cloud. Single instance; `--mode=all`.

## 5. Configuration Management

| Config class | Mechanism | Examples |
|--------------|-----------|----------|
| Build-time | Go ldflags | version, commit |
| Runtime (non-secret) | Env vars (12-factor) | PORT, LOG_LEVEL, DB pool sizes, provider base URLs, batch intervals |
| Runtime (secret) | Env vars from EnvironmentFile (prod) / .env.local (dev) | DATABASE_URL, keys |
| Domain config | Database rows | Schedules, attribution, provider status |
| Feature flags | **Database status fields** (constraints §6 boundary 8) — no flag service | Provider enable/disable |

Environment differences expressed entirely in env values — same binary everywhere (dev/prod parity rule; no env-specific code paths except a `dev-mode` build tag for local auth convenience, excluded from release builds).

## 6. Data per Environment

| Environment | Data source |
|-------------|-------------|
| local | Seed script (synthetic JB location + demo data option `make dev-demo-data`) |
| ci | Test builders + fixtures (never prod data) |
| production | Real collections |

Prod → non-prod data copy: **never** (classification rule). Anonymized subset extraction is a post-incident forensic procedure only, with explicit operator action.

## 7. Cross-Reference

- Deployment topology: `docs/architecture/06-deployment-architecture.md`
- CI/CD: `docs/delivery/02-ci-cd.md`
- IaC: `docs/delivery/04-infrastructure-as-code.md`
