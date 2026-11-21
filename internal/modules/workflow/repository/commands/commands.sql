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
-- next_fire_at is set for enabled schedule triggers, NULL otherwise.
INSERT INTO workflow_triggers (org_id, workflow_id, environment_id, type, config, next_fire_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (org_id, workflow_id, environment_id)
DO UPDATE SET type = EXCLUDED.type, config = EXCLUDED.config, next_fire_at = EXCLUDED.next_fire_at;

-- name: InsertExecution
INSERT INTO workflow_executions (id, org_id, workspace_id, workflow_id, version_id, environment_id,
  trigger_type, trigger_ref, idempotency_key, input, egress_allowlist, max_steps, created_by, request_id,
  deadline_at, trigger_depth)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
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
       step_count, max_steps, deadline_at, created_by, trigger_depth
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
-- Cancelling a running step also fences its worker out: CompleteStep and
-- ExtendLease require status = 'running'.
UPDATE workflow_execution_steps SET status = 'cancelled', finished_at = now(), lease_until = NULL, worker_id = NULL
WHERE execution_id = $1 AND status IN ('pending', 'queued', 'retry_scheduled', 'running', 'waiting');

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
RETURNING e.id, e.org_id, e.workflow_id, e.workspace_id, e.trigger_depth;

-- ---- S11: suspension, timers, human tasks ------------------------------------

-- name: SuspendStep
-- running → waiting (delay / human task). Fenced by worker like CompleteStep.
UPDATE workflow_execution_steps
SET status = 'waiting', input = $3, lease_until = NULL, worker_id = NULL
WHERE id = $1 AND status = 'running' AND worker_id = $2;

-- name: ResumeStep
-- waiting → succeeded with the resolution as output and branch port.
UPDATE workflow_execution_steps
SET status = 'succeeded', output = $2, port = $3, finished_at = now()
WHERE id = $1 AND status = 'waiting';

-- name: FailWaitingStep
UPDATE workflow_execution_steps
SET status = 'failed', error = $2, finished_at = now()
WHERE id = $1 AND status = 'waiting';

-- name: InsertTimer
INSERT INTO workflow_timers (id, org_id, execution_id, step_id, kind, fire_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (step_id, kind) DO NOTHING;

-- name: LockDueTimer
SELECT id, org_id, execution_id, step_id, kind FROM workflow_timers
WHERE id = $1 AND fired_at IS NULL AND fire_at <= now()
FOR UPDATE SKIP LOCKED;

-- name: MarkTimerFired
UPDATE workflow_timers SET fired_at = now() WHERE id = $1;

-- name: ConsumeTimers
-- Disarms an execution's pending timers (cancel, finish).
UPDATE workflow_timers SET fired_at = now() WHERE execution_id = $1 AND fired_at IS NULL;

-- name: InsertTask
INSERT INTO approval_tasks (id, org_id, workspace_id, workflow_id, execution_id, step_id, node_id, kind, title,
  instructions, context, form, assignee_users, assignee_groups, allow_self_approval, requested_by, due_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
ON CONFLICT (step_id) DO NOTHING;

-- name: LockTask
SELECT t.id, t.workspace_id, t.workflow_id, w.name AS workflow_name, t.execution_id, t.step_id, t.node_id, t.kind,
       t.title, t.instructions, t.context, t.form, t.assignee_users, t.assignee_groups, t.allow_self_approval,
       t.requested_by, t.status, t.response, t.comment, t.decided_by, t.decided_at, t.due_at, t.created_at
FROM approval_tasks t JOIN workflows w ON w.id = t.workflow_id
WHERE t.id = $1 FOR UPDATE OF t;

-- name: DecideTask
UPDATE approval_tasks SET status = $2, response = $3, comment = $4, decided_by = $5, decided_at = now()
WHERE id = $1 AND status = 'pending';

-- name: ExpireTask
UPDATE approval_tasks SET status = 'expired', decided_at = now()
WHERE step_id = $1 AND status = 'pending'
RETURNING id;

-- name: CancelTasks
UPDATE approval_tasks SET status = 'cancelled', decided_at = now()
WHERE execution_id = $1 AND status = 'pending';

-- ---- S11: re-run from the failed step ------------------------------------------

-- name: DeleteRerunSteps
-- The failed step and the steps cancelled because of it are replaced by fresh
-- ones computed by progression.
DELETE FROM workflow_execution_steps
WHERE execution_id = $1 AND status IN ('failed', 'cancelled')
RETURNING node_id;

-- name: ResetExecution
UPDATE workflow_executions
SET status = 'running', error = NULL, output = NULL, finished_at = NULL, deadline_at = $2,
    rerun_count = rerun_count + 1, step_count = greatest(step_count - $3, 0)
WHERE id = $1 AND status IN ('failed', 'timed_out');

-- ---- S11: schedule triggers -----------------------------------------------------

-- name: LockDueSchedule
SELECT t.id, t.org_id, t.workflow_id, t.environment_id, t.config, t.next_fire_at, d.enabled, d.version_id,
       w.status AS workflow_status, w.workspace_id
FROM workflow_triggers t
JOIN workflow_deployments d
  ON d.org_id = t.org_id AND d.workflow_id = t.workflow_id AND d.environment_id = t.environment_id
JOIN workflows w ON w.id = t.workflow_id
WHERE t.id = $1 AND t.type = 'trigger.schedule' AND t.next_fire_at <= now()
FOR UPDATE OF t SKIP LOCKED;

-- name: SetNextFire
UPDATE workflow_triggers SET next_fire_at = $2 WHERE id = $1;

-- name: LockStarts
SELECT pg_advisory_xact_lock(hashtextextended('hugi.wf.start:' || $1::text, 0));
