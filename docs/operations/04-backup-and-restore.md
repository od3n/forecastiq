# ForecastIQ — Backup and Restore (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: NFR-A04/A05, NFR-D06/D07; `docs/architecture/09-reliability-architecture.md` §7

---

## 1. Backup Layers

| Layer | Method | Frequency | Retention | Encryption | Owner |
|-------|--------|-----------|-----------|-----------|-------|
| Managed DB PITR | Vendor WAL archiving | Continuous | 7–30 d (tier) | Vendor-managed | Vendor |
| Logical dump | `pg_dump -Fc` (compressed, schema+data) via cron on VPS | Nightly 01:00 UTC | 30 d local (volume), 90 d offsite | At rest on encrypted volume; offsite via TLS (B2 server-side encryption) | Operator |
| Offsite sync | rsync/rclone latest dump + weekly full to Backblaze B2 (or second VPS) | Weekly Sunday + after each dump if size changed > 20% | 90 d | B2 SSE | Operator |
| Payload volume | **Not backed up** | — | — | — | Documented acceptance (ADR-011: 90 d ephemeral; normalized data in DB) |
| Configuration | Git repo (IaC, Caddyfile, systemd units, bootstrap script) | On change | Forever | Repo private | Operator |

## 2. Targets

| Scenario | RPO | RTO | Procedure |
|----------|-----|-----|-----------|
| DB corruption / logical error | < 1 h | < 2 h | PITR to pre-corruption timestamp (vendor console) |
| VPS loss (hardware) | < 1 h (DB unaffected) | < 4 h | §4 VPS rebuild |
| Accidental config deletion | 0 (git) | < 30 min | git restore + redeploy |
| Vendor DB loss (catastrophic) | 24 h | < 8 h | Restore latest offsite dump to new managed instance |

## 3. Nightly Dump Script (specification)

```bash
#!/usr/bin/env bash  # /opt/forecastiq/scripts/backup.sh (in repo)
set -euo pipefail
STAMP=$(date -u +%F)
pg_dump -Fc "$DATABASE_URL" > "/var/lib/forecastiq/backups/forecastiq-$STAMP.dump"
# integrity: immediate test-restore to scratch schema
createdb "restore_test_$STAMP"
pg_restore -d "restore_test_$STAMP" --no-owner "…dump"
psql "restore_test_$STAMP" -c "SELECT count(*) FROM forecast_snapshots;" > counts.txt
dropdb "restore_test_$STAMP"
# status file (read by /admin/health)
echo "{\"completed_at\":\"$(date -u +%FT%TZ)\",\"status\":\"success\",\"size_bytes\":$(stat -c%s …)}" \
  > /var/lib/forecastiq/backup-status.json
# prune > 30 d; weekly rclone sync to b2:forecastiq-backups/
```

Failure → non-zero exit → cron mail + status file `failed` → alert A10.

## 4. VPS Rebuild Procedure (RTO < 4 h)

```text
1. Provision new VPS (same size) + attach new volume          [15 min]
2. Run bootstrap script (repo: deploy/bootstrap.sh):
   packages, Caddy, systemd units, grafana-agent, mounts      [10 min]
3. Restore secrets from offline copy (encrypted USB/1Password) [10 min]
4. Deploy pipeline: artifact + migrations (DB is intact —
   managed, unaffected by VPS loss)                           [15 min]
5. Point DNS to new IP (Cloudflare API)                       [5 min + propagation]
6. Smoke tests (healthz, readyz, login, one ranking)          [10 min]
7. Payload volume: empty (accepted loss ≤ 90 d payloads);
   collections continue; replay unavailable for old window
Total: ~1.5 h active work, < 4 h with diagnosis
```

## 5. Monthly Restore Test (NFR-D06, automated)

- First Sunday monthly: CI-style job on VPS restores latest offsite dump to a scratch managed DB (temporary instance or schema), runs integrity checks (table row counts vs. production ±2%, checksum sample of 100 random collections' payload files if present), writes `last_restore_test` to backup status file.
- Result visible in `/admin/health` (system section) — operator reviews monthly.
- Failure → critical alert A11 + immediate investigation (backup integrity is a durability control).

## 6. Restore Decision Tree

```text
Data problem detected
  ├─ Logical error (bad migration, wrong delete) with known timestamp?
  │    → PITR to timestamp − 1 min (vendor console) → verify → swap connection string
  ├─ Single-table corruption?
  │    → pg_restore --table from nightly dump (≤ 24 h loss for that table)
  ├─ Full DB loss?
  │    → New managed instance → restore offsite dump → update DATABASE_URL → restart
  └─ Payload file needed?
       → Within 90 d: file on volume (verify checksum)
       → Older: unrecoverable (documented); normalized rows + checksum remain
```

## 7. Configuration Recovery

All infrastructure reproducible from repo: bootstrap script (packages, users, mounts), Caddyfile, systemd units, grafana-agent config, Terraform (DNS + DB project). Secrets: offline encrypted copy (1Password/encrypted file) — single operator knows location (bus-factor mitigation: documented in sealed envelope procedure, R-05).

## 8. Cross-Reference

- DB recovery runbook: `docs/operations/07-database-recovery-runbook.md`
- Deployment: `docs/operations/05-deployment-and-rollback.md`
- Alerts A10/A11: `docs/operations/03-monitoring-and-alerting.md`
