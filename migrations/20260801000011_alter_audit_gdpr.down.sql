-- Revert 20260801000011_alter_audit_gdpr.

ALTER TABLE audit_events DROP CONSTRAINT audit_events_user_fk;
ALTER TABLE audit_events
  ADD CONSTRAINT audit_events_user_fk FOREIGN KEY (user_id) REFERENCES users(id);

CREATE OR REPLACE FUNCTION raise_immutable() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'table "%" is immutable: % is forbidden', TG_TABLE_NAME, TG_OP;
END;
$$ LANGUAGE plpgsql;
