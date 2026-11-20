-- Secret vault metadata and encrypted values. Plaintext is never selected by
-- any query; resolution decrypts only after an explicit secret:use check.

-- name: ListCredentials
SELECT id, environment_id, kind, name, hint, version, created_at, updated_at
FROM secrets ORDER BY name, id;

-- name: GetCredential
SELECT id, environment_id, kind, name, hint, version, ciphertext, dek_id, created_at, updated_at
FROM secrets WHERE id = $1;

-- name: GetCredentialMetadata
SELECT id, environment_id, kind, name, hint, version, created_at, updated_at
FROM secrets WHERE id = $1;

-- name: InsertDataKey
INSERT INTO data_keys (org_id, id, kek_id, wrapped_key)
VALUES ($1, $2, $3, $4)
ON CONFLICT (org_id) DO NOTHING;

-- name: GetDataKey
SELECT id, kek_id, wrapped_key FROM data_keys WHERE org_id = $1;

-- name: InsertCredential
INSERT INTO secrets (org_id, id, environment_id, kind, name, dek_id, ciphertext, hint)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: UpdateCredential
UPDATE secrets
SET name = COALESCE($2, name),
    dek_id = COALESCE($3, dek_id),
    ciphertext = COALESCE($4, ciphertext),
    hint = COALESCE($5, hint),
    version = version + CASE WHEN $4::bytea IS NULL THEN 0 ELSE 1 END,
    rotated_at = CASE WHEN $4::bytea IS NULL THEN rotated_at ELSE now() END
WHERE id = $1;

-- name: DeleteCredential
DELETE FROM secrets WHERE id = $1;
