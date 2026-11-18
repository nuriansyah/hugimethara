-- name: InsertOrganization :exec
INSERT INTO organizations (id, slug, name, settings) VALUES (@id, @slug, @name, @settings);

-- name: GetOrganization :one
SELECT id, slug, name, status, settings, created_at, updated_at FROM organizations WHERE id = @id;

-- name: UpdateOrganization :one
UPDATE organizations SET name = @name, settings = @settings
WHERE id = @id
RETURNING id, slug, name, status, settings, created_at, updated_at;
