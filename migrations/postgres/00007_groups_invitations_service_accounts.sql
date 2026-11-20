-- +goose Up
-- S4: groups, invitations, service accounts and API keys. Role bindings gain
-- group and service-account principals next to users.

-- Groups ------------------------------------------------------------------------

CREATE TABLE groups (
  id          uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id      uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
  name        citext NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 100),
  description text NOT NULL DEFAULT '' CHECK (length(description) <= 500),
  source      text NOT NULL DEFAULT 'internal' CHECK (source IN ('internal', 'idp')),  -- idp: S5 group mapping
  created_by  uuid,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, id),
  UNIQUE (org_id, name)
);
CREATE TRIGGER groups_touch BEFORE UPDATE ON groups FOR EACH ROW EXECUTE FUNCTION hugi_touch_updated_at();

CREATE TABLE group_members (
  org_id   uuid NOT NULL,
  group_id uuid NOT NULL,
  user_id  uuid NOT NULL,
  added_by uuid,
  added_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (group_id, user_id),
  FOREIGN KEY (org_id, group_id) REFERENCES groups (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, user_id) REFERENCES org_memberships (org_id, user_id) ON DELETE CASCADE
);
CREATE INDEX group_members_user_idx ON group_members (org_id, user_id);

-- Service accounts and API keys ------------------------------------------------------

CREATE TABLE service_accounts (
  id          uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id      uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
  name        citext NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 100),
  description text NOT NULL DEFAULT '' CHECK (length(description) <= 500),
  created_by  uuid,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  disabled_at timestamptz,
  UNIQUE (org_id, id),
  UNIQUE (org_id, name)
);
CREATE TRIGGER service_accounts_touch BEFORE UPDATE ON service_accounts FOR EACH ROW EXECUTE FUNCTION hugi_touch_updated_at();

-- A key is "hugi_<prefix>_<secret>". Only the prefix (lookup) and the SHA-256
-- of the whole key are stored.
CREATE TABLE api_keys (
  id                 uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id             uuid NOT NULL,
  service_account_id uuid NOT NULL,
  name               text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 100),
  prefix             text NOT NULL UNIQUE CHECK (prefix ~ '^[a-z0-9]{12}$'),
  key_hash           bytea NOT NULL,
  created_by         uuid,
  created_at         timestamptz NOT NULL DEFAULT now(),
  expires_at         timestamptz,
  last_used_at       timestamptz,
  revoked_at         timestamptz,
  UNIQUE (org_id, id),
  FOREIGN KEY (org_id, service_account_id) REFERENCES service_accounts (org_id, id) ON DELETE CASCADE
);
CREATE INDEX api_keys_account_idx ON api_keys (org_id, service_account_id);

-- Invitations ------------------------------------------------------------------------

CREATE TABLE invitations (
  id          uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id      uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
  email       citext NOT NULL CHECK (length(email) <= 254 AND email ~ '^[^@\s]+@[^@\s]+$'),
  token_hash  bytea NOT NULL UNIQUE,     -- sha256 of the token in the invitation link
  org_role_id uuid,                      -- optional organization role on top of "member"
  invited_by  uuid,
  created_at  timestamptz NOT NULL DEFAULT now(),
  expires_at  timestamptz NOT NULL,
  accepted_at timestamptz,
  accepted_by uuid,
  revoked_at  timestamptz,
  UNIQUE (org_id, id),
  FOREIGN KEY (org_id, org_role_id) REFERENCES roles (org_id, id) ON DELETE SET NULL (org_role_id)
);
-- At most one open invitation per address; re-inviting replaces it.
CREATE UNIQUE INDEX invitations_open_email ON invitations (org_id, email)
  WHERE accepted_at IS NULL AND revoked_at IS NULL;

-- Workspaces the invitee joins, each with its own role.
CREATE TABLE invitation_workspaces (
  org_id        uuid NOT NULL,
  invitation_id uuid NOT NULL,
  workspace_id  uuid NOT NULL,
  role_id       uuid NOT NULL,
  PRIMARY KEY (invitation_id, workspace_id),
  FOREIGN KEY (org_id, invitation_id) REFERENCES invitations (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, workspace_id) REFERENCES workspaces (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, role_id) REFERENCES roles (org_id, id) ON DELETE CASCADE
);

