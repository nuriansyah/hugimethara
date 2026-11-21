-- +goose Up
-- S11 workflow engine complete (docs/architecture/workflow-runtime.md §3–§4).
-- Adds the `waiting` step state, durable timers (delay, human-task timeout),
-- human tasks (approval / manual input), execution re-run bookkeeping, and the
-- engine-side reads needed to start executions from schedule and event triggers.

ALTER TABLE workflow_execution_steps DROP CONSTRAINT workflow_execution_steps_status_check;
ALTER TABLE workflow_execution_steps ADD CONSTRAINT workflow_execution_steps_status_check
  CHECK (status IN ('pending', 'queued', 'running', 'waiting', 'succeeded', 'failed', 'skipped',
                    'cancelled', 'retry_scheduled'));

-- rerun_count: how many times the execution was re-run from its failed step.
-- trigger_depth: event-trigger chain depth (event → workflow → event …), bounded by the engine.
ALTER TABLE workflow_executions
  ADD COLUMN rerun_count   integer NOT NULL DEFAULT 0 CHECK (rerun_count BETWEEN 0 AND 100),
  ADD COLUMN trigger_depth integer NOT NULL DEFAULT 0 CHECK (trigger_depth BETWEEN 0 AND 10);
CREATE INDEX workflow_executions_running_workflow
  ON workflow_executions (org_id, workflow_id, environment_id) WHERE status IN ('pending', 'running');
CREATE INDEX workflow_triggers_due
  ON workflow_triggers (next_fire_at) WHERE type = 'trigger.schedule' AND next_fire_at IS NOT NULL;
CREATE INDEX workflow_triggers_event
  ON workflow_triggers (org_id, (config ->> 'event')) WHERE type = 'trigger.event';

-- A timer wakes a waiting step: a delay elapsing, or a human task expiring.
CREATE TABLE workflow_timers (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id       uuid NOT NULL,
  execution_id uuid NOT NULL,
  step_id      uuid NOT NULL,
  kind         text NOT NULL CHECK (kind IN ('delay', 'task_timeout')),
  fire_at      timestamptz NOT NULL,
  fired_at     timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (step_id, kind),
  FOREIGN KEY (org_id, execution_id) REFERENCES workflow_executions (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, step_id) REFERENCES workflow_execution_steps (org_id, id) ON DELETE CASCADE
);
CREATE INDEX workflow_timers_due ON workflow_timers (fire_at) WHERE fired_at IS NULL;

-- Human tasks: an approval (approve / reject) or a manual-input form. The
-- assignee rule is the union of explicit users and group members.
CREATE TABLE approval_tasks (
  id                 uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id             uuid NOT NULL,
  workspace_id       uuid NOT NULL,
  workflow_id        uuid NOT NULL,
  execution_id       uuid NOT NULL,
  step_id            uuid NOT NULL UNIQUE,
  node_id            text NOT NULL,
  kind               text NOT NULL CHECK (kind IN ('approval', 'input')),
  title              text NOT NULL CHECK (length(btrim(title)) BETWEEN 1 AND 200),
  instructions       text NOT NULL DEFAULT '' CHECK (length(instructions) <= 2000),
  context            jsonb,
  form               jsonb NOT NULL DEFAULT '[]'::jsonb,
  assignee_users     uuid[] NOT NULL DEFAULT '{}',
  assignee_groups    uuid[] NOT NULL DEFAULT '{}',
  allow_self_approval boolean NOT NULL DEFAULT false,
  requested_by       uuid,
  status             text NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('pending', 'approved', 'rejected', 'submitted', 'expired', 'cancelled')),
  response           jsonb,
  comment            text NOT NULL DEFAULT '' CHECK (length(comment) <= 2000),
  decided_by         uuid,
  decided_at         timestamptz,
  due_at             timestamptz,
  created_at         timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, id),
  FOREIGN KEY (org_id, execution_id) REFERENCES workflow_executions (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, step_id) REFERENCES workflow_execution_steps (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, workspace_id) REFERENCES workspaces (org_id, id) ON DELETE CASCADE
);
CREATE INDEX approval_tasks_pending ON approval_tasks (org_id, status, created_at DESC);
CREATE INDEX approval_tasks_users ON approval_tasks USING gin (assignee_users) WHERE status = 'pending';
CREATE INDEX approval_tasks_groups ON approval_tasks USING gin (assignee_groups) WHERE status = 'pending';

