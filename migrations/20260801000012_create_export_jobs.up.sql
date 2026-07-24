-- 20260801000012_create_export_jobs
-- GDPR export tracking (WP-19c; AUTH-09; docs/data/03-table-design.md §2,
-- reconciliation §2.2). A minimal job record scoped to account-data export only
-- (user row + API-key metadata + own audit events) — not a general report
-- engine. Ownership-bearing (workspace_id, D-08). The export file lives on the
-- payload volume (ADR-019, exports/ key prefix); expiry is 24h.

CREATE TABLE export_jobs (
  id              uuid PRIMARY KEY,
  workspace_id    uuid NOT NULL REFERENCES workspaces(id),
  requested_by    uuid NOT NULL REFERENCES users(id),
  target_user_id  uuid REFERENCES users(id) ON DELETE SET NULL,
  status          text NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending','completed','failed')),
  object_key      text,                            -- scheme-prefixed (ADR-019)
  expires_at      timestamptz,                     -- 24h download validity
  completed_at    timestamptz,
  error_message   text,
  created_at      timestamptz NOT NULL DEFAULT now()
);
-- One active (pending) export per target user (409 guard, D-06).
CREATE UNIQUE INDEX export_jobs_one_active_per_user
  ON export_jobs (target_user_id) WHERE status = 'pending';
