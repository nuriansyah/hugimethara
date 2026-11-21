-- authorization repository — Read side (side-effect-free statements).
-- Authorization: roles, permissions, bindings, decisions.

-- name: LoadUserGrants
-- Every permission pattern an active member holds, directly or through a group.
SELECT rp.permission, rb.scope_type, rb.workspace_id, rb.resource_type, rb.resource_id, r.key::text AS role_key,
       rb.id AS binding_id, rb.principal_type
FROM role_bindings rb
JOIN roles r ON r.id = rb.role_id
JOIN role_permissions rp ON rp.role_id = r.id
WHERE (rb.expires_at IS NULL OR rb.expires_at > now())
  AND EXISTS (SELECT 1 FROM org_memberships m WHERE m.user_id = $1 AND m.status = 'active')
  AND (rb.user_id = $1
       OR rb.group_id IN (SELECT gm.group_id FROM group_members gm WHERE gm.user_id = $1));

-- name: LoadServiceAccountGrants
SELECT rp.permission, rb.scope_type, rb.workspace_id, rb.resource_type, rb.resource_id, r.key::text AS role_key,
       rb.id AS binding_id, rb.principal_type
FROM role_bindings rb
JOIN roles r ON r.id = rb.role_id
JOIN role_permissions rp ON rp.role_id = r.id
JOIN service_accounts sa ON sa.org_id = rb.org_id AND sa.id = rb.service_account_id AND sa.disabled_at IS NULL
WHERE rb.service_account_id = $1 AND (rb.expires_at IS NULL OR rb.expires_at > now());

-- name: PrincipalBindingGrants
-- Permissions held through one principal's own bindings (what joining a group hands out).
SELECT rp.permission, rb.scope_type, rb.workspace_id, rb.resource_type, rb.resource_id, r.key::text AS role_key,
       rb.id AS binding_id, rb.principal_type
FROM role_bindings rb
JOIN roles r ON r.id = rb.role_id
JOIN role_permissions rp ON rp.role_id = r.id
WHERE rb.principal_type = $1 AND COALESCE(rb.user_id, rb.group_id, rb.service_account_id) = $2
  AND (rb.expires_at IS NULL OR rb.expires_at > now());

-- name: GetVersion
SELECT authz_version FROM organizations WHERE id = $1;

-- name: ListResourceTypes
SELECT type, description, array_to_json(actions)::text AS actions,
       array_to_json(sensitive_actions)::text AS sensitive_actions
FROM resource_types ORDER BY type;

-- name: ListDecisions
SELECT d.id, d.occurred_at, d.principal_id, d.principal_type, COALESCE(u.email::text, sa.name::text) AS principal_email,
       d.action, d.resource_type, d.resource_id, d.workspace_id, d.allowed, d.reason,
       array_to_json(d.matched_rules)::text AS matched_rules, d.request_id
FROM authz_decisions d
LEFT JOIN users u ON d.principal_type = 'user' AND u.id = d.principal_id
LEFT JOIN service_accounts sa ON d.principal_type = 'service_account' AND sa.id = d.principal_id
WHERE ($1::boolean IS NULL OR d.allowed = $1)
  AND ($2::timestamptz IS NULL OR (d.occurred_at, d.id) < ($2, $3::uuid))
ORDER BY d.occurred_at DESC, d.id DESC
LIMIT $4;

-- Seeding ---------------------------------------------------------------------

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

-- name: ListBindings
-- Optional filters: $1 principal type, $2 principal id, $3 scope type, $4 workspace.
SELECT rb.id, rb.principal_type, COALESCE(rb.user_id, rb.group_id, rb.service_account_id) AS principal_id,
       COALESCE(u.display_name, g.name::text, sa.name::text, '') AS principal_name, u.email::text AS principal_email,
       r.id AS role_id, r.key::text AS role_key, r.name AS role_name,
       rb.scope_type, rb.workspace_id, w.slug::text AS workspace_slug, w.name AS workspace_name,
       rb.resource_type, rb.resource_id, rb.created_at, rb.expires_at
FROM role_bindings rb
JOIN roles r ON r.id = rb.role_id
LEFT JOIN users u ON u.id = rb.user_id
LEFT JOIN groups g ON g.org_id = rb.org_id AND g.id = rb.group_id
LEFT JOIN service_accounts sa ON sa.org_id = rb.org_id AND sa.id = rb.service_account_id
LEFT JOIN workspaces w ON w.org_id = rb.org_id AND w.id = rb.workspace_id
WHERE ($1::text IS NULL OR rb.principal_type = $1)
  AND ($2::uuid IS NULL OR COALESCE(rb.user_id, rb.group_id, rb.service_account_id) = $2)
  AND ($3::text IS NULL OR rb.scope_type = $3)
  AND ($4::uuid IS NULL OR rb.workspace_id = $4)
ORDER BY rb.principal_type DESC, principal_name, rb.scope_type, r.name;

-- name: GetBinding
SELECT rb.id, rb.principal_type, COALESCE(rb.user_id, rb.group_id, rb.service_account_id) AS principal_id,
       COALESCE(u.display_name, g.name::text, sa.name::text, '') AS principal_name, u.email::text AS principal_email,
       r.id AS role_id, r.key::text AS role_key, r.name AS role_name,
       rb.scope_type, rb.workspace_id, w.slug::text AS workspace_slug, w.name AS workspace_name,
       rb.resource_type, rb.resource_id, rb.created_at, rb.expires_at
FROM role_bindings rb
JOIN roles r ON r.id = rb.role_id
LEFT JOIN users u ON u.id = rb.user_id
LEFT JOIN groups g ON g.org_id = rb.org_id AND g.id = rb.group_id
LEFT JOIN service_accounts sa ON sa.org_id = rb.org_id AND sa.id = rb.service_account_id
LEFT JOIN workspaces w ON w.org_id = rb.org_id AND w.id = rb.workspace_id
WHERE rb.id = $1;

-- name: CountOtherOwners
-- Active direct org-scope owner bindings, excluding binding $1 and the users in $2.
SELECT count(*) FROM role_bindings rb
JOIN roles r ON r.id = rb.role_id
JOIN org_memberships m ON m.org_id = rb.org_id AND m.user_id = rb.user_id AND m.status = 'active'
WHERE r.key = 'org_owner' AND rb.scope_type = 'org' AND rb.principal_type = 'user' AND rb.id <> $1
  AND rb.user_id <> ALL($2::uuid[])
  AND (rb.expires_at IS NULL OR rb.expires_at > now());

-- name: IsOrgOwner
SELECT EXISTS (SELECT 1 FROM role_bindings rb JOIN roles r ON r.id = rb.role_id
               WHERE rb.principal_type = 'user' AND rb.user_id = $1 AND rb.scope_type = 'org' AND r.key = 'org_owner'
                 AND (rb.expires_at IS NULL OR rb.expires_at > now()));
