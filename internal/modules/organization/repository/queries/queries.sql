-- organization repository — Read side (side-effect-free statements).
-- Organization domain: organizations, departments, environments, workspaces.

-- name: GetOrganization
SELECT id, slug::text AS slug, name, status, settings, created_at, updated_at
FROM organizations WHERE id = $1;

-- name: ListDepartments
SELECT d.id, d.slug::text AS slug, d.name, d.description, d.created_at, d.updated_at,
       (SELECT count(*) FROM workspaces w
         WHERE w.org_id = d.org_id AND w.department_id = d.id AND w.archived_at IS NULL) AS workspace_count
FROM departments d ORDER BY d.name;

-- name: GetDepartment
SELECT d.id, d.slug::text AS slug, d.name, d.description, d.created_at, d.updated_at,
       (SELECT count(*) FROM workspaces w
         WHERE w.org_id = d.org_id AND w.department_id = d.id AND w.archived_at IS NULL) AS workspace_count
FROM departments d WHERE d.id = $1;

-- name: CountDepartments
SELECT count(*) FROM departments;

-- Environments -----------------------------------------------------------------

-- name: ListEnvironments
SELECT id, slug::text AS slug, name, is_production, array_to_json(egress_allowlist)::text AS egress_allowlist,
       created_at, updated_at
FROM environments ORDER BY is_production, name;

-- name: GetEnvironment
SELECT id, slug::text AS slug, name, is_production, array_to_json(egress_allowlist)::text AS egress_allowlist,
       created_at, updated_at
FROM environments WHERE id = $1;

-- name: CountEnvironments
SELECT count(*) FROM environments;

-- Workspaces -------------------------------------------------------------------
-- Membership and roles come from authz's workspace_access / org_member_roles views.

-- name: ListWorkspaces
-- $1 include archived; $2 restrict to these workspace ids (NULL = all)
SELECT w.id, w.slug::text AS slug, w.name, w.description, w.department_id, d.name AS department_name,
       w.archived_at, w.created_at, w.updated_at,
       (SELECT count(DISTINCT a.user_id) FROM workspace_access a WHERE a.workspace_id = w.id) AS member_count
FROM workspaces w
LEFT JOIN departments d ON d.org_id = w.org_id AND d.id = w.department_id
WHERE ($1::boolean OR w.archived_at IS NULL)
  AND ($2::uuid[] IS NULL OR w.id = ANY($2))
ORDER BY w.archived_at NULLS FIRST, w.name;

-- name: GetWorkspaceBySlug
SELECT w.id, w.slug::text AS slug, w.name, w.description, w.department_id, d.name AS department_name,
       w.archived_at, w.created_at, w.updated_at,
       (SELECT count(DISTINCT a.user_id) FROM workspace_access a WHERE a.workspace_id = w.id) AS member_count
FROM workspaces w
LEFT JOIN departments d ON d.org_id = w.org_id AND d.id = w.department_id
WHERE w.slug = $1::citext;

-- name: CountActiveWorkspaces
SELECT count(*) FROM workspaces WHERE archived_at IS NULL;

-- name: ListWorkspaceMembers
SELECT u.id, u.email::text AS email, u.display_name, u.status, COALESCE(omr.is_owner, false) AS is_owner,
       min(a.created_at) AS added_at, array_to_json(array_agg(a.role_key ORDER BY a.role_key))::text AS roles
FROM workspace_access a
JOIN users u ON u.id = a.user_id
LEFT JOIN org_member_roles omr ON omr.org_id = a.org_id AND omr.user_id = a.user_id
WHERE a.workspace_id = $1
GROUP BY u.id, u.email, u.display_name, u.status, omr.is_owner
ORDER BY u.display_name;
