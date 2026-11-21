-- +goose Up
-- Paid features: a global catalog of features and plans, and each
-- organization's subscription plus optional per-org overrides.
-- Enforcement happens server-side in the entitlement package.

CREATE TABLE features (
  key         text PRIMARY KEY CHECK (key ~ '^[a-z_]+(\.[a-z_]+)*$'),
  name        text NOT NULL,
  description text NOT NULL DEFAULT '',
  kind        text NOT NULL CHECK (kind IN ('flag', 'limit')),
  unit        text NOT NULL DEFAULT ''   -- e.g. "workspaces", "days" (limits only)
);

CREATE TABLE plans (
  key                 text PRIMARY KEY CHECK (key ~ '^[a-z_]+$'),
  name                text NOT NULL,
  description         text NOT NULL DEFAULT '',
  price_monthly_cents bigint CHECK (price_monthly_cents >= 0),  -- display only; NULL = custom pricing
  currency            text NOT NULL DEFAULT 'USD',
  sort_order          integer NOT NULL DEFAULT 0,
  is_default          boolean NOT NULL DEFAULT false
);
CREATE UNIQUE INDEX plans_single_default ON plans (is_default) WHERE is_default;

-- A missing row means the feature is not part of the plan.
CREATE TABLE plan_entitlements (
  plan_key    text NOT NULL REFERENCES plans (key) ON DELETE CASCADE,
  feature_key text NOT NULL REFERENCES features (key) ON DELETE CASCADE,
  enabled     boolean NOT NULL DEFAULT true,
  limit_value bigint CHECK (limit_value >= 0),  -- limits only; NULL = unlimited
  PRIMARY KEY (plan_key, feature_key)
);

CREATE TABLE org_subscriptions (
  org_id             uuid PRIMARY KEY REFERENCES organizations (id) ON DELETE CASCADE,
  plan_key           text NOT NULL REFERENCES plans (key),
  status             text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'trialing', 'past_due', 'canceled')),
  current_period_end timestamptz,
  external_ref       text,                 -- billing-provider subscription id (future)
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER org_subscriptions_touch BEFORE UPDATE ON org_subscriptions
  FOR EACH ROW EXECUTE FUNCTION hugi_touch_updated_at();

-- Per-organization exceptions granted by an operator (e.g. a sales deal).
CREATE TABLE org_entitlement_overrides (
  org_id      uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
  feature_key text NOT NULL REFERENCES features (key) ON DELETE CASCADE,
  enabled     boolean NOT NULL DEFAULT true,
  limit_value bigint CHECK (limit_value >= 0),
  reason      text NOT NULL DEFAULT '',
  expires_at  timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, feature_key)
);

-- Catalog tables are global and read-only for the runtime role.
GRANT SELECT ON features, plans, plan_entitlements TO hugi_app;

ALTER TABLE org_subscriptions ENABLE ROW LEVEL SECURITY;
ALTER TABLE org_subscriptions FORCE ROW LEVEL SECURITY;
ALTER TABLE org_entitlement_overrides ENABLE ROW LEVEL SECURITY;
ALTER TABLE org_entitlement_overrides FORCE ROW LEVEL SECURITY;

CREATE POLICY org_subscriptions_tenant ON org_subscriptions TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY org_overrides_tenant ON org_entitlement_overrides TO hugi_app
  USING (org_id = hugi_current_org());

-- The app may provision the default subscription at signup but never change
-- plans itself: plan changes are operator actions (hugi-migrate admin) or, later, the
-- billing integration running under its own role.
GRANT SELECT, INSERT ON org_subscriptions TO hugi_app;
GRANT SELECT ON org_entitlement_overrides TO hugi_app;

-- Seed catalog -----------------------------------------------------------------

INSERT INTO features (key, name, description, kind, unit) VALUES
  ('workspaces.max',           'Workspaces',               'Active (non-archived) workspaces.',                  'limit', 'workspaces'),
  ('environments.max',         'Environments',             'Deployment environments per organization.',          'limit', 'environments'),
  ('departments',              'Departments',              'Group workspaces by department.',                    'flag',  ''),
  ('environments.egress_allowlist', 'Private network access', 'Allow outbound calls to internal networks per environment.', 'flag', ''),
  ('audit_log.retention_days', 'Audit log history',        'How far back the audit log can be viewed.',          'limit', 'days');

INSERT INTO plans (key, name, description, price_monthly_cents, sort_order, is_default) VALUES
  ('free',       'Free',       'For small teams getting started.',           0,    10, true),
  ('pro',        'Pro',        'For growing companies running operations.',  4900, 20, false),
  ('enterprise', 'Enterprise', 'Unlimited scale, governance and support.',   NULL, 30, false);

INSERT INTO plan_entitlements (plan_key, feature_key, enabled, limit_value) VALUES
  ('free',       'workspaces.max',                 true, 3),
  ('free',       'environments.max',               true, 3),
  ('free',       'audit_log.retention_days',       true, 30),
  ('pro',        'workspaces.max',                 true, 25),
  ('pro',        'environments.max',               true, 10),
  ('pro',        'departments',                    true, NULL),
  ('pro',        'environments.egress_allowlist',  true, NULL),
  ('pro',        'audit_log.retention_days',       true, 365),
  ('enterprise', 'workspaces.max',                 true, NULL),
  ('enterprise', 'environments.max',               true, NULL),
  ('enterprise', 'departments',                    true, NULL),
  ('enterprise', 'environments.egress_allowlist',  true, NULL),
  ('enterprise', 'audit_log.retention_days',       true, NULL);

-- Existing organizations start on the default plan.
INSERT INTO org_subscriptions (org_id, plan_key)
SELECT o.id, p.key FROM organizations o CROSS JOIN plans p WHERE p.is_default
ON CONFLICT DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS org_entitlement_overrides;
DROP TABLE IF EXISTS org_subscriptions;
DROP TABLE IF EXISTS plan_entitlements;
DROP TABLE IF EXISTS plans;
DROP TABLE IF EXISTS features;
