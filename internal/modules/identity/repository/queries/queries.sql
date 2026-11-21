-- identity repository — Read side (side-effect-free statements).
-- Identity: users, memberships, credentials, sessions.

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

-- name: ListActiveUserSessions
SELECT id, created_at, last_seen_at, idle_expires_at, expires_at, host(ip) AS ip, user_agent
FROM sessions
WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > now() AND idle_expires_at > now()
ORDER BY last_seen_at DESC;

-- Members (S4) ------------------------------------------------------------------

-- name: ListMembers
SELECT u.id, u.email::text AS email, u.display_name, u.status, m.status AS membership_status, m.joined_at,
       COALESCE(omr.is_owner, false) AS is_owner, COALESCE(array_to_json(omr.role_keys)::text, '[]') AS roles,
       '[]' AS groups
FROM org_memberships m
JOIN users u ON u.id = m.user_id
LEFT JOIN org_member_roles omr ON omr.org_id = m.org_id AND omr.user_id = m.user_id
ORDER BY u.display_name, u.email;

-- name: GetMemberDetail
SELECT u.id, u.email::text AS email, u.display_name, u.status, m.status AS membership_status, m.joined_at,
       COALESCE(omr.is_owner, false) AS is_owner, COALESCE(array_to_json(omr.role_keys)::text, '[]') AS roles,
       COALESCE((SELECT json_agg(json_build_object('id', g.id, 'name', g.name) ORDER BY g.name)::text
                 FROM group_members gm JOIN groups g ON g.id = gm.group_id WHERE gm.user_id = u.id), '[]') AS groups
FROM org_memberships m
JOIN users u ON u.id = m.user_id
LEFT JOIN org_member_roles omr ON omr.org_id = m.org_id AND omr.user_id = m.user_id
WHERE m.user_id = $1;

-- name: MemberEmailExists
SELECT EXISTS (SELECT 1 FROM org_memberships m JOIN users u ON u.id = m.user_id WHERE u.email = $1::citext);

-- name: GetMembershipStatus
SELECT status FROM org_memberships WHERE user_id = $1;

-- Groups -------------------------------------------------------------------------

-- name: ListGroups
SELECT g.id, g.name::text AS name, g.description, g.source, g.created_at, g.updated_at,
       (SELECT count(*) FROM group_members gm WHERE gm.group_id = g.id) AS member_count
FROM groups g ORDER BY g.name;

-- name: GetGroup
SELECT g.id, g.name::text AS name, g.description, g.source, g.created_at, g.updated_at,
       (SELECT count(*) FROM group_members gm WHERE gm.group_id = g.id) AS member_count
FROM groups g WHERE g.id = $1;

-- name: ListGroupMembers
SELECT u.id, u.email::text AS email, u.display_name, gm.added_at
FROM group_members gm JOIN users u ON u.id = gm.user_id
WHERE gm.group_id = $1
ORDER BY u.display_name, u.email;

-- name: ListInvitations
-- $1 = true: open (pending, not expired) only.
SELECT i.id, i.email::text AS email, i.created_at, i.expires_at, i.accepted_at,
       CASE WHEN i.accepted_at IS NOT NULL THEN 'accepted'
            WHEN i.revoked_at IS NOT NULL THEN 'revoked'
            WHEN i.expires_at <= now() THEN 'expired'
            ELSE 'pending' END AS status,
       r.id AS org_role_id, r.key::text AS org_role_key, r.name AS org_role_name,
       u.display_name AS invited_by_name,
       COALESCE((SELECT json_agg(json_build_object('workspace_id', w.id, 'workspace_name', w.name,
                                                   'workspace_slug', w.slug, 'role_id', wr.id, 'role_name', wr.name)
                                 ORDER BY w.name)::text
                 FROM invitation_workspaces iw
                 JOIN workspaces w ON w.id = iw.workspace_id
                 JOIN roles wr ON wr.id = iw.role_id
                 WHERE iw.invitation_id = i.id), '[]') AS workspaces
FROM invitations i
LEFT JOIN roles r ON r.id = i.org_role_id
LEFT JOIN users u ON u.id = i.invited_by
WHERE (NOT $1::boolean OR (i.accepted_at IS NULL AND i.revoked_at IS NULL AND i.expires_at > now()))
  AND ($2::uuid IS NULL OR i.id = $2)
ORDER BY i.created_at DESC
LIMIT 500;

-- name: InvitationExists
SELECT EXISTS (SELECT 1 FROM invitations WHERE id = $1);

-- name: InvitationWorkspaceGrants
SELECT workspace_id, role_id FROM invitation_workspaces WHERE invitation_id = $1;

-- name: InvitationLookup
SELECT invitation_id, org_id, org_slug::text AS org_slug, org_name, org_settings, email::text AS email,
       expires_at, accepted_at, revoked_at, inviter_name, user_exists
FROM hugi_auth_invitation_lookup($1);

