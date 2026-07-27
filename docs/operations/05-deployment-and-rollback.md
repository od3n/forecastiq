# ForecastIQ — Deployment and Rollback Runbook (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: NFR-M05..M07; `docs/architecture/06-deployment-architecture.md` §4–6

> **Amendment (2026-07-26, ADR-033)**: deploys are now image-based —
> `bash deploy/scripts/deploy.sh <ghcr image ref>` (pull → migrate → up →
> readyz → smoke) and rollback swaps `FIQ_IMAGE` back to the recorded
> previous digest (`bash deploy/scripts/rollback.sh`). The release-directory
> and systemd steps below are superseded; see ADR-033.

---

## 1. Normal Deploy (main → production)

```text
Preconditions: CI green (lint, tests, OpenAPI diff, image scan, migration dry-run)

1. Tag release: git tag vYYYY.MM.DD-N && push
2. GitHub Actions build: binary (linux/amd64) + migrations + checksums → artifact
3. Approve deploy job (manual gate on main; auto on release tag)
4. Pipeline:
   a. rsync artifact to VPS /opt/forecastiq/releases/<version>/
   b. Verify checksum (sha256sum -c)
   c. Run migrations: /opt/forecastiq/current/forecastiq migrate --confirm
      (forward-only; contract pattern keeps old binary compatible)
   d. systemctl restart forecastiq
      (systemd ExecStop sends SIGTERM → 30 s drain → stop; ExecStart new binary)
   e. Wait for /readyz green (max 30 s)
   f. Smoke tests: healthz, readyz, GET /rankings (200 + data), admin login
5. Record deploy in ops log (version, time, migration ids, smoke results)
```

Expected downtime: < 30 s (drain + restart). Dashboard deploys independently via Cloudflare Pages (zero downtime).

## 2. Rollback (< 5 min, NFR-M07)

**Trigger criteria:** crash-loop (≥ 2 restarts in 5 min), smoke test failure, sustained 5xx > 5% post-deploy, data integrity anomaly traced to deploy.

```text
1. Identify previous good version: ls /opt/forecastiq/releases/ (last 5 kept)
2. ln -sfn /opt/forecastiq/releases/<prev> /opt/forecastiq/current
3. systemctl restart forecastiq
4. /readyz + smoke tests
5. Incident note: version rolled back, reason, follow-up
Total: < 5 min (rehearsed monthly)
```

**Migration rollback:** NOT automatic. Binary rollback works with newer schema (contract pattern: additive columns ignored by old binary). If a migration itself was destructive (should never be — governance forbids destructive migrations without two-phase contract), use PITR (§database runbook).

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

### 4.3 VPS rebuild
`docs/operations/04-backup-and-restore.md` §4.

### 4.4 DNS failover
Cloudflare: point api record to rebuilt VPS IP (TTL 60 s pre-configured for fast switch).

## 5. Deploy Checklist (pre-flight)

- [ ] CI green including OpenAPI breaking-change diff
- [ ] Migration dry-run passed against prod-schema copy
- [ ] Release notes written (conventional commits → auto-generated)
- [ ] Rollback target identified (previous version path)
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
