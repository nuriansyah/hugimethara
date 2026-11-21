-- +goose Up
-- S12 webhook engine (docs/architecture/integration-architecture.md §5).
-- Inbound: endpoints with a signature verifier; verified events are deduplicated
-- per provider event id and relayed as hugi.events.webhook.received.
-- Outbound: subscriptions to platform events; deliveries are signed (Standard
-- Webhooks), retried on a fixed schedule, dead-lettered, and replayable.
-- Ingress runs as hugi_app (pre-tenant lookup via a SECURITY DEFINER function,
-- then tenant-bound); delivery runs in hugi-worker as hugi_system.

CREATE TABLE webhook_endpoints (
  id                   uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id               uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
  workspace_id         uuid NOT NULL,
  environment_id       uuid NOT NULL,
  public_id            text NOT NULL UNIQUE CHECK (public_id ~ '^[a-f0-9]{32}$'),
  name                 text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 100),
  description          text NOT NULL DEFAULT '' CHECK (length(description) <= 500),
  verifier             text NOT NULL
                       CHECK (verifier IN ('standard_webhooks', 'hmac_sha256', 'stripe', 'github', 'slack')),
  -- Generic HMAC options (signature/timestamp/id headers, encoding, prefix).
  verifier_config      jsonb NOT NULL DEFAULT '{}'::jsonb,
  -- AES-256-GCM, AAD bound to (org, endpoint); never returned by the API.
  secret_ciphertext    bytea NOT NULL,
  secret_hint          text NOT NULL DEFAULT '' CHECK (length(secret_hint) <= 16),
  tolerance_seconds    integer NOT NULL DEFAULT 300 CHECK (tolerance_seconds BETWEEN 30 AND 3600),
  rate_limit_per_minute integer NOT NULL DEFAULT 600 CHECK (rate_limit_per_minute BETWEEN 1 AND 10000),
  enabled              boolean NOT NULL DEFAULT true,
  received_count       bigint NOT NULL DEFAULT 0,
  rejected_count       bigint NOT NULL DEFAULT 0,
  last_received_at     timestamptz,
  last_rejected_at     timestamptz,
  last_rejected_reason text NOT NULL DEFAULT '' CHECK (length(last_rejected_reason) <= 64),
  secret_rotated_at    timestamptz,
  created_by           uuid,
  created_at           timestamptz NOT NULL DEFAULT now(),
  updated_at           timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, id),
  FOREIGN KEY (org_id, workspace_id) REFERENCES workspaces (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, environment_id) REFERENCES environments (org_id, id) ON DELETE RESTRICT
);
CREATE INDEX webhook_endpoints_workspace ON webhook_endpoints (org_id, workspace_id, created_at DESC);
CREATE TRIGGER webhook_endpoints_touch BEFORE UPDATE ON webhook_endpoints
  FOR EACH ROW EXECUTE FUNCTION hugi_touch_updated_at();

-- Only verified requests are stored. provider_event_id is NULL when the scheme
-- carries no event id (generic HMAC without an id header): no dedupe then.
CREATE TABLE webhook_events (
  id                uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id            uuid NOT NULL,
  workspace_id      uuid NOT NULL,
  endpoint_id       uuid NOT NULL,
  provider_event_id text CHECK (length(provider_event_id) BETWEEN 1 AND 200),
  event_type        text NOT NULL DEFAULT '' CHECK (length(event_type) <= 200),
  content_type      text NOT NULL DEFAULT '' CHECK (length(content_type) <= 200),
  headers           jsonb NOT NULL DEFAULT '{}'::jsonb,
  payload           jsonb,
  raw_body          text,
  payload_size      integer NOT NULL CHECK (payload_size BETWEEN 0 AND 1048576),
  source_ip         text NOT NULL DEFAULT '' CHECK (length(source_ip) <= 64),
  received_at       timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, id),
  UNIQUE (endpoint_id, provider_event_id),
  FOREIGN KEY (org_id, endpoint_id) REFERENCES webhook_endpoints (org_id, id) ON DELETE CASCADE
);
CREATE INDEX webhook_events_endpoint ON webhook_events (org_id, endpoint_id, received_at DESC);

