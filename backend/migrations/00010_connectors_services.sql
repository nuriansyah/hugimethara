-- +goose Up
-- S6 connector catalog/instances and S7 service registry. Service spec
-- documents are retained in Postgres until an object-store adapter is added.
CREATE TABLE connector_definitions (
  id          uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id      uuid REFERENCES organizations (id) ON DELETE CASCADE,
  key         text NOT NULL CHECK (key ~ '^[a-z][a-z0-9_]{1,63}$'),
  name        text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 100),
  category    text NOT NULL CHECK (length(category) BETWEEN 1 AND 64),
  tier        text NOT NULL CHECK (tier IN ('builtin', 'declarative')),
  icon        text NOT NULL DEFAULT '',
  created_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (id, org_id)
);
CREATE UNIQUE INDEX connector_definitions_global_key ON connector_definitions (key) WHERE org_id IS NULL;
CREATE UNIQUE INDEX connector_definitions_org_key ON connector_definitions (org_id, key) WHERE org_id IS NOT NULL;

CREATE TABLE connector_versions (
  id              uuid PRIMARY KEY DEFAULT uuidv7(),
  definition_id   uuid NOT NULL REFERENCES connector_definitions (id) ON DELETE CASCADE,
  version         text NOT NULL CHECK (version ~ '^[0-9]+[.][0-9]+[.][0-9]+([+-][A-Za-z0-9.-]+)?$'),
  config_schema   jsonb NOT NULL,
  auth_methods    jsonb NOT NULL DEFAULT '[]',
  operations      jsonb NOT NULL DEFAULT '[]',
  spec            jsonb,
  deprecated_at   timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (definition_id, version)
);

CREATE TABLE connector_instances (
  id                    uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id                uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
  workspace_id          uuid NOT NULL,
  environment_id        uuid NOT NULL,
  ref_key               text NOT NULL CHECK (ref_key ~ '^[a-z][a-z0-9_-]{1,63}$'),
  connector_version_id  uuid NOT NULL REFERENCES connector_versions (id) ON DELETE RESTRICT,
  config                jsonb NOT NULL DEFAULT '{}',
  auth_method           text NOT NULL DEFAULT 'none',
  secret_id             uuid,
  rate_limit            integer NOT NULL DEFAULT 60 CHECK (rate_limit BETWEEN 1 AND 10000),
  timeout_ms            integer NOT NULL DEFAULT 10000 CHECK (timeout_ms BETWEEN 100 AND 120000),
  retry_policy          jsonb NOT NULL DEFAULT '{"max_attempts":1}'::jsonb,
  status                text NOT NULL DEFAULT 'enabled' CHECK (status IN ('enabled', 'disabled')),
  created_at            timestamptz NOT NULL DEFAULT now(),
  updated_at            timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, id),
  UNIQUE (org_id, workspace_id, environment_id, ref_key),
  FOREIGN KEY (org_id, workspace_id) REFERENCES workspaces (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, environment_id) REFERENCES environments (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, secret_id) REFERENCES secrets (org_id, id) ON DELETE RESTRICT
);
CREATE TRIGGER connector_instances_touch BEFORE UPDATE ON connector_instances
  FOR EACH ROW EXECUTE FUNCTION hugi_touch_updated_at();

CREATE TABLE connector_health (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id        uuid NOT NULL,
  instance_id   uuid NOT NULL,
  checked_at    timestamptz NOT NULL DEFAULT now(),
  ok            boolean NOT NULL,
  latency_ms    integer NOT NULL CHECK (latency_ms >= 0),
  error_code    text NOT NULL DEFAULT '',
  message       text NOT NULL DEFAULT '' CHECK (length(message) <= 500),
  FOREIGN KEY (org_id, instance_id) REFERENCES connector_instances (org_id, id) ON DELETE CASCADE
);
CREATE INDEX connector_health_recent_idx ON connector_health (org_id, instance_id, checked_at DESC);

CREATE TABLE services (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id       uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
  workspace_id uuid NOT NULL,
  name         text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 100),
  owner        text NOT NULL DEFAULT '' CHECK (length(owner) <= 100),
  description  text NOT NULL DEFAULT '' CHECK (length(description) <= 1000),
  protocol     text NOT NULL DEFAULT 'rest' CHECK (protocol IN ('rest', 'graphql', 'grpc')),
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, id),
  UNIQUE (org_id, workspace_id, name),
  FOREIGN KEY (org_id, workspace_id) REFERENCES workspaces (org_id, id) ON DELETE CASCADE
);
CREATE TRIGGER services_touch BEFORE UPDATE ON services
  FOR EACH ROW EXECUTE FUNCTION hugi_touch_updated_at();

