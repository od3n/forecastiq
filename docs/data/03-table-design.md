# ForecastIQ — Table Design (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: `docs/data/02-logical-data-model.md`; domain model §3/§11; reconciliation §2; query requirements doc

DDL below is the migration source of truth (adapted to golang-migrate numbered files at implementation). Conventions: UUIDv7 PKs; `timestamptz` UTC; native enums; indexes defined in `docs/data/04-index-and-query-plan.md` (referenced inline).

---

## 1. Catalog Module

```sql
CREATE TABLE workspaces (
  id            uuid PRIMARY KEY,
  name          text NOT NULL,
  slug          text NOT NULL UNIQUE,
  status        entity_status NOT NULL DEFAULT 'active',
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
-- Seeded: one row (system workspace) at bootstrap.

CREATE TABLE providers (
  id                uuid PRIMARY KEY,
  name              text NOT NULL,
  slug              text NOT NULL UNIQUE,
  api_base_url      text NOT NULL,
  status            entity_status NOT NULL DEFAULT 'active',
  attribution_text  text NOT NULL,               -- BR-ATTR-01
  attribution_url   text NOT NULL,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now()
);
-- Seeded: open-meteo, openweather.

CREATE TABLE provider_configurations (
  id                 uuid PRIMARY KEY,
  workspace_id       uuid NOT NULL REFERENCES workspaces(id),
  provider_id        uuid NOT NULL REFERENCES providers(id),
  status             entity_status NOT NULL DEFAULT 'active',
  credential_ref     text,                        -- env key name; NEVER the secret (BR-08)
  collection_schedule jsonb NOT NULL DEFAULT '{"interval":"hourly","minute_offset":0}',
  adapter_version    text NOT NULL,
  validation_state   text NOT NULL DEFAULT 'unvalidated'
                     CHECK (validation_state IN ('unvalidated','validated','failed')),
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  UNIQUE (workspace_id, provider_id)
);

CREATE TABLE provider_circuits (
  provider_id           uuid PRIMARY KEY REFERENCES providers(id),
  state                 circuit_state NOT NULL DEFAULT 'closed',
  consecutive_failures  integer NOT NULL DEFAULT 0 CHECK (consecutive_failures >= 0),
  last_failure_at       timestamptz,
  opened_at             timestamptz,
  next_probe_at         timestamptz,
  updated_at            timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE locations (
  id            uuid PRIMARY KEY,
  workspace_id  uuid NOT NULL REFERENCES workspaces(id),
  name          text NOT NULL,
  latitude      numeric(9,6) NOT NULL CHECK (latitude BETWEEN -90 AND 90),
  longitude     numeric(9,6) NOT NULL CHECK (longitude BETWEEN -180 AND 180),
  country_code  char(2) NOT NULL,
  timezone      text NOT NULL,                    -- IANA, validated at write
  status        entity_status NOT NULL DEFAULT 'active',
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
-- Coordinates immutable after creation (application rule; not trigger-enforced).
```

## 2. Identity Module

```sql
CREATE TABLE users (
  id                   uuid PRIMARY KEY,
  workspace_id         uuid NOT NULL REFERENCES workspaces(id),
  auth_subject         text NOT NULL UNIQUE,       -- Supabase user id
  email                text NOT NULL UNIQUE,
  role                 user_role NOT NULL DEFAULT 'user',
  status               entity_status NOT NULL DEFAULT 'active',
  default_location_id  uuid REFERENCES locations(id),  -- nullable (reconciliation §2.1)
  preferences          jsonb NOT NULL DEFAULT '{}',
  created_at           timestamptz NOT NULL DEFAULT now(),
  updated_at           timestamptz NOT NULL DEFAULT now(),
  last_login_at        timestamptz
);

CREATE TABLE api_keys (
  id                 uuid PRIMARY KEY,
  user_id            uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  workspace_id       uuid NOT NULL REFERENCES workspaces(id),
  name               text NOT NULL,
  key_hash           text NOT NULL,                -- argon2id; never plaintext
  key_prefix         text NOT NULL UNIQUE,         -- e.g. fiq_abc123
  scopes             jsonb NOT NULL DEFAULT '["read:public"]',
  rate_limit_per_min integer NOT NULL DEFAULT 60 CHECK (rate_limit_per_min > 0),
  expires_at         timestamptz,
  created_at         timestamptz NOT NULL DEFAULT now(),
  revoked_at         timestamptz,
  last_used_at       timestamptz
);
-- Revoked keys: revoked_at set; never reactivated (application rule).

CREATE TABLE export_jobs (
  id              uuid PRIMARY KEY,
  workspace_id    uuid NOT NULL REFERENCES workspaces(id),
  requested_by    uuid NOT NULL REFERENCES users(id),
  target_user_id  uuid REFERENCES users(id) ON DELETE SET NULL,
  status          text NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending','completed','failed')),
  object_key      text,                            -- file on payload volume
  expires_at      timestamptz,                     -- 24h download validity
  completed_at    timestamptz,
  error_message   text,
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX export_jobs_one_active_per_user
  ON export_jobs (target_user_id) WHERE status = 'pending';  -- 409 guard (D-06)
```

