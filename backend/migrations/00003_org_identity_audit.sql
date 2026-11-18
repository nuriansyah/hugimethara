-- +goose Up
-- S1: organizations, users, memberships, local credentials, sessions, audit.
--
-- Access model:
-- * Tenant tables (organizations, org_memberships, sessions, audit_events) are
--   RLS-scoped to hugi_current_org().
-- * users and user_credentials are global identities (one person may belong to
--   several orgs). hugi_app may only see users who are members of the current
--   org. Pre-tenant lookups needed by login and session resolution go through
--   the narrow SECURITY DEFINER functions at the bottom of this file.

CREATE TABLE organizations (
  id            uuid PRIMARY KEY,
  slug          citext NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9](?:[a-z0-9-]{1,38}[a-z0-9])$'),
  name          text   NOT NULL CHECK (length(btrim(name)) BETWEEN 2 AND 100),
  status        text   NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended')),
  settings      jsonb  NOT NULL DEFAULT '{}'::jsonb,
  authz_version bigint NOT NULL DEFAULT 0,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER organizations_touch BEFORE UPDATE ON organizations
  FOR EACH ROW EXECUTE FUNCTION hugi_touch_updated_at();

CREATE TABLE users (
  id           uuid PRIMARY KEY,
  email        citext NOT NULL UNIQUE CHECK (length(email) <= 254 AND email ~ '^[^@\s]+@[^@\s]+$'),
  display_name text   NOT NULL CHECK (length(btrim(display_name)) BETWEEN 1 AND 100),
  status       text   NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER users_touch BEFORE UPDATE ON users
  FOR EACH ROW EXECUTE FUNCTION hugi_touch_updated_at();

CREATE TABLE org_memberships (
  org_id     uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
  user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  -- Interim until S3 RBAC: owners hold every permission in the org. S3 converts
  -- this flag into an org_owner role binding.
  is_owner   boolean NOT NULL DEFAULT false,
  status     text    NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended')),
  joined_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, user_id)
);
CREATE INDEX org_memberships_user_idx ON org_memberships (user_id);

CREATE TABLE user_credentials (
  user_id             uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
  password_hash       text NOT NULL,           -- argon2id PHC string
  failed_attempts     integer NOT NULL DEFAULT 0,
  locked_until        timestamptz,
  password_changed_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
  id              uuid PRIMARY KEY,
  org_id          uuid  NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
  user_id         uuid  NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  token_hash      bytea NOT NULL UNIQUE,       -- sha256 of the cookie token
  csrf_hash       bytea NOT NULL,              -- sha256 of the CSRF token
  created_at      timestamptz NOT NULL DEFAULT now(),
  last_seen_at    timestamptz NOT NULL DEFAULT now(),
  idle_expires_at timestamptz NOT NULL,
  expires_at      timestamptz NOT NULL,        -- absolute lifetime
  revoked_at      timestamptz,
  ip              inet,
  user_agent      text CHECK (length(user_agent) <= 512),
  UNIQUE (org_id, id)
);
CREATE INDEX sessions_user_idx ON sessions (org_id, user_id) WHERE revoked_at IS NULL;

CREATE TABLE audit_events (
  id            uuid PRIMARY KEY,                -- UUIDv7: time ordered
  org_id        uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
  occurred_at   timestamptz NOT NULL DEFAULT now(),
  actor_type    text NOT NULL CHECK (actor_type IN ('user', 'service_account', 'system', 'anonymous')),
  actor_id      uuid,
  action        text NOT NULL CHECK (action ~ '^[a-z_]+(\.[a-z_]+)+$'),
  resource_type text NOT NULL,
  resource_id   text,
  workspace_id  uuid,
  outcome       text NOT NULL DEFAULT 'success' CHECK (outcome IN ('success', 'failure', 'denied')),
  request_id    text,
  ip            inet,
  user_agent    text,
  details       jsonb NOT NULL DEFAULT '{}'::jsonb   -- redacted; never secrets
);
CREATE INDEX audit_events_org_time_idx ON audit_events (org_id, occurred_at DESC, id DESC);
CREATE INDEX audit_events_org_action_idx ON audit_events (org_id, action, occurred_at DESC);
CREATE INDEX audit_events_org_actor_idx ON audit_events (org_id, actor_id, occurred_at DESC);

-- Row-level security ------------------------------------------------------------------

