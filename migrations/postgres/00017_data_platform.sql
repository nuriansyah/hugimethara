-- +goose Up
-- Phase 2 S15–S20: data platform, analytics and dashboards. Postgres keeps the
-- control plane; ClickHouse (ADR-0008) holds raw ingested rows, dataset views
-- and analytical query results. Every table is tenant-scoped with RLS; the
-- worker gets narrow engine policies for scheduled ingestion and reports.

CREATE TABLE data_sources (
  id             uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id         uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
  workspace_id   uuid NOT NULL,
  environment_id uuid NOT NULL,
  name           text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 100),
  slug           citext NOT NULL CHECK (slug::text ~ '^[a-z0-9][a-z0-9-]{0,62}$'),
  kind           text NOT NULL CHECK (kind IN ('postgresql', 'mysql', 'sqlserver', 'oracle', 'rest', 'graphql',
                                               'file', 's3', 'google_sheets')),
  config         jsonb NOT NULL DEFAULT '{}'::jsonb,
  secret_id      uuid,
  status         text NOT NULL DEFAULT 'enabled' CHECK (status IN ('enabled', 'disabled')),
  created_by     uuid,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, id),
  UNIQUE (org_id, workspace_id, slug),
  FOREIGN KEY (org_id, workspace_id) REFERENCES workspaces (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, environment_id) REFERENCES environments (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, secret_id) REFERENCES secrets (org_id, id) ON DELETE RESTRICT
);
CREATE TRIGGER data_sources_touch BEFORE UPDATE ON data_sources
  FOR EACH ROW EXECUTE FUNCTION hugi_touch_updated_at();

CREATE TABLE data_source_schemas (
  id             uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id         uuid NOT NULL,
  data_source_id uuid NOT NULL,
  schema         jsonb NOT NULL DEFAULT '{"objects":[]}'::jsonb,
  checksum       text NOT NULL DEFAULT '' CHECK (length(checksum) <= 64),
  captured_at    timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (org_id, data_source_id) REFERENCES data_sources (org_id, id) ON DELETE CASCADE
);
CREATE INDEX data_source_schemas_recent ON data_source_schemas (org_id, data_source_id, captured_at DESC);

CREATE TABLE ingestion_pipelines (
  id               uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id           uuid NOT NULL,
  workspace_id     uuid NOT NULL,
  data_source_id   uuid NOT NULL,
  name             text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 100),
  slug             citext NOT NULL CHECK (slug::text ~ '^[a-z0-9][a-z0-9-]{0,62}$'),
  target_table     text NOT NULL CHECK (target_table ~ '^[a-z][a-z0-9_]{1,62}$'),
  mode             text NOT NULL CHECK (mode IN ('full', 'incremental')),
  source_object    text NOT NULL DEFAULT '' CHECK (length(source_object) <= 200),
  watermark_column text NOT NULL DEFAULT '' CHECK (length(watermark_column) <= 100),
  watermark_type   text NOT NULL DEFAULT 'timestamp' CHECK (watermark_type IN ('timestamp', 'integer', 'text')),
  schedule_cron    text NOT NULL DEFAULT '' CHECK (length(schedule_cron) <= 100),
  config           jsonb NOT NULL DEFAULT '{}'::jsonb,
  enabled          boolean NOT NULL DEFAULT true,
  last_run_at      timestamptz,
  next_run_at      timestamptz,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, id),
  UNIQUE (org_id, workspace_id, slug),
  FOREIGN KEY (org_id, workspace_id) REFERENCES workspaces (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, data_source_id) REFERENCES data_sources (org_id, id) ON DELETE CASCADE
);
CREATE TRIGGER ingestion_pipelines_touch BEFORE UPDATE ON ingestion_pipelines
  FOR EACH ROW EXECUTE FUNCTION hugi_touch_updated_at();
CREATE INDEX ingestion_pipelines_due ON ingestion_pipelines (next_run_at) WHERE enabled AND next_run_at IS NOT NULL;

