# Data Plane

## Workloads (Phase 1)

| Workload | Where | Model |
|---|---|---|
| API composition execution | `hugi-api` (in-request) | Synchronous, bounded DAG, total deadline ≤ 30s |
| Workflow step execution | `hugi-worker` | Async, at-least-once step dispatch via JetStream, state in Postgres |
| Timers (delay, retry backoff, approval timeout) | `hugi-worker` | `workflow_timers` polled with `FOR UPDATE SKIP LOCKED` |
| Cron schedules | `hugi-worker` | `workflow_triggers.next_fire_at` polled with `SKIP LOCKED` |
| Inbound webhooks | `hugi-api` ingress | Verify → persist → 202 → event via outbox |
| Outbound webhook delivery | `hugi-worker` | JetStream work queue, retry schedule, DLQ |
| Outbox relay | `hugi-worker` | Poll `outbox` (`SKIP LOCKED`), publish to JetStream, mark sent |

## Scaling

- Workers are stateless; any number may run. Coordination happens through Postgres row locks and JetStream consumer
  groups — no leader election required in Phase 1.
- Worker pools are separated by JetStream subject so heavy connector traffic cannot starve webhook delivery:
  `hugi.wf.step.>`, `hugi.webhook.deliver.>`, `hugi.events.>`.
- Per-org concurrency quotas (max in-flight steps) are enforced at claim time to prevent noisy neighbours.

## Config reads

Published versions are immutable, so the worker caches them by `version_id` in-process (LRU) with no invalidation
needed. Mutable config (connector instances, secrets) is cached with short TTL plus outbox-driven invalidation.

## Delivery semantics

Everything in the data plane is **at-least-once**. Exactly-once *effects* are approximated by:
- step claims guarded by `status` + lease in Postgres (a duplicate message finds the step already claimed/done);
- an idempotency key per step attempt (`<execution_id>:<node_id>:<attempt>`) forwarded to connectors that support it;
- idempotency keys on triggers and on API mutations (`Idempotency-Key` header).
