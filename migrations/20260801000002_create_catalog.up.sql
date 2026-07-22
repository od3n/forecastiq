-- 20260801000002_create_catalog
-- Catalog module tables (docs/data/03-table-design.md §1).
-- Ownership: catalog module. UUIDv7 PKs generated in the application.

CREATE TABLE workspaces (
  id         uuid PRIMARY KEY,
  name       text NOT NULL,
  slug       text NOT NULL UNIQUE,
  status     entity_status NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE providers (
  id               uuid PRIMARY KEY,
  name             text NOT NULL,
  slug             text NOT NULL UNIQUE,
  api_base_url     text NOT NULL,
  status           entity_status NOT NULL DEFAULT 'active',
  attribution_text text NOT NULL,               -- BR-ATTR-01
  attribution_url  text NOT NULL,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE provider_configurations (
  id                  uuid PRIMARY KEY,
  workspace_id        uuid NOT NULL REFERENCES workspaces(id),
  provider_id         uuid NOT NULL REFERENCES providers(id),
  status              entity_status NOT NULL DEFAULT 'active',
  credential_ref      text,                      -- env key name; NEVER the secret (BR-08)
  collection_schedule jsonb NOT NULL DEFAULT '{"interval":"hourly","minute_offset":0}',
  adapter_version     text NOT NULL,
  validation_state    text NOT NULL DEFAULT 'unvalidated'
                      CHECK (validation_state IN ('unvalidated','validated','failed')),
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),
  UNIQUE (workspace_id, provider_id)
);
CREATE INDEX provider_configurations_provider_idx ON provider_configurations (provider_id);

-- Circuit breaker state per provider (persistent; FC-09).
CREATE TABLE provider_circuits (
  provider_id          uuid PRIMARY KEY REFERENCES providers(id),
  state                circuit_state NOT NULL DEFAULT 'closed',
  consecutive_failures integer NOT NULL DEFAULT 0 CHECK (consecutive_failures >= 0),
  last_failure_at      timestamptz,
  opened_at            timestamptz,
  next_probe_at        timestamptz,
  updated_at           timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE locations (
  id           uuid PRIMARY KEY,
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  name         text NOT NULL,
  latitude     numeric(9,6) NOT NULL CHECK (latitude BETWEEN -90 AND 90),
  longitude    numeric(9,6) NOT NULL CHECK (longitude BETWEEN -180 AND 180),
  country_code char(2) NOT NULL,
  timezone     text NOT NULL,                    -- IANA, validated at write
  status       entity_status NOT NULL DEFAULT 'active',
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX locations_status_idx ON locations (status);
-- Active-location name uniqueness (BR: (workspace_id, name) unique among active).
CREATE UNIQUE INDEX locations_active_name_uidx
  ON locations (workspace_id, name) WHERE status = 'active';

-- updated_at maintenance on mutable catalog tables.
CREATE TRIGGER workspaces_set_updated_at BEFORE UPDATE ON workspaces
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER providers_set_updated_at BEFORE UPDATE ON providers
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER provider_configurations_set_updated_at BEFORE UPDATE ON provider_configurations
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER provider_circuits_set_updated_at BEFORE UPDATE ON provider_circuits
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER locations_set_updated_at BEFORE UPDATE ON locations
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