CREATE TABLE ingestion_runs (
  id                uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id            uuid NOT NULL,
  pipeline_id       uuid NOT NULL,
  status            text NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'running', 'succeeded', 'failed', 'cancelled')),
  mode              text NOT NULL CHECK (mode IN ('full', 'incremental')),
  watermark_before  text NOT NULL DEFAULT '',
  watermark_after   text NOT NULL DEFAULT '',
  rows_read         bigint NOT NULL DEFAULT 0 CHECK (rows_read >= 0),
  rows_written      bigint NOT NULL DEFAULT 0 CHECK (rows_written >= 0),
  schema_drift      jsonb,
  error             jsonb,
  requested_by      uuid,
  started_at        timestamptz,
  finished_at       timestamptz,
  created_at        timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (org_id, pipeline_id) REFERENCES ingestion_pipelines (org_id, id) ON DELETE CASCADE
);
CREATE INDEX ingestion_runs_pipeline ON ingestion_runs (org_id, pipeline_id, created_at DESC);

CREATE TABLE datasets (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id       uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
  workspace_id uuid NOT NULL,
  name         text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 100),
  slug         citext NOT NULL CHECK (slug::text ~ '^[a-z0-9][a-z0-9-]{0,62}$'),
  description  text NOT NULL DEFAULT '' CHECK (length(description) <= 1000),
  owner        text NOT NULL DEFAULT '' CHECK (length(owner) <= 100),
  draft_sql    text NOT NULL DEFAULT 'SELECT 1 AS ok' CHECK (length(draft_sql) <= 100000),
  status       text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
  created_by   uuid,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, id),
  UNIQUE (org_id, workspace_id, slug),
  FOREIGN KEY (org_id, workspace_id) REFERENCES workspaces (org_id, id) ON DELETE CASCADE
);
CREATE TRIGGER datasets_touch BEFORE UPDATE ON datasets
  FOR EACH ROW EXECUTE FUNCTION hugi_touch_updated_at();

CREATE TABLE dataset_versions (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id       uuid NOT NULL,
  dataset_id   uuid NOT NULL,
  version      integer NOT NULL CHECK (version >= 1),
  sql          text NOT NULL CHECK (length(sql) BETWEEN 1 AND 100000),
  columns      jsonb NOT NULL DEFAULT '[]'::jsonb,
  checksum     text NOT NULL CHECK (checksum ~ '^[a-f0-9]{64}$'),
  published_by uuid NOT NULL,
  published_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, dataset_id, version),
  UNIQUE (org_id, id),
  FOREIGN KEY (org_id, dataset_id) REFERENCES datasets (org_id, id) ON DELETE CASCADE
);

CREATE TABLE dataset_lineage (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id        uuid NOT NULL,
  dataset_id    uuid NOT NULL,
  upstream_kind text NOT NULL CHECK (upstream_kind IN ('datasource', 'dataset')),
  upstream_id   uuid NOT NULL,
  columns       jsonb NOT NULL DEFAULT '[]'::jsonb,
  created_at    timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, dataset_id, upstream_kind, upstream_id),
  FOREIGN KEY (org_id, dataset_id) REFERENCES datasets (org_id, id) ON DELETE CASCADE
);

CREATE TABLE dataset_column_policies (
  id             uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id         uuid NOT NULL,
  dataset_id     uuid NOT NULL,
  column_name    text NOT NULL CHECK (column_name ~ '^[A-Za-z_][A-Za-z0-9_]{0,62}$'),
  classification text NOT NULL DEFAULT 'internal'
                 CHECK (classification IN ('public', 'internal', 'confidential', 'restricted', 'pii')),
  mask           text NOT NULL DEFAULT 'none' CHECK (mask IN ('none', 'hash', 'redact', 'null')),
  created_at     timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, dataset_id, column_name),
  FOREIGN KEY (org_id, dataset_id) REFERENCES datasets (org_id, id) ON DELETE CASCADE
);

