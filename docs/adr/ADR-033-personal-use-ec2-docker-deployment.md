# ADR-033: Personal-Use EC2 + Docker Deployment

**Status**: Accepted (2026-07-26)
**Supersedes**: the Hetzner CX32 native-binary deployment model of
docs/architecture/06-deployment-architecture.md §3–§4 and the Neon managed
PostgreSQL provisioning of docs/delivery/04-infrastructure-as-code.md §2.
**Related**: ADR-001 (modular monolith), ADR-007 (Kubernetes deferral),
ADR-013 (deployment unit boundaries), WP-23 DRB review (DRB-WP23-001…019).

## Context

ForecastIQ is operated for personal use by a single operator. The originally
planned production topology — Hetzner CX32, native Go binary managed by
systemd, Caddy reverse proxy with origin TLS, Neon managed PostgreSQL — was
sized and shaped for a small production SaaS. The WP-23 DRB review found the
native deploy path had never been executed end-to-end and carried significant
host-configuration surface (systemd unit installation, Caddy APT packaging,
sudo wrapper provisioning, binary permission transport).

An AWS EC2 t3.small instance is already available, provisioned by a separate
Terraform project, and the repo already ships a production-grade distroless
container image (used by the `image` CI gate since WP-01).

## Decision

1. **Host**: AWS EC2 t3.small (2 vCPU burst, 2 GiB), Ubuntu 22.04+, provisioned
   externally. This repo's Terraform manages Cloudflare DNS only and consumes
   the instance's Elastic IP as `var.vps_ip` (a `terraform_remote_state` hook
   is documented in terraform/main.tf for later automation).
2. **Runtime**: everything in containers via Docker Compose
   (deploy/compose/docker-compose.prod.yml): the app image plus a
   `postgres:16-alpine` sibling. Restart policy replaces systemd; there is no
   host-installed application software beyond Docker itself.
3. **Database**: PostgreSQL runs **on the instance** (pgdata on the EBS
   volume), replacing Neon. This deviates from the Phase-1 "managed
   PostgreSQL" assumption and makes **WP-24 backups the only durability net**
   — the nightly pg_dump + offsite sync and monthly restore test are elevated
   from "important" to "critical" acceptance criteria for WP-24.
4. **TLS**: terminates at Cloudflare (proxied `api` A record). The origin
   serves plain HTTP on :80; no Caddy on the instance. Client IPs arrive via
   `CF-Connecting-IP` — IP-keyed rate limiting sees Cloudflare egress IPs
   until a trusted-proxy header mapping is added (tracked follow-up; the
   admin surface is token-gated regardless).
5. **Release artifact**: the production image, pushed to GHCR by
   `build-release` on main, referenced immutably **by digest**, and signed
   with cosign keyless (image signature replaces the blob signature; the
   CI/CD doc's "sign" deliverable is satisfied at the image level).
   `deploy-api` verifies the signature before any host contact.
6. **Deploy/rollback**: `deploy/scripts/deploy.sh <image-ref>` ships the
   compose file, records the previous image ref, pulls, migrates
   (`docker compose run --rm app migrate up` — migrations are embedded in the
   binary), starts, readyz-gates, smoke-tests. Rollback swaps `FIQ_IMAGE`
   back to the recorded previous digest and restarts — no registry
   round-trip, well inside the NFR-M07 five-minute budget.
7. **Privilege model**: the `deploy` user is in the `docker` group (≈ root on
   this host). Accepted for a single-operator personal deployment; the sudo
   wrapper apparatus of the native model is retired with it.

## Consequences

- The four WP-23 Critical findings (invalid migrate flag, rsync layout
  flattening, unversioned smoke URLs, lost exec bit) are structurally
  eliminated rather than patched: no binary transport, no config copying,
  and a single deploy script shared by CI and operators.
- `deploy/systemd/` and `deploy/caddy/` are removed; grafana-agent host
  installation moves out of bootstrap (observability shipping is revisited
  under the WP-22 stack when needed — the local `obs` compose profile remains
  the reference).
- docs/architecture/06-deployment-architecture.md §3–§4, §8 and
  docs/operations/05-deployment-and-rollback.md §1–§2 are amended by
  reference to this ADR (banner notes added; full rewrite deferred to the
  WP-27 docs pass).
- Cloudflare Pages deploy automation remains descoped from WP-23 (dashboard
  ships as a static export; wiring a Pages project is operator work tracked
  for a follow-up).
- Revisit trigger: if the platform outgrows personal use (external users,
  uptime commitments), re-open this ADR — the managed-DB and origin-TLS
  decisions are the first to reverse.