## 3. Collection Module

```sql
CREATE TABLE forecast_collections (
  id                        uuid PRIMARY KEY,
  provider_id               uuid NOT NULL REFERENCES providers(id),
  location_id               uuid NOT NULL REFERENCES locations(id),
  provider_configuration_id uuid NOT NULL REFERENCES provider_configurations(id),
  requested_at              timestamptz NOT NULL,
  completed_at              timestamptz,
  collection_status         collection_status NOT NULL DEFAULT 'pending',
  provider_request_id       text,
  provider_model_run_time   timestamptz,
  raw_payload_object_key    text,                  -- scheme-prefixed (ADR-011)
  raw_payload_checksum      text,                  -- SHA-256 hex
  response_status_code      integer,
  response_latency_ms       integer,
  records_received          integer NOT NULL DEFAULT 0,
  snapshots_stored          integer NOT NULL DEFAULT 0,
  snapshots_deduplicated    integer NOT NULL DEFAULT 0,
  snapshots_invalid         integer NOT NULL DEFAULT 0,
  schema_version            text,
  adapter_version           text,
  error_code                text,                  -- classified (FC-13)
  error_message             text,                  -- truncated, first N reasons
  created_at                timestamptz NOT NULL DEFAULT now(),
  CHECK (collection_status <> 'pending' OR completed_at IS NULL),
  CHECK (completed_at IS NULL OR completed_at >= requested_at)
);
-- Dedup key (collection-level): unique on success for same model run
CREATE UNIQUE INDEX forecast_collections_dedup
  ON forecast_collections (provider_id, location_id, COALESCE(provider_model_run_time, requested_at))
  WHERE collection_status IN ('success','partial');

CREATE TABLE forecast_snapshots (
  id                          uuid NOT NULL,
  forecast_collection_id      uuid NOT NULL,
  provider_id                 uuid NOT NULL,
  location_id                 uuid NOT NULL,
  issued_at                   timestamptz NOT NULL,
  target_time                 timestamptz NOT NULL,
  forecast_horizon_minutes    integer NOT NULL,
  temperature_c               numeric(5,2),
  feels_like_temperature_c    numeric(5,2),
  precipitation_probability   numeric(5,4) CHECK (precipitation_probability BETWEEN 0 AND 1),
  precipitation_amount_mm     numeric(7,2) CHECK (precipitation_amount_mm >= 0),
  humidity_pct                numeric(5,2) CHECK (humidity_pct BETWEEN 0 AND 100),
  wind_speed_ms               numeric(6,2) CHECK (wind_speed_ms >= 0),
  wind_direction_deg          numeric(5,1) CHECK (wind_direction_deg BETWEEN 0 AND 360),
  pressure_hpa                numeric(7,2),
  cloud_cover_pct             numeric(5,2) CHECK (cloud_cover_pct BETWEEN 0 AND 100),
  provider_condition_code     text,
  canonical_condition_code    text,
  condition_taxonomy_version  text,
  created_at                  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (id, target_time),                   -- partition key must be in PK
  CHECK (target_time > issued_at)
) PARTITION BY RANGE (target_time);
-- FKs declared per-partition inheritance: parent-level FKs supported in PG16
-- (provider_id, location_id reference catalog; forecast_collection_id → collections)
CREATE UNIQUE INDEX snapshots_dedup ON forecast_snapshots
  (provider_id, location_id, issued_at, target_time);
-- Monthly partitions created 3 months ahead by maintenance job.

CREATE TABLE observations (
  id                        uuid NOT NULL,
  location_id               uuid NOT NULL,
  source                    text NOT NULL,          -- e.g. openmeteo_historical
  observation_type          observation_type NOT NULL,
  observed_at               timestamptz NOT NULL,
  temperature_c             numeric(5,2),
  humidity_pct              numeric(5,2),
  wind_speed_ms             numeric(6,2),
  wind_direction_deg        numeric(5,1),
  pressure_hpa              numeric(7,2),
  precipitation_mm          numeric(7,2),
  provider_condition_code   text,
  canonical_condition_code  text,
  quality_flag              quality_flag NOT NULL DEFAULT 'valid',
  superseded_observation_id uuid,                   -- set on OLD row at correction
  created_at                timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (id, observed_at),
  CHECK (observed_at <= now())
) PARTITION BY RANGE (observed_at);
-- Live-row dedup (OC-03): one NON-SUPERSEDED row per (source, location, hour).
-- The predicate is required so a correction (a new row sharing the same key,
-- with the old row marked superseded) does not violate uniqueness (DR-05;
-- WP-10 migration 20260801000007).
CREATE UNIQUE INDEX observations_dedup ON observations (source, location_id, observed_at)
  WHERE superseded_observation_id IS NULL;
```

