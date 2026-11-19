-- Organization domain: organizations, departments, environments, workspaces.

-- name: InsertOrganization
INSERT INTO organizations (id, slug, name, settings) VALUES ($1, $2, $3, $4);

-- name: GetOrganization
SELECT id, slug::text AS slug, name, status, settings, created_at, updated_at
FROM organizations WHERE id = $1;

-- name: UpdateOrganization
UPDATE organizations SET name = $2, settings = $3 WHERE id = $1
RETURNING id, slug::text AS slug, name, status, settings, created_at, updated_at;

-- Departments ------------------------------------------------------------------

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

-- name: InsertDepartment
INSERT INTO departments (id, org_id, slug, name, description) VALUES ($1, $2, $3, $4, $5);

-- name: UpdateDepartment
UPDATE departments SET name = $2, description = $3 WHERE id = $1;

-- name: DeleteDepartment
DELETE FROM departments WHERE id = $1;

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

-- name: InsertEnvironment
INSERT INTO environments (id, org_id, slug, name, is_production, egress_allowlist)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: UpdateEnvironment
UPDATE environments SET name = $2, is_production = $3, egress_allowlist = $4 WHERE id = $1;

-- name: DeleteEnvironment
DELETE FROM environments WHERE id = $1;

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

-- name: InsertWorkspace
INSERT INTO workspaces (id, org_id, slug, name, description, department_id, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: UpdateWorkspace
UPDATE workspaces SET name = $2, description = $3, department_id = $4 WHERE id = $1;

-- name: SetWorkspaceArchived
-- $2 = true archives, false restores; no-op (0 rows) when already in that state.
UPDATE workspaces SET archived_at = CASE WHEN $2::boolean THEN now() ELSE NULL END
WHERE id = $1 AND (archived_at IS NULL) = $2::boolean;

-- name: ListWorkspaceMembers
SELECT u.id, u.email::text AS email, u.display_name, u.status, COALESCE(omr.is_owner, false) AS is_owner,
       min(a.created_at) AS added_at, array_to_json(array_agg(a.role_key ORDER BY a.role_key))::text AS roles
FROM workspace_access a
JOIN users u ON u.id = a.user_id
LEFT JOIN org_member_roles omr ON omr.org_id = a.org_id AND omr.user_id = a.user_id
WHERE a.workspace_id = $1
GROUP BY u.id, u.email, u.display_name, u.status, omr.is_owner
ORDER BY u.display_name;

-- Organization members (read-only until S4 user management) -----------------------

-- name: ListOrgMembers
SELECT u.id, u.email::text AS email, u.display_name, u.status, COALESCE(omr.is_owner, false) AS is_owner,
       om.status AS membership_status, om.joined_at,
       COALESCE(array_to_json(omr.role_keys)::text, '[]') AS roles
FROM org_memberships om
JOIN users u ON u.id = om.user_id
LEFT JOIN org_member_roles omr ON omr.org_id = om.org_id AND omr.user_id = om.user_id
ORDER BY u.display_name;
