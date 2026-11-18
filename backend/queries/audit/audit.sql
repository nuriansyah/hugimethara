-- name: InsertAuditEvent :exec
INSERT INTO audit_events (id, org_id, occurred_at, actor_type, actor_id, action, resource_type, resource_id,
                          workspace_id, outcome, request_id, ip, user_agent, details)
VALUES (@id, @org_id, @occurred_at, @actor_type, @actor_id, @action, @resource_type, @resource_id,
        @workspace_id, @outcome, @request_id, @ip, @user_agent, @details);

-- name: ListAuditEvents :many
-- Keyset pagination on (occurred_at, id) descending; optional filters.
SELECT e.id, e.occurred_at, e.actor_type, e.actor_id, u.email AS actor_email, e.action, e.resource_type,
       e.resource_id, e.workspace_id, e.outcome, e.request_id, e.ip, e.details
FROM audit_events e
LEFT JOIN users u ON u.id = e.actor_id
WHERE (sqlc.narg('action')::text IS NULL OR e.action = sqlc.narg('action') OR e.action LIKE sqlc.narg('action') || '.%')
  AND (sqlc.narg('actor_id')::uuid IS NULL OR e.actor_id = sqlc.narg('actor_id'))
  AND (sqlc.narg('resource_type')::text IS NULL OR e.resource_type = sqlc.narg('resource_type'))
  AND (sqlc.narg('since')::timestamptz IS NULL OR e.occurred_at >= sqlc.narg('since'))
  AND (sqlc.narg('until')::timestamptz IS NULL OR e.occurred_at < sqlc.narg('until'))
  AND (sqlc.narg('cursor_time')::timestamptz IS NULL
       OR (e.occurred_at, e.id) < (sqlc.narg('cursor_time'), sqlc.narg('cursor_id')::uuid))
ORDER BY e.occurred_at DESC, e.id DESC
LIMIT @page_limit;