## 4. Analysis Module

```sql
CREATE TABLE matched_evaluations (
  id                       uuid PRIMARY KEY,
  forecast_snapshot_id     uuid NOT NULL,
  observation_id           uuid NOT NULL,
  provider_id              uuid NOT NULL,
  location_id              uuid NOT NULL,
  forecast_horizon_minutes integer NOT NULL,
  target_time              timestamptz NOT NULL,
  match_rule               text NOT NULL CHECK (match_rule IN ('exact_hour','sub_hourly_15min')),
  time_delta_minutes       integer NOT NULL DEFAULT 0,
  computed_at              timestamptz NOT NULL DEFAULT now(),
  UNIQUE (forecast_snapshot_id, observation_id)
);
-- FKs: snapshot (logical — partitioned PK requires (id, target_time); enforced via
-- composite reference: (forecast_snapshot_id, target_time) → snapshots(id, target_time))
-- observation_id → observations(id, observed_at) composite likewise.

CREATE TABLE accuracy_metrics (
  id                 uuid PRIMARY KEY,
  provider_id        uuid NOT NULL REFERENCES providers(id),
  location_id        uuid NOT NULL REFERENCES locations(id),
  horizon_minutes    integer NOT NULL,
  variable           text NOT NULL,        -- temperature, precipitation, wind_speed, ...
  metric_type        text NOT NULL,        -- mae, rmse, bias, recall, precision, f1, far,
                                           -- threat_score, brier, coverage, reliability, ...
  value              double precision,     -- NULL ⇔ sample_count = 0
  ci_lower           double precision,
  ci_upper           double precision,
  sample_count       integer NOT NULL DEFAULT 0 CHECK (sample_count >= 0),
  methodology_version text NOT NULL,
  period_start       timestamptz NOT NULL,
  period_end         timestamptz NOT NULL,
  calculated_at      timestamptz NOT NULL DEFAULT now(),
  superseded_by      uuid REFERENCES accuracy_metrics(id),
  CHECK (period_start < period_end),
  CHECK ((value IS NULL) = (sample_count = 0)),
  CHECK (ci_lower IS NULL OR (ci_lower <= value AND value <= ci_upper))
);

CREATE TABLE provider_rankings (
  id                  uuid PRIMARY KEY,
  provider_id         uuid NOT NULL REFERENCES providers(id),
  location_id         uuid NOT NULL REFERENCES locations(id),
  horizon_minutes     integer NOT NULL,
  composite_score     double precision,    -- NULL ⇔ unranked
  ci_lower            double precision,
  ci_upper            double precision,
  ranking_status      ranking_status NOT NULL,
  sample_count        integer NOT NULL DEFAULT 0,
  coverage            double precision CHECK (coverage BETWEEN 0 AND 1),
  reliability         double precision CHECK (reliability BETWEEN 0 AND 1),
  component_scores    jsonb NOT NULL,
  methodology_version text NOT NULL,
  weights_version     text NOT NULL,
  horizon_profile     text NOT NULL DEFAULT 'uniform',
  period_start        timestamptz NOT NULL,
  period_end          timestamptz NOT NULL,
  calculated_at       timestamptz NOT NULL DEFAULT now(),
  superseded_by       uuid REFERENCES provider_rankings(id),
  CHECK ((composite_score IS NULL) = (ranking_status = 'unranked')),
  CHECK (composite_score IS NULL OR composite_score BETWEEN 0 AND 1)
);
```

