-- authorization repository — Write side (state-changing statements).
-- Authorization: roles, permissions, bindings, decisions.

-- name: BumpVersion
UPDATE organizations SET authz_version = authz_version + 1 WHERE id = $1 RETURNING authz_version;

-- name: InsertDecision
INSERT INTO authz_decisions (org_id, principal_id, action, resource_type, resource_id, workspace_id,
                             allowed, reason, matched_rules, authz_version, request_id, principal_type)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);

-- name: SeedRoles
INSERT INTO roles (org_id, key, name, description, is_system, immutable)
SELECT $1, t.key, t.name, t.description, true, t.immutable FROM role_templates t;

-- name: SeedRolePermissions
INSERT INTO role_permissions (org_id, role_id, permission)
SELECT r.org_id, r.id, tp.permission
FROM roles r JOIN role_template_permissions tp ON tp.template_key = r.key::text
WHERE r.org_id = $1;

-- Roles ------------------------------------------------------------------------

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

-- name: InsertBinding
INSERT INTO role_bindings (id, org_id, role_id, principal_type, user_id, group_id, service_account_id,
                           scope_type, workspace_id, resource_type, resource_id, created_by, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
ON CONFLICT DO NOTHING;

-- name: DeleteBinding
DELETE FROM role_bindings WHERE id = $1;

-- name: DeleteWorkspaceBindings
-- Removes every workspace-scope binding of a user in a workspace (leave workspace).
DELETE FROM role_bindings WHERE scope_type = 'workspace' AND workspace_id = $1 AND principal_type = 'user' AND user_id = $2;
