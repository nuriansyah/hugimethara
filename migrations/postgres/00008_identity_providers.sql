-- +goose Up
-- S5: OIDC federation. Organizations configure identity providers (presets for
-- Entra ID, Google Workspace, Keycloak or generic OIDC), link external
-- subjects to members, provision users just in time and map IdP groups to
-- internal groups.
--
-- Client secrets are write-only over the API and stored AES-256-GCM sealed
-- (HUGI_MASTER_KEY). S6's envelope-encryption vault replaces this.

CREATE TABLE identity_providers (
  id                       uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id                   uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
  name                     citext NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 100),
  protocol                 text NOT NULL DEFAULT 'oidc' CHECK (protocol IN ('oidc')),  -- saml reserved
  preset                   text NOT NULL CHECK (preset IN ('entra', 'google', 'keycloak', 'generic')),
  issuer                   text NOT NULL CHECK (length(issuer) BETWEEN 1 AND 500 AND issuer ~ '^https?://'),
  client_id                text NOT NULL CHECK (length(btrim(client_id)) BETWEEN 1 AND 255),
  client_secret_ciphertext bytea,                    -- bytea = nonce||AES-GCM(secret); never returned
  scopes                   text[] NOT NULL DEFAULT ARRAY['openid', 'email', 'profile'],
  -- Claim names: {"email": "...", "name": "...", "groups": "..."} (groups may be empty).
  claim_mapping            jsonb NOT NULL DEFAULT '{"email": "email", "name": "name", "groups": "groups"}'::jsonb,
  jit_provisioning         boolean NOT NULL DEFAULT true,
  allowed_domains          text[] NOT NULL DEFAULT '{}',
  enabled                  boolean NOT NULL DEFAULT true,
  created_by               uuid,
  created_at               timestamptz NOT NULL DEFAULT now(),
  updated_at               timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, id),
  UNIQUE (org_id, name)
);
CREATE TRIGGER identity_providers_touch BEFORE UPDATE ON identity_providers
  FOR EACH ROW EXECUTE FUNCTION hugi_touch_updated_at();

-- One external subject per provider maps to one member of this organization.
-- Removing the membership removes the link (the person can be re-provisioned).
CREATE TABLE identity_links (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id        uuid NOT NULL,
  idp_id        uuid NOT NULL,
  subject       text NOT NULL CHECK (length(subject) BETWEEN 1 AND 255),
  user_id       uuid NOT NULL,
  created_at    timestamptz NOT NULL DEFAULT now(),
  last_login_at timestamptz,
  UNIQUE (org_id, idp_id, subject),
  UNIQUE (org_id, idp_id, user_id),
  FOREIGN KEY (org_id, idp_id) REFERENCES identity_providers (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, user_id) REFERENCES org_memberships (org_id, user_id) ON DELETE CASCADE
);

-- External IdP group -> internal group. One internal group belongs to at most
-- one mapping so IdP synchronization always owns an unambiguous membership set.
CREATE TABLE group_mappings (
  id             uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id         uuid NOT NULL,
  idp_id         uuid NOT NULL,
  external_group text NOT NULL CHECK (length(btrim(external_group)) BETWEEN 1 AND 200),
  group_id       uuid NOT NULL,
  created_by     uuid,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, idp_id, external_group),
  UNIQUE (org_id, group_id),
  FOREIGN KEY (org_id, idp_id) REFERENCES identity_providers (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, group_id) REFERENCES groups (org_id, id) ON DELETE CASCADE
);
CREATE TRIGGER group_mappings_touch BEFORE UPDATE ON group_mappings
  FOR EACH ROW EXECUTE FUNCTION hugi_touch_updated_at();

-- RLS and privileges ------------------------------------------------------------------

ALTER TABLE identity_providers ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity_providers FORCE ROW LEVEL SECURITY;
ALTER TABLE identity_links     ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity_links     FORCE ROW LEVEL SECURITY;
ALTER TABLE group_mappings     ENABLE ROW LEVEL SECURITY;
ALTER TABLE group_mappings     FORCE ROW LEVEL SECURITY;

CREATE POLICY identity_providers_tenant ON identity_providers TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY identity_links_tenant ON identity_links TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY group_mappings_tenant ON group_mappings TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());

GRANT SELECT, INSERT, UPDATE, DELETE ON identity_providers, identity_links, group_mappings TO hugi_app;

-- Pre-tenant lookups: the SSO start and callback run before an organization is
-- known. The provider id is an unguessable UUID; the discovery lookup is by
-- organization slug and returns only display metadata for enabled providers.
GRANT SELECT ON identity_providers, identity_links TO hugi_auth;
CREATE POLICY identity_providers_auth ON identity_providers FOR SELECT TO hugi_auth USING (true);
CREATE POLICY identity_links_auth ON identity_links FOR SELECT TO hugi_auth USING (true);