-- Role bindings: users, groups and service accounts ------------------------------------

ALTER TABLE role_bindings
  ADD COLUMN principal_type text NOT NULL DEFAULT 'user'
    CHECK (principal_type IN ('user', 'group', 'service_account')),
  ADD COLUMN group_id uuid,
  ADD COLUMN service_account_id uuid,
  ALTER COLUMN user_id DROP NOT NULL,
  ADD FOREIGN KEY (org_id, group_id) REFERENCES groups (org_id, id) ON DELETE CASCADE,
  ADD FOREIGN KEY (org_id, service_account_id) REFERENCES service_accounts (org_id, id) ON DELETE CASCADE,
  ADD CONSTRAINT role_bindings_principal CHECK (
    (principal_type = 'user'            AND user_id IS NOT NULL AND group_id IS NULL     AND service_account_id IS NULL) OR
    (principal_type = 'group'           AND user_id IS NULL     AND group_id IS NOT NULL AND service_account_id IS NULL) OR
    (principal_type = 'service_account' AND user_id IS NULL     AND group_id IS NULL     AND service_account_id IS NOT NULL));

DROP INDEX role_bindings_unique;
CREATE UNIQUE INDEX role_bindings_unique ON role_bindings (
  org_id, role_id, principal_type, COALESCE(user_id, group_id, service_account_id), scope_type,
  COALESCE(workspace_id, '00000000-0000-0000-0000-000000000000'::uuid),
  COALESCE(resource_type, ''), COALESCE(resource_id, ''));
CREATE INDEX role_bindings_group_idx ON role_bindings (org_id, group_id) WHERE group_id IS NOT NULL;
CREATE INDEX role_bindings_sa_idx ON role_bindings (org_id, service_account_id) WHERE service_account_id IS NOT NULL;

-- Membership views stay about people bound directly.
CREATE OR REPLACE VIEW workspace_access WITH (security_invoker = true) AS
SELECT rb.org_id, rb.workspace_id, rb.user_id, rb.id AS binding_id, r.id AS role_id, r.key::text AS role_key,
       r.name AS role_name, rb.created_at
FROM role_bindings rb JOIN roles r ON r.id = rb.role_id
WHERE rb.scope_type = 'workspace' AND rb.principal_type = 'user'
  AND (rb.expires_at IS NULL OR rb.expires_at > now());

CREATE OR REPLACE VIEW org_member_roles WITH (security_invoker = true) AS
SELECT rb.org_id, rb.user_id, array_agg(r.key::text ORDER BY r.key) AS role_keys,
       bool_or(r.key = 'org_owner') AS is_owner
FROM role_bindings rb JOIN roles r ON r.id = rb.role_id
WHERE rb.scope_type = 'org' AND rb.principal_type = 'user'
  AND (rb.expires_at IS NULL OR rb.expires_at > now())
GROUP BY rb.org_id, rb.user_id;

ALTER TABLE authz_decisions ADD COLUMN principal_type text NOT NULL DEFAULT 'user';

-- Catalog -------------------------------------------------------------------------------

UPDATE resource_types SET description = 'Organization members and invitations.',
  actions = '{read,invite,update,remove}', sensitive_actions = '{invite,update,remove}'
WHERE type = 'user';
INSERT INTO resource_types (type, description, actions, sensitive_actions) VALUES
  ('group',           'Groups of members that roles can be assigned to.', '{read,create,update,delete,manage_members}', '{delete,manage_members}'),
  ('service_account', 'Non-human identities and their API keys.',         '{read,create,update,delete,manage_keys}',    '{create,delete,manage_keys}');

DELETE FROM role_template_permissions WHERE template_key = 'org_admin' AND permission = 'user:read';
INSERT INTO role_template_permissions (template_key, permission) VALUES
  ('org_admin', 'user:*'), ('org_admin', 'group:*'), ('org_admin', 'service_account:*');

