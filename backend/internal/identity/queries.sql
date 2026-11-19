-- Identity: users, memberships, credentials, sessions.

-- name: InsertUser
INSERT INTO users (id, email, display_name) VALUES ($1, $2, $3);

-- name: InsertMembership
INSERT INTO org_memberships (org_id, user_id) VALUES ($1, $2);

-- name: InsertCredentials
INSERT INTO user_credentials (user_id, password_hash) VALUES ($1, $2);

-- name: GetMember
SELECT u.id, u.email::text AS email, u.display_name, u.status, m.joined_at
FROM users u JOIN org_memberships m ON m.user_id = u.id AND m.org_id = $1
WHERE u.id = $2;

-- Pre-tenant authentication (SECURITY DEFINER functions, migration 00003) -----------

-- name: LoginLookup
SELECT user_id, user_status, display_name, password_hash, failed_attempts, locked_until
FROM hugi_auth_login_lookup($1);

-- name: UserOrganizations
SELECT org_id, slug::text AS slug, name, settings FROM hugi_auth_user_orgs($1);

-- name: RecordLoginAttempt
-- $4 lockout duration in seconds
SELECT hugi_auth_record_attempt($1, $2, $3, make_interval(secs => $4::double precision));

-- name: ResolveSession
SELECT session_id, org_id, user_id, csrf_hash, idle_expires_at, expires_at, revoked_at,
       last_seen_at, user_status, membership_status, org_status, org_settings
FROM hugi_auth_resolve_session($1);

-- Sessions ------------------------------------------------------------------------

-- name: InsertSession
INSERT INTO sessions (id, org_id, user_id, token_hash, csrf_hash, idle_expires_at, expires_at, ip, user_agent)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8::inet, $9);

-- name: TouchSession
UPDATE sessions SET last_seen_at = now(), idle_expires_at = LEAST($2::timestamptz, expires_at)
WHERE id = $1 AND revoked_at IS NULL;

-- name: RevokeUserSession
UPDATE sessions SET revoked_at = now()
WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL;

-- name: ListActiveUserSessions
SELECT id, created_at, last_seen_at, idle_expires_at, expires_at, host(ip) AS ip, user_agent
FROM sessions
WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > now() AND idle_expires_at > now()
ORDER BY last_seen_at DESC;