CREATE TABLE dataset_row_policies (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id       uuid NOT NULL,
  dataset_id   uuid NOT NULL,
  role_key     citext NOT NULL CHECK (role_key::text ~ '^[a-z0-9_]{2,40}$'),
  row_filter   text NOT NULL DEFAULT '' CHECK (length(row_filter) <= 4000),
  column_masks jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, dataset_id, role_key),
  FOREIGN KEY (org_id, dataset_id) REFERENCES datasets (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, role_key) REFERENCES roles (org_id, key) ON DELETE CASCADE
);

CREATE TABLE semantic_models (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id       uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
  workspace_id uuid NOT NULL,
  dataset_id   uuid NOT NULL,
  name         text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 100),
  slug         citext NOT NULL CHECK (slug::text ~ '^[a-z0-9][a-z0-9-]{0,62}$'),
  description  text NOT NULL DEFAULT '' CHECK (length(description) <= 1000),
  dimensions   jsonb NOT NULL DEFAULT '[]'::jsonb,
  status       text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, id),
  UNIQUE (org_id, workspace_id, slug),
  FOREIGN KEY (org_id, workspace_id) REFERENCES workspaces (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, dataset_id) REFERENCES datasets (org_id, id) ON DELETE CASCADE
);
CREATE TRIGGER semantic_models_touch BEFORE UPDATE ON semantic_models
  FOR EACH ROW EXECUTE FUNCTION hugi_touch_updated_at();

CREATE TABLE metrics (
  id                uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id            uuid NOT NULL,
  workspace_id      uuid NOT NULL,
  semantic_model_id uuid NOT NULL,
  name              text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 100),
  slug              citext NOT NULL CHECK (slug::text ~ '^[a-z0-9][a-z0-9-]{0,62}$'),
  description       text NOT NULL DEFAULT '' CHECK (length(description) <= 1000),
  expression        text NOT NULL CHECK (length(expression) BETWEEN 1 AND 4000),
  filters           jsonb NOT NULL DEFAULT '[]'::jsonb,
  format            text NOT NULL DEFAULT 'number' CHECK (format IN ('number', 'currency', 'percent', 'duration')),
  status            text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, id),
  UNIQUE (org_id, workspace_id, slug),
  FOREIGN KEY (org_id, workspace_id) REFERENCES workspaces (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, semantic_model_id) REFERENCES semantic_models (org_id, id) ON DELETE CASCADE
);
CREATE TRIGGER metrics_touch BEFORE UPDATE ON metrics
  FOR EACH ROW EXECUTE FUNCTION hugi_touch_updated_at();

CREATE TABLE query_runs (
  id              uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id          uuid NOT NULL,
  workspace_id    uuid NOT NULL,
  dataset_id      uuid,
  metric_id       uuid,
  principal_id    uuid NOT NULL,
  principal_type  text NOT NULL CHECK (principal_type IN ('user', 'service_account')),
  sql             text NOT NULL CHECK (length(sql) BETWEEN 1 AND 100000),
  clickhouse_id   text NOT NULL DEFAULT '' CHECK (length(clickhouse_id) <= 100),
  status          text NOT NULL DEFAULT 'running'
                  CHECK (status IN ('running', 'succeeded', 'failed', 'cancelled')),
  row_count       bigint NOT NULL DEFAULT 0 CHECK (row_count >= 0),
  bytes_read      bigint NOT NULL DEFAULT 0 CHECK (bytes_read >= 0),
  duration_ms     integer NOT NULL DEFAULT 0 CHECK (duration_ms >= 0),
  result_preview  jsonb,
  columns         jsonb NOT NULL DEFAULT '[]'::jsonb,
  error           text NOT NULL DEFAULT '' CHECK (length(error) <= 1000),
  exported        boolean NOT NULL DEFAULT false,
  created_at      timestamptz NOT NULL DEFAULT now(),
  finished_at     timestamptz,
  FOREIGN KEY (org_id, workspace_id) REFERENCES workspaces (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, dataset_id) REFERENCES datasets (org_id, id) ON DELETE SET NULL (dataset_id),
  FOREIGN KEY (org_id, metric_id) REFERENCES metrics (org_id, id) ON DELETE SET NULL (metric_id)
);
CREATE INDEX query_runs_workspace ON query_runs (org_id, workspace_id, created_at DESC);
CREATE INDEX query_runs_running ON query_runs (org_id) WHERE status = 'running';

