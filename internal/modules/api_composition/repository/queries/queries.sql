-- api_composition repository — Read side (side-effect-free statements).

-- name: ListCompositions
SELECT id, workspace_id, name, slug, draft, exposed, created_at, updated_at
FROM api_compositions
WHERE workspace_id = $1
ORDER BY name, id;

-- name: GetComposition
SELECT id, workspace_id, name, slug, draft, exposed, created_at, updated_at
FROM api_compositions WHERE id = $1;

-- name: ListCompositionVersions
SELECT id, composition_id, version, definition, checksum, published_by, published_at
FROM api_composition_versions WHERE composition_id = $1
ORDER BY version DESC;

-- name: NextCompositionVersion
SELECT COALESCE(MAX(version), 0) + 1
FROM api_composition_versions WHERE composition_id = $1;

-- name: GetCompositionVersion
SELECT id, composition_id, version, definition, checksum, published_by, published_at
FROM api_composition_versions WHERE composition_id = $1 AND version = $2;

-- name: ListCompositionExecutions
SELECT id, composition_id, version_id, environment_id, started_at, duration_ms,
       status, step_summary, output, error, request_id
FROM api_composition_executions
WHERE workspace_id = $1 AND ($2::uuid IS NULL OR composition_id = $2)
ORDER BY started_at DESC, id DESC
LIMIT 100;

-- name: GetCompositionExecution
SELECT e.id, e.composition_id, e.version_id, e.environment_id, e.started_at,
       e.duration_ms, e.status, e.step_summary, e.output, e.error, e.request_id,
       c.workspace_id
FROM api_composition_executions e
JOIN api_compositions c ON c.org_id = e.org_id AND c.id = e.composition_id
WHERE e.id = $1;