ALTER TABLE organizations    ENABLE ROW LEVEL SECURITY;
ALTER TABLE organizations    FORCE ROW LEVEL SECURITY;
ALTER TABLE users            ENABLE ROW LEVEL SECURITY;
ALTER TABLE users            FORCE ROW LEVEL SECURITY;
ALTER TABLE org_memberships  ENABLE ROW LEVEL SECURITY;
ALTER TABLE org_memberships  FORCE ROW LEVEL SECURITY;
ALTER TABLE user_credentials ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_credentials FORCE ROW LEVEL SECURITY;
ALTER TABLE sessions         ENABLE ROW LEVEL SECURITY;
ALTER TABLE sessions         FORCE ROW LEVEL SECURITY;
ALTER TABLE audit_events     ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_events     FORCE ROW LEVEL SECURITY;

CREATE POLICY organizations_tenant ON organizations TO hugi_app
  USING (id = hugi_current_org()) WITH CHECK (id = hugi_current_org());

CREATE POLICY memberships_tenant ON org_memberships TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());

-- A user row is visible/updatable only when the user belongs to the current org.
-- Inserting a new user is allowed in a tenant-bound tx (signup / invitation);
-- the membership is created in the same transaction.
CREATE POLICY users_select ON users FOR SELECT TO hugi_app
  USING (EXISTS (SELECT 1 FROM org_memberships m
                 WHERE m.user_id = users.id AND m.org_id = hugi_current_org()));
CREATE POLICY users_update ON users FOR UPDATE TO hugi_app
  USING (EXISTS (SELECT 1 FROM org_memberships m
                 WHERE m.user_id = users.id AND m.org_id = hugi_current_org()));
CREATE POLICY users_insert ON users FOR INSERT TO hugi_app
  WITH CHECK (hugi_current_org() IS NOT NULL);

CREATE POLICY credentials_member ON user_credentials TO hugi_app
  USING (EXISTS (SELECT 1 FROM org_memberships m
                 WHERE m.user_id = user_credentials.user_id AND m.org_id = hugi_current_org()))
  WITH CHECK (EXISTS (SELECT 1 FROM org_memberships m
                 WHERE m.user_id = user_credentials.user_id AND m.org_id = hugi_current_org()));

CREATE POLICY sessions_tenant ON sessions TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());

CREATE POLICY audit_tenant ON audit_events TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());

-- Privileges: audit is append-only for the runtime role.
GRANT SELECT, INSERT, UPDATE ON organizations TO hugi_app;
GRANT SELECT, INSERT, UPDATE ON users TO hugi_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON org_memberships TO hugi_app;
GRANT SELECT, INSERT, UPDATE ON user_credentials TO hugi_app;
GRANT SELECT, INSERT, UPDATE ON sessions TO hugi_app;
GRANT SELECT, INSERT ON audit_events TO hugi_app;

-- Pre-tenant authentication functions.
-- They run as SECURITY DEFINER owned by hugi_auth: a NOLOGIN role whose only
-- rights are the read paths below plus the lockout counters. Owning them with
-- the migration role would either bypass RLS entirely (superuser) or see
-- nothing (FORCE RLS on a non-superuser owner).
-- +goose StatementBegin
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'hugi_auth') THEN
    CREATE ROLE hugi_auth NOLOGIN NOBYPASSRLS;
  END IF;
END
$$;
-- +goose StatementEnd
GRANT hugi_auth TO CURRENT_USER;  -- needed to transfer function ownership below
GRANT USAGE ON SCHEMA public TO hugi_auth;
GRANT SELECT (id, email, display_name, status) ON users TO hugi_auth;
GRANT SELECT ON user_credentials TO hugi_auth;
GRANT UPDATE (failed_attempts, locked_until) ON user_credentials TO hugi_auth;
GRANT SELECT ON org_memberships, organizations TO hugi_auth;
GRANT SELECT ON sessions TO hugi_auth;
CREATE POLICY users_auth ON users FOR SELECT TO hugi_auth USING (true);
CREATE POLICY credentials_auth_read ON user_credentials FOR SELECT TO hugi_auth USING (true);
CREATE POLICY credentials_auth_lockout ON user_credentials FOR UPDATE TO hugi_auth USING (true) WITH CHECK (true);
CREATE POLICY memberships_auth ON org_memberships FOR SELECT TO hugi_auth USING (true);
CREATE POLICY organizations_auth ON organizations FOR SELECT TO hugi_auth USING (true);
CREATE POLICY sessions_auth ON sessions FOR SELECT TO hugi_auth USING (true);

