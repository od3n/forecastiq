# Migration Guide

Schema migrations live in `migrations/` as `NNNN_description.{up,down}.sql`
(timestamp-prefixed to avoid merge collisions) and are **embedded in the binary**
(`migrations/migrations.go`), applied with golang-migrate.

## Commands

```bash
make migrate               # apply all pending migrations (forecastiq migrate up)
make migrate-down          # roll back the most recent migration
forecastiq migrate down 3  # roll back 3 migrations
forecastiq migrate status  # print current version + dirty flag
forecastiq migrate force N # clear a dirty state at version N (escape hatch)
```

The app can also auto-migrate on boot with `FIQ_AUTO_MIGRATE=true` (used by
`make dev-up`).

## Current migrations (first slice)

| Version | Name | Contents |
|---------|------|----------|
| 20260801000001 | create_enums | `entity_status`, `circuit_state`, `collection_status` enums; `set_updated_at` + `raise_immutable` trigger functions |
| 20260801000002 | create_catalog | workspaces, providers, provider_configurations, provider_circuits, locations + `updated_at` triggers |
| 20260801000003 | create_collection | forecast_collections (+ dedup partial index), forecast_snapshots (monthly partitions + immutability trigger + dedup index) |
| 20260801000004 | create_scheduler | collection_schedules (slot uniqueness index), schedule_runs |
| 20260801000005 | create_audit | audit_events (+ immutability trigger) |

Seed data (system workspace, providers, Open-Meteo config, Johor Bahru) is applied
by `forecastiq seed` / `FIQ_AUTO_SEED=true` — **not** a migration — and is idempotent.

## Writing a migration

1. Create the next timestamp-prefixed pair: `migrations/<YYYYMMDDHHMMSS>_name.up.sql`
   and `.down.sql`.
2. Keep it **reversible** where possible (the down migration restores the prior state).
3. Use `CREATE INDEX CONCURRENTLY` only outside transactions for live tables; the
   slice's tables are created fresh, so plain DDL is fine.
4. Every PR touching schema lists the migration numbers in its description.

## Conventions

- UUIDv7 primary keys (generated in the application; ADR-022).
- `timestamptz` stored UTC; native enums for closed sets.
- Pipeline tables are immutable: `BEFORE UPDATE OR DELETE` triggers raise.
  `forecast_collections` is mutable only while `pending`.
- `forecast_snapshots` is `PARTITION BY RANGE (target_time)`; partitions are created
  3 months ahead at migration time and on demand at insert time
  (`create_monthly_partition`).
- Expression uniqueness (e.g. `COALESCE(location_id, …)`) requires a unique
  **index**, not a table constraint.

## Production posture

Migrations are forward-only in production (expand-contract for breaking changes);
`down` migrations are a development convenience. A migration dry-run runs against a
prod-schema copy before deploy (CI/CD doc §2).
