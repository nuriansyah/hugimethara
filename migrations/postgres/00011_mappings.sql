-- +goose Up
-- S8 mapping engine and studio.
CREATE TABLE mappings (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id       uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
  workspace_id uuid NOT NULL,
  name         text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 100),
  draft        jsonb NOT NULL DEFAULT '{"rules":[]}'::jsonb,
  input_schema jsonb,
  output_schema jsonb,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, id),
  UNIQUE (org_id, workspace_id, name),
  FOREIGN KEY (org_id, workspace_id) REFERENCES workspaces (org_id, id) ON DELETE CASCADE
);
CREATE TRIGGER mappings_touch BEFORE UPDATE ON mappings
  FOR EACH ROW EXECUTE FUNCTION hugi_touch_updated_at();

CREATE TABLE mapping_versions (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id       uuid NOT NULL,
  mapping_id   uuid NOT NULL,
  version      integer NOT NULL CHECK (version >= 1),
  definition   jsonb NOT NULL,
  checksum     text NOT NULL CHECK (checksum ~ '^[a-f0-9]{64}$'),
  published_by uuid NOT NULL,
  published_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, mapping_id, version),
  UNIQUE (org_id, id),
  FOREIGN KEY (org_id, mapping_id) REFERENCES mappings (org_id, id) ON DELETE CASCADE
);

CREATE TABLE mapping_tests (
  id         uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id     uuid NOT NULL,
  mapping_id uuid NOT NULL,
  name       text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 100),
  input      jsonb NOT NULL,
  expected   jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, id),
  UNIQUE (org_id, mapping_id, name),
  FOREIGN KEY (org_id, mapping_id) REFERENCES mappings (org_id, id) ON DELETE CASCADE
);
CREATE TRIGGER mapping_tests_touch BEFORE UPDATE ON mapping_tests
  FOR EACH ROW EXECUTE FUNCTION hugi_touch_updated_at();

ALTER TABLE mappings ENABLE ROW LEVEL SECURITY;
ALTER TABLE mappings FORCE ROW LEVEL SECURITY;
ALTER TABLE mapping_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE mapping_versions FORCE ROW LEVEL SECURITY;
ALTER TABLE mapping_tests ENABLE ROW LEVEL SECURITY;
ALTER TABLE mapping_tests FORCE ROW LEVEL SECURITY;

CREATE POLICY mappings_tenant ON mappings TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY mapping_versions_tenant ON mapping_versions TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY mapping_tests_tenant ON mapping_tests TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());

GRANT SELECT, INSERT, UPDATE, DELETE ON mappings, mapping_versions, mapping_tests TO hugi_app;

INSERT INTO resource_types (type, description, actions, sensitive_actions) VALUES
  ('mapping', 'Versioned JSON transformation mappings and test cases.', '{read,create,update,delete,publish,test}', '{create,update,delete,publish,test}');
INSERT INTO role_template_permissions (template_key, permission) VALUES
  ('org_admin', 'mapping:*'), ('workspace_admin', 'mapping:*');
INSERT INTO role_permissions (org_id, role_id, permission)
SELECT r.org_id, r.id, p.permission
FROM roles r CROSS JOIN (VALUES ('mapping:*')) p(permission)
WHERE r.key IN ('org_admin', 'workspace_admin') AND r.is_system
ON CONFLICT DO NOTHING;
UPDATE organizations SET authz_version = authz_version + 1;

-- +goose Down
DELETE FROM role_permissions rp USING roles r
WHERE rp.role_id = r.id AND r.key IN ('org_admin', 'workspace_admin') AND r.is_system
  AND rp.permission = 'mapping:*';
DELETE FROM role_template_permissions WHERE template_key IN ('org_admin', 'workspace_admin')
  AND permission = 'mapping:*';
DELETE FROM resource_types WHERE type = 'mapping';
UPDATE organizations SET authz_version = authz_version + 1;
DROP TABLE IF EXISTS mapping_tests;
DROP TABLE IF EXISTS mapping_versions;
DROP TABLE IF EXISTS mappings;
