-- +goose Up
-- S3: dynamic RBAC (ADR-0007). Roles and permissions are data; role bindings
-- grant a role to a user at an organization, workspace or single-resource
-- scope. Workspace membership becomes a workspace-scope binding, so the
-- interim workspace_members table and org_memberships.is_owner go away.

-- Resource types and their actions (global catalog, seeded per slice).
CREATE TABLE resource_types (
  type              text PRIMARY KEY CHECK (type ~ '^[a-z_]+$'),
  description       text NOT NULL DEFAULT '',
  actions           text[] NOT NULL,
  sensitive_actions text[] NOT NULL DEFAULT '{}',   -- allowed decisions are audited too
  CHECK (sensitive_actions <@ actions)
);

INSERT INTO resource_types (type, description, actions, sensitive_actions) VALUES
  ('organization',  'The organization and its settings.',       '{read,update}',                                   '{update}'),
  ('department',    'Departments grouping workspaces.',         '{read,create,update,delete}',                     '{}'),
  ('environment',   'Deployment environments.',                 '{read,create,update,delete}',                     '{update,delete}'),
  ('workspace',     'Workspaces and their membership.',         '{read,create,update,archive,manage_members}',     '{manage_members}'),
  ('user',          'Organization members.',                    '{read}',                                          '{}'),
  ('audit_event',   'The organization audit trail.',            '{read}',                                          '{}'),
  ('subscription',  'Plan, paid features and usage.',           '{read}',                                          '{}'),
  ('role',          'Roles, permissions and role assignments.', '{read,create,update,delete,bind}',                '{create,update,delete,bind}'),
  ('authorization', 'Access checks and decision history.',      '{simulate,read_decisions}',                       '{}');

-- Role templates copied into every organization (single source of truth for
-- both the backfill below and signup).
CREATE TABLE role_templates (
  key         text PRIMARY KEY,
  name        text NOT NULL,
  description text NOT NULL,
  immutable   boolean NOT NULL DEFAULT false,
  sort_order  integer NOT NULL
);
CREATE TABLE role_template_permissions (
  template_key text NOT NULL REFERENCES role_templates (key) ON DELETE CASCADE,
  permission   text NOT NULL,
  PRIMARY KEY (template_key, permission)
);

INSERT INTO role_templates (key, name, description, immutable, sort_order) VALUES
  ('org_owner',       'Owner',           'Full control of the organization, including roles and access.', true,  10),
  ('org_admin',       'Administrator',   'Manages workspaces, environments and departments; reads the audit log.', false, 20),
  ('member',          'Member',          'Baseline access every organization member has.',                false, 30),
  ('workspace_admin', 'Workspace admin', 'Manages a workspace and its members.',                           false, 40),
  ('editor',          'Editor',          'Works inside a workspace.',                                      false, 50),
  ('viewer',          'Viewer',          'Read-only access to a workspace.',                               false, 60);

INSERT INTO role_template_permissions (template_key, permission) VALUES
  ('org_owner', '*:*'),
  ('org_admin', 'organization:*'), ('org_admin', 'department:*'), ('org_admin', 'environment:*'),
  ('org_admin', 'workspace:*'), ('org_admin', 'user:read'), ('org_admin', 'audit_event:read'),
  ('org_admin', 'subscription:read'), ('org_admin', 'role:read'), ('org_admin', 'authorization:simulate'),
  ('member', 'organization:read'), ('member', 'department:read'), ('member', 'environment:read'),
  ('member', 'subscription:read'),
  ('workspace_admin', 'workspace:read'), ('workspace_admin', 'workspace:update'),
  ('workspace_admin', 'workspace:archive'), ('workspace_admin', 'workspace:manage_members'),
  ('editor', 'workspace:read'),
  ('viewer', 'workspace:read');

