# Integration Architecture (Connectors, Services, Composition, Mapping, Webhooks)

## 1. Connectors

```
ConnectorDefinition (catalog, global or org-private, versioned)
   └─ ConnectorVersion  (semver; config JSON Schema, auth methods, operations[ input/output schema ])
        └─ ConnectorInstance (org + workspace + environment; config + secret refs)  ← what users "connect"
              └─ Invoke(operation, input) → output     via Connector Runtime
```

### Runtime contract

```go
package connector

type Connector interface {
    Describe() Definition                                   // static metadata + schemas
    Test(ctx context.Context, cfg Config, creds Credentials) (TestResult, error)
    Invoke(ctx context.Context, req InvokeRequest) (InvokeResult, error)
}

type InvokeRequest struct {
    Operation      string
    Config         Config
    Credentials    Credentials        // resolved by secret.Resolve, never logged, zeroed after use
    Input          json.RawMessage    // validated against operation input schema before call
    IdempotencyKey string
    Deadline       time.Time
}
```

The **runtime wrapper** (not the connector) owns: input/output schema validation, timeout, retry policy
(bounded: ≤10 attempts, exponential + jitter, retry only on classified-retryable errors), per-instance rate limit
(Redis token bucket), per-instance concurrency limit, circuit breaker, metrics/traces, and redaction.

### Trust tiers ([ADR-0005](../adr/0005-connector-isolation.md))

| Tier | Examples | Execution |
|---|---|---|
| Built-in | HTTP/REST, GraphQL, PostgreSQL, MySQL, SMTP, Slack, MS Teams, S3/MinIO, Redis, n8n/Make/Zapier | Compiled into `hugi-worker`/`hugi-api` |
| Declarative | Org-defined REST connectors (OpenAPI/JSON spec, no code) | In-process, interpreted by HTTP connector — no custom code runs |
| Custom code | Built with `pkg/connectorsdk` | **Out-of-process** container speaking the Connector Protocol (gRPC); never loaded into the core process |

Phase 1 delivers built-in + declarative and the SDK/protocol definition with a reference sidecar; the managed
sandboxed runner for third-party code is Phase 3 (plugins).

### Authentication types

`api_key` (header/query), `basic`, `bearer`, `oauth2_auth_code` (with refresh, token stored as secret),
`oauth2_client_credentials`, `username_password`, `db_credentials`, `custom_headers`, `plugin` (Phase 3).
Auth is implemented as `AuthProvider`s applied to the egress request, shared by all HTTP-based connectors.

### Egress & SSRF ([security model](../security/security-model.md#ssrf))

All outbound traffic uses `platform/egress`:
- custom `net.Dialer.Control` rejects loopback, link-local (incl. 169.254.169.254), multicast, and RFC1918/ULA ranges
  **after DNS resolution** (prevents DNS rebinding), unless the destination matches the environment's
  **egress allowlist** (admins register internal CIDRs/hosts — needed because the Service Registry targets internal services);
- redirects re-validated per hop, max 5; response size cap; total timeout.

## 2. Service Registry

`services` (name, owner team, base URLs per environment, auth profile → connector instance) and `service_endpoints`
(method, path, params, request/response schema, auth requirement, description).

**OpenAPI import** (3.0/3.1, JSON/YAML, upload or URL via egress): parse with `kin-openapi`, resolve `$ref`s,
store the original spec in object storage (`service_specs`), extract endpoints, and show a **diff** on re-import
(added/removed/changed endpoints, breaking-schema flags) before applying. Compositions that reference a removed endpoint
fail validation on next publish.

## 3. API Composition

A composition version is a DAG:

```json
{
  "steps": [
    { "id": "order",    "call": { "service_id": "…", "endpoint_id": "…",
                         "input": { "path_values": { "id": "$.request.path.id" } } } },
    { "id": "customer", "depends_on": ["order"], "call": { "service_id": "…", "endpoint_id": "…",
                         "input": { "path_values": { "id": "$.steps.order.body.customer_id" } } } },
    { "id": "payment",  "depends_on": ["order"], "when": "steps.order.body.status != 'draft'", "call": {…} },
    { "id": "shipment", "depends_on": ["order"], "on_error": "null", "call": {…} }
  ],
  "response": { "mapping_id": "…", "mapping_version": 3 },
  "limits": { "timeout_ms": 5000, "max_concurrency": 4 }
}
```

- The execute request supplies `environment_id` and a request object (for example `path`, `query`, and `body`).
- Steps whose dependencies are satisfied run in bounded parallel goroutines; `when` is CEL.
- `on_error`: `fail` (default) | `null` | `default` (with a `default` value) | `fallback_step` (with a direct `fallback_step` dependency).
- Per-step timeout/retry; circuit breaker per (org, service, environment).
- Circuit-breaker counters are local to each API process; cross-replica sharing requires a distributed breaker store.
- Endpoint and mapping references are UUIDs pinned into immutable composition versions. Call inputs support the mapping
  engine's bounded JSONPath subset; the response mapping references a published mapping version.
- Caller credentials are **not** forwarded by default; each service uses its configured connector instance auth.
  Optional explicit propagation of a caller header allowlist.
- Hard limits (platform-level, org can only lower): 25 steps, 8 concurrent, 30s total, 5 MB aggregate response.
- Exposure: `POST /api-compositions/{id}/execute` (Phase 1 P0) and published gateway route
  `/{org}/{workspace}/{path}` on a separate listener with API-key/session auth (P1).

