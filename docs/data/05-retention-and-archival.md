# ForecastIQ — Retention and Archival (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: BR-09; NFR-D01..D05; ADR-011 (payloads); domain model §11

---

## 1. Retention Schedule (binding)

| Data | Retention | Mechanism | Rationale |
|------|-----------|-----------|-----------|
| forecast_snapshots | 2 years | Monthly partition drop | NFR-D01; supports 2 y of accuracy history |
| observations | 5 years | Monthly partition drop | NFR-D03; longer truth baseline |
| forecast_collections | Indefinite | — (small: ~200K rows/y) | Lineage metadata; cheap |
| matched_evaluations | 2 years | Bounded DELETE batches (monthly job) | Aligns with snapshot retention (FK integrity: purge children before partition drop) |
| accuracy_metrics | Indefinite | — | NFR-D02; reproducibility |
| provider_rankings | Indefinite | — | History of published rankings |
| audit_events | 1 year | Bounded DELETE batches (monthly job) | NFR-D04 |
| raw payloads (volume) | 90 days | Daily path-based deletion job | ADR-011 |
| collection_schedules | 90 days | DELETE batches | Operational only |
| schedule_runs | 90 days | DELETE batches | Operational history (S-13 window) |
| export_jobs + files | 24 h after expiry | Job deletes row + file | GDPR minimization |
| users/api_keys | Until account deletion | AUTH-09 flow | GDPR |

## 2. Partition Lifecycle (snapshots + observations)

```text
Maintenance job (daily at 03:00 UTC):
  1. Ensure partitions exist: current month + next 3 months
     (CREATE TABLE IF NOT EXISTS ... PARTITION OF ... FOR VALUES FROM (...) TO (...))
  2. Drop expired partitions:
     - forecast_snapshots: partitions with upper bound < now() − 2 y
     - observations: upper bound < now() − 5 y
  3. Before snapshot partition drop: delete matched_evaluations rows referencing
     that partition's time range (bounded batches of 10K, FK composite reference)
  4. Log + metric: partitions_created, partitions_dropped, rows_purged
```

- Partition creation is idempotent (IF NOT EXISTS); safe under restarts and double-fires.
- DROP PARTITION is near-instant DDL (no row scanning) — preferred over DELETE for time-expiry.
- Partitions are created by the **migrations role** (DDL rights); the app role cannot create/drop.
- First partition set created in the initial migration (bootstrap covers current + 3 months).

## 3. Aged-Row Purge (non-partitioned tables)

```text
Monthly job (first Sunday 04:00 UTC), per table:
  WHILE deleted_count = batch_size:
    DELETE FROM <table> WHERE created_at < cutoff LIMIT 10000;  -- via ctid subquery
    pg_sleep(0.1);  -- throttle
```

- Bounded batches prevent long locks and WAL spikes.
- Immutability trigger exemption: maintenance session sets `forecastiq.maintenance = on` GUC; trigger checks GUC + row age (double safety; unit-tested).
- Purge order respects FKs: matched_evaluations before snapshot partitions; nothing references audit/schedules/runs.

## 4. Raw Payload Retention Job

```text
Daily at 02:00 UTC:
  Delete files under payloads/{provider}/{yyyy}/{mm}/{dd}/ where date < now() − 90 d
  (directory-level deletion by path date — no file scanning needed)
  Remove empty parent directories.
  Metric: payload_files_deleted_total; log summary.
```

- Checksums remain on collection rows forever (integrity claims checkable while payloads exist; ADR-011).
- Volume usage gauge alerts at 80% (well before the 50 GB promotion trigger).

## 5. Archival Strategy

**No cold archive tier in MVP.** Rationale:
- Metrics/rankings (the permanent record) are small (~10⁵ rows) and stay hot.
- Snapshots beyond 2 y have no product surface (trends cap at 365 d; methodology periods ≤ 90 d).
- Collections metadata (indefinite) preserves the *fact* of historical collection even after snapshot expiry.
- If archival is ever required (compliance): `pg_dump` of expired partitions to object storage before drop — additive procedure, no schema change.

## 6. GDPR Data Lifecycle (AUTH-09)

| Step | Action |
|------|--------|
| Export request | Create export_jobs row (409 if active job exists) → async job assembles JSON (user row, keys list, created-resources list) → file on volume → download link (UUID path, 24 h) |
| Expiry | Retention job deletes file + row after expires_at |
| Account deletion | Delete api_keys, export_jobs, user row (audit.user_id → SET NULL); Supabase account deleted via Admin API; weather data untouched (documented position: not personal data, NFR-D08) |

## 7. Retention Governance

- Retention changes require: business-rule amendment (BR-09) + migration + risk review.
- Monthly volume review (watchlist item): snapshot count, observation count, payload volume, DB size — tracked in Grafana; deviations > 2× model trigger investigation (R-13).
- Partition drop is **irreversible** — the monthly restore test (NFR-D06) validates that backups cover accidental over-deletion (drop wrong partition → PITR recovery < 2 h).

## 8. Cross-Reference

- Growth model: `docs/data/06-data-growth-and-cost-model.md`
- Backup/restore: `docs/operations/04-backup-and-restore.md`
- ADR-011 (payload retention decision)