CREATE TABLE webhook_subscriptions (
  id                   uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id               uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
  workspace_id         uuid NOT NULL,
  -- The environment whose egress allowlist governs delivery.
  environment_id       uuid NOT NULL,
  name                 text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 100),
  url                  text NOT NULL CHECK (url ~ '^https?://' AND length(url) <= 2000),
  event_types          text[] NOT NULL CHECK (cardinality(event_types) BETWEEN 1 AND 50),
  secret_ciphertext    bytea NOT NULL,
  secret_hint          text NOT NULL DEFAULT '' CHECK (length(secret_hint) <= 16),
  enabled              boolean NOT NULL DEFAULT true,
  disabled_reason      text NOT NULL DEFAULT '' CHECK (length(disabled_reason) <= 64),
  -- Set at the first failed attempt after a success; cleared by a success.
  failing_since        timestamptz,
  last_success_at      timestamptz,
  created_by           uuid,
  created_at           timestamptz NOT NULL DEFAULT now(),
  updated_at           timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, id),
  FOREIGN KEY (org_id, workspace_id) REFERENCES workspaces (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, environment_id) REFERENCES environments (org_id, id) ON DELETE RESTRICT
);
CREATE INDEX webhook_subscriptions_workspace ON webhook_subscriptions (org_id, workspace_id, created_at DESC);
CREATE INDEX webhook_subscriptions_events ON webhook_subscriptions USING gin (event_types) WHERE enabled;
CREATE TRIGGER webhook_subscriptions_touch BEFORE UPDATE ON webhook_subscriptions
  FOR EACH ROW EXECUTE FUNCTION hugi_touch_updated_at();

-- One delivery per (subscription, event); a replay is a new delivery linked to
-- its original. next_attempt_at doubles as the claim lease while an attempt runs.
CREATE TABLE webhook_deliveries (
  id               uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id           uuid NOT NULL,
  workspace_id     uuid NOT NULL,
  subscription_id  uuid NOT NULL,
  event_id         text NOT NULL CHECK (length(event_id) BETWEEN 1 AND 200),
  event_type       text NOT NULL CHECK (length(event_type) BETWEEN 1 AND 200),
  payload          jsonb NOT NULL,
  replay_of        uuid,
  status           text NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending', 'succeeded', 'dead_lettered')),
  attempts         integer NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 20),
  next_attempt_at  timestamptz,
  last_attempt_at  timestamptz,
  response_status  integer,
  response_excerpt text NOT NULL DEFAULT '' CHECK (length(response_excerpt) <= 2048),
  latency_ms       integer,
  error            text NOT NULL DEFAULT '' CHECK (length(error) <= 500),
  created_at       timestamptz NOT NULL DEFAULT now(),
  finished_at      timestamptz,
  UNIQUE (org_id, id),
  FOREIGN KEY (org_id, subscription_id) REFERENCES webhook_subscriptions (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, replay_of) REFERENCES webhook_deliveries (org_id, id) ON DELETE SET NULL (replay_of)
);
CREATE UNIQUE INDEX webhook_deliveries_once
  ON webhook_deliveries (subscription_id, event_id) WHERE replay_of IS NULL;
CREATE INDEX webhook_deliveries_due ON webhook_deliveries (next_attempt_at) WHERE status = 'pending';
CREATE INDEX webhook_deliveries_subscription ON webhook_deliveries (org_id, subscription_id, created_at DESC);
CREATE INDEX webhook_deliveries_workspace ON webhook_deliveries (org_id, workspace_id, created_at DESC);

CREATE TABLE webhook_delivery_attempts (
  id               uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id           uuid NOT NULL,
  delivery_id      uuid NOT NULL,
  attempt          integer NOT NULL CHECK (attempt BETWEEN 1 AND 20),
  response_status  integer,
  response_excerpt text NOT NULL DEFAULT '' CHECK (length(response_excerpt) <= 2048),
  latency_ms       integer NOT NULL DEFAULT 0,
  error            text NOT NULL DEFAULT '' CHECK (length(error) <= 500),
  attempted_at     timestamptz NOT NULL DEFAULT now(),
  UNIQUE (delivery_id, attempt),
  FOREIGN KEY (org_id, delivery_id) REFERENCES webhook_deliveries (org_id, id) ON DELETE CASCADE
);

ALTER TABLE webhook_endpoints ENABLE ROW LEVEL SECURITY;
ALTER TABLE webhook_endpoints FORCE ROW LEVEL SECURITY;
ALTER TABLE webhook_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE webhook_events FORCE ROW LEVEL SECURITY;
ALTER TABLE webhook_subscriptions ENABLE ROW LEVEL SECURITY;
ALTER TABLE webhook_subscriptions FORCE ROW LEVEL SECURITY;
ALTER TABLE webhook_deliveries ENABLE ROW LEVEL SECURITY;
ALTER TABLE webhook_deliveries FORCE ROW LEVEL SECURITY;
ALTER TABLE webhook_delivery_attempts ENABLE ROW LEVEL SECURITY;
ALTER TABLE webhook_delivery_attempts FORCE ROW LEVEL SECURITY;

CREATE POLICY webhook_endpoints_tenant ON webhook_endpoints TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY webhook_events_tenant ON webhook_events TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY webhook_subscriptions_tenant ON webhook_subscriptions TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY webhook_deliveries_tenant ON webhook_deliveries TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());
CREATE POLICY webhook_delivery_attempts_tenant ON webhook_delivery_attempts TO hugi_app
  USING (org_id = hugi_current_org()) WITH CHECK (org_id = hugi_current_org());