Implementation (S9, verified end to end in S14): the executor renders each step result as a plain JSON object
(`status`, `body`, `duration_ms`, `skipped`, `error`) so `when` conditions, input references and the response mapping
can all address `$.steps.<id>.body.*` with the same JSONPath subset. The `TestOrderOverviewCompositionEndToEnd`
integration test imports two OpenAPI services, runs `order → customer` with a `when` condition through the
egress-guarded explorer client, applies a published response mapping and checks the stored execution. The gateway
route is P1.

## 4. Mapping & Transformation

Mapping IR (stored as versioned JSON, edited visually or as JSON):

```json
{ "rules": [
  { "target": "name",   "source": "$.customer_name", "transforms": [{ "op": "trim" }] },
  { "target": "amount", "source": "$.total_amount",  "transforms": [{ "op": "to_number" }], "default": 0 },
  { "target": "items[]", "foreach": "$.lines", "rules": [ { "target": "sku", "source": "$.item.sku" } ] },
  { "target": "tier",   "expr": "src.total_amount > 1000000 ? 'gold' : 'std'", "when": "has(src.total_amount)" }
]}
```

- `source` uses a restricted JSONPath subset (field, index, wildcard) — simple mode.
- `expr`/`when` are CEL — advanced mode. Same IR, so simple-mode mappings are editable in advanced mode.
- Built-in transform ops: rename (implicit), default, to_string/number/bool/date, date_format (tz-aware), string ops
  (trim, upper, lower, concat, substring, replace, split, join), numeric (round, add, multiply…), null handling
  (`null_as`, `omit_if_null`), array/object mapping (`foreach`, nested rules), validation (target JSON Schema).
- **Test cases** (`mapping_tests`: input, expected output) run on every save and block publish if failing.
- Evaluator is pure, with limits on output size, iteration count and CEL cost.

## 5. Webhooks

### Inbound

```mermaid
sequenceDiagram
  participant S as Sender
  participant A as hugi-api /hooks/{public_id}
  participant P as Postgres
  participant N as JetStream
  participant W as hugi-worker
  S->>A: POST payload + signature headers
  A->>A: rate limit (per endpoint), size cap 1MB
  A->>A: verify signature (HMAC-SHA256, timestamp ±5min, constant-time compare)
  alt invalid
    A-->>S: 401 (logged, counted, payload not stored)
  else valid
    A->>P: dedupe on (endpoint, provider event id) ; insert webhook_events + outbox ; COMMIT
    A-->>S: 202
    W->>P: relay outbox → N: hugi.events.webhook.received
    N->>W: match filters → start bound workflow executions (idempotent on event id)
  end
```

Signature schemes supported per endpoint: Standard Webhooks (default), generic HMAC (configurable header/encoding),
provider presets (Stripe, GitHub, Slack) — implemented as `Verifier` strategies.

### Outbound

Subscriptions to platform events (catalog in `event` domain) or explicit workflow "send webhook" nodes.
Delivery: signed with Standard Webhooks headers; retry schedule `30s, 2m, 10m, 30m, 2h, 6h, 12h` (8 attempts ≈ 21h);
then `dead_lettered`. Every attempt recorded (status, latency, truncated response). **Replay** creates a new delivery
linked to the original. Endpoints auto-disabled after N consecutive days of failure, with notification.

Implementation (S12): a subscription belongs to a workspace and receives only events whose payload carries that
`workspace_id`; its environment's egress allowlist governs the destination (a refused destination dead-letters at
once). The `hugi-webhook-fanout` consumer on `HUGI_EVENTS` inserts one delivery per (subscription, event), idempotent on
the outbox message id. The dispatcher in `hugi-worker` claims due rows with `FOR UPDATE SKIP LOCKED` and leases them by
moving `next_attempt_at` past the attempt timeout, so a crashed worker's delivery becomes due again; the attempt
outcome is recorded only by the holder of that attempt number. `webhook-id` is `msg_<delivery id>`, stable across
retries; a replay has a new id. Auto-disable happens after 5 days of continuous failure (no notification yet).

## 6. External workflow tools (n8n, Make, Zapier)

Modeled as **built-in connectors**, nothing more: operations `trigger_workflow` (call their webhook / API) and inbound
status/events arrive through normal inbound webhook endpoints with a preset verifier. If they are down, only workflows
that explicitly call them are affected; the platform has no dependency on them.

Implementation (S13): the definitions `n8n`, `make` and `zapier` declare `trigger_workflow`, which the workflow node
`action.connector` invokes: the engine resolves the instance by ref key in the execution's workspace and environment
and opens its credential inside the step's tenant transaction, then POSTs the input to the configured `webhook_url`
through the environment's egress policy (headers `Idempotency-Key`, `X-Hugi-Execution-Id`, `X-Hugi-Node-Id`). An
unreachable tool yields the retryable `connector_unavailable`/`connector_timeout`; nothing else in the platform calls
the tools (deploy only checks the reference). Inbound status uses the verifier presets `n8n`/`make`/`zapier`: a
generated token in `X-Hugi-Token`, since none of these tools signs requests; payload convention
`{"run_id", "status"}` gives the event type `<tool>.<status>` and dedupe key `<run_id>:<status>`.
