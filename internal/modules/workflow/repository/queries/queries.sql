-- workflow repository — Read side (side-effect-free statements).

-- name: ListWorkflows
SELECT w.id, w.workspace_id, w.name, w.slug::text AS slug, w.description, w.draft, w.status,
       w.created_at, w.updated_at,
       (SELECT max(v.version) FROM workflow_versions v WHERE v.workflow_id = w.id) AS latest_version
FROM workflows w WHERE w.workspace_id = $1 ORDER BY w.name;

-- name: GetWorkflow
SELECT w.id, w.workspace_id, w.name, w.slug::text AS slug, w.description, w.draft, w.status,
       w.created_at, w.updated_at,
       (SELECT max(v.version) FROM workflow_versions v WHERE v.workflow_id = w.id) AS latest_version
FROM workflows w WHERE w.id = $1;

-- name: ListVersions
SELECT id, workflow_id, version, definition, checksum, published_by, published_at
FROM workflow_versions WHERE workflow_id = $1 ORDER BY version DESC;

-- name: LatestVersion
SELECT id, workflow_id, version, definition, checksum, published_by, published_at
FROM workflow_versions WHERE workflow_id = $1 ORDER BY version DESC LIMIT 1;

-- name: GetVersion
SELECT id, workflow_id, version, definition, checksum, published_by, published_at
FROM workflow_versions WHERE id = $1;

-- name: ListDeployments
SELECT d.workflow_id, d.environment_id, d.version_id, v.version, d.enabled, d.updated_at
FROM workflow_deployments d JOIN workflow_versions v ON v.id = d.version_id
WHERE d.workflow_id = $1 ORDER BY d.environment_id;

-- name: GetDeployment
SELECT d.workflow_id, d.environment_id, d.version_id, v.version, d.enabled, d.updated_at
FROM workflow_deployments d JOIN workflow_versions v ON v.id = d.version_id
WHERE d.workflow_id = $1 AND d.environment_id = $2;

-- name: EnvironmentAllowlist
SELECT egress_allowlist FROM environments WHERE id = $1;

-- name: ExecutionByIdempotencyKey
SELECT id FROM workflow_executions WHERE workflow_id = $1 AND idempotency_key = $2;

-- name: ListExecutions
SELECT e.id, e.workspace_id, e.workflow_id, e.version_id, v.version, e.environment_id, e.trigger_type,
       e.status, e.input, e.output, e.error, e.step_count, e.rerun_count, e.created_at, e.started_at,
       e.finished_at, e.deadline_at
FROM workflow_executions e JOIN workflow_versions v ON v.id = e.version_id
WHERE e.workspace_id = $1
  AND ($2::uuid IS NULL OR e.workflow_id = $2)
  AND ($3 = '' OR e.status = $3)
ORDER BY e.created_at DESC
LIMIT $4;

-- name: GetExecution
SELECT e.id, e.workspace_id, e.workflow_id, e.version_id, v.version, e.environment_id, e.trigger_type,
       e.status, e.input, e.output, e.error, e.step_count, e.rerun_count, e.created_at, e.started_at,
       e.finished_at, e.deadline_at
FROM workflow_executions e JOIN workflow_versions v ON v.id = e.version_id
WHERE e.id = $1;

-- name: ListSteps
SELECT id, node_id, node_type, attempt, status, input, output, port, error, queued_at, started_at,
       finished_at
FROM workflow_execution_steps WHERE execution_id = $1
ORDER BY queued_at NULLS LAST, finished_at NULLS LAST, node_id;

-- name: StepStates
-- Engine: every step's state and output, read under the execution lock.
SELECT node_id, status, port, output FROM workflow_execution_steps WHERE execution_id = $1;

-- name: EngineVersion
SELECT definition FROM workflow_versions WHERE id = $1;

-- ---- S11 ------------------------------------------------------------------------

-- name: StepByID
SELECT id, node_id, node_type, status, input FROM workflow_execution_steps WHERE id = $1;

-- name: DueTimers
SELECT id, org_id FROM workflow_timers
WHERE fired_at IS NULL AND fire_at <= now() ORDER BY fire_at LIMIT $1;

-- name: CountActiveExecutions
SELECT count(*) FROM workflow_executions
WHERE workflow_id = $1 AND environment_id = $2 AND status IN ('pending', 'running');

-- name: UserGroups
SELECT group_id FROM group_members WHERE user_id = $1;

-- name: ListTasks
-- $3 (assignee) matches tasks naming the user directly or through any group.
SELECT t.id, t.workspace_id, t.workflow_id, w.name AS workflow_name, t.execution_id, t.step_id, t.node_id, t.kind,
       t.title, t.instructions, t.context, t.form, t.assignee_users, t.assignee_groups, t.allow_self_approval,
       t.requested_by, t.status, t.response, t.comment, t.decided_by, t.decided_at, t.due_at, t.created_at
FROM approval_tasks t JOIN workflows w ON w.id = t.workflow_id
WHERE ($1::uuid IS NULL OR t.workspace_id = $1)
  AND ($2 = '' OR t.status = $2)
  AND ($3::uuid IS NULL OR $3 = ANY (t.assignee_users)
       OR t.assignee_groups && ARRAY(SELECT gm.group_id FROM group_members gm WHERE gm.user_id = $3))
  AND ($4::uuid IS NULL OR t.execution_id = $4)
ORDER BY t.created_at DESC
LIMIT $5;

-- name: GetTask
SELECT t.id, t.workspace_id, t.workflow_id, w.name AS workflow_name, t.execution_id, t.step_id, t.node_id, t.kind,
       t.title, t.instructions, t.context, t.form, t.assignee_users, t.assignee_groups, t.allow_self_approval,
       t.requested_by, t.status, t.response, t.comment, t.decided_by, t.decided_at, t.due_at, t.created_at
FROM approval_tasks t JOIN workflows w ON w.id = t.workflow_id
WHERE t.id = $1;

-- name: ListSchedules
SELECT t.workflow_id, w.name AS workflow_name, w.status AS workflow_status, t.environment_id,
       en.name AS environment_name, v.version, d.enabled, coalesce(t.config ->> 'cron', '') AS cron,
       coalesce(nullif(t.config ->> 'timezone', ''), 'UTC') AS timezone, t.next_fire_at
FROM workflow_triggers t
JOIN workflows w ON w.id = t.workflow_id
JOIN workflow_deployments d
  ON d.org_id = t.org_id AND d.workflow_id = t.workflow_id AND d.environment_id = t.environment_id
JOIN workflow_versions v ON v.id = d.version_id
JOIN environments en ON en.id = t.environment_id
WHERE w.workspace_id = $1 AND t.type = 'trigger.schedule'
ORDER BY t.next_fire_at NULLS LAST, w.name;

-- name: DueSchedules
SELECT id, org_id FROM workflow_triggers
WHERE type = 'trigger.schedule' AND next_fire_at <= now()
ORDER BY next_fire_at LIMIT $1;

-- name: EventTriggers
SELECT t.id, t.org_id, t.workflow_id, t.environment_id, t.config, d.version_id, w.workspace_id
FROM workflow_triggers t
JOIN workflow_deployments d
  ON d.org_id = t.org_id AND d.workflow_id = t.workflow_id AND d.environment_id = t.environment_id
JOIN workflows w ON w.id = t.workflow_id
WHERE t.org_id = $1 AND t.type = 'trigger.event' AND t.config ->> 'event' = $2
  AND d.enabled AND w.status = 'active';

-- name: TimerExecution
SELECT execution_id FROM workflow_timers WHERE id = $1;
