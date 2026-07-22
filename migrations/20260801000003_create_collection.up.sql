-- 20260801000003_create_collection
-- Collection module tables (docs/data/03-table-design.md §3).
-- Ownership: collection module. forecast_snapshots is monthly-partitioned
-- by target_time (standard declarative partitioning; ADR-004/029).

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
  raw_payload_object_key    text,                -- scheme-prefixed (ADR-011)
  raw_payload_checksum      text,                -- SHA-256 hex
  response_status_code      integer,
  response_latency_ms       integer,
  records_received          integer NOT NULL DEFAULT 0,
  snapshots_stored          integer NOT NULL DEFAULT 0,
  snapshots_deduplicated    integer NOT NULL DEFAULT 0,
  snapshots_invalid         integer NOT NULL DEFAULT 0,
  schema_version            text,
  adapter_version           text,
  error_code                text,                -- classified (FC-13)
  error_message             text,                -- truncated reasons
  created_at                timestamptz NOT NULL DEFAULT now(),
  CHECK (collection_status <> 'pending' OR completed_at IS NULL),
  CHECK (completed_at IS NULL OR completed_at >= requested_at),
  CHECK (records_received = snapshots_stored + snapshots_deduplicated + snapshots_invalid
         OR collection_status NOT IN ('success','partial'))
);
CREATE INDEX forecast_collections_query_idx
  ON forecast_collections (provider_id, location_id, requested_at DESC);
-- Collection-level dedup: one successful/partial collection per model run
-- (or per requested_at when model run time is unavailable). Domain §4.3.
CREATE UNIQUE INDEX forecast_collections_dedup
  ON forecast_collections (provider_id, location_id, COALESCE(provider_model_run_time, requested_at))
  WHERE collection_status IN ('success','partial');

-- forecast_snapshots: immutable children, monthly partitions on target_time.
CREATE TABLE forecast_snapshots (
  id                         uuid NOT NULL,
  forecast_collection_id     uuid NOT NULL,
  provider_id                uuid NOT NULL,
  location_id                uuid NOT NULL,
  issued_at                  timestamptz NOT NULL,
  target_time                timestamptz NOT NULL,
  forecast_horizon_minutes   integer NOT NULL,
  temperature_c              numeric(5,2),
  feels_like_temperature_c   numeric(5,2),
  precipitation_probability  numeric(5,4) CHECK (precipitation_probability BETWEEN 0 AND 1),
  precipitation_amount_mm    numeric(7,2) CHECK (precipitation_amount_mm >= 0),
  humidity_pct               numeric(5,2) CHECK (humidity_pct BETWEEN 0 AND 100),
  wind_speed_ms              numeric(6,2) CHECK (wind_speed_ms >= 0),
  wind_direction_deg         numeric(5,1) CHECK (wind_direction_deg BETWEEN 0 AND 360),
  pressure_hpa               numeric(7,2),
  cloud_cover_pct            numeric(5,2) CHECK (cloud_cover_pct BETWEEN 0 AND 100),
  provider_condition_code    text,
  canonical_condition_code   text,
  condition_taxonomy_version text,
  created_at                 timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (id, target_time),                 -- partition key must be in PK
  CHECK (target_time > issued_at),
  CONSTRAINT fk_snapshot_collection FOREIGN KEY (forecast_collection_id)
    REFERENCES forecast_collections(id),
  CONSTRAINT fk_snapshot_provider FOREIGN KEY (provider_id) REFERENCES providers(id),
  CONSTRAINT fk_snapshot_location FOREIGN KEY (location_id) REFERENCES locations(id)
) PARTITION BY RANGE (target_time);

-- Snapshot dedup boundary (domain §4.3); includes the partition key.
CREATE UNIQUE INDEX snapshots_dedup
  ON forecast_snapshots (provider_id, location_id, issued_at, target_time);
-- Query indexes (docs/data/04-index-and-query-plan.md).
CREATE INDEX snapshots_provider_location_target_idx
  ON forecast_snapshots (provider_id, location_id, target_time);
CREATE INDEX snapshots_location_target_horizon_idx
  ON forecast_snapshots (location_id, target_time, forecast_horizon_minutes);

-- Immutability: snapshots are never updated/deleted; collections are
-- immutable once they reach a terminal status and never deleted.
CREATE TRIGGER forecast_snapshots_immutable BEFORE UPDATE OR DELETE ON forecast_snapshots
  FOR EACH ROW EXECUTE FUNCTION raise_immutable();

CREATE OR REPLACE FUNCTION forecast_collections_immutable() RETURNS trigger AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'forecast_collections rows are never deleted';
  END IF;
  IF OLD.collection_status <> 'pending' THEN
    RAISE EXCEPTION 'forecast_collections is immutable after completion (status=%)',
      OLD.collection_status;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER forecast_collections_immutable_trg BEFORE UPDATE OR DELETE ON forecast_collections
  FOR EACH ROW EXECUTE FUNCTION forecast_collections_immutable();

-- Monthly partition helper (idempotent). Used here for the initial window
-- and at runtime by the application before inserts (EnsurePartition).
CREATE OR REPLACE FUNCTION create_monthly_partition(tbl regclass, month_start date)
RETURNS void AS $$
DECLARE
  part_name  text;
  next_month date;
BEGIN
  part_name  := format('%s_y%s_m%s', tbl, to_char(month_start, 'YYYY'), to_char(month_start, 'MM'));
  next_month := (month_start + interval '1 month')::date;
  IF NOT EXISTS (SELECT 1 FROM pg_class WHERE relname = part_name) THEN
    EXECUTE format('CREATE TABLE %I PARTITION OF %s FOR VALUES FROM (%L) TO (%L)',
      part_name, tbl, month_start, next_month);
  END IF;
END;
$$ LANGUAGE plpgsql;

-- Initial partitions: current month + next 3 months (WP-02 convention).
DO $$
DECLARE
  base date := date_trunc('month', now())::date;
BEGIN
  PERFORM create_monthly_partition('forecast_snapshots', (base + (i || ' month')::interval)::date)
  FROM generate_series(0, 3) AS i;
END $$;
