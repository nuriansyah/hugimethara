# Workflow Runtime

Native, bounded, Postgres-backed step engine with JetStream dispatch ([ADR-0003](../adr/0003-workflow-execution.md)).
It is deliberately **not** a general durable-execution system: no replayed user code, no unbounded histories.
Workflow definitions are declarative graphs, which is what makes a simple engine sufficient. Long-running/complex
cases are re-evaluated against Temporal in Phase 3 behind the `Executor` interface.

## 1. Definition

`workflow_versions.definition` (JSONB, immutable after publish, sha256 checksum):

```json
{
  "schema_version": 1,
  "trigger": { "type": "webhook", "config": { "endpoint_id": "…" } },
  "nodes": [
    { "id": "n1", "type": "action.connector", "config": { "connector_ref": "crm", "operation": "upsert_lead",
       "input_mapping": { "rules": [ … ] } }, "retry": { "max_attempts": 3, "backoff": "exponential", "initial_ms": 1000 },
       "timeout_ms": 15000 },
    { "id": "n2", "type": "logic.if", "config": { "condition": "nodes.n1.output.score > 50" } },
    { "id": "n3", "type": "human.approval", "config": { "assignees": { "group": "sales-managers" }, "timeout": "72h" } }
  ],
  "edges": [ { "from": "trigger", "to": "n1" }, { "from": "n1", "to": "n2" },
             { "from": "n2", "to": "n3", "port": "true" } ],
  "limits": { "max_steps": 500, "max_duration": "7d" }
}
```

Graph stored as one validated document (not `workflow_nodes`/`workflow_edges` rows): a published version is an atomic,
immutable unit and is never queried node-by-node. Recorded as a deliberate deviation from PRD §9's suggested tables.

### Publish-time validation
Acyclic (loops are explicit `logic.loop` containers, not back-edges); single trigger; all nodes reachable;
port names valid for node type; `logic.merge` inputs ≥ 2; CEL expressions compile and type-check against the known
context; connector refs/operations exist; limits within platform caps.

## 2. Node types (Phase 1)

| Kind | Types |
|---|---|
| Trigger | `trigger.manual`, `trigger.schedule` (cron + tz), `trigger.webhook`, `trigger.event` (platform event) |
| Action | `action.http`, `action.connector`, `action.db_query` (parameterized, via connector), `action.transform` (mapping), `action.notify` (email/Slack/Teams), `action.record_create`/`record_update` (connector ops), `action.composition` |
| Logic | `logic.if`, `logic.switch`, `logic.parallel` (fan-out), `logic.merge` (join: all/any), `logic.loop` (for-each over array, body subgraph, max 1000 iterations, bounded concurrency), `logic.delay`, `logic.filter` |
| Human | `human.approval`, `human.input` (form schema) |

Each node type is a Go implementation of:

```go
type NodeExecutor interface {
    Type() string
    ConfigSchema() *jsonschema.Schema
    Execute(ctx context.Context, in NodeInput) (NodeResult, error)
}
// NodeResult: Output (json), Port (for branching), Suspend (wait for timer/approval/input)
```

## 3. Execution state machine

```
Execution: pending → running → (waiting ⇄ running) → succeeded | failed | cancelled | timed_out
Step:      pending → queued → running → succeeded | failed | skipped | cancelled
                               └→ waiting (approval / input / delay) → queued
                               └→ retry_scheduled → queued
```

Transitions are performed by a single function `transition(step, event) (newState, effects)` — pure, table-driven,
unit-tested exhaustively. Effects (create next steps, schedule timer, publish outbox) are applied in **one Postgres
transaction** with the state change.

## 4. Step lifecycle

```mermaid
sequenceDiagram
  participant T as Trigger (API/webhook/cron)
  participant P as Postgres
  participant R as Outbox relay
  participant N as JetStream
  participant W as Worker
  participant X as External system
  T->>P: tx: insert execution(version pinned) + first steps(queued) + outbox(step.dispatch)
  R->>P: SELECT … FROM outbox FOR UPDATE SKIP LOCKED
  R->>N: publish hugi.wf.step.{org} {step_id}
  N->>W: deliver (consumer group)
  W->>P: UPDATE step SET status='running', lease_until=now()+T WHERE id=$1 AND status='queued'
  alt 0 rows (duplicate / already taken)
    W->>N: ack, stop
  end
  W->>X: execute node (ctx deadline, idempotency key)
  W->>P: tx: step→succeeded(output) ; compute ready successors ; insert steps + outbox ; maybe execution→succeeded
  W->>N: ack
```

