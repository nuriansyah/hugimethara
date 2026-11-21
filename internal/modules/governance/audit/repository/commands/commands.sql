-- governance/audit repository — Write side (state-changing statements).
-- Audit trail. Append-only for the runtime role.

-- name: InsertAuditEvent
INSERT INTO audit_events (id, org_id, occurred_at, actor_type, actor_id, action, resource_type, resource_id,
                          workspace_id, outcome, request_id, ip, user_agent, details)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14);
