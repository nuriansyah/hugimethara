-- webhook repository — Read side (side-effect free).

-- name: ListEndpoints
SELECT id, workspace_id, environment_id, public_id, name, description, verifier, verifier_config, secret_hint,
       tolerance_seconds, rate_limit_per_minute, enabled, received_count, rejected_count, last_received_at,
       last_rejected_at, last_rejected_reason, secret_rotated_at, created_at, updated_at
FROM webhook_endpoints WHERE workspace_id = $1 ORDER BY created_at DESC;

-- name: GetEndpoint
SELECT id, workspace_id, environment_id, public_id, name, description, verifier, verifier_config, secret_hint,
       tolerance_seconds, rate_limit_per_minute, enabled, received_count, rejected_count, last_received_at,
       last_rejected_at, last_rejected_reason, secret_rotated_at, created_at, updated_at
FROM webhook_endpoints WHERE id = $1;

-- name: EndpointSecret
SELECT secret_ciphertext FROM webhook_endpoints WHERE id = $1;

-- name: IngressLookup
-- Pre-tenant: resolves a public id to its tenant (SECURITY DEFINER, hugi_auth).
SELECT endpoint_id, org_id, enabled, rate_limit_per_minute FROM hugi_webhook_endpoint_lookup($1);

-- name: ListEvents
SELECT id, workspace_id, endpoint_id, provider_event_id, event_type, content_type, headers, NULL::jsonb AS payload,
       NULL::text AS raw_body, payload_size, source_ip, received_at
FROM webhook_events
WHERE workspace_id = $1 AND ($2::uuid IS NULL OR endpoint_id = $2)
ORDER BY received_at DESC LIMIT $3;

-- name: GetEvent
SELECT id, workspace_id, endpoint_id, provider_event_id, event_type, content_type, headers, payload, raw_body,
       payload_size, source_ip, received_at
FROM webhook_events WHERE id = $1;

-- name: ListSubscriptions
SELECT id, workspace_id, environment_id, name, url, event_types, secret_hint, enabled, disabled_reason,
       failing_since, last_success_at, created_at, updated_at
FROM webhook_subscriptions WHERE workspace_id = $1 ORDER BY created_at DESC;

-- name: GetSubscription
SELECT id, workspace_id, environment_id, name, url, event_types, secret_hint, enabled, disabled_reason,
       failing_since, last_success_at, created_at, updated_at
FROM webhook_subscriptions WHERE id = $1;

-- name: ListDeliveries
SELECT d.id, d.workspace_id, d.subscription_id, s.name AS subscription_name, d.event_id, d.event_type,
       NULL::jsonb AS payload, d.replay_of, d.status, d.attempts, d.next_attempt_at, d.last_attempt_at,
       d.response_status, d.response_excerpt, d.latency_ms, d.error, d.created_at, d.finished_at
FROM webhook_deliveries d JOIN webhook_subscriptions s ON s.id = d.subscription_id
WHERE d.workspace_id = $1 AND ($2::uuid IS NULL OR d.subscription_id = $2) AND ($3 = '' OR d.status = $3)
ORDER BY d.created_at DESC LIMIT $4;

-- name: GetDelivery
SELECT d.id, d.workspace_id, d.subscription_id, s.name AS subscription_name, d.event_id, d.event_type,
       d.payload, d.replay_of, d.status, d.attempts, d.next_attempt_at, d.last_attempt_at,
       d.response_status, d.response_excerpt, d.latency_ms, d.error, d.created_at, d.finished_at
FROM webhook_deliveries d JOIN webhook_subscriptions s ON s.id = d.subscription_id
WHERE d.id = $1;

-- name: ListAttempts
SELECT attempt, response_status, response_excerpt, latency_ms, error, attempted_at
FROM webhook_delivery_attempts WHERE delivery_id = $1 ORDER BY attempt;

-- name: MatchingSubscriptions
-- Fan-out (hugi_system): enabled subscriptions of the event's workspace.
SELECT id, workspace_id FROM webhook_subscriptions
WHERE org_id = $1 AND workspace_id = $2 AND enabled AND $3 = ANY (event_types);

-- name: DeliveryTarget
-- Delivery (hugi_system): destination, sealed secret and egress allowlist.
SELECT s.id, s.org_id, s.url, s.secret_ciphertext, s.enabled, e.egress_allowlist
FROM webhook_subscriptions s JOIN environments e ON e.id = s.environment_id
WHERE s.id = $1;

-- name: EnvironmentInOrg
SELECT id FROM environments WHERE id = $1;
