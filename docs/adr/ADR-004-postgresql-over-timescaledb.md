# ADR-004: Standard PostgreSQL over TimescaleDB for MVP

**Status**: Accepted (Phase 0 Amendment)
**Date**: 2026-07-22

## Context
Phase 0 assumed TimescaleDB (hypertables, continuous aggregates, compression). The ARB
questioned whether it is required and noted no partition strategy existed. The
amendment directed: prefer standard PostgreSQL unless justified; do not adopt
continuous aggregates merely because they exist.

## Options considered
1. **Standard PostgreSQL 16** with declarative monthly partitioning on
   `forecast_snapshots.target_time` and `observations.observed_at`, composite indexes,
   retention via partition drop.
2. TimescaleDB hypertables + compression + continuous aggregates for metrics.
3. Dedicated time-series DB (e.g., InfluxDB) + relational DB — two stores.

## Decision
Option 1. Metrics/rankings are batch-computed **table rows**, not views or continuous
aggregates — the batch engine already materializes them, so in-DB aggregation adds
nothing at MVP query volumes.

## Rationale
- MVP write volume (~30K snapshots/day ≈ 0.35 rows/s average) is trivial for vanilla
  Postgres; reads are served by precomputed metric/ranking rows + indexes.
- Managed Postgres offerings without extensions are cheaper and more portable
  (Neon/Supabase/RDS all work); TimescaleDB narrows hosting options and adds an
  extension-version operational surface.
- Partitioning + `pg_partman`-style jobs cover retention; compression is unnecessary
  at this scale (storage ≈ single-digit GB/year).

## Consequences
- (+) Zero extension dependencies; widest managed-DB choice; simpler ops.
- (+) Partition-drop retention is boring and testable.
- (−) If aggregation query costs explode later, migration to TimescaleDB is required
  (path documented: hypertable conversion on the two time tables; continuous
  aggregates only then, with measured justification).
- (−) No columnar compression — acceptable at MVP storage volumes.

## Migration trigger
Adopt TimescaleDB when: p95 query > 200 ms on partitioned+indexed tables at real load,
OR manual retention jobs become an operational burden, OR storage > 100 GB makes
compression economically meaningful. Load test at 2× projected volume validates the
baseline first (NFR-S01).

## Review date
At the post-launch load test (Phase 4 equivalent) and monthly with volume metrics.