CREATE TABLE dashboards (
  id               uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id           uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
  workspace_id     uuid NOT NULL,
  name             text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 100),
  slug             citext NOT NULL CHECK (slug::text ~ '^[a-z0-9][a-z0-9-]{0,62}$'),
  description      text NOT NULL DEFAULT '' CHECK (length(description) <= 1000),
  draft            jsonb NOT NULL DEFAULT '{"schema_version":1,"widgets":[],"filters":[]}'::jsonb,
  status           text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
  share_token_hash text NOT NULL DEFAULT '' CHECK (length(share_token_hash) <= 64),
  share_enabled    boolean NOT NULL DEFAULT false,
  share_expires_at timestamptz,
  created_by       uuid,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, id),
  UNIQUE (org_id, workspace_id, slug),
  FOREIGN KEY (org_id, workspace_id) REFERENCES workspaces (org_id, id) ON DELETE CASCADE
);
CREATE TRIGGER dashboards_touch BEFORE UPDATE ON dashboards
  FOR EACH ROW EXECUTE FUNCTION hugi_touch_updated_at();

CREATE TABLE dashboard_versions (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id       uuid NOT NULL,
  dashboard_id uuid NOT NULL,
  version      integer NOT NULL CHECK (version >= 1),
  definition   jsonb NOT NULL,
  checksum     text NOT NULL CHECK (checksum ~ '^[a-f0-9]{64}$'),
  published_by uuid NOT NULL,
  published_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, dashboard_id, version),
  UNIQUE (org_id, id),
  FOREIGN KEY (org_id, dashboard_id) REFERENCES dashboards (org_id, id) ON DELETE CASCADE
);

CREATE TABLE scheduled_reports (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id       uuid NOT NULL,
  workspace_id uuid NOT NULL,
  dashboard_id uuid NOT NULL,
  name         text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 100),
  schedule_cron text NOT NULL CHECK (length(schedule_cron) BETWEEN 1 AND 100),
  recipients   jsonb NOT NULL DEFAULT '[]'::jsonb,
  format       text NOT NULL DEFAULT 'csv' CHECK (format IN ('csv', 'json')),
  enabled      boolean NOT NULL DEFAULT true,
  next_fire_at timestamptz,
  last_run_at  timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, id),
  FOREIGN KEY (org_id, dashboard_id) REFERENCES dashboards (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, workspace_id) REFERENCES workspaces (org_id, id) ON DELETE CASCADE
);
CREATE TRIGGER scheduled_reports_touch BEFORE UPDATE ON scheduled_reports
  FOR EACH ROW EXECUTE FUNCTION hugi_touch_updated_at();
CREATE INDEX scheduled_reports_due ON scheduled_reports (next_fire_at) WHERE enabled AND next_fire_at IS NOT NULL;

