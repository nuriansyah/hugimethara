-- +goose Up
-- S2: departments, environments, workspaces and workspace membership.
-- Composite foreign keys (org_id, x_id) make cross-tenant references impossible.

CREATE TABLE departments (
  id          uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id      uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
  slug        citext NOT NULL CHECK (slug ~ '^[a-z0-9](?:[a-z0-9-]{0,38}[a-z0-9])?$'),
  name        text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 100),
  description text NOT NULL DEFAULT '' CHECK (length(description) <= 500),
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, id),
  UNIQUE (org_id, slug)
);
CREATE TRIGGER departments_touch BEFORE UPDATE ON departments
  FOR EACH ROW EXECUTE FUNCTION hugi_touch_updated_at();

CREATE TABLE environments (
  id               uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id           uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
  slug             citext NOT NULL CHECK (slug ~ '^[a-z0-9](?:[a-z0-9-]{0,38}[a-z0-9])?$'),
  name             text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 100),
  is_production    boolean NOT NULL DEFAULT false,
  -- CIDRs that outbound calls from this environment may reach even though they
  -- are private/internal (SSRF guard allowlist, see platform/egress).
  egress_allowlist text[] NOT NULL DEFAULT '{}' CHECK (cardinality(egress_allowlist) <= 100),
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, id),
  UNIQUE (org_id, slug)
);
CREATE TRIGGER environments_touch BEFORE UPDATE ON environments
  FOR EACH ROW EXECUTE FUNCTION hugi_touch_updated_at();

CREATE TABLE workspaces (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id        uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
  slug          citext NOT NULL CHECK (slug ~ '^[a-z0-9](?:[a-z0-9-]{0,38}[a-z0-9])?$'),
  name          text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 100),
  description   text NOT NULL DEFAULT '' CHECK (length(description) <= 500),
  department_id uuid,
  archived_at   timestamptz,
  created_by    uuid,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, id),
  UNIQUE (org_id, slug),
  FOREIGN KEY (org_id, department_id) REFERENCES departments (org_id, id) ON DELETE SET NULL (department_id)
);
CREATE TRIGGER workspaces_touch BEFORE UPDATE ON workspaces
  FOR EACH ROW EXECUTE FUNCTION hugi_touch_updated_at();

CREATE TABLE workspace_members (
  org_id       uuid NOT NULL,
  workspace_id uuid NOT NULL,
  user_id      uuid NOT NULL,
  added_by     uuid,
  added_at     timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, user_id),
  FOREIGN KEY (org_id, workspace_id) REFERENCES workspaces (org_id, id) ON DELETE CASCADE,
  -- Leaving the organization removes all workspace memberships.
  FOREIGN KEY (org_id, user_id) REFERENCES org_memberships (org_id, user_id) ON DELETE CASCADE
);
CREATE INDEX workspace_members_user_idx ON workspace_members (org_id, user_id);

ALTER TABLE departments       ENABLE ROW LEVEL SECURITY;
ALTER TABLE departments       FORCE ROW LEVEL SECURITY;
ALTER TABLE environments      ENABLE ROW LEVEL SECURITY;
ALTER TABLE environments      FORCE ROW LEVEL SECURITY;
ALTER TABLE workspaces        ENABLE ROW LEVEL SECURITY;
ALTER TABLE workspaces        FORCE ROW LEVEL SECURITY;
ALTER TABLE workspace_members ENABLE ROW LEVEL SECURITY;
ALTER TABLE workspace_members FORCE ROW LEVEL SECURITY;

CREATE POLICY departments_tenant ON departments TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY environments_tenant ON environments TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY workspaces_tenant ON workspaces TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY workspace_members_tenant ON workspace_members TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());

GRANT SELECT, INSERT, UPDATE, DELETE ON departments, environments, workspace_members TO hugi_app;
GRANT SELECT, INSERT, UPDATE ON workspaces TO hugi_app;  -- archived, never hard-deleted by the app

-- Backfill: every existing organization gets the default environments.
INSERT INTO environments (id, org_id, slug, name, is_production)
SELECT uuidv7(), o.id, e.slug, e.name, e.is_production
FROM organizations o
CROSS JOIN (VALUES ('development', 'Development', false),
                   ('staging', 'Staging', false),
                   ('production', 'Production', true)) AS e (slug, name, is_production)
ON CONFLICT DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS workspace_members;
DROP TABLE IF EXISTS workspaces;
DROP TABLE IF EXISTS environments;
DROP TABLE IF EXISTS departments;