CREATE TABLE roles (
  id          uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id      uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
  key         citext NOT NULL CHECK (key ~ '^[a-z0-9_]{2,40}$'),
  name        text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 100),
  description text NOT NULL DEFAULT '' CHECK (length(description) <= 500),
  is_system   boolean NOT NULL DEFAULT false,   -- copied from a template
  immutable   boolean NOT NULL DEFAULT false,   -- permissions cannot change, role cannot be deleted
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, id),
  UNIQUE (org_id, key)
);
CREATE TRIGGER roles_touch BEFORE UPDATE ON roles FOR EACH ROW EXECUTE FUNCTION hugi_touch_updated_at();

CREATE TABLE role_permissions (
  org_id     uuid NOT NULL,
  role_id    uuid NOT NULL,
  permission text NOT NULL CHECK (permission ~ '^([a-z_]+|\*):([a-z_]+|\*)$'),
  PRIMARY KEY (role_id, permission),
  FOREIGN KEY (org_id, role_id) REFERENCES roles (org_id, id) ON DELETE CASCADE
);

CREATE TABLE role_bindings (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id        uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
  role_id       uuid NOT NULL,
  user_id       uuid NOT NULL,   -- groups and service accounts get their own columns in S4
  scope_type    text NOT NULL CHECK (scope_type IN ('org', 'workspace', 'resource')),
  workspace_id  uuid,
  resource_type text REFERENCES resource_types (type),
  resource_id   text CHECK (length(resource_id) <= 200),
  created_by    uuid,
  created_at    timestamptz NOT NULL DEFAULT now(),
  expires_at    timestamptz,
  UNIQUE (org_id, id),
  FOREIGN KEY (org_id, role_id) REFERENCES roles (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, user_id) REFERENCES org_memberships (org_id, user_id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, workspace_id) REFERENCES workspaces (org_id, id) ON DELETE CASCADE,
  CHECK (
    (scope_type = 'org'       AND workspace_id IS NULL     AND resource_type IS NULL     AND resource_id IS NULL) OR
    (scope_type = 'workspace' AND workspace_id IS NOT NULL AND resource_type IS NULL     AND resource_id IS NULL) OR
    (scope_type = 'resource'  AND workspace_id IS NULL     AND resource_type IS NOT NULL AND resource_id IS NOT NULL)
  )
);
CREATE UNIQUE INDEX role_bindings_unique ON role_bindings (
  org_id, role_id, user_id, scope_type,
  COALESCE(workspace_id, '00000000-0000-0000-0000-000000000000'::uuid),
  COALESCE(resource_type, ''), COALESCE(resource_id, ''));
CREATE INDEX role_bindings_user_idx ON role_bindings (org_id, user_id);
CREATE INDEX role_bindings_workspace_idx ON role_bindings (org_id, workspace_id) WHERE workspace_id IS NOT NULL;

-- Workspace access as seen by the organization domain (membership lists and
-- counts). security_invoker keeps RLS of the underlying tables in force.
CREATE VIEW workspace_access WITH (security_invoker = true) AS
SELECT rb.org_id, rb.workspace_id, rb.user_id, rb.id AS binding_id, r.id AS role_id, r.key::text AS role_key,
       r.name AS role_name, rb.created_at
FROM role_bindings rb JOIN roles r ON r.id = rb.role_id
WHERE rb.scope_type = 'workspace' AND (rb.expires_at IS NULL OR rb.expires_at > now());

-- Organization-level roles per member (for member lists in other domains).
CREATE VIEW org_member_roles WITH (security_invoker = true) AS
SELECT rb.org_id, rb.user_id, array_agg(r.key::text ORDER BY r.key) AS role_keys,
       bool_or(r.key = 'org_owner') AS is_owner
FROM role_bindings rb JOIN roles r ON r.id = rb.role_id
WHERE rb.scope_type = 'org' AND (rb.expires_at IS NULL OR rb.expires_at > now())
GROUP BY rb.org_id, rb.user_id;

