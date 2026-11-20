-- +goose Up
-- S6 secret vault: one randomly generated DEK per organization, wrapped by
-- the configured master key; secret values use row-bound AES-256-GCM AAD.
CREATE TABLE data_keys (
  org_id      uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
  id          uuid NOT NULL DEFAULT uuidv7(),
  kek_id      text NOT NULL CHECK (length(kek_id) BETWEEN 1 AND 100),
  wrapped_key bytea NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  retired_at  timestamptz,
  PRIMARY KEY (org_id, id),
  UNIQUE (org_id)
);

CREATE TABLE secrets (
  org_id         uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
  id             uuid NOT NULL DEFAULT uuidv7(),
  environment_id uuid,
  kind           text NOT NULL CHECK (length(kind) BETWEEN 1 AND 64),
  name           text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 100),
  dek_id         uuid NOT NULL,
  ciphertext     bytea NOT NULL,
  hint           text NOT NULL DEFAULT '' CHECK (length(hint) <= 16),
  version        bigint NOT NULL DEFAULT 1 CHECK (version > 0),
  rotated_at     timestamptz,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, id),
  FOREIGN KEY (org_id, dek_id) REFERENCES data_keys (org_id, id) ON DELETE RESTRICT,
  FOREIGN KEY (org_id, environment_id) REFERENCES environments (org_id, id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX secrets_org_name_global ON secrets (org_id, lower(name)) WHERE environment_id IS NULL;
CREATE UNIQUE INDEX secrets_org_name_environment ON secrets (org_id, environment_id, lower(name)) WHERE environment_id IS NOT NULL;
CREATE INDEX secrets_dek_idx ON secrets (org_id, dek_id);
CREATE TRIGGER secrets_touch BEFORE UPDATE ON secrets
  FOR EACH ROW EXECUTE FUNCTION hugi_touch_updated_at();

ALTER TABLE data_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE data_keys FORCE ROW LEVEL SECURITY;
ALTER TABLE secrets ENABLE ROW LEVEL SECURITY;
ALTER TABLE secrets FORCE ROW LEVEL SECURITY;
CREATE POLICY data_keys_tenant ON data_keys TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY secrets_tenant ON secrets TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
GRANT SELECT, INSERT ON data_keys TO hugi_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON secrets TO hugi_app;

INSERT INTO resource_types (type, description, actions, sensitive_actions) VALUES
  ('secret', 'Encrypted credential values and metadata.', '{read,create,update,delete,use}', '{create,update,delete,use}');
INSERT INTO role_template_permissions (template_key, permission) VALUES ('org_admin', 'secret:*');
INSERT INTO role_permissions (org_id, role_id, permission)
SELECT r.org_id, r.id, 'secret:*'
FROM roles r WHERE r.key = 'org_admin' AND r.is_system
ON CONFLICT DO NOTHING;
UPDATE organizations SET authz_version = authz_version + 1;

-- +goose Down
DELETE FROM role_permissions rp USING roles r
WHERE rp.role_id = r.id AND r.key = 'org_admin' AND r.is_system AND rp.permission = 'secret:*';
DELETE FROM role_template_permissions WHERE template_key = 'org_admin' AND permission = 'secret:*';
DELETE FROM resource_types WHERE type = 'secret';
UPDATE organizations SET authz_version = authz_version + 1;
DROP TABLE IF EXISTS secrets;
DROP TABLE IF EXISTS data_keys;