**Join semantics** — `logic.merge` becomes ready when every inbound edge is resolved; branches not taken are marked
`skipped` and propagate skip downstream (dead-path elimination), so merges after `if` never deadlock.

**Recovery** — a reaper (in every worker, `SKIP LOCKED`) re-queues `running` steps whose `lease_until` expired
(worker crash) and re-publishes outbox rows stuck unsent. Workers heartbeat-extend leases for long steps.

**Payloads** — step outputs ≤ 64 KB inline (JSONB); larger go to object storage with a reference. Hard cap 10 MB.

**Retries** — on retryable failure: `retry_scheduled` + `workflow_timers` row at backoff time; timers become queued
steps. Bounded by node policy and platform cap (10).

**Timeouts** — per-step (context deadline) and per-execution (`deadline_at`, enforced by reaper).

**Cancellation** — `POST /workflow-executions/{id}/cancel` moves the execution straight to `cancelled` under its
row lock and cancels open steps, pending tasks and timers. A step running on a worker is fenced: its completion is
discarded and its lease heartbeat fails, which cancels the node's context within a third of the lease. (No
separate `cancelling` state or `hugi.wf.cancel` subject is needed for that.)

**Idempotency** — trigger: unique `(workflow_id, idempotency_key)`; step side-effects: key
`{execution_id}:{node_id}:{iteration}:{attempt}`.

**Human tasks** — approval/input create `approval_tasks`; the step is `waiting`. Decision API (authorized against
the assignee rule) resumes it; timeout timer resolves to the node's `timeout` port, or fails the execution with
`task_expired` when the node has no timeout branch. The requester cannot approve their own execution unless the
node sets `allow_self_approval`. Resuming completes the waiting step directly (`waiting → succeeded`, output =
the decision, carrying the original input as `request`) instead of re-queuing it: there is no work left to run.
Decisions and timers both lock the execution row before the task/timer row, so they cannot deadlock.

**Timers** — `workflow_timers` rows (`delay`, `task_timeout`) are fired by the reaper in every worker; each is
handled in its own tenant transaction (`SKIP LOCKED`), so a waiting step never holds a worker or a lease.

**Triggers** — schedule triggers store `next_fire_at` on deploy; the trigger runner claims due rows
(`SKIP LOCKED`), starts one execution per activation (idempotency key = scheduled instant) and computes the next
activation from now, so downtime collapses missed slots into one run. Event triggers consume `HUGI_EVENTS`
(new messages only), key executions by the outbox message id, skip a workflow's own events and stop chains deeper
than 5 (`trigger_depth`).

**Re-run** — `POST /workflow-executions/{id}/retry` on a failed/timed-out execution deletes its failed and
cancelled steps, reopens it with a fresh deadline and lets progression re-queue the affected nodes; succeeded
steps keep their outputs. The previous error is kept in the audit event.

**Loops (Phase 1 scope)** — `logic.loop` runs one action (`action.http` / `action.transform`) per item rather than
a body sub-graph; its timeout bounds the whole loop. Sub-graph loops stay a Phase 3 item with sub-workflows.

**Limits** — per execution: max steps (500 default, 5000 cap), max loop iterations (1000), max parallel branches (50),
max duration, `max_concurrent_executions` per workflow and environment (refused with
`concurrency_limit_exceeded`); per org: concurrent running steps, executions/minute (not yet enforced). Exceeding a
limit fails or refuses with a specific reason code — never silently truncates.

## 5. Executor abstraction

```go
type Executor interface {
    Start(ctx context.Context, req StartRequest) (ExecutionID, error)
    Cancel(ctx context.Context, id ExecutionID) error
    Signal(ctx context.Context, id ExecutionID, signal Signal) error // approval / input
}
```

`NativeExecutor` (Phase 1). `TemporalExecutor` (Phase 3 candidate) would translate the same definition into a generic
interpreter workflow — definitions never contain Temporal concepts.

## 6. Observability

Each execution is a trace (root span from trigger; step spans as children, links across async hops via propagated
trace context in the JetStream message headers). Metrics: `hugi_workflow_executions_total{status}`,
`hugi_workflow_step_duration_seconds{node_type}`, `hugi_workflow_queue_lag_seconds`, `hugi_workflow_steps_inflight{org}`.
The execution-history UI reads `workflow_execution_steps` (inputs/outputs redacted per secret refs).
