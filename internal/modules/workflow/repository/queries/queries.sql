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
       e.status, e.input, e.output, e.error, e.step_count, e.created_at, e.started_at, e.finished_at,
       e.deadline_at
FROM workflow_executions e JOIN workflow_versions v ON v.id = e.version_id
WHERE e.workspace_id = $1
  AND ($2::uuid IS NULL OR e.workflow_id = $2)
  AND ($3 = '' OR e.status = $3)
ORDER BY e.created_at DESC
LIMIT $4;

-- name: GetExecution
SELECT e.id, e.workspace_id, e.workflow_id, e.version_id, v.version, e.environment_id, e.trigger_type,
       e.status, e.input, e.output, e.error, e.step_count, e.created_at, e.started_at, e.finished_at,
       e.deadline_at
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
