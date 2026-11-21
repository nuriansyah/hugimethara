-- governance/audit repository — Read side (side-effect-free statements).
-- Audit trail. Append-only for the runtime role.

-- name: ListAuditEvents
-- Keyset pagination on (occurred_at, id) descending; every filter is optional (NULL = no filter).
-- $1 action (exact or prefix), $2 actor_id, $3 resource_type, $4 since, $5 until,
-- $6 cursor time, $7 cursor id, $8 limit
SELECT e.id, e.occurred_at, e.actor_type, e.actor_id, COALESCE(u.email::text, sa.name::text) AS actor_email,
       e.action, e.resource_type, e.resource_id, e.workspace_id, e.outcome, e.request_id, host(e.ip) AS ip, e.details
FROM audit_events e
LEFT JOIN users u ON e.actor_type = 'user' AND u.id = e.actor_id
LEFT JOIN service_accounts sa ON e.actor_type = 'service_account' AND sa.id = e.actor_id
WHERE ($1::text IS NULL OR e.action = $1 OR e.action LIKE $1 || '.%')
  AND ($2::uuid IS NULL OR e.actor_id = $2)
  AND ($3::text IS NULL OR e.resource_type = $3)
  AND ($4::timestamptz IS NULL OR e.occurred_at >= $4)
  AND ($5::timestamptz IS NULL OR e.occurred_at < $5)
  AND ($6::timestamptz IS NULL OR (e.occurred_at, e.id) < ($6, $7::uuid))
ORDER BY e.occurred_at DESC, e.id DESC
LIMIT $8;
