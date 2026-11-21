-- integration/mapping repository — Read side (side-effect-free statements).

-- name: ListMappings
SELECT id, workspace_id, name, draft, input_schema, output_schema, created_at, updated_at
FROM mappings
WHERE workspace_id = $1
ORDER BY name, id;

-- name: GetMapping
SELECT id, workspace_id, name, draft, input_schema, output_schema, created_at, updated_at
FROM mappings
WHERE id = $1;

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

-- name: NextMappingVersion
SELECT COALESCE(max(version), 0) + 1
FROM mapping_versions
WHERE mapping_id = $1;