CREATE TABLE authz_decisions (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id        uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
  occurred_at   timestamptz NOT NULL DEFAULT now(),
  principal_id  uuid NOT NULL,
  action        text NOT NULL,
  resource_type text NOT NULL,
  resource_id   text,
  workspace_id  uuid,
  allowed       boolean NOT NULL,
  reason        text NOT NULL,
  matched_rules text[] NOT NULL DEFAULT '{}',
  authz_version bigint NOT NULL,
  request_id    text
);
CREATE INDEX authz_decisions_org_time_idx ON authz_decisions (org_id, occurred_at DESC, id DESC);

-- RLS and privileges ---------------------------------------------------------------

ALTER TABLE roles            ENABLE ROW LEVEL SECURITY;
ALTER TABLE roles            FORCE ROW LEVEL SECURITY;
ALTER TABLE role_permissions ENABLE ROW LEVEL SECURITY;
ALTER TABLE role_permissions FORCE ROW LEVEL SECURITY;
ALTER TABLE role_bindings    ENABLE ROW LEVEL SECURITY;
ALTER TABLE role_bindings    FORCE ROW LEVEL SECURITY;
ALTER TABLE authz_decisions  ENABLE ROW LEVEL SECURITY;
ALTER TABLE authz_decisions  FORCE ROW LEVEL SECURITY;

CREATE POLICY roles_tenant ON roles TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY role_permissions_tenant ON role_permissions TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY role_bindings_tenant ON role_bindings TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY authz_decisions_tenant ON authz_decisions TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());

GRANT SELECT ON resource_types, role_templates, role_template_permissions TO hugi_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON roles, role_permissions, role_bindings TO hugi_app;
GRANT SELECT ON workspace_access, org_member_roles TO hugi_app;
GRANT SELECT, INSERT ON authz_decisions TO hugi_app;   -- append-only

-- Backfill existing organizations ------------------------------------------------------

INSERT INTO roles (org_id, key, name, description, is_system, immutable)
SELECT o.id, t.key, t.name, t.description, true, t.immutable
FROM organizations o CROSS JOIN role_templates t;

INSERT INTO role_permissions (org_id, role_id, permission)
SELECT r.org_id, r.id, tp.permission
FROM roles r JOIN role_template_permissions tp ON tp.template_key = r.key::text;

-- Owners -> org_owner; every member -> member (org scope).
INSERT INTO role_bindings (org_id, role_id, user_id, scope_type)
SELECT m.org_id, r.id, m.user_id, 'org'
FROM org_memberships m JOIN roles r ON r.org_id = m.org_id AND r.key = 'org_owner'
WHERE m.is_owner;
INSERT INTO role_bindings (org_id, role_id, user_id, scope_type)
SELECT m.org_id, r.id, m.user_id, 'org'
FROM org_memberships m JOIN roles r ON r.org_id = m.org_id AND r.key = 'member';

-- Workspace members -> workspace scope: the creator as workspace_admin, others as editor.
INSERT INTO role_bindings (org_id, role_id, user_id, scope_type, workspace_id, created_by, created_at)
SELECT wm.org_id, r.id, wm.user_id, 'workspace', wm.workspace_id, wm.added_by, wm.added_at
FROM workspace_members wm
JOIN workspaces w ON w.id = wm.workspace_id
JOIN roles r ON r.org_id = wm.org_id
 AND r.key = CASE WHEN w.created_by = wm.user_id THEN 'workspace_admin' ELSE 'editor' END;

DROP TABLE workspace_members;

-- is_owner is now the org_owner role binding. Recreate the session resolver without it.
DROP FUNCTION hugi_auth_resolve_session(bytea);
ALTER TABLE org_memberships DROP COLUMN is_owner;

