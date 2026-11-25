-- analytics repository — Read side.

-- name: ListSemanticModels
SELECT id, org_id, workspace_id, dataset_id, name, slug, description, dimensions, status, created_at, updated_at
FROM semantic_models WHERE workspace_id = $1 AND status = 'active' ORDER BY name;

-- name: GetSemanticModel
SELECT id, org_id, workspace_id, dataset_id, name, slug, description, dimensions, status, created_at, updated_at
FROM semantic_models WHERE id = $1;

-- name: ListMetrics
SELECT id, org_id, workspace_id, semantic_model_id, name, slug, description, expression, filters, format, status,
       created_at, updated_at
FROM metrics WHERE workspace_id = $1 AND status = 'active' ORDER BY name;

-- name: ListMetricsForModel
SELECT id, org_id, workspace_id, semantic_model_id, name, slug, description, expression, filters, format, status,
       created_at, updated_at
FROM metrics WHERE semantic_model_id = $1 AND status = 'active' ORDER BY name;

-- name: GetMetric
SELECT id, org_id, workspace_id, semantic_model_id, name, slug, description, expression, filters, format, status,
       created_at, updated_at
FROM metrics WHERE id = $1;

-- name: ListQueryRuns
SELECT id, org_id, workspace_id, dataset_id, metric_id, principal_id, principal_type, sql, clickhouse_id,
       status, row_count, bytes_read, duration_ms, columns, result_preview, error, exported, created_at, finished_at
FROM query_runs WHERE workspace_id = $1 ORDER BY created_at DESC LIMIT $2;

-- name: GetQueryRun
SELECT id, org_id, workspace_id, dataset_id, metric_id, principal_id, principal_type, sql, clickhouse_id,
       status, row_count, bytes_read, duration_ms, columns, result_preview, error, exported, created_at, finished_at
FROM query_runs WHERE id = $1;
