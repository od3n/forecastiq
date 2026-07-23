-- 20260801000008_create_analysis
-- Analysis module: matched_evaluations (docs/data/03-table-design.md §4; WP-11).
-- One immutable row per snapshot–observation pair; a correction adds a NEW pair
-- (never edits), so uniqueness is on (forecast_snapshot_id, observation_id).
-- References to the partitioned forecast_snapshots/observations tables are
-- logical (no enforced composite FK — the §4 DDL declares none; enforcing them
-- would require carrying target_time + observed_at as FK columns). Integrity is
-- maintained by the matching engine (it only inserts ids it just selected).

CREATE TABLE matched_evaluations (
  id                       uuid PRIMARY KEY,
  forecast_snapshot_id     uuid NOT NULL,
  observation_id           uuid NOT NULL,
  provider_id              uuid NOT NULL REFERENCES providers(id),
  location_id              uuid NOT NULL REFERENCES locations(id),
  forecast_horizon_minutes integer NOT NULL,
  target_time              timestamptz NOT NULL,
  match_rule               text NOT NULL CHECK (match_rule IN ('exact_hour','sub_hourly_15min')),
  time_delta_minutes       integer NOT NULL DEFAULT 0,
  computed_at              timestamptz NOT NULL DEFAULT now(),
  UNIQUE (forecast_snapshot_id, observation_id)
);
-- Unmatched-snapshot scan (NOT EXISTS on forecast_snapshot_id) is served by the
-- UNIQUE index prefix. Rematch scans by observation_id; downstream evaluation
-- (WP-12) queries by provider/location/target_time.
CREATE INDEX matched_evaluations_observation_idx ON matched_evaluations (observation_id);
CREATE INDEX matched_evaluations_provider_location_target_idx
  ON matched_evaluations (provider_id, location_id, target_time);

-- Immutability (domain §2.7/§5.2): pairs are never updated or deleted at the
-- application level; a match only ever changes by ADDING a new pair (rematch).
-- Retention purge (2 y) is a future maintenance job run under an exempt role.
CREATE TRIGGER matched_evaluations_immutable BEFORE UPDATE OR DELETE ON matched_evaluations
  FOR EACH ROW EXECUTE FUNCTION raise_immutable();
