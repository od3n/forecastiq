-- 20260801000006_create_identity
-- Identity module tables (docs/data/03-table-design.md §2; ADR-008 Supabase Auth).
-- The users table stores the Supabase subject mapping + app role — never
-- password hashes (ADR-008). WP-02 deferred these tables (and the audit
-- user_id FK) to this package; see the audit migration header. export_jobs
-- (GDPR) is deferred to WP-19 (its scope owns the export/delete flows).

CREATE TYPE user_role AS ENUM ('user', 'admin');

CREATE TABLE users (
  id                   uuid PRIMARY KEY,
  workspace_id         uuid NOT NULL REFERENCES workspaces(id),
  auth_subject         text NOT NULL UNIQUE,          -- Supabase user id (JWT sub)
  email                text NOT NULL UNIQUE,
  role                 user_role NOT NULL DEFAULT 'user',
  status               entity_status NOT NULL DEFAULT 'active',
  default_location_id  uuid REFERENCES locations(id), -- nullable (reconciliation §2.1)
  preferences          jsonb NOT NULL DEFAULT '{}',
  created_at           timestamptz NOT NULL DEFAULT now(),
  updated_at           timestamptz NOT NULL DEFAULT now(),
  last_login_at        timestamptz
);

CREATE TRIGGER users_set_updated_at BEFORE UPDATE ON users
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE api_keys (
  id                 uuid PRIMARY KEY,
  user_id            uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  workspace_id       uuid NOT NULL REFERENCES workspaces(id),
  name               text NOT NULL,
  key_hash           text NOT NULL,                   -- argon2id; never plaintext
  key_prefix         text NOT NULL UNIQUE,            -- e.g. fiq_abc123 (lookup handle)
  scopes             jsonb NOT NULL DEFAULT '["read:public"]',
  rate_limit_per_min integer NOT NULL DEFAULT 60 CHECK (rate_limit_per_min > 0),
  expires_at         timestamptz,
  created_at         timestamptz NOT NULL DEFAULT now(),
  revoked_at         timestamptz,
  last_used_at       timestamptz
);
-- Revoked keys: revoked_at set; never reactivated (application rule).
CREATE INDEX api_keys_user_idx ON api_keys (user_id);

-- The audit user_id FK was deferred here because it references users(id).
ALTER TABLE audit_events
  ADD CONSTRAINT audit_events_user_fk FOREIGN KEY (user_id) REFERENCES users(id);