CREATE TABLE service_environments (
  org_id                uuid NOT NULL,
  service_id            uuid NOT NULL,
  environment_id        uuid NOT NULL,
  base_url              text NOT NULL CHECK (length(base_url) BETWEEN 1 AND 2000),
  connector_instance_id uuid,
  created_at            timestamptz NOT NULL DEFAULT now(),
  updated_at            timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (service_id, environment_id),
  FOREIGN KEY (org_id, service_id) REFERENCES services (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, environment_id) REFERENCES environments (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, connector_instance_id) REFERENCES connector_instances (org_id, id) ON DELETE SET NULL (connector_instance_id)
);
CREATE TRIGGER service_environments_touch BEFORE UPDATE ON service_environments
  FOR EACH ROW EXECUTE FUNCTION hugi_touch_updated_at();

CREATE TABLE service_specs (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id        uuid NOT NULL,
  service_id    uuid NOT NULL,
  format        text NOT NULL CHECK (format IN ('json', 'yaml')),
  version_label text NOT NULL DEFAULT '' CHECK (length(version_label) <= 100),
  sha256        text NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
  document      jsonb NOT NULL,
  imported_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, id),
  FOREIGN KEY (org_id, service_id) REFERENCES services (org_id, id) ON DELETE CASCADE
);

CREATE TABLE service_endpoints (
  id                uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id            uuid NOT NULL,
  service_id        uuid NOT NULL,
  spec_id           uuid NOT NULL,
  operation_id      text NOT NULL CHECK (length(operation_id) BETWEEN 1 AND 200),
  method            text NOT NULL CHECK (method IN ('GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS')),
  path              text NOT NULL CHECK (length(path) BETWEEN 1 AND 1000 AND path LIKE '/%'),
  summary           text NOT NULL DEFAULT '' CHECK (length(summary) <= 500),
  parameters        jsonb NOT NULL DEFAULT '[]',
  request_schema    jsonb,
  response_schemas  jsonb NOT NULL DEFAULT '{}',
  security          jsonb NOT NULL DEFAULT '[]',
  deprecated        boolean NOT NULL DEFAULT false,
  created_at        timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, service_id, id),
  UNIQUE (org_id, service_id, method, path),
  FOREIGN KEY (org_id, service_id) REFERENCES services (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, spec_id) REFERENCES service_specs (org_id, id) ON DELETE CASCADE
);
CREATE INDEX service_endpoints_operation_idx ON service_endpoints (org_id, service_id, operation_id);

ALTER TABLE connector_definitions ENABLE ROW LEVEL SECURITY;
ALTER TABLE connector_definitions FORCE ROW LEVEL SECURITY;
ALTER TABLE connector_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE connector_versions FORCE ROW LEVEL SECURITY;
ALTER TABLE connector_instances ENABLE ROW LEVEL SECURITY;
ALTER TABLE connector_instances FORCE ROW LEVEL SECURITY;
ALTER TABLE connector_health ENABLE ROW LEVEL SECURITY;
ALTER TABLE connector_health FORCE ROW LEVEL SECURITY;
ALTER TABLE services ENABLE ROW LEVEL SECURITY;
ALTER TABLE services FORCE ROW LEVEL SECURITY;
ALTER TABLE service_environments ENABLE ROW LEVEL SECURITY;
ALTER TABLE service_environments FORCE ROW LEVEL SECURITY;
ALTER TABLE service_specs ENABLE ROW LEVEL SECURITY;
ALTER TABLE service_specs FORCE ROW LEVEL SECURITY;
ALTER TABLE service_endpoints ENABLE ROW LEVEL SECURITY;
ALTER TABLE service_endpoints FORCE ROW LEVEL SECURITY;

CREATE POLICY connector_definitions_tenant ON connector_definitions TO hugi_app
  USING (org_id IS NULL OR org_id = hugi_current_org())
  WITH CHECK (org_id IS NOT NULL AND org_id = hugi_current_org());
CREATE POLICY connector_versions_tenant ON connector_versions TO hugi_app
  USING (EXISTS (SELECT 1 FROM connector_definitions d WHERE d.id = definition_id
                 AND (d.org_id IS NULL OR d.org_id = hugi_current_org())))
  WITH CHECK (EXISTS (SELECT 1 FROM connector_definitions d WHERE d.id = definition_id
                      AND d.org_id = hugi_current_org()));
CREATE POLICY connector_instances_tenant ON connector_instances TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY connector_health_tenant ON connector_health TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY services_tenant ON services TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY service_environments_tenant ON service_environments TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY service_specs_tenant ON service_specs TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY service_endpoints_tenant ON service_endpoints TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());

GRANT SELECT, INSERT, UPDATE, DELETE ON connector_definitions, connector_versions TO hugi_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON connector_instances, connector_health TO hugi_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON services, service_environments, service_specs, service_endpoints TO hugi_app;

