-- integration/connector repository — Read side (side-effect-free statements).

-- name: ListDefinitions
SELECT id, org_id, key, name, category, tier, icon
FROM connector_definitions
ORDER BY name, key;

-- name: GetDefinition
SELECT id, org_id, key, name, category, tier, icon
FROM connector_definitions
WHERE id = $1;

-- name: ListVersions
SELECT v.id, v.definition_id, v.version, v.config_schema, v.auth_methods,
       v.operations, v.deprecated_at
FROM connector_versions v
JOIN connector_definitions d ON d.id = v.definition_id
WHERE d.id = $1
ORDER BY v.created_at DESC, v.version DESC;

-- name: GetVersion
SELECT v.id, v.definition_id, v.version, v.config_schema, v.auth_methods,
       v.operations, v.deprecated_at, d.key AS definition_key
FROM connector_versions v
JOIN connector_definitions d ON d.id = v.definition_id
WHERE v.id = $1;

-- name: ListInstances
SELECT i.id, i.workspace_id, i.environment_id, i.ref_key, i.connector_version_id,
       i.config, i.auth_method, i.secret_id, i.rate_limit, i.timeout_ms, i.status,
       i.created_at, i.updated_at, v.version, d.id AS definition_id, d.key AS definition_key,
       (i.secret_id IS NOT NULL) AS credential_configured
FROM connector_instances i
JOIN connector_versions v ON v.id = i.connector_version_id
JOIN connector_definitions d ON d.id = v.definition_id
ORDER BY d.key, i.ref_key, i.id;

-- name: GetInstance
SELECT i.id, i.workspace_id, i.environment_id, i.ref_key, i.connector_version_id,
       i.config, i.auth_method, i.secret_id, i.rate_limit, i.timeout_ms, i.status,
       i.created_at, i.updated_at, v.version, d.id AS definition_id, d.key AS definition_key,
       (i.secret_id IS NOT NULL) AS credential_configured,
       v.config_schema, v.auth_methods
FROM connector_instances i
JOIN connector_versions v ON v.id = i.connector_version_id
JOIN connector_definitions d ON d.id = v.definition_id
WHERE i.id = $1;

-- name: GetSecretForInstance
SELECT id FROM secrets
WHERE id = $1 AND org_id = $2 AND (environment_id IS NULL OR environment_id = $3);

-- name: GetEnvironmentEgressAllowlist
SELECT array_to_json(egress_allowlist)::text
FROM environments
WHERE id = $1 AND org_id = $2;

-- name: ListHealth
SELECT checked_at, ok, latency_ms, error_code, message
FROM connector_health
WHERE instance_id = $1
ORDER BY checked_at DESC, id DESC
LIMIT 50;
