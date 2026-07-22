-- 20260801000004_create_scheduler
-- Scheduler module tables (docs/data/03-table-design.md §5; ADR-005).
-- Slot rows are the coordination point: FOR UPDATE SKIP LOCKED claims.

CREATE TABLE collection_schedules (
  id                        uuid PRIMARY KEY,
  provider_configuration_id uuid NOT NULL REFERENCES provider_configurations(id),
  job_type                  text NOT NULL,       -- forecast_collection | observation_collection
                                                 -- | analysis_batch | maintenance
  location_id               uuid REFERENCES locations(id),  -- null for global jobs
  slot_time                 timestamptz NOT NULL,
  status                    text NOT NULL DEFAULT 'due'
                            CHECK (status IN ('due','claimed','completed','failed','expired')),
  claimed_by                text,                -- worker instance id
  claimed_at                timestamptz,
  lease_expires_at          timestamptz,
  attempts                  integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  next_retry_at             timestamptz,
  schedule_run_id           uuid,
  created_at                timestamptz NOT NULL DEFAULT now(),
  updated_at                timestamptz NOT NULL DEFAULT now()
);
-- One slot per (config, job_type, location, time): prevents double-generation.
-- Expression (COALESCE) requires a unique INDEX, not a table constraint.
CREATE UNIQUE INDEX collection_schedules_slot_uidx
  ON collection_schedules (provider_configuration_id, job_type,
    COALESCE(location_id, '00000000-0000-0000-0000-000000000000'::uuid), slot_time);
-- Due-slot claim query plan (status, next_retry_at, slot_time).
CREATE INDEX collection_schedules_due_idx
  ON collection_schedules (status, slot_time)
  WHERE status IN ('due','claimed');
CREATE INDEX collection_schedules_config_idx
  ON collection_schedules (provider_configuration_id, slot_time);

CREATE TRIGGER collection_schedules_set_updated_at BEFORE UPDATE ON collection_schedules
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Run history (powers the admin schedules screen; 90-day retention).
CREATE TABLE schedule_runs (
  id               uuid PRIMARY KEY,
  job_type         text NOT NULL,
  slot_id          uuid REFERENCES collection_schedules(id),
  started_at       timestamptz NOT NULL,
  completed_at     timestamptz,
  status           text NOT NULL CHECK (status IN ('running','completed','failed')),
  error_code       text,
  error_message    text,
  duration_ms      integer,
  records_affected integer,
  created_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX schedule_runs_started_idx ON schedule_runs (started_at DESC);
CREATE INDEX schedule_runs_slot_idx ON schedule_runs (slot_id);
