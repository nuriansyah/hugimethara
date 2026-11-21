-- integration/service_registry repository — Write side (state-changing statements).

-- name: InsertService
INSERT INTO services (org_id, id, workspace_id, name, owner, description, protocol)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: UpdateService
UPDATE services
SET name = COALESCE($2, name), owner = COALESCE($3, owner),
    description = COALESCE($4, description), protocol = COALESCE($5, protocol)
WHERE id = $1;

-- name: DeleteService
DELETE FROM services WHERE id = $1;

-- name: UpsertServiceEnvironment
INSERT INTO service_environments (org_id, service_id, environment_id, base_url, connector_instance_id)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (service_id, environment_id) DO UPDATE
SET base_url = EXCLUDED.base_url, connector_instance_id = EXCLUDED.connector_instance_id;

-- name: DeleteServiceEnvironment
DELETE FROM service_environments WHERE service_id = $1 AND environment_id = $2;

-- name: InsertServiceSpec
INSERT INTO service_specs (org_id, id, service_id, format, version_label, sha256, document)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: DeleteServiceEndpoints
DELETE FROM service_endpoints WHERE service_id = $1;

-- name: InsertServiceEndpoint
INSERT INTO service_endpoints
  (org_id, id, service_id, spec_id, operation_id, method, path, summary,
   parameters, request_schema, response_schemas, security, deprecated)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13);