-- +goose StatementBegin
CREATE FUNCTION hugi_auth_oidc_provider(p_id uuid)
RETURNS TABLE (provider_id uuid, org_id uuid, org_slug citext, org_name text, org_settings jsonb, org_status text,
               provider_name text, preset text, issuer text, client_id text, client_secret bytea, scopes text[],
               claim_mapping jsonb, jit_provisioning boolean, allowed_domains text[], enabled boolean)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public, pg_temp AS $$
  SELECT p.id, p.org_id, o.slug, o.name, o.settings, o.status,
         p.name::text, p.preset, p.issuer, p.client_id, p.client_secret_ciphertext, p.scopes,
         p.claim_mapping, p.jit_provisioning, p.allowed_domains, p.enabled
  FROM identity_providers p JOIN organizations o ON o.id = p.org_id
  WHERE p.id = p_id
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION hugi_auth_oidc_providers(p_org_slug citext)
RETURNS TABLE (provider_id uuid, org_id uuid, org_slug citext, org_name text, provider_name text, preset text)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public, pg_temp AS $$
  SELECT p.id, p.org_id, o.slug, o.name, p.name::text, p.preset
  FROM identity_providers p JOIN organizations o ON o.id = p.org_id
  WHERE o.slug = p_org_slug AND o.status = 'active' AND p.enabled
  ORDER BY p.name
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION hugi_auth_user_by_email(p_email citext)
RETURNS TABLE (user_id uuid, display_name text, status text)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public, pg_temp AS $$
  SELECT u.id, u.display_name, u.status FROM users u WHERE u.email = p_email
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION hugi_auth_identity_link(p_idp_id uuid, p_subject text)
RETURNS TABLE (link_id uuid, user_id uuid, email citext, display_name text, user_status text,
               membership_status text, last_login_at timestamptz)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public, pg_temp AS $$
  SELECT l.id, u.id, u.email, u.display_name, u.status, m.status, l.last_login_at
  FROM identity_links l
  JOIN users u ON u.id = l.user_id
  JOIN org_memberships m ON m.org_id = l.org_id AND m.user_id = l.user_id
  WHERE l.idp_id = p_idp_id AND l.subject = p_subject
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION hugi_auth_membership_status(p_user_id uuid, p_org_id uuid)
RETURNS text
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public, pg_temp AS $$
  SELECT status FROM org_memberships WHERE user_id = p_user_id AND org_id = p_org_id
$$;
-- +goose StatementEnd

ALTER FUNCTION hugi_auth_oidc_provider(uuid) OWNER TO hugi_auth;
ALTER FUNCTION hugi_auth_oidc_providers(citext) OWNER TO hugi_auth;
ALTER FUNCTION hugi_auth_user_by_email(citext) OWNER TO hugi_auth;
ALTER FUNCTION hugi_auth_identity_link(uuid, text) OWNER TO hugi_auth;
ALTER FUNCTION hugi_auth_membership_status(uuid, uuid) OWNER TO hugi_auth;
REVOKE ALL ON FUNCTION hugi_auth_oidc_provider(uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION hugi_auth_oidc_providers(citext) FROM PUBLIC;
REVOKE ALL ON FUNCTION hugi_auth_user_by_email(citext) FROM PUBLIC;
REVOKE ALL ON FUNCTION hugi_auth_identity_link(uuid, text) FROM PUBLIC;
REVOKE ALL ON FUNCTION hugi_auth_membership_status(uuid, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION hugi_auth_oidc_provider(uuid) TO hugi_app;
GRANT EXECUTE ON FUNCTION hugi_auth_oidc_providers(citext) TO hugi_app;
GRANT EXECUTE ON FUNCTION hugi_auth_user_by_email(citext) TO hugi_app;
GRANT EXECUTE ON FUNCTION hugi_auth_identity_link(uuid, text) TO hugi_app;
GRANT EXECUTE ON FUNCTION hugi_auth_membership_status(uuid, uuid) TO hugi_app;

-- Catalog: identity_provider permissions for admins (owners hold *:*). ---------------

INSERT INTO resource_types (type, description, actions, sensitive_actions) VALUES
  ('identity_provider', 'OIDC identity providers (SSO) and group mappings.',
   '{read,create,update,delete,test}', '{create,update,delete,test}');

INSERT INTO role_template_permissions (template_key, permission) VALUES ('org_admin', 'identity_provider:*');

INSERT INTO role_permissions (org_id, role_id, permission)
SELECT r.org_id, r.id, 'identity_provider:*'
FROM roles r
WHERE r.key = 'org_admin' AND r.is_system
ON CONFLICT DO NOTHING;
UPDATE organizations SET authz_version = authz_version + 1;

-- +goose Down
DROP FUNCTION IF EXISTS hugi_auth_membership_status(uuid, uuid);
DROP FUNCTION IF EXISTS hugi_auth_identity_link(uuid, text);
DROP FUNCTION IF EXISTS hugi_auth_user_by_email(citext);
DROP FUNCTION IF EXISTS hugi_auth_oidc_providers(citext);
DROP FUNCTION IF EXISTS hugi_auth_oidc_provider(uuid);

DELETE FROM role_permissions rp USING roles r
WHERE rp.role_id = r.id AND r.key = 'org_admin' AND r.is_system
  AND rp.permission = 'identity_provider:*';
DELETE FROM role_template_permissions WHERE template_key = 'org_admin' AND permission = 'identity_provider:*';
DELETE FROM resource_types WHERE type = 'identity_provider';
UPDATE organizations SET authz_version = authz_version + 1;

DROP TABLE IF EXISTS group_mappings;
DROP TABLE IF EXISTS identity_links;
DROP TABLE IF EXISTS identity_providers;
