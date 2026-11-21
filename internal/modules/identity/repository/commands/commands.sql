-- identity repository — Write side (state-changing statements).
-- Identity: users, memberships, credentials, sessions.

-- name: InsertUser
INSERT INTO users (id, email, display_name) VALUES ($1, $2, $3);

-- name: InsertMembership
INSERT INTO org_memberships (org_id, user_id) VALUES ($1, $2);

-- name: InsertCredentials
INSERT INTO user_credentials (user_id, password_hash) VALUES ($1, $2);

-- name: InsertSession
INSERT INTO sessions (id, org_id, user_id, token_hash, csrf_hash, idle_expires_at, expires_at, ip, user_agent)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8::inet, $9);

-- name: TouchSession
UPDATE sessions SET last_seen_at = now(), idle_expires_at = LEAST($2::timestamptz, expires_at)
WHERE id = $1 AND revoked_at IS NULL;

-- name: RevokeUserSession
UPDATE sessions SET revoked_at = now()
WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL;

-- name: SetMembershipStatus
UPDATE org_memberships SET status = $2 WHERE user_id = $1 AND status <> $2;

-- name: DeleteMembership
DELETE FROM org_memberships WHERE user_id = $1;

-- name: RevokeAllUserSessions
UPDATE sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL;

-- name: InsertMembershipIfAbsent
INSERT INTO org_memberships (org_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: InsertGroup
INSERT INTO groups (id, org_id, name, description, created_by) VALUES ($1, $2, $3, $4, $5);

-- name: UpdateGroup
UPDATE groups SET name = $2, description = $3 WHERE id = $1;

-- name: DeleteGroup
DELETE FROM groups WHERE id = $1;

-- name: InsertGroupMember
INSERT INTO group_members (org_id, group_id, user_id, added_by) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING;

-- name: DeleteGroupMember
DELETE FROM group_members WHERE group_id = $1 AND user_id = $2;

-- Invitations ----------------------------------------------------------------------

-- name: RevokeOpenInvitationsFor
UPDATE invitations SET revoked_at = now() WHERE email = $1::citext AND accepted_at IS NULL AND revoked_at IS NULL;

-- name: InsertInvitation
INSERT INTO invitations (id, org_id, email, token_hash, org_role_id, invited_by, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: InsertInvitationWorkspace
INSERT INTO invitation_workspaces (org_id, invitation_id, workspace_id, role_id) VALUES ($1, $2, $3, $4);

-- name: RevokeInvitation
UPDATE invitations SET revoked_at = now() WHERE id = $1 AND accepted_at IS NULL AND revoked_at IS NULL;

-- name: LockInvitation
SELECT id, email::text AS email, org_role_id, invited_by, expires_at, accepted_at, revoked_at
FROM invitations WHERE id = $1 FOR UPDATE;

-- name: MarkInvitationAccepted
UPDATE invitations SET accepted_at = now(), accepted_by = $2 WHERE id = $1;

-- name: InsertServiceAccount
INSERT INTO service_accounts (id, org_id, name, description, created_by) VALUES ($1, $2, $3, $4, $5);

-- name: UpdateServiceAccount
UPDATE service_accounts SET name = $2, description = $3,
       disabled_at = CASE WHEN $4::boolean THEN COALESCE(disabled_at, now()) ELSE NULL END
WHERE id = $1;

-- name: DeleteServiceAccount
DELETE FROM service_accounts WHERE id = $1;

-- name: InsertAPIKey
INSERT INTO api_keys (id, org_id, service_account_id, name, prefix, key_hash, created_by, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: RevokeAPIKey
UPDATE api_keys SET revoked_at = now() WHERE id = $1 AND service_account_id = $2 AND revoked_at IS NULL;

-- name: TouchAPIKey
UPDATE api_keys SET last_used_at = now() WHERE id = $1;

-- Identity providers (S5) ------------------------------------------------------------

-- name: InsertIdentityProvider
INSERT INTO identity_providers (id, org_id, name, preset, issuer, client_id, client_secret_ciphertext,
                                 scopes, claim_mapping, jit_provisioning, allowed_domains, enabled, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13);

-- name: UpdateIdentityProvider
UPDATE identity_providers
SET name = $2, preset = $3, issuer = $4, client_id = $5,
    client_secret_ciphertext = CASE WHEN $6::boolean THEN $7 ELSE client_secret_ciphertext END,
    scopes = $8, claim_mapping = $9, jit_provisioning = $10, allowed_domains = $11, enabled = $12
WHERE id = $1;

-- name: DeleteIdentityProvider
DELETE FROM identity_providers WHERE id = $1;

-- name: InsertIdentityLink
INSERT INTO identity_links (id, org_id, idp_id, subject, user_id) VALUES ($1, $2, $3, $4, $5);

-- name: TouchIdentityLink
UPDATE identity_links SET last_login_at = now() WHERE id = $1;

-- name: InsertGroupMapping
INSERT INTO group_mappings (id, org_id, idp_id, external_group, group_id, created_by)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: DeleteGroupMapping
DELETE FROM group_mappings WHERE id = $1 AND idp_id = $2;

-- name: InsertIdPGroup
INSERT INTO groups (id, org_id, name, description, source, created_by)
VALUES ($1, $2, $3, '', 'idp', $4);

-- name: SyncIdPGroupMembers
-- $1 user id, $2 added_by, $3 group id
INSERT INTO group_members (org_id, group_id, user_id, added_by)
SELECT g.org_id, g.id, $1, $2 FROM groups g WHERE g.id = $3
ON CONFLICT DO NOTHING;

-- name: DeleteIdPGroupMember
DELETE FROM group_members WHERE group_id = $1 AND user_id = $2;

-- name: ClearMappedGroupMembers
DELETE FROM group_members gm USING group_mappings m, groups g
WHERE m.idp_id = $1 AND g.id = m.group_id AND gm.group_id = m.group_id AND g.source = 'idp';

-- name: ClearIdPGroupMembers
DELETE FROM group_members gm USING groups g
WHERE g.id = $1 AND gm.group_id = g.id AND g.source = 'idp';
