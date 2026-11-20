-- name: ListCompositions
SELECT id, workspace_id, name, slug, draft, exposed, created_at, updated_at
FROM api_compositions
WHERE workspace_id = $1
ORDER BY name, id;

-- name: GetComposition
SELECT id, workspace_id, name, slug, draft, exposed, created_at, updated_at
FROM api_compositions WHERE id = $1;

-- name: InsertComposition
INSERT INTO api_compositions (org_id, id, workspace_id, name, slug, draft)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: UpdateComposition
UPDATE api_compositions
SET name = COALESCE($2, name), slug = COALESCE($3, slug),
    draft = COALESCE($4, draft), exposed = COALESCE($5, exposed)
WHERE id = $1;

-- name: DeleteComposition
DELETE FROM api_compositions WHERE id = $1;

-- name: ListCompositionVersions
SELECT id, composition_id, version, definition, checksum, published_by, published_at
FROM api_composition_versions WHERE composition_id = $1
ORDER BY version DESC;

-- name: NextCompositionVersion
SELECT COALESCE(MAX(version), 0) + 1
FROM api_composition_versions WHERE composition_id = $1;

-- name: InsertCompositionVersion
INSERT INTO api_composition_versions
  (org_id, id, composition_id, version, definition, checksum, published_by)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: GetCompositionVersion
SELECT id, composition_id, version, definition, checksum, published_by, published_at
FROM api_composition_versions WHERE composition_id = $1 AND version = $2;

-- name: InsertCompositionExecution
INSERT INTO api_composition_executions
  (org_id, id, workspace_id, composition_id, version_id, environment_id,
   started_at, duration_ms, status, step_summary, output, error, request_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13);

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
