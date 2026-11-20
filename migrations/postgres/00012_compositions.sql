-- +goose Up
-- S9 API composition drafts, immutable versions, and execution history.
CREATE TABLE api_compositions (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id       uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
  workspace_id uuid NOT NULL,
  name         text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 100),
  slug         citext NOT NULL CHECK (slug::text ~ '^[a-z0-9][a-z0-9-]{0,62}$'),
  draft        jsonb NOT NULL DEFAULT '{"steps":[]}'::jsonb,
  exposed      boolean NOT NULL DEFAULT false,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, id),
  UNIQUE (org_id, workspace_id, slug),
  FOREIGN KEY (org_id, workspace_id) REFERENCES workspaces (org_id, id) ON DELETE CASCADE
);
CREATE TRIGGER api_compositions_touch BEFORE UPDATE ON api_compositions
  FOR EACH ROW EXECUTE FUNCTION hugi_touch_updated_at();

CREATE TABLE api_composition_versions (
  id             uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id         uuid NOT NULL,
  composition_id uuid NOT NULL,
  version        integer NOT NULL CHECK (version >= 1),
  definition     jsonb NOT NULL,
  checksum       text NOT NULL CHECK (checksum ~ '^[a-f0-9]{64}$'),
  published_by   uuid NOT NULL,
  published_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, composition_id, version),
  UNIQUE (org_id, id),
  FOREIGN KEY (org_id, composition_id) REFERENCES api_compositions (org_id, id) ON DELETE CASCADE
);

CREATE TABLE api_composition_executions (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id       uuid NOT NULL,
  workspace_id uuid NOT NULL,
  composition_id uuid NOT NULL,
  version_id   uuid NOT NULL,
  environment_id uuid NOT NULL,
  started_at   timestamptz NOT NULL DEFAULT now(),
  duration_ms  integer NOT NULL DEFAULT 0 CHECK (duration_ms >= 0),
  status       text NOT NULL CHECK (status IN ('succeeded', 'failed')),
  step_summary jsonb NOT NULL DEFAULT '{}'::jsonb,
  output       jsonb,
  error        text CHECK (length(error) <= 1000),
  request_id   text NOT NULL DEFAULT '' CHECK (length(request_id) <= 200),
  UNIQUE (org_id, id),
  FOREIGN KEY (org_id, composition_id) REFERENCES api_compositions (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, version_id) REFERENCES api_composition_versions (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, workspace_id) REFERENCES workspaces (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, environment_id) REFERENCES environments (org_id, id) ON DELETE RESTRICT
);
CREATE INDEX api_composition_executions_workspace_started
  ON api_composition_executions (org_id, workspace_id, started_at DESC);
CREATE INDEX api_composition_executions_composition_started
  ON api_composition_executions (org_id, composition_id, started_at DESC);

ALTER TABLE api_compositions ENABLE ROW LEVEL SECURITY;
ALTER TABLE api_compositions FORCE ROW LEVEL SECURITY;
ALTER TABLE api_composition_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE api_composition_versions FORCE ROW LEVEL SECURITY;
ALTER TABLE api_composition_executions ENABLE ROW LEVEL SECURITY;
ALTER TABLE api_composition_executions FORCE ROW LEVEL SECURITY;

CREATE POLICY api_compositions_tenant ON api_compositions TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY api_composition_versions_tenant ON api_composition_versions TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY api_composition_executions_tenant ON api_composition_executions TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());

GRANT SELECT, INSERT, UPDATE, DELETE ON api_compositions, api_composition_versions,
  api_composition_executions TO hugi_app;

INSERT INTO resource_types (type, description, actions, sensitive_actions) VALUES
  ('api_composition', 'Versioned API compositions and execution history.',
   '{read,create,update,delete,publish,execute}', '{create,update,delete,publish,execute}');
INSERT INTO role_template_permissions (template_key, permission) VALUES
  ('org_admin', 'api_composition:*'), ('workspace_admin', 'api_composition:*');
INSERT INTO role_permissions (org_id, role_id, permission)
SELECT r.org_id, r.id, p.permission
FROM roles r CROSS JOIN (VALUES ('api_composition:*')) p(permission)
WHERE r.key IN ('org_admin', 'workspace_admin') AND r.is_system
ON CONFLICT DO NOTHING;
UPDATE organizations SET authz_version = authz_version + 1;

-- +goose Down
DELETE FROM role_permissions rp USING roles r
WHERE rp.role_id = r.id AND r.key IN ('org_admin', 'workspace_admin') AND r.is_system
  AND rp.permission = 'api_composition:*';
DELETE FROM role_template_permissions WHERE template_key IN ('org_admin', 'workspace_admin')
  AND permission = 'api_composition:*';
DELETE FROM resource_types WHERE type = 'api_composition';
UPDATE organizations SET authz_version = authz_version + 1;
DROP TABLE IF EXISTS api_composition_executions;
DROP TABLE IF EXISTS api_composition_versions;
DROP TABLE IF EXISTS api_compositions;