-- name: GetWorkspaceForInvite
SELECT id, name, archived_at FROM workspaces WHERE id = $1;

-- Service accounts -------------------------------------------------------------------

-- name: ListServiceAccounts
SELECT sa.id, sa.name::text AS name, sa.description, sa.disabled_at, sa.created_at, sa.updated_at,
       (SELECT count(*) FROM api_keys k WHERE k.service_account_id = sa.id AND k.revoked_at IS NULL
          AND (k.expires_at IS NULL OR k.expires_at > now())) AS active_key_count
FROM service_accounts sa
WHERE ($1::uuid IS NULL OR sa.id = $1)
ORDER BY sa.name;

-- name: ListAPIKeys
SELECT id, name, prefix, created_at, expires_at, last_used_at, revoked_at
FROM api_keys WHERE service_account_id = $1
ORDER BY revoked_at NULLS FIRST, created_at DESC;

-- name: GetAPIKey
SELECT id, name, prefix, created_at, expires_at, last_used_at, revoked_at FROM api_keys WHERE id = $1;

-- name: CountActiveAPIKeys
SELECT count(*) FROM api_keys WHERE service_account_id = $1 AND revoked_at IS NULL
  AND (expires_at IS NULL OR expires_at > now());

-- name: ResolveAPIKey
SELECT key_id, org_id, service_account_id, key_hash, expires_at, revoked_at, last_used_at, account_disabled_at, org_status
FROM hugi_auth_resolve_api_key($1);

-- name: ListIdentityProviders
SELECT p.id, p.name::text AS name, p.protocol, p.preset, p.issuer, p.client_id,
       (p.client_secret_ciphertext IS NOT NULL) AS has_client_secret,
       p.scopes, p.claim_mapping, p.jit_provisioning, p.allowed_domains, p.enabled,
       p.created_at, p.updated_at,
       (SELECT count(*) FROM group_mappings gm WHERE gm.idp_id = p.id) AS group_mapping_count
FROM identity_providers p
ORDER BY p.name;

-- name: GetIdentityProvider
SELECT p.id, p.name::text AS name, p.protocol, p.preset, p.issuer, p.client_id,
       (p.client_secret_ciphertext IS NOT NULL) AS has_client_secret,
       p.scopes, p.claim_mapping, p.jit_provisioning, p.allowed_domains, p.enabled,
       p.created_at, p.updated_at,
       (SELECT count(*) FROM group_mappings gm WHERE gm.idp_id = p.id) AS group_mapping_count
FROM identity_providers p WHERE p.id = $1;

-- name: ListGroupMappings
SELECT gm.id, gm.external_group, gm.group_id, g.name::text AS group_name, g.source AS group_source, gm.created_at
FROM group_mappings gm JOIN groups g ON g.id = gm.group_id
WHERE gm.idp_id = $1
ORDER BY gm.external_group;

-- name: GroupMappingExists
SELECT EXISTS (SELECT 1 FROM group_mappings WHERE id = $1 AND idp_id = $2);

-- name: GroupIDByNameAndSource
SELECT id FROM groups WHERE name = $1::citext AND source = $2;

-- name: ListIdPGroupIDsForUser
SELECT gm.group_id
FROM group_members gm JOIN groups g ON g.id = gm.group_id
WHERE g.source = 'idp' AND gm.user_id = $1;

-- Pre-tenant OIDC lookups (SECURITY DEFINER, migration 00008) -------------------------

-- name: OIDCProvider
SELECT provider_id, org_id, org_slug::text AS org_slug, org_name, org_settings, org_status,
       provider_name, preset, issuer, client_id, client_secret, scopes, claim_mapping,
       jit_provisioning, allowed_domains, enabled
FROM hugi_auth_oidc_provider($1);

-- name: OIDCProviders
SELECT provider_id, provider_name, preset FROM hugi_auth_oidc_providers($1::citext);

-- name: OIDCUserByEmail
SELECT user_id, display_name, status FROM hugi_auth_user_by_email($1::citext);

-- name: OIDCIdentityLink
SELECT link_id, user_id, email::text AS email, display_name, user_status, membership_status, last_login_at
FROM hugi_auth_identity_link($1, $2);

-- name: GetMembershipStatusAny
SELECT COALESCE(hugi_auth_membership_status($1, $2), '') AS status;

-- name: MappedGroupIDs
SELECT group_id AS id, external_group FROM group_mappings WHERE idp_id = $1;

-- name: ListUserMappedGroupIDs
SELECT gm.group_id
FROM group_members gm JOIN group_mappings m ON m.group_id = gm.group_id AND m.idp_id = $1
WHERE gm.user_id = $2;

-- name: GetGroupMapping
SELECT gm.id, gm.external_group, gm.group_id, g.name::text AS group_name, g.source AS group_source, gm.created_at
FROM group_mappings gm JOIN groups g ON g.id = gm.group_id
WHERE gm.id = $1 AND gm.idp_id = $2;
