-- integration/connector repository — Write side (state-changing statements).

-- name: InsertInstance
INSERT INTO connector_instances
  (org_id, id, workspace_id, environment_id, ref_key, connector_version_id,
   config, auth_method, secret_id, rate_limit, timeout_ms)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);

-- name: UpdateInstance
UPDATE connector_instances
SET config = COALESCE($2::jsonb, config),
    auth_method = COALESCE($3, auth_method),
    secret_id = COALESCE($4, secret_id),
    rate_limit = COALESCE($5, rate_limit),
    timeout_ms = COALESCE($6, timeout_ms),
    status = COALESCE($7, status)
WHERE id = $1;

-- name: DeleteInstance
DELETE FROM connector_instances WHERE id = $1;

-- name: InsertHealth
INSERT INTO connector_health (org_id, instance_id, ok, latency_ms, error_code, message)
VALUES ($1, $2, $3, $4, $5, $6);
