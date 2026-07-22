# ForecastIQ — Database Recovery Runbook (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: NFR-A04/A05; `docs/operations/04-backup-and-restore.md`; DR table (NFR §10)

---

## 1. Symptom → Procedure Map

| Symptom | Likely cause | Procedure | RTO |
|---------|-------------|-----------|-----|
| API 503 + readyz DB check failing, app logs `connection refused` | Managed DB outage / network | §2 connectivity | Vendor-dependent |
| API 503, DB reachable but `relation does not exist` / weird errors | Bad migration | §3 migration incident | < 1 h |
| Wrong data appeared at a known time (bad batch, bad deploy) | Logical corruption | §4 PITR | < 2 h |
| Managed DB catastrophic loss (vendor incident) | Vendor failure | §5 full restore | < 8 h |
| Single table anomaly (e.g., rankings wrong, metrics off) | Data bug | §6 targeted restore | < 2 h |
| Disk-full on DB (managed: storage limit) | Growth | §7 storage | < 1 h |

## 2. Connectivity Failure

```text
1. Managed DB status page — vendor incident?
   YES → wait (vendor SLA); API serves stale cache with labels (automatic);
         log incident; no local action
   NO  → 2. From VPS: psql "$DATABASE_URL" -c 'SELECT 1'
        3. Check: VPS egress (curl 1.1.1.1), DB IP allowlist (VPS IP changed after
           rebuild?), credential rotation mismatch (env file vs vendor console)
        4. Fix allowlist/credential → systemctl restart forecastiq → readyz green
```

## 3. Bad Migration Incident

```text
1. STOP: prevent further deploys (lock main branch deploy gate)
2. Assess: which migration? what changed? (schema_migrations table + PR)
3. IF additive-only (typical): binary rollback is enough
   (old binary ignores new columns) → rollback runbook
4. IF data was altered destructively (should be impossible under governance):
   → §4 PITR to pre-migration timestamp
5. Write corrective forward migration (never edit applied migration files)
6. Post-mortem: why did dry-run miss this? (add to CI scenarios)
```

## 4. Point-in-Time Recovery (PITR)

**Precondition:** managed DB with PITR (Neon/Supabase paid tier).

```text
1. Identify corruption boundary: last-known-good timestamp T
   (audit_events + collections around the anomaly; err on the early side)
2. Vendor console: create restored branch/instance at T − 1 min
   (Neon: branch from timestamp; Supabase: PITR restore to new instance)
3. Verify restored data:
   - Row counts per table vs. production (expected delta = T→now writes)
   - Spot-check: rankings for 3 cells match pre-incident values
   - Application-level smoke: point a LOCAL instance at restored DB, run key queries
4. Swap: update DATABASE_URL on VPS → systemctl restart forecastiq
5. Verify: readyz, rankings endpoint, admin health
6. Decommission corrupted instance AFTER 7 d hold (cheap insurance)
7. Incident record: timeline, T chosen, data lost window (T→now writes),
   re-collection plan for the gap (providers still have recent data;
   observation source re-queryable for ≤ 2 h; older gap = honest coverage loss)
```

## 5. Full Restore from Offsite Dump

```text
1. Provision new managed DB instance (same version: PostgreSQL 16)
2. Download latest offsite dump (B2: forecastiq-backups/forecastiq-<date>.dump)
3. pg_restore -d <new-db> --no-owner --jobs=4 <dump>
4. Run forward migrations since dump date (migrations/ NNNN > dump schema_version)
5. Integrity: table counts, immutability triggers present, partitions exist
   (create missing monthly partitions via maintenance job on first start)
6. Swap DATABASE_URL → restart → smoke tests
7. Data loss window: up to 24 h (nightly dump RPO) — acceptable per DR table
```

## 6. Targeted Table Restore

```text
1. pg_restore --list dump | identify table
2. pg_restore --table=provider_rankings -d scratch_db dump
3. Copy rows for affected scope from scratch → production
   (DELETE affected logical keys in tx, INSERT from scratch, verify counts)
4. IF table is immutability-trigger-protected: use maintenance GUC session
   (documented exemption, audit-logged)
5. Recompute downstream if needed (rankings recompute endpoint)
```

## 7. Storage Pressure (managed DB)

```text
1. Vendor alert at 80% → check: pg_database_size + top tables
   (forecast_snapshots/observations partitions as expected? retention job ran?)
2. IF retention job failed: run maintenance manually (partition drop for expired)
3. IF legitimate growth: upgrade storage tier (vendor console, online operation)
4. Sustained growth > model: trigger data-growth review (docs/data/06 §5)
```

## 8. Prevention Controls (why most of this should never fire)

- Immutability triggers block accidental pipeline deletes (largest class of logical errors).
- Forward-only migrations + dry-run + expand-contract governance.
- PITR continuous (RPO < 1 h) makes timing pressure low.
- Monthly restore test validates the dump path before it's needed.

## 9. Cross-Reference

- Backup layers: `docs/operations/04-backup-and-restore.md`
- Deployment/migration rules: `docs/operations/05-deployment-and-rollback.md` §3
- DR targets: NFR §10 table
