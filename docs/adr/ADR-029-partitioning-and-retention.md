# ADR-029: Partitioning and Retention Mechanics — Declarative Monthly Partitions

**Status**: Accepted (Phase 1) — implements ADR-004
**Date**: 2026-07-22

## Context
ADR-004 chose standard PostgreSQL with declarative partitioning. Phase 1 fixes the mechanics: partition granularity, creation, retention execution, and the immutability-trigger interaction.

## Options considered
1. **Monthly RANGE partitions on forecast_snapshots(target_time) and observations(observed_at); auto-created 3 months ahead by maintenance job; retention = DROP PARTITION after 2 y / 5 y; child-table purges (matches 2 y, audit 1 y) as bounded DELETE batches.**
2. Daily partitions — 365 partitions/year of overhead for hourly data; monthly is the granularity sweet spot at ~1.6M rows/partition.
3. pg_partman extension — capable, but adds an extension dependency the constraints avoid; a 40-line maintenance job replaces it.
4. Retention via DELETE on partitioned tables — scans and WAL-heavy; DROP PARTITION is O(1).

## Decision
Option 1, per `docs/data/05-retention-and-archival.md`.

## Rationale
- Monthly granularity matches query patterns (day queries prune to 1–2 partitions) and retention granularity (drop whole months).
- The custom maintenance job is trivial, fully tested, and owned — no extension surface (consistent with ADR-004's portability rationale).
- Purging matched_evaluations BEFORE dropping the snapshot partitions preserves FK integrity without cascades.
- The maintenance-GUC exemption for aged purges keeps immutability triggers absolute for application code while allowing documented, audited retention.

## Consequences
- (+) Retention is the fastest possible operation (DDL drop) and cannot accidentally touch live data (boundaries are month-aligned and age-checked).
- (+) Partition creation idempotent (IF NOT EXISTS) — safe under double-fires and restarts.
- (−) PK must include the partition key ((id, target_time)) — handled; logical identity unchanged (id unique by generation).
- (−) Cross-partition unique indexes are per-partition — the dedup uniqueness is naturally per-partition-safe because (provider, location, issued_at, target_time) duplicates land in the same partition.

## Risks
Maintenance job failure → partitions missing 3 months out (alert on job failure; creation catch-up is idempotent).

## Migration trigger
TimescaleDB promotion (ADR-004 triggers) converts these tables to hypertables; retention becomes policy-based.

## Review date
Quarterly with volume metrics; at first actual partition drop (month 25).
