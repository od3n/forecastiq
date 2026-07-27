# ForecastIQ — Deployment Architecture (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: ADR-007 (single VPS + Caddy); constraints §5 (hosting model, $50–150/mo target); NFR-M05..M07

> **Amendment (2026-07-26, ADR-033)**: production now runs on an AWS EC2
> t3.small with Docker Compose (app + PostgreSQL containers), TLS terminated
> at Cloudflare (proxied DNS, no origin Caddy), releases shipped as
> cosign-signed GHCR images referenced by digest. §3–§4 and §8 below describe
> the superseded Hetzner/native model; see
> `docs/adr/ADR-033-personal-use-ec2-docker-deployment.md` for the current
> topology. Full doc rewrite is deferred to the WP-27 docs pass.

---

## 1. Platform Decision

**Primary (ADR-033, personal-use): AWS EC2 t3.small + Docker Compose
(containerized PostgreSQL 16) + Cloudflare (TLS/CDN/DNS).**

The original Phase-1 plan (Hetzner VPS + native systemd Go binary + Neon
managed PostgreSQL + origin Caddy) was superseded by ADR-033 for a
single-operator personal deployment. The instance is provisioned by a separate
Terraform project; this repo's Terraform manages Cloudflare DNS only.

| Criterion | EC2 t3.small + Docker (selected) | Native systemd (superseded) |
|-----------|----------------------------------|-----------------------------|
| App runtime | Container (distroless image, GHCR, cosign-signed) | Native Go binary via systemd |
| Database | postgres:16 container, pgdata on EBS | Neon/Supabase managed |
| TLS | Cloudflare (proxied); origin plain HTTP :80 | Origin Caddy 2 (ACME) |
| Scheduled jobs | In-process (ADR-005) | In-process (ADR-005) |
| Secrets | `/etc/forecastiq/secrets.env` via compose `env_file` (read by `deploy` user) | systemd EnvironmentFile |
| Rollback | Swap `FIQ_IMAGE` to recorded previous digest (< 5 min) | Redeploy previous artifact |
| Durability | Nightly pg_dump + weekly B2 offsite (WP-24) — only net (no vendor PITR) | Managed-DB PITR |

Decision rationale: ADR-033 — an EC2 instance already exists, the repo already
ships a production distroless image, and containerizing removes the entire
host-config surface (systemd unit, Caddy packaging, sudo wrappers, binary
transport). No Kubernetes (ADR-007 binding).

## 2. Environment Topology

| Environment | Purpose | Infrastructure |
|-------------|---------|----------------|
| **local** | Development | Docker Compose: app (Go, hot-reload via air) + PostgreSQL 16 + payload volume mount. `.env.local` for secrets. |
| **ci** | Test execution | GitHub Actions: ephemeral PostgreSQL 16 service container; no persistent state. |
| **production** | Public deployment | EC2 t3.small: `docker compose` (app + postgres:16) + EBS volume + Cloudflare (proxied TLS/DNS). |
| staging | **Not justified for MVP** | Single operator; production-like validation via Cloudflare Pages PR previews of the dashboard + the deploy rehearsal documented in the WP-23 re-review. |

## 3. Production Topology

```mermaid
graph TB
    DNS["Cloudflare DNS + TLS<br/>(api.<domain> → EC2 Elastic IP, proxied)"]
    CF["Cloudflare Pages<br/>(dashboard static export)"]
    subgraph "AWS EC2 t3.small (Docker Compose)"
        APP["app container<br/>(distroless, :8080 → host :80)"]
        DB[("postgres:16 container<br/>(pgdata on EBS)")]
        VOL["EBS volume<br/>(payloads, backups)"]
        BACKUP["Nightly pg_dump (cron)<br/>+ weekly rclone → B2"]
    end
    GHCR["GHCR<br/>(cosign-signed image by digest)"]
    SB["Supabase Auth"]
    GC["Grafana Cloud free tier<br/>(logs, metrics, alerts)"]

    DNS --> APP
    CF -.dashboard.-> DNS
    APP --> DB
    APP --> VOL
    APP --> SB
    APP --> GC
    GHCR -.pulled by deploy.sh.-> APP
    BACKUP --> DB
    BACKUP --> VOL
```

TLS terminates at Cloudflare; the origin serves plain HTTP :80 (the EC2
security group should restrict :80 to Cloudflare IP ranges — ADR-033 §4
follow-up). Metrics bind loopback-only on the host (`127.0.0.1:9090`).

## 4. Deployment Flow

```text
merge to main
  → GitHub Actions:
      1. lint + test + OpenAPI diff + image build (distroless) + Trivy (vuln+secret)
      2. build-release: push ghcr.io/od3n/forecastiq:<version>, record digest,
         cosign sign (keyless, OIDC)
  → deploy-api job (production environment manual approval):
      3. cosign verify the image against this repo's main workflow identity
      4. deploy/scripts/deploy.sh <digest> over SSH (pinned host key):
         pull → up -d db → `compose run --rm app migrate up` → up -d app →
         readyz → smoke → image prune
      (deploy-api skips cleanly when EC2 secrets are unset — image still built + signed)
  → dashboard: Cloudflare Pages builds the static export independently
```

