-- +goose Up
-- Transactional outbox (ADR-0004). Written by business transactions under RLS;
-- drained across tenants by the relay running as hugi_system.
CREATE TABLE outbox (
  id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  org_id      uuid        NOT NULL,
  subject     text        NOT NULL CHECK (subject ~ '^hugi\.[a-z0-9_.-]+$'),
  payload     jsonb       NOT NULL,
  headers     jsonb       NOT NULL DEFAULT '{}'::jsonb,
  created_at  timestamptz NOT NULL DEFAULT now(),
  sent_at     timestamptz,
  attempts    integer     NOT NULL DEFAULT 0,
  last_error  text
);
CREATE INDEX outbox_pending_idx ON outbox (id) WHERE sent_at IS NULL;

ALTER TABLE outbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE outbox FORCE ROW LEVEL SECURITY;

CREATE POLICY outbox_tenant ON outbox TO hugi_app
  USING (org_id = hugi_current_org())
  WITH CHECK (org_id = hugi_current_org());
CREATE POLICY outbox_relay ON outbox TO hugi_system
  USING (true) WITH CHECK (true);

GRANT INSERT ON outbox TO hugi_app;
GRANT SELECT ON outbox TO hugi_app;  -- tenant-scoped by policy; used for diagnostics
GRANT SELECT, UPDATE (sent_at, attempts, last_error), DELETE ON outbox TO hugi_system;

-- +goose Down
DROP TABLE IF EXISTS outbox;
