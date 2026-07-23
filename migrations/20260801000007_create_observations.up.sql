-- 20260801000007_create_observations
-- Observation collection tables (docs/data/03-table-design.md §3; WP-10).
-- Ownership: collection module. observations is monthly-partitioned by
-- observed_at. Enums observation_type / quality_flag land here (deferred by
-- migration 20260801000001) with the table that first needs them.
--
-- DR-05: the table-design §3 DDL declared observations_dedup as a plain unique
-- index on (source, location_id, observed_at). That is incompatible with the
-- correction model (workflow §4 / domain §2.7): a correction inserts a NEW row
-- with the SAME (source, location_id, observed_at) and marks the old row
-- superseded. The dedup index must therefore be PARTIAL on non-superseded rows
-- (WHERE superseded_observation_id IS NULL) so exactly one live row exists per
-- (source, location, hour) while superseded history is retained.

CREATE TYPE observation_type AS ENUM
  ('station_observation', 'interpolated', 'reanalysis', 'provider_estimated');
CREATE TYPE quality_flag AS ENUM ('valid', 'suspect', 'corrected');

CREATE TABLE observations (
  id                        uuid NOT NULL,
  location_id               uuid NOT NULL,
  source                    text NOT NULL,          -- e.g. openmeteo_historical
  observation_type          observation_type NOT NULL,
  observed_at               timestamptz NOT NULL,
  temperature_c             numeric(5,2),
  humidity_pct              numeric(5,2) CHECK (humidity_pct BETWEEN 0 AND 100),
  wind_speed_ms             numeric(6,2) CHECK (wind_speed_ms >= 0),
  wind_direction_deg        numeric(5,1) CHECK (wind_direction_deg BETWEEN 0 AND 360),
  pressure_hpa              numeric(7,2),
  precipitation_mm          numeric(7,2) CHECK (precipitation_mm >= 0),
  provider_condition_code   text,
  canonical_condition_code  text,
  quality_flag              quality_flag NOT NULL DEFAULT 'valid',
  superseded_observation_id uuid,                   -- set on the OLD row at correction
  created_at                timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (id, observed_at),                     -- partition key must be in PK
  CONSTRAINT fk_observation_location FOREIGN KEY (location_id) REFERENCES locations(id)
) PARTITION BY RANGE (observed_at);

-- Live-row dedup boundary (OC-03): one non-superseded row per
-- (source, location, hour). Superseded rows are excluded so corrections can
-- coexist with the row they replace (DR-05).
CREATE UNIQUE INDEX observations_dedup
  ON observations (source, location_id, observed_at)
  WHERE superseded_observation_id IS NULL;
-- Freshness (MAX(observed_at) per location) + matching lookup.
CREATE INDEX observations_location_observed_idx
  ON observations (location_id, observed_at DESC);
CREATE INDEX observations_source_location_observed_idx
  ON observations (source, location_id, observed_at);

-- Immutability (domain §2.7): weather values, provenance, and timestamps are
-- immutable; the ONLY permitted mutation is setting superseded_observation_id
-- (NULL → the correcting row's id). Rows are never deleted (partition drop at
-- 5 y handles removal).
CREATE OR REPLACE FUNCTION observations_supersede_only() RETURNS trigger AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'observations rows are never deleted';
  END IF;
  IF OLD.superseded_observation_id IS NOT NULL THEN
    RAISE EXCEPTION 'observation % is already superseded (immutable)', OLD.id;
  END IF;
  IF NEW.id IS DISTINCT FROM OLD.id
     OR NEW.location_id IS DISTINCT FROM OLD.location_id
     OR NEW.source IS DISTINCT FROM OLD.source
     OR NEW.observation_type IS DISTINCT FROM OLD.observation_type
     OR NEW.observed_at IS DISTINCT FROM OLD.observed_at
     OR NEW.temperature_c IS DISTINCT FROM OLD.temperature_c
     OR NEW.humidity_pct IS DISTINCT FROM OLD.humidity_pct
     OR NEW.wind_speed_ms IS DISTINCT FROM OLD.wind_speed_ms
     OR NEW.wind_direction_deg IS DISTINCT FROM OLD.wind_direction_deg
     OR NEW.pressure_hpa IS DISTINCT FROM OLD.pressure_hpa
     OR NEW.precipitation_mm IS DISTINCT FROM OLD.precipitation_mm
     OR NEW.provider_condition_code IS DISTINCT FROM OLD.provider_condition_code
     OR NEW.canonical_condition_code IS DISTINCT FROM OLD.canonical_condition_code
     OR NEW.quality_flag IS DISTINCT FROM OLD.quality_flag
     OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
    RAISE EXCEPTION 'observations is immutable except superseded_observation_id';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER observations_supersede_only_trg BEFORE UPDATE OR DELETE ON observations
  FOR EACH ROW EXECUTE FUNCTION observations_supersede_only();

-- Initial partitions: current month + next 3 months (WP-02 convention; reuses
-- the create_monthly_partition helper from 20260801000003).
DO $$
DECLARE
  base date := date_trunc('month', now())::date;
BEGIN
  PERFORM create_monthly_partition('observations', (base + (i || ' month')::interval)::date)
  FROM generate_series(0, 3) AS i;
END $$;