ALTER TABLE workflow_timers ENABLE ROW LEVEL SECURITY;
ALTER TABLE workflow_timers FORCE ROW LEVEL SECURITY;
ALTER TABLE approval_tasks ENABLE ROW LEVEL SECURITY;
ALTER TABLE approval_tasks FORCE ROW LEVEL SECURITY;

CREATE POLICY workflow_timers_tenant ON workflow_timers TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY approval_tasks_tenant ON approval_tasks TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY workflow_timers_engine ON workflow_timers TO hugi_system USING (true) WITH CHECK (true);
CREATE POLICY approval_tasks_engine ON approval_tasks TO hugi_system USING (true) WITH CHECK (true);

-- Trigger runner (hugi_system): starts executions from schedules and platform
-- events, so it reads what the control plane's Execute reads.
CREATE POLICY workflows_engine ON workflows TO hugi_system USING (true);
CREATE POLICY workflow_deployments_engine ON workflow_deployments TO hugi_system USING (true);
CREATE POLICY workflow_triggers_engine ON workflow_triggers TO hugi_system USING (true) WITH CHECK (true);
CREATE POLICY environments_engine ON environments TO hugi_system USING (true);

GRANT SELECT, INSERT, UPDATE, DELETE ON workflow_timers, approval_tasks TO hugi_app;
-- Re-run from the failed step removes the failed/cancelled steps it replaces.
GRANT DELETE ON workflow_execution_steps TO hugi_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON workflow_timers TO hugi_system;
GRANT SELECT, INSERT, UPDATE ON approval_tasks TO hugi_system;
GRANT SELECT (id, org_id, workspace_id, name, status) ON workflows TO hugi_system;
GRANT SELECT ON workflow_deployments TO hugi_system;
GRANT SELECT, UPDATE (next_fire_at) ON workflow_triggers TO hugi_system;
GRANT SELECT (id, org_id, egress_allowlist) ON environments TO hugi_system;
GRANT INSERT ON workflow_executions TO hugi_system;

-- +goose Down
REVOKE INSERT ON workflow_executions FROM hugi_system;
REVOKE SELECT (id, org_id, egress_allowlist) ON environments FROM hugi_system;
REVOKE SELECT, UPDATE (next_fire_at) ON workflow_triggers FROM hugi_system;
REVOKE SELECT ON workflow_triggers FROM hugi_system;
REVOKE SELECT ON workflow_deployments FROM hugi_system;
REVOKE SELECT (id, org_id, workspace_id, name, status) ON workflows FROM hugi_system;
REVOKE DELETE ON workflow_execution_steps FROM hugi_app;
DROP POLICY IF EXISTS environments_engine ON environments;
DROP POLICY IF EXISTS workflow_triggers_engine ON workflow_triggers;
DROP POLICY IF EXISTS workflow_deployments_engine ON workflow_deployments;
DROP POLICY IF EXISTS workflows_engine ON workflows;
DROP TABLE IF EXISTS approval_tasks;
DROP TABLE IF EXISTS workflow_timers;
DROP INDEX IF EXISTS workflow_triggers_event;
DROP INDEX IF EXISTS workflow_triggers_due;
DROP INDEX IF EXISTS workflow_executions_running_workflow;
ALTER TABLE workflow_executions DROP COLUMN IF EXISTS trigger_depth, DROP COLUMN IF EXISTS rerun_count;
UPDATE workflow_execution_steps SET status = 'cancelled' WHERE status = 'waiting';
ALTER TABLE workflow_execution_steps DROP CONSTRAINT workflow_execution_steps_status_check;
ALTER TABLE workflow_execution_steps ADD CONSTRAINT workflow_execution_steps_status_check
  CHECK (status IN ('pending', 'queued', 'running', 'succeeded', 'failed', 'skipped',
                    'cancelled', 'retry_scheduled'));
