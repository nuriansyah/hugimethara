-- name: ListMappings
SELECT id, workspace_id, name, draft, input_schema, output_schema, created_at, updated_at
FROM mappings
WHERE workspace_id = $1
ORDER BY name, id;

-- name: GetMapping
SELECT id, workspace_id, name, draft, input_schema, output_schema, created_at, updated_at
FROM mappings
WHERE id = $1;

-- name: InsertMapping
INSERT INTO mappings (org_id, id, workspace_id, name, draft, input_schema, output_schema)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: UpdateMapping
UPDATE mappings
SET name = COALESCE($2, name),
    draft = COALESCE($3, draft),
    input_schema = COALESCE($4, input_schema),
    output_schema = COALESCE($5, output_schema)
WHERE id = $1;

-- name: DeleteMapping
DELETE FROM mappings WHERE id = $1;

-- name: InsertMappingVersion
INSERT INTO mapping_versions (org_id, id, mapping_id, version, definition, checksum, published_by)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: ListMappingVersions
SELECT id, mapping_id, version, definition, checksum, published_by, published_at
FROM mapping_versions
WHERE mapping_id = $1
ORDER BY version DESC;

-- name: ListMappingTests
SELECT id, mapping_id, name, input, expected, created_at, updated_at
FROM mapping_tests
WHERE mapping_id = $1
ORDER BY name, id;

-- name: GetMappingTest
SELECT id, mapping_id, name, input, expected, created_at, updated_at
FROM mapping_tests
WHERE id = $1;

-- name: InsertMappingTest
INSERT INTO mapping_tests (org_id, id, mapping_id, name, input, expected)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: UpdateMappingTest
UPDATE mapping_tests
SET name = COALESCE($2, name),
    input = COALESCE($3, input),
    expected = COALESCE($4, expected)
WHERE id = $1;

-- name: DeleteMappingTest
DELETE FROM mapping_tests WHERE id = $1;

-- name: NextMappingVersion
SELECT COALESCE(max(version), 0) + 1
FROM mapping_versions
WHERE mapping_id = $1;