-- Login step 1: credentials and memberships for an email.
-- +goose StatementBegin
CREATE FUNCTION hugi_auth_login_lookup(p_email citext)
RETURNS TABLE (user_id uuid, user_status text, display_name text, password_hash text,
               failed_attempts integer, locked_until timestamptz)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public, pg_temp AS $$
  SELECT u.id, u.status, u.display_name, c.password_hash, c.failed_attempts, c.locked_until
  FROM users u JOIN user_credentials c ON c.user_id = u.id
  WHERE u.email = p_email
$$;
-- +goose StatementEnd

-- Login step 2: organizations the authenticated user may sign in to.
-- +goose StatementBegin
CREATE FUNCTION hugi_auth_user_orgs(p_user_id uuid)
RETURNS TABLE (org_id uuid, slug citext, name text, settings jsonb)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public, pg_temp AS $$
  SELECT o.id, o.slug, o.name, o.settings
  FROM org_memberships m JOIN organizations o ON o.id = m.org_id
  WHERE m.user_id = p_user_id AND m.status = 'active' AND o.status = 'active'
  ORDER BY o.name
$$;
-- +goose StatementEnd

-- Record a login attempt outcome (lockout bookkeeping). Locks after
-- p_max_failures consecutive failures for p_lock_for.
-- +goose StatementBegin
CREATE FUNCTION hugi_auth_record_attempt(p_user_id uuid, p_success boolean,
                                         p_max_failures integer, p_lock_for interval)
RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = public, pg_temp AS $$
BEGIN
  IF p_success THEN
    UPDATE user_credentials SET failed_attempts = 0, locked_until = NULL WHERE user_id = p_user_id;
  ELSE
    UPDATE user_credentials
       SET failed_attempts = failed_attempts + 1,
           locked_until = CASE WHEN failed_attempts + 1 >= p_max_failures
                               THEN now() + p_lock_for ELSE locked_until END
     WHERE user_id = p_user_id;
  END IF;
END
$$;
-- +goose StatementEnd

-- Session resolution: everything needed to authenticate a request, by token hash.
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

ALTER FUNCTION hugi_auth_login_lookup(citext) OWNER TO hugi_auth;
ALTER FUNCTION hugi_auth_user_orgs(uuid) OWNER TO hugi_auth;
ALTER FUNCTION hugi_auth_record_attempt(uuid, boolean, integer, interval) OWNER TO hugi_auth;
ALTER FUNCTION hugi_auth_resolve_session(bytea) OWNER TO hugi_auth;

REVOKE ALL ON FUNCTION hugi_auth_login_lookup(citext) FROM PUBLIC;
REVOKE ALL ON FUNCTION hugi_auth_user_orgs(uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION hugi_auth_record_attempt(uuid, boolean, integer, interval) FROM PUBLIC;
REVOKE ALL ON FUNCTION hugi_auth_resolve_session(bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION hugi_auth_login_lookup(citext) TO hugi_app;
GRANT EXECUTE ON FUNCTION hugi_auth_user_orgs(uuid) TO hugi_app;
GRANT EXECUTE ON FUNCTION hugi_auth_record_attempt(uuid, boolean, integer, interval) TO hugi_app;
GRANT EXECUTE ON FUNCTION hugi_auth_resolve_session(bytea) TO hugi_app;

-- The outbox and audit reference orgs created in S1 onward; tie them together.
ALTER TABLE outbox ADD CONSTRAINT outbox_org_fk FOREIGN KEY (org_id) REFERENCES organizations (id) ON DELETE CASCADE;

-- +goose Down
ALTER TABLE outbox DROP CONSTRAINT IF EXISTS outbox_org_fk;
DROP FUNCTION IF EXISTS hugi_auth_resolve_session(bytea);
DROP FUNCTION IF EXISTS hugi_auth_record_attempt(uuid, boolean, integer, interval);
DROP FUNCTION IF EXISTS hugi_auth_user_orgs(uuid);
DROP FUNCTION IF EXISTS hugi_auth_login_lookup(citext);
DROP POLICY IF EXISTS sessions_auth ON sessions;
DROP TABLE IF EXISTS audit_events;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS user_credentials;
DROP TABLE IF EXISTS org_memberships;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS organizations;
REVOKE USAGE ON SCHEMA public FROM hugi_auth;
