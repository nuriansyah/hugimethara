-- workflow repository — Write side (state-changing statements).

-- name: InsertWorkflow
INSERT INTO workflows (id, org_id, workspace_id, name, slug, description, draft, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, workspace_id, name, slug::text AS slug, description, draft, status, created_at, updated_at,
          NULL::integer AS latest_version;

-- name: UpdateWorkflow
UPDATE workflows SET name = $2, description = $3, draft = $4, status = $5 WHERE id = $1
RETURNING id, workspace_id, name, slug::text AS slug, description, draft, status, created_at, updated_at,
          (SELECT max(v.version) FROM workflow_versions v WHERE v.workflow_id = workflows.id) AS latest_version;

-- name: DeleteWorkflow
DELETE FROM workflows WHERE id = $1;

-- name: LockWorkflow
SELECT id FROM workflows WHERE id = $1 FOR UPDATE;

-- name: InsertVersion
INSERT INTO workflow_versions (id, org_id, workflow_id, version, definition, checksum, published_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, workflow_id, version, definition, checksum, published_by, published_at;

-- name: ActivateWorkflow
UPDATE workflows SET status = 'active' WHERE id = $1 AND status = 'draft';

-- name: UpsertDeployment
INSERT INTO workflow_deployments (org_id, workflow_id, environment_id, version_id, enabled, updated_by)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (org_id, workflow_id, environment_id)
DO UPDATE SET version_id = EXCLUDED.version_id, enabled = EXCLUDED.enabled,
              updated_by = EXCLUDED.updated_by, updated_at = now();

-- name: UpsertTrigger
INSERT INTO workflow_triggers (org_id, workflow_id, environment_id, type, config)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (org_id, workflow_id, environment_id)
DO UPDATE SET type = EXCLUDED.type, config = EXCLUDED.config;

-- name: InsertExecution
INSERT INTO workflow_executions (id, org_id, workspace_id, workflow_id, version_id, environment_id,
  trigger_type, trigger_ref, idempotency_key, input, egress_allowlist, max_steps, created_by, request_id,
  deadline_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
ON CONFLICT (org_id, workflow_id, idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING
RETURNING id;

-- name: InsertStep
-- New steps are created queued (ready) or skipped (dead path). The unique key
-- makes concurrent progression idempotent.
INSERT INTO workflow_execution_steps (id, org_id, execution_id, node_id, node_type, status, input,
  queued_at, dispatched_at, finished_at)
VALUES ($1, $2, $3, $4, $5, $6, $7,
  CASE WHEN $6 = 'queued' THEN now() END, CASE WHEN $6 = 'queued' THEN now() END,
  CASE WHEN $6 = 'skipped' THEN now() END)
ON CONFLICT (execution_id, node_id, iteration_path) DO NOTHING
RETURNING id;

-- name: AddStepCount
UPDATE workflow_executions SET step_count = step_count + $2 WHERE id = $1;

-- name: LockExecution
-- Serializes progression of one execution so concurrent branch completions
-- always observe each other (otherwise a merge could never become ready).
SELECT id, org_id, workspace_id, workflow_id, version_id, environment_id, status, input, egress_allowlist,
       step_count, max_steps, deadline_at
FROM workflow_executions WHERE id = $1 FOR UPDATE;

-- name: StartExecution
UPDATE workflow_executions SET status = 'running', started_at = coalesce(started_at, now())
WHERE id = $1 AND status = 'pending';

-- name: FinishExecution
UPDATE workflow_executions SET status = $2, output = $3, error = $4, finished_at = now()
WHERE id = $1 AND status IN ('pending', 'running');

-- name: ClaimStep
-- Lease-based claim: exactly one worker moves a queued step to running.
UPDATE workflow_execution_steps
SET status = 'running', lease_until = now() + make_interval(secs => $2), worker_id = $3,
    started_at = coalesce(started_at, now())
WHERE id = $1 AND status = 'queued'
RETURNING id, org_id, execution_id, node_id, node_type, attempt;

-- name: ExtendLease
UPDATE workflow_execution_steps SET lease_until = now() + make_interval(secs => $2)
WHERE id = $1 AND status = 'running' AND worker_id = $3;

-- name: CompleteStep
-- Fenced by worker_id: a worker whose lease was taken over cannot overwrite.
UPDATE workflow_execution_steps
SET status = $2, input = $3, output = $4, port = $5, error = $6, finished_at = now(),
    lease_until = NULL, worker_id = NULL
WHERE id = $1 AND status = 'running' AND worker_id = $7;

-- name: ScheduleRetry
UPDATE workflow_execution_steps
SET status = 'retry_scheduled', attempt = attempt + 1, error = $2, retry_at = $3, lease_until = NULL,
    worker_id = NULL, input = $5
WHERE id = $1 AND status = 'running' AND worker_id = $4;

-- name: CancelOpenSteps
UPDATE workflow_execution_steps SET status = 'cancelled', finished_at = now(), lease_until = NULL
WHERE execution_id = $1 AND status IN ('pending', 'queued', 'retry_scheduled', 'running');

-- name: RequeueExpiredLeases
-- Reaper: steps whose worker died go back to the queue.
UPDATE workflow_execution_steps s
SET status = 'queued', lease_until = NULL, worker_id = NULL, dispatched_at = now()
WHERE s.id IN (
  SELECT id FROM workflow_execution_steps
  WHERE status = 'running' AND lease_until < now()
  ORDER BY lease_until LIMIT $1 FOR UPDATE SKIP LOCKED)
RETURNING s.id, s.org_id;

-- name: RequeueDueRetries
UPDATE workflow_execution_steps s
SET status = 'queued', retry_at = NULL, queued_at = now(), dispatched_at = now()
WHERE s.id IN (
  SELECT id FROM workflow_execution_steps
  WHERE status = 'retry_scheduled' AND retry_at <= now()
  ORDER BY retry_at LIMIT $1 FOR UPDATE SKIP LOCKED)
RETURNING s.id, s.org_id;

-- name: RedispatchStale
-- Queued steps whose dispatch message was lost (e.g. stream purge) are re-sent.
UPDATE workflow_execution_steps s SET dispatched_at = now()
WHERE s.id IN (
  SELECT id FROM workflow_execution_steps
  WHERE status = 'queued' AND dispatched_at < now() - make_interval(secs => $2)
  ORDER BY dispatched_at LIMIT $1 FOR UPDATE SKIP LOCKED)
RETURNING s.id, s.org_id;

-- name: ExpireDeadlines
UPDATE workflow_executions e
SET status = 'timed_out', finished_at = now(),
    error = jsonb_build_object('code', 'execution_deadline_exceeded',
                               'message', 'The execution exceeded its maximum duration.')
WHERE e.id IN (
  SELECT id FROM workflow_executions
  WHERE status IN ('pending', 'running') AND deadline_at < now()
  ORDER BY deadline_at LIMIT $1 FOR UPDATE SKIP LOCKED)
RETURNING e.id;