ALTER TABLE data_sources ENABLE ROW LEVEL SECURITY;
ALTER TABLE data_sources FORCE ROW LEVEL SECURITY;
ALTER TABLE data_source_schemas ENABLE ROW LEVEL SECURITY;
ALTER TABLE data_source_schemas FORCE ROW LEVEL SECURITY;
ALTER TABLE ingestion_pipelines ENABLE ROW LEVEL SECURITY;
ALTER TABLE ingestion_pipelines FORCE ROW LEVEL SECURITY;
ALTER TABLE ingestion_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE ingestion_runs FORCE ROW LEVEL SECURITY;
ALTER TABLE datasets ENABLE ROW LEVEL SECURITY;
ALTER TABLE datasets FORCE ROW LEVEL SECURITY;
ALTER TABLE dataset_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE dataset_versions FORCE ROW LEVEL SECURITY;
ALTER TABLE dataset_lineage ENABLE ROW LEVEL SECURITY;
ALTER TABLE dataset_lineage FORCE ROW LEVEL SECURITY;
ALTER TABLE dataset_column_policies ENABLE ROW LEVEL SECURITY;
ALTER TABLE dataset_column_policies FORCE ROW LEVEL SECURITY;
ALTER TABLE dataset_row_policies ENABLE ROW LEVEL SECURITY;
ALTER TABLE dataset_row_policies FORCE ROW LEVEL SECURITY;
ALTER TABLE semantic_models ENABLE ROW LEVEL SECURITY;
ALTER TABLE semantic_models FORCE ROW LEVEL SECURITY;
ALTER TABLE metrics ENABLE ROW LEVEL SECURITY;
ALTER TABLE metrics FORCE ROW LEVEL SECURITY;
ALTER TABLE query_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE query_runs FORCE ROW LEVEL SECURITY;
ALTER TABLE dashboards ENABLE ROW LEVEL SECURITY;
ALTER TABLE dashboards FORCE ROW LEVEL SECURITY;
ALTER TABLE dashboard_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE dashboard_versions FORCE ROW LEVEL SECURITY;
ALTER TABLE scheduled_reports ENABLE ROW LEVEL SECURITY;
ALTER TABLE scheduled_reports FORCE ROW LEVEL SECURITY;

CREATE POLICY data_sources_tenant ON data_sources TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY data_source_schemas_tenant ON data_source_schemas TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY ingestion_pipelines_tenant ON ingestion_pipelines TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY ingestion_runs_tenant ON ingestion_runs TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY datasets_tenant ON datasets TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY dataset_versions_tenant ON dataset_versions TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY dataset_lineage_tenant ON dataset_lineage TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY dataset_column_policies_tenant ON dataset_column_policies TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY dataset_row_policies_tenant ON dataset_row_policies TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY semantic_models_tenant ON semantic_models TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY metrics_tenant ON metrics TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY query_runs_tenant ON query_runs TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY dashboards_tenant ON dashboards TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY dashboard_versions_tenant ON dashboard_versions TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY scheduled_reports_tenant ON scheduled_reports TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());

-- The ingestion scheduler and scheduled-report runner work across tenants.
CREATE POLICY ingestion_pipelines_engine ON ingestion_pipelines TO hugi_system
  USING (true) WITH CHECK (true);
CREATE POLICY ingestion_runs_engine ON ingestion_runs TO hugi_system
  USING (true) WITH CHECK (true);
CREATE POLICY scheduled_reports_engine ON scheduled_reports TO hugi_system
  USING (true) WITH CHECK (true);

GRANT SELECT, INSERT, UPDATE, DELETE ON data_sources, ingestion_pipelines, datasets,
  semantic_models, metrics, dashboards, dataset_column_policies, dataset_row_policies TO hugi_app;
GRANT SELECT, INSERT ON data_source_schemas, dataset_versions, dashboard_versions TO hugi_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ingestion_runs, query_runs TO hugi_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON scheduled_reports TO hugi_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON dataset_lineage TO hugi_app;
GRANT SELECT, INSERT, UPDATE ON ingestion_pipelines TO hugi_system;
GRANT SELECT, INSERT, UPDATE ON ingestion_runs TO hugi_system;
GRANT SELECT, UPDATE ON scheduled_reports TO hugi_system;

INSERT INTO resource_types (type, description, actions, sensitive_actions) VALUES
  ('datasource', 'External data sources and their discovered schemas.',
   '{read,create,update,delete,test,ingest}', '{create,update,delete,test,ingest}'),
  ('dataset', 'Versioned analytical datasets and their governance policies.',
   '{read,create,update,delete,publish,query,export}', '{create,update,delete,publish,export}'),
  ('semantic_model', 'Semantic models over datasets.', '{read,create,update,delete}', '{create,update,delete}'),
  ('metric', 'Metrics defined once in the semantic layer.', '{read,create,update,delete}', '{create,update,delete}'),
  ('dashboard', 'Dashboards, versions and shares.', '{read,create,update,delete,publish,share}',
   '{create,update,delete,publish,share}'),
  ('query', 'SQL workbench query runs and exports.', '{read,execute,cancel,export}', '{execute,export}');
