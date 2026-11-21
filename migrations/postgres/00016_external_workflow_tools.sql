-- +goose Up
-- S13 external workflow tools: n8n, Make and Zapier are built-in connectors
-- with one operation (trigger_workflow: POST the input to the tool's webhook),
-- and their inbound status callbacks use token-header verifier presets on the
-- ordinary webhook endpoints. Nothing in the platform depends on these tools
-- being reachable; only workflow steps that call them are affected.
INSERT INTO connector_definitions (key, name, category, tier, icon) VALUES
  ('n8n', 'n8n', 'automation', 'builtin', 'workflow'),
  ('make', 'Make', 'automation', 'builtin', 'workflow'),
  ('zapier', 'Zapier', 'automation', 'builtin', 'workflow');

INSERT INTO connector_versions (definition_id, version, config_schema, auth_methods, operations)
SELECT d.id, '1.0.0',
       CASE d.key
         WHEN 'zapier' THEN '{"type":"object","properties":{"webhook_url":{"type":"string","format":"uri"}},"required":["webhook_url"],"additionalProperties":false}'::jsonb
         ELSE '{"type":"object","properties":{"webhook_url":{"type":"string","format":"uri"},"auth_header":{"type":"string"}},"required":["webhook_url"],"additionalProperties":false}'::jsonb
       END,
       CASE d.key WHEN 'zapier' THEN '["none"]'::jsonb ELSE '["none","header"]'::jsonb END,
       '[{"key":"trigger_workflow","name":"Trigger workflow","description":"POSTs the input as JSON to the webhook URL; the response body is the output."}]'::jsonb
FROM connector_definitions d WHERE d.org_id IS NULL AND d.key IN ('n8n', 'make', 'zapier');

ALTER TABLE webhook_endpoints DROP CONSTRAINT webhook_endpoints_verifier_check;
ALTER TABLE webhook_endpoints ADD CONSTRAINT webhook_endpoints_verifier_check
  CHECK (verifier IN ('standard_webhooks', 'hmac_sha256', 'stripe', 'github', 'slack', 'n8n', 'make', 'zapier'));

-- The workflow engine (hugi_system) resolves an action.connector step's
-- instance and credential inside the step's tenant transaction. Unlike the
-- engine's other policies these stay tenant-bound: the engine only ever reads
-- the connector of the tenant whose step it holds.
CREATE POLICY connector_definitions_engine ON connector_definitions TO hugi_system
  USING (org_id IS NULL OR org_id = hugi_current_org());
CREATE POLICY connector_versions_engine ON connector_versions TO hugi_system
  USING (EXISTS (SELECT 1 FROM connector_definitions d WHERE d.id = definition_id
                 AND (d.org_id IS NULL OR d.org_id = hugi_current_org())));
CREATE POLICY connector_instances_engine ON connector_instances TO hugi_system
  USING (org_id = hugi_current_org());
CREATE POLICY secrets_engine ON secrets TO hugi_system
  USING (org_id = hugi_current_org());
CREATE POLICY data_keys_engine ON data_keys TO hugi_system
  USING (org_id = hugi_current_org());
GRANT SELECT (id, org_id, key) ON connector_definitions TO hugi_system;
GRANT SELECT (id, definition_id, operations) ON connector_versions TO hugi_system;
GRANT SELECT (id, org_id, workspace_id, environment_id, ref_key, connector_version_id, config, auth_method,
              secret_id, timeout_ms, status) ON connector_instances TO hugi_system;
GRANT SELECT (org_id, id, dek_id, ciphertext) ON secrets TO hugi_system;
GRANT SELECT (org_id, id, kek_id, wrapped_key) ON data_keys TO hugi_system;

-- +goose Down
REVOKE SELECT ON data_keys, secrets, connector_instances, connector_versions, connector_definitions FROM hugi_system;
DROP POLICY IF EXISTS data_keys_engine ON data_keys;
DROP POLICY IF EXISTS secrets_engine ON secrets;
DROP POLICY IF EXISTS connector_instances_engine ON connector_instances;
DROP POLICY IF EXISTS connector_versions_engine ON connector_versions;
DROP POLICY IF EXISTS connector_definitions_engine ON connector_definitions;
DELETE FROM webhook_endpoints WHERE verifier IN ('n8n', 'make', 'zapier');
ALTER TABLE webhook_endpoints DROP CONSTRAINT webhook_endpoints_verifier_check;
ALTER TABLE webhook_endpoints ADD CONSTRAINT webhook_endpoints_verifier_check
  CHECK (verifier IN ('standard_webhooks', 'hmac_sha256', 'stripe', 'github', 'slack'));
DELETE FROM connector_instances i USING connector_versions v, connector_definitions d
WHERE i.connector_version_id = v.id AND v.definition_id = d.id
  AND d.org_id IS NULL AND d.key IN ('n8n', 'make', 'zapier');
DELETE FROM connector_definitions WHERE org_id IS NULL AND key IN ('n8n', 'make', 'zapier');