INSERT INTO connector_definitions (id, key, name, category, tier, icon) VALUES
  (uuidv7(), 'http_rest', 'HTTP / REST', 'http', 'builtin', 'globe'),
  (uuidv7(), 'postgresql', 'PostgreSQL', 'database', 'builtin', 'database'),
  (uuidv7(), 'mysql', 'MySQL', 'database', 'builtin', 'database'),
  (uuidv7(), 'smtp', 'SMTP', 'messaging', 'builtin', 'mail'),
  (uuidv7(), 'slack', 'Slack', 'messaging', 'builtin', 'message-square'),
  (uuidv7(), 'msteams', 'Microsoft Teams', 'messaging', 'builtin', 'message-square'),
  (uuidv7(), 's3', 'S3 compatible storage', 'storage', 'builtin', 'archive');

INSERT INTO connector_versions (definition_id, version, config_schema, auth_methods, operations)
SELECT d.id, '1.0.0',
       CASE d.key
         WHEN 'http_rest' THEN '{"type":"object","properties":{"base_url":{"type":"string","format":"uri"}}, "required":["base_url"],"additionalProperties":false}'::jsonb
         WHEN 'postgresql' THEN '{"type":"object","properties":{"host":{"type":"string"},"port":{"type":"integer","minimum":1,"maximum":65535},"database":{"type":"string"}}, "required":["host","port","database"],"additionalProperties":false}'::jsonb
         WHEN 'mysql' THEN '{"type":"object","properties":{"host":{"type":"string"},"port":{"type":"integer","minimum":1,"maximum":65535},"database":{"type":"string"}}, "required":["host","port","database"],"additionalProperties":false}'::jsonb
         WHEN 'smtp' THEN '{"type":"object","properties":{"host":{"type":"string"},"port":{"type":"integer","minimum":1,"maximum":65535},"start_tls":{"type":"boolean"}}, "required":["host","port"],"additionalProperties":false}'::jsonb
         WHEN 'slack' THEN '{"type":"object","properties":{"webhook_url":{"type":"string","format":"uri"}}, "required":["webhook_url"],"additionalProperties":false}'::jsonb
         WHEN 'msteams' THEN '{"type":"object","properties":{"webhook_url":{"type":"string","format":"uri"}}, "required":["webhook_url"],"additionalProperties":false}'::jsonb
         ELSE '{"type":"object","properties":{"endpoint":{"type":"string","format":"uri"},"bucket":{"type":"string"},"region":{"type":"string"}}, "required":["endpoint","bucket"],"additionalProperties":false}'::jsonb
       END,
       CASE WHEN d.key IN ('http_rest', 'slack', 'msteams', 's3')
            THEN '["none","bearer"]'::jsonb
            ELSE '["none","username_password"]'::jsonb END,
       CASE WHEN d.key = 'http_rest' THEN '[{"key":"request","name":"HTTP request"}]'::jsonb
            ELSE '[]'::jsonb END
FROM connector_definitions d WHERE d.org_id IS NULL;

INSERT INTO resource_types (type, description, actions, sensitive_actions) VALUES
  ('connector', 'Connector catalog and configured instances.', '{read,create,update,delete,test,invoke,define}', '{create,update,delete,test,invoke,define}'),
  ('service', 'Registered APIs, environments and imported OpenAPI specifications.', '{read,create,update,delete,invoke}', '{create,update,delete,invoke}');
INSERT INTO role_template_permissions (template_key, permission) VALUES
  ('org_admin', 'connector:*'), ('org_admin', 'service:*'),
  ('workspace_admin', 'connector:*'), ('workspace_admin', 'service:*');
INSERT INTO role_permissions (org_id, role_id, permission)
SELECT r.org_id, r.id, p.permission
FROM roles r CROSS JOIN (VALUES ('connector:*'), ('service:*')) p(permission)
WHERE r.key IN ('org_admin', 'workspace_admin') AND r.is_system
ON CONFLICT DO NOTHING;
UPDATE organizations SET authz_version = authz_version + 1;

-- +goose Down
DELETE FROM role_permissions rp USING roles r
WHERE rp.role_id = r.id AND r.key IN ('org_admin', 'workspace_admin') AND r.is_system
  AND rp.permission IN ('connector:*', 'service:*');
DELETE FROM role_template_permissions WHERE template_key IN ('org_admin', 'workspace_admin')
  AND permission IN ('connector:*', 'service:*');
DELETE FROM resource_types WHERE type IN ('connector', 'service');
UPDATE organizations SET authz_version = authz_version + 1;
DROP TABLE IF EXISTS service_endpoints;
DROP TABLE IF EXISTS service_specs;
DROP TABLE IF EXISTS service_environments;
DROP TABLE IF EXISTS services;
DROP TABLE IF EXISTS connector_health;
DROP TABLE IF EXISTS connector_instances;
DROP TABLE IF EXISTS connector_versions;
DROP TABLE IF EXISTS connector_definitions;
