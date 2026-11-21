-- integration/service_registry repository — Read side (side-effect-free statements).

-- name: ListServices
SELECT s.id, s.workspace_id, s.name, s.owner, s.description, s.protocol,
       s.created_at, s.updated_at,
       (SELECT count(*) FROM service_endpoints ep WHERE ep.org_id = s.org_id AND ep.service_id = s.id) AS endpoint_count
FROM services s
WHERE ($1::uuid[] IS NULL OR s.workspace_id = ANY($1))
ORDER BY s.name, s.id;

-- name: GetService
SELECT s.id, s.workspace_id, s.name, s.owner, s.description, s.protocol,
       s.created_at, s.updated_at,
       (SELECT count(*) FROM service_endpoints ep WHERE ep.org_id = s.org_id AND ep.service_id = s.id) AS endpoint_count
FROM services s WHERE s.id = $1;

-- name: ListServiceEnvironments
SELECT environment_id, base_url, connector_instance_id
FROM service_environments WHERE service_id = $1 ORDER BY environment_id;

-- name: GetServiceEnvironment
SELECT environment_id, base_url, connector_instance_id
FROM service_environments WHERE service_id = $1 AND environment_id = $2;

-- name: ListServiceEndpoints
SELECT id, service_id, operation_id, method, path, summary, parameters,
       COALESCE(request_schema, 'null'::jsonb) AS request_schema,
       response_schemas, security, deprecated
FROM service_endpoints WHERE service_id = $1
ORDER BY path, method, operation_id;

-- name: ListServiceEndpointsForDiff
SELECT operation_id, method, path, summary, parameters,
       COALESCE(request_schema, 'null'::jsonb) AS request_schema,
       response_schemas, security, deprecated
FROM service_endpoints WHERE service_id = $1;

-- name: GetEndpointForTry
SELECT e.id, e.service_id, e.method, e.path, s.workspace_id, env.base_url,
       to_json(runtime_env.egress_allowlist) AS egress_allowlist
FROM service_endpoints e
JOIN services s ON s.id = e.service_id
JOIN service_environments env ON env.service_id = e.service_id AND env.environment_id = $2
JOIN environments runtime_env ON runtime_env.id = env.environment_id AND runtime_env.org_id = env.org_id
WHERE e.id = $1;
