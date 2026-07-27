# ForecastIQ — Deployment and Rollback Runbook (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: NFR-M05..M07; `docs/architecture/06-deployment-architecture.md` §4–6

> **ADR-033 (2026-07-27, WP-27 docs pass)**: production is AWS EC2 t3.small +
> Docker Compose (containerized postgres:16), TLS at Cloudflare (proxied DNS),
> origin plain HTTP :80. Releases are cosign-signed GHCR images referenced by
> digest. The sections below reflect this topology (the earlier Hetzner/native
> systemd + Neon model is retired).

---

## 1. Normal Deploy (main → production)

```text
Preconditions: CI green (lint, tests, OpenAPI diff, image scan incl. trivy secret, promtool rules)

1. Merge to main → GitHub Actions `build-release`:
   builds + pushes ghcr.io/od3n/forecastiq:<version>, records the digest,
   cosign-signs the image (keyless, OIDC).
2. Approve the `deploy-api` job (production environment manual gate).
3. deploy-api verifies the cosign signature, then runs deploy/scripts/deploy.sh
   <digest> against the EC2 host over SSH (pinned host key). deploy.sh:
   a. scp docker-compose.prod.yml + smoke-test + backup/restore scripts to /opt/forecastiq
   b. record current FIQ_IMAGE → .previous-image (rollback target)
   c. docker compose pull app db; docker compose up -d db
   d. migrations: docker compose run --rm app migrate up (forward-only; contract pattern)
   e. docker compose up -d app
   f. wait for /readyz green (max 30 s)
   g. smoke tests: healthz, readyz, GET /api/v1/rankings (200), admin 401 gate
   h. docker image prune (keep recent)
4. Record deploy in ops log (image digest, time, migration ids, smoke results)
```

Expected downtime: a few seconds (compose recreation). Dashboard deploys
independently via Cloudflare Pages (zero downtime).

## 2. Rollback (< 5 min, NFR-M07)

**Trigger criteria:** crash-loop, smoke test failure, sustained 5xx > 5%
post-deploy, data integrity anomaly traced to deploy.

```text
1. bash deploy/scripts/rollback.sh   (on host, or remote with VPS_HOST set)
   - reads /opt/forecastiq/.previous-image (the prior digest deploy.sh recorded)
   - swaps FIQ_IMAGE back + records the rolled-back-from image (so a second
     run rolls forward again — this is what the monthly drill exercises)
   - docker compose up -d app  (image already present locally; no registry pull)
   - waits for /readyz + runs smoke tests; prints elapsed vs the 300 s NFR-M07 gate
2. Incident note: image rolled back, reason, follow-up
Total: < 5 min (rehearsed monthly via .github/workflows/scheduled.yml)
```

**Migration rollback:** NOT automatic. Image rollback works with newer schema
(contract pattern: additive columns ignored by the old image). Destructive
migrations are forbidden without a two-phase contract; recovery from a bad
migration is a new forward migration or a restore from the nightly pg_dump
(`docs/operations/04-backup-and-restore.md`; no vendor PITR under ADR-033).

## 3. Migration Safety Rules

| Rule | Enforcement |
|------|-------------|
| Additive-first (expand-contract) | Review checklist; destructive column drops only in a release AFTER all readers updated |
| No long locks | CREATE INDEX CONCURRENTLY; ALTER TABLE ... ADD COLUMN (PG11+ instant) |
| Reversible OR documented irreversible | Every migration PR states reversibility |
| Dry-run in CI | Against copy of production schema (anonymized subset) |
| Forward-only in production | No down migrations run in prod; recovery via new migration or PITR |

## 4. Emergency Procedures

### 4.1 Kill switch (stop all provider calls)
`PATCH /admin/providers/{id}/status {status: disabled}` per provider — scheduler stops within one tick; API continues serving stored data. Re-enable when resolved.

### 4.2 Scheduler-only stop (API unaffected)
Not separable in single process — use provider disable (§4.1) for collection issues. Full process stop only for process-level faults (API down too; acceptable tradeoff documented).

### 4.3 Host rebuild
`docs/operations/04-backup-and-restore.md` §4 (restore the nightly pg_dump into
a fresh compose stack on a rebuilt instance).

### 4.4 DNS failover
Cloudflare: point the `api` record at the rebuilt instance's Elastic IP
(`terraform/cloudflare.tf`; proxied, TTL auto).

## 5. Deploy Checklist (pre-flight)

- [ ] CI green including OpenAPI breaking-change diff
- [ ] Migrations reviewed (forward-only, additive-first)
- [ ] Release notes written (conventional commits → auto-generated)
- [ ] Rollback target present (`/opt/forecastiq/.previous-image`)
- [ ] Backup status green (last nightly < 26 h, last restore test < 35 d)
- [ ] No active incidents / error budget not exhausted

## 6. Post-Deploy Verification (10 min watch)

- Grafana API dashboard: error rate, latency p95, restart count
- Pipeline dashboard: next collection cycle completes normally
- One manual admin health check (circuits, freshness)
- Anomaly → rollback decision within 15 min

## 7. Cross-Reference

- CI/CD pipeline: `docs/delivery/02-ci-cd.md`
- Database recovery: `docs/operations/07-database-recovery-runbook.md`
- Provider failure: `docs/operations/06-provider-failure-runbook.md`
