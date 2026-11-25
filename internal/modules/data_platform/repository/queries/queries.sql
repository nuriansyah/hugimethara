-- data_platform repository — Read side.

-- name: ListDataSources
SELECT id, org_id, workspace_id, environment_id, name, slug, kind, config, secret_id, status, created_at, updated_at
FROM data_sources WHERE workspace_id = $1 ORDER BY name;

-- name: GetDataSource
SELECT id, org_id, workspace_id, environment_id, name, slug, kind, config, secret_id, status, created_at, updated_at
FROM data_sources WHERE id = $1;

-- name: ListSchemas
SELECT id, data_source_id, schema, checksum, captured_at
FROM data_source_schemas WHERE data_source_id = $1 ORDER BY captured_at DESC LIMIT $2;

-- name: LatestSchema
SELECT id, data_source_id, schema, checksum, captured_at
FROM data_source_schemas WHERE data_source_id = $1 ORDER BY captured_at DESC LIMIT 1;

-- name: ListPipelines
SELECT id, org_id, workspace_id, data_source_id, name, slug, target_table, mode, source_object,
       watermark_column, watermark_type, schedule_cron, config, enabled, last_run_at, next_run_at, created_at, updated_at
FROM ingestion_pipelines WHERE workspace_id = $1 ORDER BY name;

-- name: GetPipeline
SELECT id, org_id, workspace_id, data_source_id, name, slug, target_table, mode, source_object,
       watermark_column, watermark_type, schedule_cron, config, enabled, last_run_at, next_run_at, created_at, updated_at
FROM ingestion_pipelines WHERE id = $1;

-- name: DuePipelines
SELECT id, org_id, workspace_id, data_source_id, name, slug, target_table, mode, source_object,
       watermark_column, watermark_type, schedule_cron, config, enabled, last_run_at, next_run_at, created_at, updated_at
FROM ingestion_pipelines
WHERE enabled AND next_run_at IS NOT NULL AND next_run_at <= now()
ORDER BY next_run_at LIMIT $1 FOR UPDATE SKIP LOCKED;

-- name: ListRuns
SELECT id, pipeline_id, status, mode, watermark_before, watermark_after, rows_read, rows_written,
       schema_drift, error, started_at, finished_at, created_at
FROM ingestion_runs WHERE pipeline_id = $1 ORDER BY created_at DESC LIMIT $2;

-- name: GetRun
SELECT id, pipeline_id, status, mode, watermark_before, watermark_after, rows_read, rows_written,
       schema_drift, error, started_at, finished_at, created_at
FROM ingestion_runs WHERE id = $1;

-- name: LatestSuccessfulRun
SELECT id, pipeline_id, status, mode, watermark_before, watermark_after, rows_read, rows_written,
       schema_drift, error, started_at, finished_at, created_at
FROM ingestion_runs WHERE pipeline_id = $1 AND status = 'succeeded'
ORDER BY finished_at DESC NULLS LAST, created_at DESC LIMIT 1;

-- name: ListDatasets
SELECT id, org_id, workspace_id, name, slug, description, owner, draft_sql, status, created_at, updated_at
FROM datasets WHERE workspace_id = $1 AND status <> 'archived' ORDER BY name;

-- name: GetDataset
SELECT id, org_id, workspace_id, name, slug, description, owner, draft_sql, status, created_at, updated_at
FROM datasets WHERE id = $1;

-- name: GetDatasetBySlug
SELECT id, org_id, workspace_id, name, slug, description, owner, draft_sql, status, created_at, updated_at
FROM datasets WHERE workspace_id = $1 AND slug = $2;

-- name: ListDatasetVersions
SELECT id, dataset_id, version, sql, columns, checksum, published_by, published_at
FROM dataset_versions WHERE dataset_id = $1 ORDER BY version DESC;

-- name: GetDatasetVersion
SELECT id, dataset_id, version, sql, columns, checksum, published_by, published_at
FROM dataset_versions WHERE id = $1;

-- name: LatestDatasetVersion
SELECT id, dataset_id, version, sql, columns, checksum, published_by, published_at
FROM dataset_versions WHERE dataset_id = $1 ORDER BY version DESC LIMIT 1;

-- name: ListLineage
SELECT dataset_id, upstream_kind, upstream_id, columns FROM dataset_lineage WHERE dataset_id = $1;

-- name: ListColumnPolicies
SELECT column_name, classification, mask FROM dataset_column_policies WHERE dataset_id = $1 ORDER BY column_name;

-- name: ListRowPolicies
SELECT role_key::text AS role_key, row_filter, column_masks FROM dataset_row_policies WHERE dataset_id = $1;

-- name: RawTables
SELECT DISTINCT target_table AS name FROM ingestion_pipelines WHERE org_id = $1;

-- name: PublishedDatasets
SELECT id, slug::text AS slug FROM datasets WHERE org_id = $1 AND status = 'published';

-- name: PipelineByTarget
SELECT id, data_source_id FROM ingestion_pipelines WHERE org_id = $1 AND target_table = $2 LIMIT 1;

-- name: EnvironmentWorkspace
SELECT workspace_id FROM environments WHERE id = $1;

-- name: EnvironmentEgress
SELECT array_to_json(egress_allowlist)::text AS allowlist FROM environments WHERE id = $1;
