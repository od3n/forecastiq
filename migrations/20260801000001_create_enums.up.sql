-- 20260801000001_create_enums
-- Native enum types used by the first-slice tables (docs/data/03-table-design.md).
-- Only the enums required by the slice are created here; enums for future
-- tables (user_role, observation_type, quality_flag, ranking_status) land
-- with the work package that creates those tables.

CREATE TYPE entity_status AS ENUM ('active', 'disabled', 'archived');
CREATE TYPE circuit_state AS ENUM ('closed', 'half_open', 'open');
CREATE TYPE collection_status AS ENUM
  ('pending', 'success', 'partial', 'failed', 'deduplicated', 'rate_limited', 'timeout');

-- Shared trigger: keep updated_at current on mutable tables.
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
  NEW.updated_at := now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Shared trigger: forbid UPDATE/DELETE on fully-immutable tables.
CREATE OR REPLACE FUNCTION raise_immutable() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'table "%" is immutable: % is forbidden', TG_TABLE_NAME, TG_OP;
END;
$$ LANGUAGE plpgsql;