-- Existing organizations: extend the built-in admin role the same way.
INSERT INTO role_permissions (org_id, role_id, permission)
SELECT r.org_id, r.id, p.permission
FROM roles r CROSS JOIN (VALUES ('user:*'), ('group:*'), ('service_account:*')) AS p (permission)
WHERE r.key = 'org_admin' AND r.is_system
ON CONFLICT DO NOTHING;
UPDATE organizations SET authz_version = authz_version + 1;

-- RLS and privileges ------------------------------------------------------------------

ALTER TABLE groups                ENABLE ROW LEVEL SECURITY;
ALTER TABLE groups                FORCE ROW LEVEL SECURITY;
ALTER TABLE group_members         ENABLE ROW LEVEL SECURITY;
ALTER TABLE group_members         FORCE ROW LEVEL SECURITY;
ALTER TABLE service_accounts      ENABLE ROW LEVEL SECURITY;
ALTER TABLE service_accounts      FORCE ROW LEVEL SECURITY;
ALTER TABLE api_keys              ENABLE ROW LEVEL SECURITY;
ALTER TABLE api_keys              FORCE ROW LEVEL SECURITY;
ALTER TABLE invitations           ENABLE ROW LEVEL SECURITY;
ALTER TABLE invitations           FORCE ROW LEVEL SECURITY;
ALTER TABLE invitation_workspaces ENABLE ROW LEVEL SECURITY;
ALTER TABLE invitation_workspaces FORCE ROW LEVEL SECURITY;

CREATE POLICY groups_tenant ON groups TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY group_members_tenant ON group_members TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY service_accounts_tenant ON service_accounts TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY api_keys_tenant ON api_keys TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY invitations_tenant ON invitations TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY invitation_workspaces_tenant ON invitation_workspaces TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());

GRANT SELECT, INSERT, UPDATE, DELETE ON groups, group_members, service_accounts, invitation_workspaces TO hugi_app;
GRANT SELECT, INSERT, UPDATE ON api_keys, invitations TO hugi_app;   -- keys and invitations are revoked, not deleted

-- Pre-tenant lookups (same pattern as 00003): API-key authentication and the
-- public invitation page run before the organization is known.
GRANT SELECT ON api_keys, service_accounts, invitations TO hugi_auth;
CREATE POLICY api_keys_auth ON api_keys FOR SELECT TO hugi_auth USING (true);
CREATE POLICY service_accounts_auth ON service_accounts FOR SELECT TO hugi_auth USING (true);
CREATE POLICY invitations_auth ON invitations FOR SELECT TO hugi_auth USING (true);

-- +goose StatementBegin
CREATE FUNCTION hugi_auth_resolve_api_key(p_prefix text)
RETURNS TABLE (key_id uuid, org_id uuid, service_account_id uuid, key_hash bytea, expires_at timestamptz,
               revoked_at timestamptz, last_used_at timestamptz, account_disabled_at timestamptz, org_status text)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public, pg_temp AS $$
  SELECT k.id, k.org_id, k.service_account_id, k.key_hash, k.expires_at, k.revoked_at, k.last_used_at,
         sa.disabled_at, o.status
  FROM api_keys k
  JOIN service_accounts sa ON sa.id = k.service_account_id
  JOIN organizations o ON o.id = k.org_id
  WHERE k.prefix = p_prefix
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION hugi_auth_invitation_lookup(p_token_hash bytea)
RETURNS TABLE (invitation_id uuid, org_id uuid, org_slug citext, org_name text, org_settings jsonb, email citext,
               expires_at timestamptz, accepted_at timestamptz, revoked_at timestamptz, inviter_name text,
               user_exists boolean)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public, pg_temp AS $$
  SELECT i.id, i.org_id, o.slug, o.name, o.settings, i.email, i.expires_at, i.accepted_at, i.revoked_at,
         (SELECT u.display_name FROM users u WHERE u.id = i.invited_by),
         EXISTS (SELECT 1 FROM users u WHERE u.email = i.email)
  FROM invitations i JOIN organizations o ON o.id = i.org_id
  WHERE i.token_hash = p_token_hash AND o.status = 'active'
$$;
-- +goose StatementEnd

