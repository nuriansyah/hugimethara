-- organization repository — Write side (state-changing statements).
-- Organization domain: organizations, departments, environments, workspaces.

-- name: InsertOrganization
INSERT INTO organizations (id, slug, name, settings) VALUES ($1, $2, $3, $4);

-- name: UpdateOrganization
UPDATE organizations SET name = $2, settings = $3 WHERE id = $1
RETURNING id, slug::text AS slug, name, status, settings, created_at, updated_at;

-- Departments ------------------------------------------------------------------

-- name: InsertDepartment
INSERT INTO departments (id, org_id, slug, name, description) VALUES ($1, $2, $3, $4, $5);

-- name: UpdateDepartment
UPDATE departments SET name = $2, description = $3 WHERE id = $1;

-- name: DeleteDepartment
DELETE FROM departments WHERE id = $1;

-- name: InsertEnvironment
INSERT INTO environments (id, org_id, slug, name, is_production, egress_allowlist)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: UpdateEnvironment
UPDATE environments SET name = $2, is_production = $3, egress_allowlist = $4 WHERE id = $1;

-- name: DeleteEnvironment
DELETE FROM environments WHERE id = $1;

-- name: InsertWorkspace
INSERT INTO workspaces (id, org_id, slug, name, description, department_id, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: UpdateWorkspace
UPDATE workspaces SET name = $2, description = $3, department_id = $4 WHERE id = $1;

-- name: SetWorkspaceArchived
-- $2 = true archives, false restores; no-op (0 rows) when already in that state.
UPDATE workspaces SET archived_at = CASE WHEN $2::boolean THEN now() ELSE NULL END
WHERE id = $1 AND (archived_at IS NULL) = $2::boolean;
