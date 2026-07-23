-- 20260801000006_create_identity (down)
-- Reverse the identity migration. Drop the audit FK first (it depends on
-- users), then api_keys and users, then the enum.

ALTER TABLE audit_events DROP CONSTRAINT IF EXISTS audit_events_user_fk;

DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS users;

DROP TYPE IF EXISTS user_role;
