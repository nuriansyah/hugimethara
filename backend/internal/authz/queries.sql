-- Authorization: roles, permissions, bindings, decisions.

-- name: LoadGrants
-- Every permission pattern a user holds, with the scope it applies at.
SELECT rp.permission, rb.scope_type, rb.workspace_id, rb.resource_type, rb.resource_id, r.key::text AS role_key, rb.id AS binding_id
FROM role_bindings rb
JOIN roles r ON r.id = rb.role_id
JOIN role_permissions rp ON rp.role_id = r.id
JOIN org_memberships m ON m.org_id = rb.org_id AND m.user_id = rb.user_id AND m.status = 'active'
WHERE rb.user_id = $1 AND (rb.expires_at IS NULL OR rb.expires_at > now());

-- name: GetVersion
SELECT authz_version FROM organizations WHERE id = $1;

-- name: BumpVersion
UPDATE organizations SET authz_version = authz_version + 1 WHERE id = $1 RETURNING authz_version;

-- name: ListResourceTypes
SELECT type, description, array_to_json(actions)::text AS actions,
       array_to_json(sensitive_actions)::text AS sensitive_actions
FROM resource_types ORDER BY type;

-- name: InsertDecision
INSERT INTO authz_decisions (org_id, principal_id, action, resource_type, resource_id, workspace_id,
                             allowed, reason, matched_rules, authz_version, request_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);

-- name: ListDecisions
SELECT d.id, d.occurred_at, d.principal_id, u.email::text AS principal_email, d.action, d.resource_type,
       d.resource_id, d.workspace_id, d.allowed, d.reason, array_to_json(d.matched_rules)::text AS matched_rules, d.request_id
FROM authz_decisions d LEFT JOIN users u ON u.id = d.principal_id
WHERE ($1::boolean IS NULL OR d.allowed = $1)
  AND ($2::timestamptz IS NULL OR (d.occurred_at, d.id) < ($2, $3::uuid))
ORDER BY d.occurred_at DESC, d.id DESC
LIMIT $4;

-- Seeding ---------------------------------------------------------------------

-- name: SeedRoles
INSERT INTO roles (org_id, key, name, description, is_system, immutable)
SELECT $1, t.key, t.name, t.description, true, t.immutable FROM role_templates t;

-- name: SeedRolePermissions
INSERT INTO role_permissions (org_id, role_id, permission)
SELECT r.org_id, r.id, tp.permission
FROM roles r JOIN role_template_permissions tp ON tp.template_key = r.key::text
WHERE r.org_id = $1;

-- Roles ------------------------------------------------------------------------

-- name: ListRoles
SELECT r.id, r.key::text AS key, r.name, r.description, r.is_system, r.immutable, r.created_at, r.updated_at,
       COALESCE((SELECT array_to_json(array_agg(rp.permission ORDER BY rp.permission))::text
                 FROM role_permissions rp WHERE rp.role_id = r.id), '[]') AS permissions,
       (SELECT count(*) FROM role_bindings rb WHERE rb.role_id = r.id) AS binding_count
FROM roles r
ORDER BY r.is_system DESC, r.name;

-- name: GetRole
SELECT r.id, r.key::text AS key, r.name, r.description, r.is_system, r.immutable, r.created_at, r.updated_at,
       COALESCE((SELECT array_to_json(array_agg(rp.permission ORDER BY rp.permission))::text
                 FROM role_permissions rp WHERE rp.role_id = r.id), '[]') AS permissions,
       (SELECT count(*) FROM role_bindings rb WHERE rb.role_id = r.id) AS binding_count
FROM roles r WHERE r.id = $1;

-- name: GetRoleByKey
SELECT r.id, r.key::text AS key, r.name, r.description, r.is_system, r.immutable, r.created_at, r.updated_at,
       COALESCE((SELECT array_to_json(array_agg(rp.permission ORDER BY rp.permission))::text
                 FROM role_permissions rp WHERE rp.role_id = r.id), '[]') AS permissions,
       (SELECT count(*) FROM role_bindings rb WHERE rb.role_id = r.id) AS binding_count
FROM roles r WHERE r.key = $1::citext;

-- name: InsertRole
INSERT INTO roles (id, org_id, key, name, description) VALUES ($1, $2, $3, $4, $5);

-- name: UpdateRole
UPDATE roles SET name = $2, description = $3 WHERE id = $1;

-- name: DeleteRole
DELETE FROM roles WHERE id = $1;

-- name: InsertRolePermission
INSERT INTO role_permissions (org_id, role_id, permission) VALUES ($1, $2, $3);

-- name: DeleteRolePermissions
DELETE FROM role_permissions WHERE role_id = $1;

-- Bindings ---------------------------------------------------------------------

-- name: ListBindings
-- Optional filters: $1 user, $2 scope type, $3 workspace.
SELECT rb.id, rb.user_id, u.email::text AS user_email, u.display_name AS user_name,
       r.id AS role_id, r.key::text AS role_key, r.name AS role_name,
       rb.scope_type, rb.workspace_id, w.slug::text AS workspace_slug, w.name AS workspace_name,
       rb.resource_type, rb.resource_id, rb.created_at, rb.expires_at
FROM role_bindings rb
JOIN roles r ON r.id = rb.role_id
JOIN users u ON u.id = rb.user_id
LEFT JOIN workspaces w ON w.org_id = rb.org_id AND w.id = rb.workspace_id
WHERE ($1::uuid IS NULL OR rb.user_id = $1)
  AND ($2::text IS NULL OR rb.scope_type = $2)
  AND ($3::uuid IS NULL OR rb.workspace_id = $3)
ORDER BY u.display_name, rb.scope_type, r.name;

-- name: GetBinding
SELECT rb.id, rb.user_id, u.email::text AS user_email, u.display_name AS user_name,
       r.id AS role_id, r.key::text AS role_key, r.name AS role_name,
       rb.scope_type, rb.workspace_id, w.slug::text AS workspace_slug, w.name AS workspace_name,
       rb.resource_type, rb.resource_id, rb.created_at, rb.expires_at
FROM role_bindings rb
JOIN roles r ON r.id = rb.role_id
JOIN users u ON u.id = rb.user_id
LEFT JOIN workspaces w ON w.org_id = rb.org_id AND w.id = rb.workspace_id
WHERE rb.id = $1;

-- name: InsertBinding
INSERT INTO role_bindings (id, org_id, role_id, user_id, scope_type, workspace_id, resource_type, resource_id, created_by, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT DO NOTHING;

-- name: DeleteBinding
DELETE FROM role_bindings WHERE id = $1;

-- name: DeleteWorkspaceBindings
-- Removes every workspace-scope binding of a user in a workspace (leave workspace).
DELETE FROM role_bindings WHERE scope_type = 'workspace' AND workspace_id = $1 AND user_id = $2;

-- name: CountOtherOwners
-- Active org-scope owner bindings excluding binding $1.
SELECT count(*) FROM role_bindings rb
JOIN roles r ON r.id = rb.role_id
JOIN org_memberships m ON m.org_id = rb.org_id AND m.user_id = rb.user_id AND m.status = 'active'
WHERE r.key = 'org_owner' AND rb.scope_type = 'org' AND rb.id <> $1
  AND (rb.expires_at IS NULL OR rb.expires_at > now());

-- name: IsOrgOwner
SELECT EXISTS (SELECT 1 FROM role_bindings rb JOIN roles r ON r.id = rb.role_id
               WHERE rb.user_id = $1 AND rb.scope_type = 'org' AND r.key = 'org_owner'
                 AND (rb.expires_at IS NULL OR rb.expires_at > now()));
