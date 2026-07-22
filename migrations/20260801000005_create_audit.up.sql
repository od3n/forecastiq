-- 20260801000005_create_audit
-- Audit module table (docs/data/03-table-design.md §6). Append-only and
-- immutable. The user_id foreign key to the users table is added by the
-- identity work package (WP-03) that creates users; until then user_id is a
-- plain nullable UUID and the actor is recorded in details.

CREATE TABLE audit_events (
  id            uuid PRIMARY KEY,
  user_id       uuid,                            -- FK to users(id) added in WP-03
  action        text NOT NULL,                   -- registry: location.*, collection.*, ...
  resource_type text NOT NULL,
  resource_id   uuid,
  details       jsonb NOT NULL DEFAULT '{}',
  ip_address    inet,
  created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_events_action_idx ON audit_events (action, created_at DESC);
CREATE INDEX audit_events_resource_idx ON audit_events (resource_type, resource_id);

CREATE TRIGGER audit_events_immutable BEFORE UPDATE OR DELETE ON audit_events
  FOR EACH ROW EXECUTE FUNCTION raise_immutable();