-- The delivery worker fans events out across tenants and claims due deliveries.
CREATE POLICY webhook_subscriptions_engine ON webhook_subscriptions TO hugi_system
  USING (true) WITH CHECK (true);
CREATE POLICY webhook_deliveries_engine ON webhook_deliveries TO hugi_system
  USING (true) WITH CHECK (true);
CREATE POLICY webhook_delivery_attempts_engine ON webhook_delivery_attempts TO hugi_system
  USING (true) WITH CHECK (true);
-- The workflow trigger runner checks that a webhook trigger's endpoint exists.
CREATE POLICY webhook_endpoints_engine ON webhook_endpoints TO hugi_system USING (true);

GRANT SELECT, INSERT, UPDATE, DELETE ON webhook_endpoints, webhook_subscriptions TO hugi_app;
GRANT SELECT, INSERT ON webhook_events TO hugi_app;
GRANT SELECT, INSERT, UPDATE ON webhook_deliveries TO hugi_app;
GRANT SELECT ON webhook_delivery_attempts TO hugi_app;
GRANT SELECT (id, org_id, workspace_id, environment_id, enabled) ON webhook_endpoints TO hugi_system;
GRANT SELECT, UPDATE (enabled, disabled_reason, failing_since, last_success_at) ON webhook_subscriptions TO hugi_system;
GRANT SELECT, INSERT, UPDATE ON webhook_deliveries TO hugi_system;
GRANT SELECT, INSERT ON webhook_delivery_attempts TO hugi_system;

-- Workflow trigger.webhook bindings are looked up per received event.
CREATE INDEX workflow_triggers_webhook
  ON workflow_triggers (org_id, (config ->> 'endpoint_id')) WHERE type = 'trigger.webhook';

-- Pre-tenant ingress lookup: maps a public id to its tenant, nothing more.
-- +goose StatementBegin
CREATE FUNCTION hugi_webhook_endpoint_lookup(p_public_id text)
RETURNS TABLE (endpoint_id uuid, org_id uuid, enabled boolean, rate_limit_per_minute integer)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public, pg_temp AS $$
  SELECT e.id, e.org_id, e.enabled AND o.status = 'active', e.rate_limit_per_minute
  FROM webhook_endpoints e JOIN organizations o ON o.id = e.org_id
  WHERE e.public_id = p_public_id
$$;
-- +goose StatementEnd
GRANT SELECT (id, org_id, public_id, enabled, rate_limit_per_minute) ON webhook_endpoints TO hugi_auth;
CREATE POLICY webhook_endpoints_auth ON webhook_endpoints FOR SELECT TO hugi_auth USING (true);
ALTER FUNCTION hugi_webhook_endpoint_lookup(text) OWNER TO hugi_auth;
REVOKE ALL ON FUNCTION hugi_webhook_endpoint_lookup(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION hugi_webhook_endpoint_lookup(text) TO hugi_app;

INSERT INTO resource_types (type, description, actions, sensitive_actions) VALUES
  ('webhook', 'Inbound webhook endpoints, outbound subscriptions, events and deliveries.',
   '{read,create,update,delete,replay}', '{create,update,delete,replay}');
INSERT INTO role_template_permissions (template_key, permission) VALUES
  ('org_admin', 'webhook:*'), ('workspace_admin', 'webhook:*');
INSERT INTO role_permissions (org_id, role_id, permission)
SELECT r.org_id, r.id, p.permission
FROM roles r CROSS JOIN (VALUES ('webhook:*')) p(permission)
WHERE r.key IN ('org_admin', 'workspace_admin') AND r.is_system
ON CONFLICT DO NOTHING;
UPDATE organizations SET authz_version = authz_version + 1;

-- +goose Down
DELETE FROM role_permissions rp USING roles r
WHERE rp.role_id = r.id AND r.key IN ('org_admin', 'workspace_admin') AND r.is_system
  AND rp.permission = 'webhook:*';
DELETE FROM role_template_permissions WHERE template_key IN ('org_admin', 'workspace_admin')
  AND permission = 'webhook:*';
DELETE FROM resource_types WHERE type = 'webhook';
UPDATE organizations SET authz_version = authz_version + 1;
DROP FUNCTION IF EXISTS hugi_webhook_endpoint_lookup(text);
DROP INDEX IF EXISTS workflow_triggers_webhook;
DROP TABLE IF EXISTS webhook_delivery_attempts;
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhook_subscriptions;
DROP TABLE IF EXISTS webhook_events;
DROP TABLE IF EXISTS webhook_endpoints;
