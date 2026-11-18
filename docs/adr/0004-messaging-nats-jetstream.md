# ADR-0004: NATS JetStream with a transactional outbox

**Status:** Proposed

## Decision
Use NATS JetStream for dispatch and platform events. Never publish directly from a request handler: write an `outbox`
row in the business transaction; a relay publishes and marks it sent (at-least-once, `Nats-Msg-Id` = outbox id for
JetStream de-dup window). Streams: `HUGI_WF` (work-queue retention), `HUGI_WEBHOOK` (work-queue), `HUGI_EVENTS`
(limits retention, 7d). Subjects include org id for per-tenant observability and future per-tenant consumers.

## Alternatives
Kafka (heavier ops, overkill at this scale), RabbitMQ (fine, but NATS is lighter and already specified),
Redis Streams (durability trade-offs), Postgres LISTEN/NOTIFY (not durable).

## Consequences
No dual-write inconsistency. Consumers must be idempotent (they are, by step claim / delivery status).
