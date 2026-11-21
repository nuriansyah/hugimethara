-- Transactional outbox (ADR-0004).

-- name: InsertOutbox
-- Runs inside a tenant-bound business transaction (RLS checks org_id).
INSERT INTO outbox (org_id, subject, payload, headers)
VALUES ($1, $2, $3, $4)
RETURNING id;

-- name: ClaimPendingOutbox
-- Relay (hugi_system). Rows stay locked until the relay's transaction ends,
-- so concurrent relays never publish the same row.
SELECT id, org_id, subject, payload, headers, attempts
FROM outbox
WHERE sent_at IS NULL
ORDER BY id
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: MarkOutboxSent
UPDATE outbox SET sent_at = now(), attempts = attempts + 1, last_error = NULL
WHERE id = ANY($1::bigint[]);

-- name: MarkOutboxFailed
UPDATE outbox SET attempts = attempts + 1, last_error = $2
WHERE id = $1;

-- name: CountPendingOutbox
SELECT count(*) FROM outbox WHERE sent_at IS NULL;
