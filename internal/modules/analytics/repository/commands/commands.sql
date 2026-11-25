-- analytics repository — Write side.

-- name: InsertSemanticModel
INSERT INTO semantic_models (org_id, id, workspace_id, dataset_id, name, slug, description, dimensions, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: UpdateSemanticModel
UPDATE semantic_models
SET name = COALESCE($2, name),
    slug = COALESCE($3, slug),
    description = COALESCE($4, description),
    dimensions = COALESCE($5, dimensions),
    status = COALESCE($6, status)
WHERE id = $1;

-- name: DeleteSemanticModel
DELETE FROM semantic_models WHERE id = $1;

-- name: InsertMetric
INSERT INTO metrics (org_id, id, workspace_id, semantic_model_id, name, slug, description, expression, filters, format, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);

-- name: UpdateMetric
UPDATE metrics
SET name = COALESCE($2, name),
    slug = COALESCE($3, slug),
    description = COALESCE($4, description),
    expression = COALESCE($5, expression),
    filters = COALESCE($6, filters),
    format = COALESCE($7, format),
    status = COALESCE($8, status)
WHERE id = $1;

-- name: DeleteMetric
DELETE FROM metrics WHERE id = $1;

-- name: InsertQueryRun
INSERT INTO query_runs (org_id, id, workspace_id, dataset_id, metric_id, principal_id, principal_type,
  sql, clickhouse_id, status, exported)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'running', $10);

-- name: FinishQueryRun
UPDATE query_runs
SET status = $2, row_count = $3, bytes_read = $4, duration_ms = $5, result_preview = $6,
    columns = $7, error = $8, finished_at = now()
WHERE id = $1 AND status = 'running';

-- name: MarkQueryExported
UPDATE query_runs SET exported = true WHERE id = $1;
