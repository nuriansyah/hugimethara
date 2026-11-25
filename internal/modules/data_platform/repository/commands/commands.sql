-- data_platform repository — Write side.

-- name: InsertDataSource
INSERT INTO data_sources (org_id, id, workspace_id, environment_id, name, slug, kind, config, secret_id, status, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);

-- name: UpdateDataSource
UPDATE data_sources
SET name = COALESCE($2, name),
    slug = COALESCE($3, slug),
    config = COALESCE($4, config),
    secret_id = CASE WHEN $5 THEN $6 ELSE secret_id END,
    status = COALESCE($7, status)
WHERE id = $1;

-- name: DeleteDataSource
DELETE FROM data_sources WHERE id = $1;

-- name: InsertSchema
INSERT INTO data_source_schemas (org_id, id, data_source_id, schema, checksum)
VALUES ($1, $2, $3, $4, $5);

-- name: InsertPipeline
INSERT INTO ingestion_pipelines (org_id, id, workspace_id, data_source_id, name, slug, target_table, mode,
  source_object, watermark_column, watermark_type, schedule_cron, config, enabled, next_run_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15);

-- name: UpdatePipeline
UPDATE ingestion_pipelines
SET name = COALESCE($2, name),
    target_table = COALESCE($3, target_table),
    mode = COALESCE($4, mode),
    source_object = COALESCE($5, source_object),
    watermark_column = COALESCE($6, watermark_column),
    watermark_type = COALESCE($7, watermark_type),
    schedule_cron = COALESCE($8, schedule_cron),
    config = COALESCE($9, config),
    enabled = COALESCE($10, enabled),
    next_run_at = CASE WHEN $11 THEN $12 ELSE next_run_at END
WHERE id = $1;

-- name: DeletePipeline
DELETE FROM ingestion_pipelines WHERE id = $1;

-- name: InsertRun
INSERT INTO ingestion_runs (org_id, id, pipeline_id, status, mode, watermark_before, requested_by)
VALUES ($1, $2, $3, 'pending', $4, $5, $6);

-- name: StartRun
UPDATE ingestion_runs SET status = 'running', started_at = now() WHERE id = $1;

-- name: FinishRun
UPDATE ingestion_runs
SET status = $2, watermark_after = $3, rows_read = $4, rows_written = $5,
    schema_drift = $6, error = $7, finished_at = now()
WHERE id = $1;

-- name: ClaimPipeline
UPDATE ingestion_pipelines SET next_run_at = $2 WHERE id = $1;

-- name: TouchPipelineRun
UPDATE ingestion_pipelines SET last_run_at = now(), next_run_at = $2 WHERE id = $1;

-- name: InsertDataset
INSERT INTO datasets (org_id, id, workspace_id, name, slug, description, owner, draft_sql, status, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: UpdateDataset
UPDATE datasets
SET name = COALESCE($2, name),
    slug = COALESCE($3, slug),
    description = COALESCE($4, description),
    owner = COALESCE($5, owner),
    draft_sql = COALESCE($6, draft_sql),
    status = COALESCE($7, status)
WHERE id = $1;

-- name: DeleteDataset
DELETE FROM datasets WHERE id = $1;

-- name: InsertDatasetVersion
INSERT INTO dataset_versions (org_id, id, dataset_id, version, sql, columns, checksum, published_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: DeleteLineage
DELETE FROM dataset_lineage WHERE dataset_id = $1;

-- name: InsertLineage
INSERT INTO dataset_lineage (org_id, id, dataset_id, upstream_kind, upstream_id, columns)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (org_id, dataset_id, upstream_kind, upstream_id) DO UPDATE SET columns = EXCLUDED.columns;

-- name: UpsertColumnPolicy
INSERT INTO dataset_column_policies (org_id, id, dataset_id, column_name, classification, mask)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (org_id, dataset_id, column_name)
DO UPDATE SET classification = EXCLUDED.classification, mask = EXCLUDED.mask;

-- name: DeleteColumnPolicy
DELETE FROM dataset_column_policies WHERE dataset_id = $1 AND column_name = $2;

-- name: UpsertRowPolicy
INSERT INTO dataset_row_policies (org_id, id, dataset_id, role_key, row_filter, column_masks)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (org_id, dataset_id, role_key)
DO UPDATE SET row_filter = EXCLUDED.row_filter, column_masks = EXCLUDED.column_masks;

-- name: DeleteRowPolicy
DELETE FROM dataset_row_policies WHERE dataset_id = $1 AND role_key = $2;
