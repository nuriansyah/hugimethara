-- api_composition repository — Write side (state-changing statements).

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

-- name: InsertCompositionVersion
INSERT INTO api_composition_versions
  (org_id, id, composition_id, version, definition, checksum, published_by)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: InsertCompositionExecution
INSERT INTO api_composition_executions
  (org_id, id, workspace_id, composition_id, version_id, environment_id,
   started_at, duration_ms, status, step_summary, output, error, request_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13);
