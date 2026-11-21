-- +goose Up
-- S10 workflow engine core (docs/architecture/workflow-runtime.md).
-- Control plane (hugi_app, tenant RLS): workflows, versions, deployments, triggers, execution start.
-- Data plane (hugi_system, engine policy): step claim/lease/complete, reaper, outbox dispatch.

CREATE TABLE workflows (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id       uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
  workspace_id uuid NOT NULL,
  name         text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 100),
  slug         citext NOT NULL CHECK (slug::text ~ '^[a-z0-9][a-z0-9-]{0,62}$'),
  description  text NOT NULL DEFAULT '' CHECK (length(description) <= 500),
  draft        jsonb NOT NULL DEFAULT '{"schema_version":1,"trigger":{"type":"trigger.manual"},"nodes":[],"edges":[]}'::jsonb,
  status       text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'paused', 'archived')),
  created_by   uuid,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, id),
  UNIQUE (org_id, workspace_id, slug),
  FOREIGN KEY (org_id, workspace_id) REFERENCES workspaces (org_id, id) ON DELETE CASCADE
);
CREATE TRIGGER workflows_touch BEFORE UPDATE ON workflows
  FOR EACH ROW EXECUTE FUNCTION hugi_touch_updated_at();

-- Immutable once written: the definition an execution runs is pinned by version id.
CREATE TABLE workflow_versions (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id       uuid NOT NULL,
  workflow_id  uuid NOT NULL,
  version      integer NOT NULL CHECK (version >= 1),
  definition   jsonb NOT NULL,
  checksum     text NOT NULL CHECK (checksum ~ '^[a-f0-9]{64}$'),
  published_by uuid NOT NULL,
  published_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, workflow_id, version),
  UNIQUE (org_id, id),
  FOREIGN KEY (org_id, workflow_id) REFERENCES workflows (org_id, id) ON DELETE CASCADE
);

CREATE TABLE workflow_deployments (
  org_id         uuid NOT NULL,
  workflow_id    uuid NOT NULL,
  environment_id uuid NOT NULL,
  version_id     uuid NOT NULL,
  enabled        boolean NOT NULL DEFAULT true,
  updated_by     uuid,
  updated_at     timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, workflow_id, environment_id),
  FOREIGN KEY (org_id, workflow_id) REFERENCES workflows (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, version_id) REFERENCES workflow_versions (org_id, id) ON DELETE RESTRICT,
  FOREIGN KEY (org_id, environment_id) REFERENCES environments (org_id, id) ON DELETE RESTRICT
);

-- One row per deployed trigger. S10 writes manual triggers; S11 adds schedule
-- (next_fire_at), webhook and event triggers.
CREATE TABLE workflow_triggers (
  id             uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id         uuid NOT NULL,
  workflow_id    uuid NOT NULL,
  environment_id uuid NOT NULL,
  type           text NOT NULL CHECK (type ~ '^trigger\.[a-z_]+$'),
  config         jsonb NOT NULL DEFAULT '{}'::jsonb,
  next_fire_at   timestamptz,
  created_at     timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, workflow_id, environment_id),
  FOREIGN KEY (org_id, workflow_id, environment_id)
    REFERENCES workflow_deployments (org_id, workflow_id, environment_id) ON DELETE CASCADE
);