**Zero-downtime expectation:** MVP accepts a few seconds' unavailability during
compose recreation (single instance). True zero-downtime requires a second
instance (promotion).

## 5. Database Migrations

- Tool: **golang-migrate** (numbered SQL: `NNNN_description.up.sql` / `.down.sql`).
- Ownership: `migrations/` directory in monorepo; applied by deploy pipeline only (never by the app at runtime in production; local dev may auto-migrate).
- Rules: every migration reversible OR documented irreversible; expand-contract pattern for column changes; no long locks (CREATE INDEX CONCURRENTLY where applicable); migration dry-run in CI against a production snapshot copy.
- Rollback: binary rollback does NOT auto-rollback migrations (forward-only in production; contract pattern makes old binary compatible with new schema during transition).

## 6. Rollback Procedure (NFR-M07: < 5 min)

1. `bash deploy/scripts/rollback.sh` → swap `FIQ_IMAGE` to the recorded previous
   digest (`/opt/forecastiq/.previous-image`); the image is still present
   locally (no registry pull).
2. `docker compose up -d app` recreates the container on the previous image.
3. Smoke test. Total: < 5 min (rehearsed monthly via scheduled.yml).

Schema rollback: only via a new forward migration (contract pattern); restore
from the nightly pg_dump for data corruption (no vendor PITR under ADR-033;
separate backup runbook).

## 7. Secrets Management (summary; detail in `docs/security/04-secrets-management.md`)

| Secret | Storage | Rotation |
|--------|---------|----------|
| POSTGRES_PASSWORD | `/opt/forecastiq/.env` (compose interpolation, `deploy`-owned 0600) | On suspicion; recreate db + app |
| OpenWeather API key | `/etc/forecastiq/secrets.env` (compose `env_file`); referenced by `credential_ref` | On suspicion; runbook |
| SUPABASE_SERVICE_ROLE_KEY | Same file; backend-only | Vendor dashboard |
| JWT signing | Not stored (JWKS fetch) | Vendor-managed rotation tolerated |
| Deploy SSH key | GitHub Actions secret; `deploy` user key on the EC2 host | 180 d |

Secrets are read client-side by the `deploy` user via the compose `env_file`
(not a root systemd EnvironmentFile). Rotation is exercised monthly by
`deploy/scripts/rotation-drill.sh --live` (recreates the app container). No
secrets in repository, images, or CI logs (gitleaks in CI; `.env*` gitignored).

## 8. Domain and TLS

- `api.<domain>` → EC2 Elastic IP (Cloudflare DNS, **proxied**; TLS terminates
  at Cloudflare, origin serves plain HTTP :80 per ADR-033). Client IPs arrive
  via `CF-Connecting-IP`; the EC2 security group should restrict :80 to
  Cloudflare IP ranges (ADR-033 §4 follow-up).
- HSTS + Always-Use-HTTPS: Cloudflare zone settings (`terraform/cloudflare.tf`).
  API response security headers: app `SecurityHeaders` middleware (WP-25).
- Dashboard: `app.<domain>` on Cloudflare Pages (edge TLS; CSP via
  `web/public/_headers`).

## 9. Infrastructure as Code Scope

Detail: `docs/delivery/04-infrastructure-as-code.md`. Summary: this repo's
Terraform manages **Cloudflare DNS + zone settings only**; the EC2 instance is
provisioned by a separate Terraform project (consumed here via `var.vps_ip`).
Host preparation is `deploy/bootstrap.sh` (Docker Engine, `deploy` user,
ufw/fail2ban, data dirs). Platform-managed: Pages, Grafana Cloud, Supabase
project (manual bootstrap, documented).

## 10. Cost Summary (constraints §5)

| Component | Est. $/mo |
|-----------|-----------|
| AWS EC2 t3.small (on-demand; less if reserved/existing) | ~15 |
| EBS volume (gp3, ~30 GB) | 3 |
| Cloudflare (Pages free + DNS free) | 0 |
| Supabase Auth (free tier) | 0 |
| Grafana Cloud free tier | 0 |
| Domain | 1.5 |
| Backup storage (B2 ~50 GB) | 3 |
| **Total** | **~$22–25** |

Well within the $50–150 target. PostgreSQL is self-hosted on the instance
(no managed-DB line), which is why WP-24 backups are the only durability net.

## 11. Cross-Reference

- CI/CD detail: `docs/delivery/02-ci-cd.md`
- Environments: `docs/delivery/03-environments.md`
- IaC: `docs/delivery/04-infrastructure-as-code.md`
- Rollback runbook: `docs/operations/05-deployment-and-rollback.md`
- ADR: ADR-007