-- +goose StatementBegin
CREATE FUNCTION hugi_auth_resolve_session(p_token_hash bytea)
RETURNS TABLE (session_id uuid, org_id uuid, user_id uuid, csrf_hash bytea,
               idle_expires_at timestamptz, expires_at timestamptz, revoked_at timestamptz,
               last_seen_at timestamptz, user_status text, membership_status text,
               org_status text, org_settings jsonb)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public, pg_temp AS $$
  SELECT s.id, s.org_id, s.user_id, s.csrf_hash, s.idle_expires_at, s.expires_at, s.revoked_at,
         s.last_seen_at, u.status, m.status, o.status, o.settings
  FROM sessions s
  JOIN users u            ON u.id = s.user_id
  JOIN organizations o    ON o.id = s.org_id
  JOIN org_memberships m  ON m.org_id = s.org_id AND m.user_id = s.user_id
  WHERE s.token_hash = p_token_hash
$$;
-- +goose StatementEnd
ALTER FUNCTION hugi_auth_resolve_session(bytea) OWNER TO hugi_auth;
REVOKE ALL ON FUNCTION hugi_auth_resolve_session(bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION hugi_auth_resolve_session(bytea) TO hugi_app;

-- +goose Down
DROP FUNCTION hugi_auth_resolve_session(bytea);
ALTER TABLE org_memberships ADD COLUMN is_owner boolean NOT NULL DEFAULT false;
UPDATE org_memberships m SET is_owner = true
FROM role_bindings rb JOIN roles r ON r.id = rb.role_id
WHERE rb.org_id = m.org_id AND rb.user_id = m.user_id AND rb.scope_type = 'org' AND r.key = 'org_owner';

-- +goose StatementBegin
CREATE FUNCTION hugi_auth_resolve_session(p_token_hash bytea)
RETURNS TABLE (session_id uuid, org_id uuid, user_id uuid, csrf_hash bytea,
               idle_expires_at timestamptz, expires_at timestamptz, revoked_at timestamptz,
               last_seen_at timestamptz, user_status text, membership_status text,
               is_owner boolean, org_status text, org_settings jsonb)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public, pg_temp AS $$
  SELECT s.id, s.org_id, s.user_id, s.csrf_hash, s.idle_expires_at, s.expires_at, s.revoked_at,
         s.last_seen_at, u.status, m.status, m.is_owner, o.status, o.settings
  FROM sessions s
  JOIN users u            ON u.id = s.user_id
  JOIN organizations o    ON o.id = s.org_id
  JOIN org_memberships m  ON m.org_id = s.org_id AND m.user_id = s.user_id
  WHERE s.token_hash = p_token_hash
$$;
-- +goose StatementEnd
ALTER FUNCTION hugi_auth_resolve_session(bytea) OWNER TO hugi_auth;
REVOKE ALL ON FUNCTION hugi_auth_resolve_session(bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION hugi_auth_resolve_session(bytea) TO hugi_app;

CREATE TABLE workspace_members (
  org_id       uuid NOT NULL,
  workspace_id uuid NOT NULL,
  user_id      uuid NOT NULL,
  added_by     uuid,
  added_at     timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, user_id),
  FOREIGN KEY (org_id, workspace_id) REFERENCES workspaces (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, user_id) REFERENCES org_memberships (org_id, user_id) ON DELETE CASCADE
);
INSERT INTO workspace_members (org_id, workspace_id, user_id, added_by, added_at)
SELECT DISTINCT ON (workspace_id, user_id) org_id, workspace_id, user_id, created_by, created_at
FROM role_bindings WHERE scope_type = 'workspace';
ALTER TABLE workspace_members ENABLE ROW LEVEL SECURITY;
ALTER TABLE workspace_members FORCE ROW LEVEL SECURITY;
CREATE POLICY workspace_members_tenant ON workspace_members TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
GRANT SELECT, INSERT, UPDATE, DELETE ON workspace_members TO hugi_app;

DROP TABLE IF EXISTS authz_decisions;
DROP VIEW IF EXISTS workspace_access;
DROP VIEW IF EXISTS org_member_roles;
DROP TABLE IF EXISTS role_bindings;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS role_template_permissions;
DROP TABLE IF EXISTS role_templates;
DROP TABLE IF EXISTS resource_types;