CREATE TABLE workflow_executions (
  id               uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id           uuid NOT NULL,
  workspace_id     uuid NOT NULL,
  workflow_id      uuid NOT NULL,
  version_id       uuid NOT NULL,
  environment_id   uuid NOT NULL,
  trigger_type     text NOT NULL CHECK (trigger_type ~ '^trigger\.[a-z_]+$'),
  trigger_ref      text NOT NULL DEFAULT '' CHECK (length(trigger_ref) <= 200),
  idempotency_key  text CHECK (length(idempotency_key) BETWEEN 1 AND 200),
  status           text NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending', 'running', 'succeeded', 'failed', 'cancelled', 'timed_out')),
  input            jsonb NOT NULL DEFAULT '{}'::jsonb,
  output           jsonb,
  error            jsonb,
  -- Snapshot of the environment's egress allowlist at start: the engine never
  -- reads control-plane tables, and a running execution is not affected by edits.
  egress_allowlist jsonb NOT NULL DEFAULT '[]'::jsonb,
  step_count       integer NOT NULL DEFAULT 0 CHECK (step_count >= 0),
  max_steps        integer NOT NULL CHECK (max_steps BETWEEN 1 AND 5000),
  created_by       uuid,
  request_id       text NOT NULL DEFAULT '' CHECK (length(request_id) <= 200),
  created_at       timestamptz NOT NULL DEFAULT now(),
  started_at       timestamptz,
  finished_at      timestamptz,
  deadline_at      timestamptz NOT NULL,
  UNIQUE (org_id, id),
  FOREIGN KEY (org_id, workflow_id) REFERENCES workflows (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, version_id) REFERENCES workflow_versions (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, workspace_id) REFERENCES workspaces (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, environment_id) REFERENCES environments (org_id, id) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX workflow_executions_idempotency
  ON workflow_executions (org_id, workflow_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX workflow_executions_workflow_created
  ON workflow_executions (org_id, workflow_id, created_at DESC);
CREATE INDEX workflow_executions_workspace_created
  ON workflow_executions (org_id, workspace_id, created_at DESC);
CREATE INDEX workflow_executions_deadline
  ON workflow_executions (deadline_at) WHERE status IN ('pending', 'running');

CREATE TABLE workflow_execution_steps (
  id             uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id         uuid NOT NULL,
  execution_id   uuid NOT NULL,
  node_id        text NOT NULL CHECK (node_id ~ '^[A-Za-z][A-Za-z0-9_-]{0,63}$'),
  node_type      text NOT NULL,
  iteration_path text NOT NULL DEFAULT '',
  attempt        integer NOT NULL DEFAULT 1 CHECK (attempt BETWEEN 1 AND 10),
  status         text NOT NULL
                 CHECK (status IN ('pending', 'queued', 'running', 'succeeded', 'failed', 'skipped',
                                   'cancelled', 'retry_scheduled')),
  lease_until    timestamptz,
  worker_id      text,
  input          jsonb,
  output         jsonb,
  port           text NOT NULL DEFAULT '',
  error          jsonb,
  queued_at      timestamptz,
  dispatched_at  timestamptz,
  started_at     timestamptz,
  finished_at    timestamptz,
  retry_at       timestamptz,
  UNIQUE (org_id, id),
  UNIQUE (execution_id, node_id, iteration_path),
  FOREIGN KEY (org_id, execution_id) REFERENCES workflow_executions (org_id, id) ON DELETE CASCADE
);
CREATE INDEX workflow_steps_execution ON workflow_execution_steps (org_id, execution_id, queued_at);
CREATE INDEX workflow_steps_lease ON workflow_execution_steps (lease_until) WHERE status = 'running';
CREATE INDEX workflow_steps_retry ON workflow_execution_steps (retry_at) WHERE status = 'retry_scheduled';
CREATE INDEX workflow_steps_queued ON workflow_execution_steps (dispatched_at) WHERE status = 'queued';

ALTER TABLE workflows ENABLE ROW LEVEL SECURITY;
ALTER TABLE workflows FORCE ROW LEVEL SECURITY;
ALTER TABLE workflow_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE workflow_versions FORCE ROW LEVEL SECURITY;
ALTER TABLE workflow_deployments ENABLE ROW LEVEL SECURITY;
ALTER TABLE workflow_deployments FORCE ROW LEVEL SECURITY;
ALTER TABLE workflow_triggers ENABLE ROW LEVEL SECURITY;
ALTER TABLE workflow_triggers FORCE ROW LEVEL SECURITY;
ALTER TABLE workflow_executions ENABLE ROW LEVEL SECURITY;
ALTER TABLE workflow_executions FORCE ROW LEVEL SECURITY;
ALTER TABLE workflow_execution_steps ENABLE ROW LEVEL SECURITY;
ALTER TABLE workflow_execution_steps FORCE ROW LEVEL SECURITY;

CREATE POLICY workflows_tenant ON workflows TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY workflow_versions_tenant ON workflow_versions TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY workflow_deployments_tenant ON workflow_deployments TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY workflow_triggers_tenant ON workflow_triggers TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY workflow_executions_tenant ON workflow_executions TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY workflow_steps_tenant ON workflow_execution_steps TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());

-- The engine works across tenants (claim by id, reaper scans). It still binds
-- the tenant per step transaction; these policies only admit its role.
CREATE POLICY workflow_versions_engine ON workflow_versions TO hugi_system USING (true);
CREATE POLICY workflow_executions_engine ON workflow_executions TO hugi_system
  USING (true) WITH CHECK (true);
CREATE POLICY workflow_steps_engine ON workflow_execution_steps TO hugi_system
  USING (true) WITH CHECK (true);

GRANT SELECT, INSERT, UPDATE, DELETE ON workflows, workflow_versions, workflow_deployments,
  workflow_triggers TO hugi_app;
GRANT SELECT, INSERT, UPDATE ON workflow_executions, workflow_execution_steps TO hugi_app;
GRANT SELECT ON workflow_versions TO hugi_system;
GRANT SELECT, UPDATE ON workflow_executions TO hugi_system;
GRANT SELECT, INSERT, UPDATE ON workflow_execution_steps TO hugi_system;
-- The engine enqueues successor steps through the transactional outbox.
GRANT INSERT ON outbox TO hugi_system;

INSERT INTO resource_types (type, description, actions, sensitive_actions) VALUES
  ('workflow', 'Workflow definitions, versions, deployments and executions.',
   '{read,create,update,delete,publish,deploy,execute}', '{create,update,delete,publish,deploy,execute}');
INSERT INTO role_template_permissions (template_key, permission) VALUES
  ('org_admin', 'workflow:*'), ('workspace_admin', 'workflow:*');
INSERT INTO role_permissions (org_id, role_id, permission)
SELECT r.org_id, r.id, p.permission
FROM roles r CROSS JOIN (VALUES ('workflow:*')) p(permission)
WHERE r.key IN ('org_admin', 'workspace_admin') AND r.is_system
ON CONFLICT DO NOTHING;
UPDATE organizations SET authz_version = authz_version + 1;

-- +goose Down
DELETE FROM role_permissions rp USING roles r
WHERE rp.role_id = r.id AND r.key IN ('org_admin', 'workspace_admin') AND r.is_system
  AND rp.permission = 'workflow:*';
DELETE FROM role_template_permissions WHERE template_key IN ('org_admin', 'workspace_admin')
  AND permission = 'workflow:*';
DELETE FROM resource_types WHERE type = 'workflow';
UPDATE organizations SET authz_version = authz_version + 1;
REVOKE INSERT ON outbox FROM hugi_system;
DROP TABLE IF EXISTS workflow_execution_steps;
DROP TABLE IF EXISTS workflow_executions;
DROP TABLE IF EXISTS workflow_triggers;
DROP TABLE IF EXISTS workflow_deployments;
DROP TABLE IF EXISTS workflow_versions;
DROP TABLE IF EXISTS workflows;
