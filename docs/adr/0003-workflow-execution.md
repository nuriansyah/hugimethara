# ADR-0003: Native workflow engine on Postgres state; Temporal deferred

**Status:** Proposed

## Context
PRD requires a native engine (no n8n), with conditions, parallelism, approvals, schedules, retries, timeouts,
cancellation, recovery and version pinning — and warns against "an incomplete imitation of Temporal".

## Decision
Workflow definitions are **declarative DAGs** (no user code). The engine is a step scheduler:
Postgres holds authoritative execution/step state; transitions are a pure state machine applied transactionally with
their effects; JetStream dispatches step IDs; workers claim steps with a lease; a reaper recovers expired leases;
timers live in a table. See [workflow-runtime.md](../architecture/workflow-runtime.md).

Because there is no replayed user code, we need neither event-sourced histories nor deterministic-replay — the
features that make Temporal hard to imitate. Scope is explicitly bounded by limits.

All callers go through an `Executor` interface. Re-evaluate Temporal in Phase 3 when we need: sub-workflow
compensation sagas, very long timers at high volume, or cross-region execution.

## Alternatives
- **Temporal now**: strongest guarantees, but adds a cluster (+ its DB) to the mandatory footprint, conflicting with
  "optional infra not mandatory in first release"; definitions would still need an interpreter workflow on top.
- **Postgres-only queue (no NATS)**: viable and simpler; rejected only because NATS is a stated core component and
  gives us cheap fan-out for events/cancellation signals. Engine correctness does not depend on NATS (Postgres is truth;
  lost messages are recovered by the reaper/outbox re-publish).

## Consequences
+ Small mandatory footprint, transparent state, easy history UI. − We own correctness of the scheduler: mitigated
by exhaustive state-machine tests, chaos tests (kill workers mid-step), and keeping semantics at-least-once.
