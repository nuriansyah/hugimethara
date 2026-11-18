-- name: InsertUser :exec
INSERT INTO users (id, email, display_name) VALUES (@id, @email, @display_name);

-- name: InsertMembership :exec
INSERT INTO org_memberships (org_id, user_id, is_owner) VALUES (@org_id, @user_id, @is_owner);

-- name: InsertCredentials :exec
INSERT INTO user_credentials (user_id, password_hash) VALUES (@user_id, @password_hash);

-- name: GetMember :one
SELECT u.id, u.email, u.display_name, u.status, m.is_owner, m.status AS membership_status, m.joined_at
FROM users u JOIN org_memberships m ON m.user_id = u.id AND m.org_id = @org_id
WHERE u.id = @user_id;

-- name: InsertSession :exec
INSERT INTO sessions (id, org_id, user_id, token_hash, csrf_hash, idle_expires_at, expires_at, ip, user_agent)
VALUES (@id, @org_id, @user_id, @token_hash, @csrf_hash, @idle_expires_at, @expires_at, @ip, @user_agent);

-- name: TouchSession :exec
UPDATE sessions SET last_seen_at = now(), idle_expires_at = LEAST(@idle_expires_at::timestamptz, expires_at)
WHERE id = @id AND revoked_at IS NULL;

-- name: RevokeUserSession :execrows
UPDATE sessions SET revoked_at = now()
WHERE id = @id AND user_id = @user_id AND revoked_at IS NULL;

-- name: ListActiveUserSessions :many
SELECT id, created_at, last_seen_at, idle_expires_at, expires_at, ip, user_agent
FROM sessions
WHERE user_id = @user_id AND revoked_at IS NULL AND expires_at > now() AND idle_expires_at > now()
ORDER BY last_seen_at DESC;
