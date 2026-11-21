-- integration/secrets repository — Read side (side-effect-free statements).
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

-- name: GetDataKey
SELECT id, kek_id, wrapped_key FROM data_keys WHERE org_id = $1;
