-- name: InsertOutbox :one
-- Runs inside a tenant-bound business transaction (RLS checks org_id).
INSERT INTO outbox (org_id, subject, payload, headers)
VALUES (@org_id, @subject, @payload, @headers)
RETURNING id;

-- name: ClaimPendingOutbox :many
-- Relay (hugi_system). Rows stay locked until the relay's transaction ends,
-- so concurrent relays never publish the same row.
SELECT id, org_id, subject, payload, headers, attempts
FROM outbox
WHERE sent_at IS NULL
ORDER BY id
LIMIT @batch_size
FOR UPDATE SKIP LOCKED;

-- name: MarkOutboxSent :exec
UPDATE outbox SET sent_at = now(), attempts = attempts + 1, last_error = NULL
WHERE id = ANY(@ids::bigint[]);

-- name: MarkOutboxFailed :exec
UPDATE outbox SET attempts = attempts + 1, last_error = @last_error
WHERE id = @id;

-- name: CountPendingOutbox :one
SELECT count(*) FROM outbox WHERE sent_at IS NULL;
