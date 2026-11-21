-- webhook repository — Write side (state-changing statements).

-- name: InsertEndpoint
INSERT INTO webhook_endpoints (id, org_id, workspace_id, environment_id, public_id, name, description, verifier,
  verifier_config, secret_ciphertext, secret_hint, tolerance_seconds, rate_limit_per_minute, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14);

-- name: UpdateEndpoint
UPDATE webhook_endpoints SET name = $2, description = $3, verifier_config = $4, tolerance_seconds = $5,
  rate_limit_per_minute = $6, enabled = $7
WHERE id = $1;

-- name: DeleteEndpoint
DELETE FROM webhook_endpoints WHERE id = $1;

-- name: RotateEndpointSecret
UPDATE webhook_endpoints SET secret_ciphertext = $2, secret_hint = $3, secret_rotated_at = now() WHERE id = $1;

-- name: RecordReceived
UPDATE webhook_endpoints SET received_count = received_count + 1, last_received_at = now() WHERE id = $1;

-- name: RecordRejected
UPDATE webhook_endpoints SET rejected_count = rejected_count + 1, last_rejected_at = now(),
  last_rejected_reason = $2
WHERE id = $1;

-- name: InsertEvent
-- A repeated provider event id is a duplicate: nothing is inserted.
INSERT INTO webhook_events (id, org_id, workspace_id, endpoint_id, provider_event_id, event_type, content_type,
  headers, payload, raw_body, payload_size, source_ip)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
ON CONFLICT (endpoint_id, provider_event_id) DO NOTHING
RETURNING id;

-- name: InsertSubscription
INSERT INTO webhook_subscriptions (id, org_id, workspace_id, environment_id, name, url, event_types,
  secret_ciphertext, secret_hint, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: UpdateSubscription
-- Re-enabling clears the failure streak and the automatic disabled reason.
UPDATE webhook_subscriptions SET name = $2, url = $3, event_types = $4, enabled = $5,
  disabled_reason = CASE WHEN $5 THEN '' ELSE disabled_reason END,
  failing_since = CASE WHEN $5 AND NOT enabled THEN NULL ELSE failing_since END
WHERE id = $1;

-- name: DeleteSubscription
DELETE FROM webhook_subscriptions WHERE id = $1;

-- name: RotateSubscriptionSecret
UPDATE webhook_subscriptions SET secret_ciphertext = $2, secret_hint = $3 WHERE id = $1;

-- name: InsertDelivery
-- Fan-out is idempotent per (subscription, event): a redelivered event adds nothing.
INSERT INTO webhook_deliveries (id, org_id, workspace_id, subscription_id, event_id, event_type, payload,
  next_attempt_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, now())
ON CONFLICT (subscription_id, event_id) WHERE replay_of IS NULL DO NOTHING;

-- name: InsertReplay
INSERT INTO webhook_deliveries (id, org_id, workspace_id, subscription_id, event_id, event_type, payload,
  replay_of, next_attempt_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now());

-- name: ClaimDueDeliveries
-- Claims due deliveries for one attempt each. next_attempt_at becomes the
-- lease: a worker that dies mid-attempt leaves the delivery due again.
WITH due AS (
  SELECT id FROM webhook_deliveries
  WHERE status = 'pending' AND next_attempt_at <= now()
  ORDER BY next_attempt_at LIMIT $1
  FOR UPDATE SKIP LOCKED
)
UPDATE webhook_deliveries d
SET attempts = d.attempts + 1, next_attempt_at = now() + make_interval(secs => $2), last_attempt_at = now()
FROM due WHERE d.id = due.id
RETURNING d.id, d.org_id, d.subscription_id, d.event_id, d.event_type, d.payload, d.attempts, d.created_at;

-- name: InsertAttempt
INSERT INTO webhook_delivery_attempts (id, org_id, delivery_id, attempt, response_status, response_excerpt,
  latency_ms, error)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (delivery_id, attempt) DO NOTHING;

-- name: FinishAttempt
-- Only the claim holder (same attempt number) may record the outcome.
UPDATE webhook_deliveries SET status = $3, next_attempt_at = $4, response_status = $5, response_excerpt = $6,
  latency_ms = $7, error = $8, finished_at = CASE WHEN $3 = 'pending' THEN NULL ELSE now() END
WHERE id = $1 AND attempts = $2 AND status = 'pending';

-- name: SubscriptionSucceeded
UPDATE webhook_subscriptions SET failing_since = NULL, last_success_at = now()
WHERE id = $1 AND (failing_since IS NOT NULL OR last_success_at IS NULL OR last_success_at < now() - interval '1 minute');

-- name: SubscriptionFailed
-- Starts (or continues) the failure streak; returns when it started.
UPDATE webhook_subscriptions SET failing_since = COALESCE(failing_since, now())
WHERE id = $1 RETURNING failing_since;

-- name: DisableSubscription
UPDATE webhook_subscriptions SET enabled = false, disabled_reason = $2 WHERE id = $1 AND enabled;
