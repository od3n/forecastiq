-- 20260801000011_alter_audit_gdpr
-- WP-19b account deletion (AUTH-09): deleting a user must anonymize their audit
-- rows (user_id -> NULL) rather than block the delete (audit-requirements §4:
-- "user_id ON DELETE SET NULL; event preserved, actor anonymized"). Two changes:
--   1. raise_immutable() gains a session-GUC exemption (app.allow_immutable_write
--      = 'on'). This is the documented "retention purge GUC" mechanism and is
--      what lets the FK-driven SET NULL below run against the immutable
--      audit_events table. Every other UPDATE/DELETE stays forbidden unless the
--      transaction explicitly opts in via `SET LOCAL app.allow_immutable_write`.
--   2. audit_events.user_id FK is re-created ON DELETE SET NULL.

CREATE OR REPLACE FUNCTION raise_immutable() RETURNS trigger AS $$
BEGIN
  IF current_setting('app.allow_immutable_write', true) = 'on' THEN
    RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
  END IF;
  RAISE EXCEPTION 'table "%" is immutable: % is forbidden', TG_TABLE_NAME, TG_OP;
END;
$$ LANGUAGE plpgsql;

ALTER TABLE audit_events DROP CONSTRAINT audit_events_user_fk;
ALTER TABLE audit_events
  ADD CONSTRAINT audit_events_user_fk FOREIGN KEY (user_id)
  REFERENCES users(id) ON DELETE SET NULL;