INSERT INTO role_template_permissions (template_key, permission) VALUES
  ('org_admin', 'datasource:*'), ('org_admin', 'dataset:*'), ('org_admin', 'semantic_model:*'),
  ('org_admin', 'metric:*'), ('org_admin', 'dashboard:*'), ('org_admin', 'query:*'),
  ('workspace_admin', 'datasource:*'), ('workspace_admin', 'dataset:*'), ('workspace_admin', 'semantic_model:*'),
  ('workspace_admin', 'metric:*'), ('workspace_admin', 'dashboard:*'), ('workspace_admin', 'query:*'),
  ('editor', 'datasource:read'), ('editor', 'dataset:read'), ('editor', 'dataset:query'),
  ('editor', 'semantic_model:read'), ('editor', 'metric:read'), ('editor', 'dashboard:read'),
  ('editor', 'dashboard:create'), ('editor', 'dashboard:update'), ('editor', 'query:read'),
  ('editor', 'query:execute'),
  ('member', 'dataset:read'), ('member', 'semantic_model:read'), ('member', 'metric:read'),
  ('member', 'dashboard:read'), ('member', 'query:read'),
  ('viewer', 'dataset:read'), ('viewer', 'semantic_model:read'), ('viewer', 'metric:read'),
  ('viewer', 'dashboard:read'), ('viewer', 'query:read');
INSERT INTO role_permissions (org_id, role_id, permission)
SELECT r.org_id, r.id, p.permission
FROM roles r
JOIN role_template_permissions tp ON tp.template_key = r.key::text AND r.is_system
CROSS JOIN LATERAL (VALUES (tp.permission)) p(permission)
WHERE r.key IN ('org_admin', 'workspace_admin', 'editor', 'member', 'viewer')
ON CONFLICT DO NOTHING;
UPDATE organizations SET authz_version = authz_version + 1;

-- +goose Down
DELETE FROM role_permissions rp USING roles r
WHERE rp.role_id = r.id AND r.is_system AND r.key IN ('org_admin', 'workspace_admin', 'editor', 'member', 'viewer')
  AND (rp.permission LIKE 'datasource:%' OR rp.permission LIKE 'dataset:%'
       OR rp.permission LIKE 'semantic_model:%' OR rp.permission LIKE 'metric:%'
       OR rp.permission LIKE 'dashboard:%' OR rp.permission LIKE 'query:%');
DELETE FROM role_template_permissions WHERE permission LIKE 'datasource:%' OR permission LIKE 'dataset:%'
  OR permission LIKE 'semantic_model:%' OR permission LIKE 'metric:%' OR permission LIKE 'dashboard:%'
  OR permission LIKE 'query:%';
DELETE FROM resource_types WHERE type IN ('datasource', 'dataset', 'semantic_model', 'metric', 'dashboard', 'query');
UPDATE organizations SET authz_version = authz_version + 1;
DROP TABLE IF EXISTS scheduled_reports;
DROP TABLE IF EXISTS dashboard_versions;
DROP TABLE IF EXISTS dashboards;
DROP TABLE IF EXISTS query_runs;
DROP TABLE IF EXISTS metrics;
DROP TABLE IF EXISTS semantic_models;
DROP TABLE IF EXISTS dataset_row_policies;
DROP TABLE IF EXISTS dataset_column_policies;
DROP TABLE IF EXISTS dataset_lineage;
DROP TABLE IF EXISTS dataset_versions;
DROP TABLE IF EXISTS datasets;
DROP TABLE IF EXISTS ingestion_runs;
DROP TABLE IF EXISTS ingestion_pipelines;
DROP TABLE IF EXISTS data_source_schemas;
DROP TABLE IF EXISTS data_sources;