## 5. Scheduler Module

```sql
CREATE TABLE collection_schedules (
  id                        uuid PRIMARY KEY,
  provider_configuration_id uuid NOT NULL REFERENCES provider_configurations(id),
  job_type                  text NOT NULL,   -- forecast_collection | observation_collection
                                             -- | analysis_batch | maintenance
  location_id               uuid REFERENCES locations(id),  -- null for global jobs
  slot_time                 timestamptz NOT NULL,
  status                    text NOT NULL DEFAULT 'due'
                            CHECK (status IN ('due','claimed','completed','failed','expired')),
  claimed_by                text,             -- instance id
  claimed_at                timestamptz,
  lease_expires_at          timestamptz,
  attempts                  integer NOT NULL DEFAULT 0,
  next_retry_at             timestamptz,
  schedule_run_id           uuid,
  UNIQUE (provider_configuration_id, job_type, COALESCE(location_id, '00000000-0000-0000-0000-000000000000'::uuid), slot_time)
);

CREATE TABLE schedule_runs (
  id            uuid PRIMARY KEY,
  job_type      text NOT NULL,
  slot_id       uuid REFERENCES collection_schedules(id),
  started_at    timestamptz NOT NULL,
  completed_at  timestamptz,
  status        text NOT NULL CHECK (status IN ('running','completed','failed')),
  error_code    text,
  error_message text,
  duration_ms   integer,
  records_affected integer,
  created_at    timestamptz NOT NULL DEFAULT now()
);
```

## 6. Audit Module

```sql
CREATE TABLE audit_events (
  id            uuid PRIMARY KEY,
  user_id       uuid REFERENCES users(id) ON DELETE SET NULL,
  action        text NOT NULL,              -- registry: auth.*, location.*, provider.*, etc.
  resource_type text NOT NULL,
  resource_id   uuid,
  details       jsonb NOT NULL DEFAULT '{}',
  ip_address    inet,
  created_at    timestamptz NOT NULL DEFAULT now()
);
```

## 7. Design Notes

| Decision | Specification |
|----------|---------------|
| Partitioned PK | `(id, target_time)` / `(id, observed_at)` — PostgreSQL requires partition key in PK; UUIDv7 id remains the logical identity |
| Composite FKs to partitioned tables | matched_evaluations references snapshots via `(forecast_snapshot_id, target_time)` composite FK (PG11+ supports FKs to partitioned tables) |
| Decimal vs double | Weather values: `numeric` (exact, provider-precision); metric/ranking values: `double precision` (statistical computation; full precision stored, API rounds per methodology §5) |
| Expected growth | See `docs/data/06-data-growth-and-cost-model.md` (snapshots ~11M rows/y; observations ~90K/y) |
| Indexes | All in `docs/data/04-index-and-query-plan.md` (created CONCURRENTLY where table is live) |

## 8. Cross-Reference

- Logical model: `docs/data/02-logical-data-model.md`
- Query/index plan: `docs/data/04-index-and-query-plan.md`
- Retention mechanics: `docs/data/05-retention-and-archival.md`