ALTER FUNCTION hugi_auth_resolve_api_key(text) OWNER TO hugi_auth;
ALTER FUNCTION hugi_auth_invitation_lookup(bytea) OWNER TO hugi_auth;
REVOKE ALL ON FUNCTION hugi_auth_resolve_api_key(text) FROM PUBLIC;
REVOKE ALL ON FUNCTION hugi_auth_invitation_lookup(bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION hugi_auth_resolve_api_key(text) TO hugi_app;
GRANT EXECUTE ON FUNCTION hugi_auth_invitation_lookup(bytea) TO hugi_app;

-- +goose Down
DROP FUNCTION IF EXISTS hugi_auth_invitation_lookup(bytea);
DROP FUNCTION IF EXISTS hugi_auth_resolve_api_key(text);

DELETE FROM role_permissions rp USING roles r
WHERE rp.role_id = r.id AND r.key = 'org_admin' AND r.is_system
  AND rp.permission IN ('user:*', 'group:*', 'service_account:*');
INSERT INTO role_permissions (org_id, role_id, permission)
SELECT r.org_id, r.id, 'user:read' FROM roles r WHERE r.key = 'org_admin' AND r.is_system
ON CONFLICT DO NOTHING;
DELETE FROM role_permissions WHERE permission LIKE 'group:%' OR permission LIKE 'service_account:%'
  OR permission IN ('user:invite', 'user:update', 'user:remove');
DELETE FROM role_template_permissions WHERE template_key = 'org_admin'
  AND permission IN ('user:*', 'group:*', 'service_account:*');
INSERT INTO role_template_permissions (template_key, permission) VALUES ('org_admin', 'user:read');
DELETE FROM role_bindings WHERE resource_type IN ('group', 'service_account');
DELETE FROM resource_types WHERE type IN ('group', 'service_account');
UPDATE resource_types SET description = 'Organization members.', actions = '{read}', sensitive_actions = '{}'
WHERE type = 'user';
UPDATE organizations SET authz_version = authz_version + 1;

ALTER TABLE authz_decisions DROP COLUMN principal_type;

CREATE OR REPLACE VIEW workspace_access WITH (security_invoker = true) AS
SELECT rb.org_id, rb.workspace_id, rb.user_id, rb.id AS binding_id, r.id AS role_id, r.key::text AS role_key,
       r.name AS role_name, rb.created_at
FROM role_bindings rb JOIN roles r ON r.id = rb.role_id
WHERE rb.scope_type = 'workspace' AND (rb.expires_at IS NULL OR rb.expires_at > now());
CREATE OR REPLACE VIEW org_member_roles WITH (security_invoker = true) AS
SELECT rb.org_id, rb.user_id, array_agg(r.key::text ORDER BY r.key) AS role_keys,
       bool_or(r.key = 'org_owner') AS is_owner
FROM role_bindings rb JOIN roles r ON r.id = rb.role_id
WHERE rb.scope_type = 'org' AND (rb.expires_at IS NULL OR rb.expires_at > now())
GROUP BY rb.org_id, rb.user_id;

DELETE FROM role_bindings WHERE principal_type <> 'user';
DROP INDEX role_bindings_unique;
DROP INDEX IF EXISTS role_bindings_group_idx;
DROP INDEX IF EXISTS role_bindings_sa_idx;
ALTER TABLE role_bindings DROP CONSTRAINT role_bindings_principal;
ALTER TABLE role_bindings DROP COLUMN service_account_id, DROP COLUMN group_id, DROP COLUMN principal_type,
  ALTER COLUMN user_id SET NOT NULL;
CREATE UNIQUE INDEX role_bindings_unique ON role_bindings (
  org_id, role_id, user_id, scope_type,
  COALESCE(workspace_id, '00000000-0000-0000-0000-000000000000'::uuid),
  COALESCE(resource_type, ''), COALESCE(resource_id, ''));

DROP TABLE IF EXISTS invitation_workspaces;
DROP TABLE IF EXISTS invitations;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS service_accounts;
DROP TABLE IF EXISTS group_members;
DROP TABLE IF EXISTS groups;
