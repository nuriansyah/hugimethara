# System Overview

## 1. Shape of the system

Hugimethara is a **modular monolith** (see [ADR-0001](../adr/0001-modular-monolith.md)) written in Go, deployed as
a small number of independently scalable **binaries built from one codebase**:

| Binary | Plane | Responsibility | Scales on |
|---|---|---|---|
| `hugi-api` | Control + sync data plane | REST API, auth/session, config CRUD, synchronous API-composition execution, inbound webhook ingress | HTTP RPS |
| `hugi-worker` | Data plane | Workflow step execution, connector calls, outbound webhook delivery, outbox relay, timers/schedules | Queue depth |
| `hugi-migrate` | Ops | Database migrations + seed | One-shot |
| `web` (Next.js) | UI | Browser application | HTTP RPS |

The Control Plane writes configuration to PostgreSQL. The Data Plane reads **immutable, versioned snapshots** of that
configuration (published workflow versions, composition versions, connector instance configs) and caches them, so
execution never round-trips to "the control plane service" per request — it reads Postgres/cache directly through
the owning domain's service interface. That is how §4.2's "control plane must not be a bottleneck" is met without a
network hop between planes.

## 2. Component diagram

```mermaid
flowchart LR
  subgraph Browser
    WEB[Next.js web]
  end
  subgraph External
    IDP[OIDC IdP<br/>Entra / Google / Keycloak]
    EXT[External APIs / DBs / SaaS]
    SRC[Webhook senders]
    DST[Webhook receivers]
  end
  subgraph hugi-api
    MW[Middleware: request-id, otel, session, tenant, rate-limit, CSRF]
    CP[Control-plane domains<br/>org · identity · authz · connector · service · mapping · composition · workflow · webhook · audit]
    COMP[Composition runtime - sync]
    INGRESS[Webhook ingress]
  end
  subgraph hugi-worker
    RELAY[Outbox relay]
    WF[Workflow step executor]
    TIMER[Timers & schedules]
    DELIV[Outbound webhook delivery]
  end
  PG[(PostgreSQL<br/>metadata + execution state)]
  RD[(Redis<br/>cache · rate limits · authz version)]
  NATS[(NATS JetStream<br/>dispatch)]
  S3[(S3 / MinIO<br/>large payloads, specs)]

  WEB -->|cookie session| MW --> CP
  MW --> COMP
  SRC --> INGRESS
  WEB -. OIDC redirect .-> IDP
  CP --> PG
  CP --> RD
  COMP --> EXT
  INGRESS --> PG
  RELAY --> PG
  RELAY --> NATS
  NATS --> WF
  NATS --> DELIV
  WF --> PG
  WF --> EXT
  WF --> S3
  DELIV --> DST
  TIMER --> PG
```

ClickHouse joins in Phase 2 (analytics), see [data-architecture.md](data-architecture.md).

## 3. Repository layout

```
hugimethara/
├── api/openapi/                 # OpenAPI 3.0.3 spec, split per domain, bundled in CI
├── backend/
│   ├── go.mod                   # module github.com/<org>/hugimethara/backend
│   ├── cmd/
│   │   ├── hugi-api/            # composition root for the API process
│   │   ├── hugi-worker/         # composition root for workers
│   │   └── hugi-migrate/
│   ├── internal/
│   │   ├── platform/            # cross-cutting, domain-free: config, db (pgx pool, tx, RLS session),
│   │   │                        #   httpx (errors, pagination, middleware), telemetry, crypto (envelope enc),
│   │   │                        #   egress (SSRF-safe HTTP client), bus (NATS), cache (Redis), ids (UUIDv7), clock
│   │   ├── audit/
│   │   ├── organization/        # orgs, settings, workspaces, members, departments, environments
│   │   ├── identity/            # users, sessions, local auth, OIDC, invitations, groups, service accounts
│   │   ├── authz/               # Authorizer, RBAC engine, resource registry, roles, bindings
│   │   ├── secret/              # credential vault (envelope encryption)
│   │   ├── connector/           # registry, runtime, built-in connectors (connector/builtin/*)
│   │   ├── servicereg/          # service registry, OpenAPI import
│   │   ├── mapping/             # mapping IR, compiler, evaluator, test cases
│   │   ├── composition/         # API composition definitions + sync runtime
│   │   ├── workflow/            # definitions, validator, engine, node implementations
│   │   ├── webhook/             # inbound endpoints, outbound subscriptions, deliveries
│   │   └── event/               # transactional outbox, platform event catalog
│   ├── migrations/              # goose SQL migrations
│   └── internal/platform/testsupport/  # integration harness: fresh DB per test, in-process NATS
├── pkg/connectorsdk/            # public Go SDK for out-of-process connectors (separate module)
├── web/
│   └── src/
│       ├── app/                 # Next.js routes: (auth)/…, (app)/[org]/[workspace]/…
│       ├── features/<feature>/  # api hooks, components, zod schemas per feature
│       ├── components/ui/       # shadcn/ui primitives
│       └── lib/                 # generated API client, auth, permission helpers
├── deploy/
│   ├── compose/                 # docker-compose.yml (+ keycloak, minio, otel profiles)
│   └── helm/
└── docs/
```

Each backend domain package follows the same internal shape:

```
internal/workflow/
  model.go      # domain types + invariants (no I/O)
  service.go    # use cases; the ONLY exported entry point for other domains
  queries.sql   # plain, named SQL statements (embedded; ADR-0012)
  store.go      # row structs + persistence helpers (sqlx)
  http.go       # handlers; translate HTTP <-> service calls
  *_test.go
```

Dependency rules are in [domain-boundaries.md](domain-boundaries.md) and are enforced with `depguard`/`go-arch-lint` in CI.

## 4. Request path (synchronous API call)

```mermaid
sequenceDiagram
  participant B as Browser
  participant A as hugi-api
  participant R as Redis
  participant P as PostgreSQL
  B->>A: POST /v1/hugimethara/workflows/{id}/execute (cookie, X-CSRF-Token)
  A->>A: request-id, trace span, rate limit
  A->>R: session lookup (fallback PG)
  A->>A: resolve principal {user, org, groups}
  A->>P: BEGIN; SET LOCAL app.org_id = …   (RLS)
  A->>A: authz.Evaluate(principal, workflow:execute, resource)
  A->>P: insert execution + outbox row; COMMIT
  A-->>B: 202 {execution_id}
  Note over A,P: audit event written in the same transaction
```

## 5. Cross-cutting concerns

| Concern | Approach | Doc |
|---|---|---|
| Tenancy | `org_id` on every tenant row, composite FKs, Postgres RLS as defense-in-depth | [ADR-0002](../adr/0002-postgresql-tenancy.md) |
| AuthN | Server-side sessions (opaque cookie), local + OIDC login, API keys for service accounts | [ADR-0006](../adr/0006-identity-federation.md) |
| AuthZ | Single `Authorizer` contract, native RBAC in Phase 1 | [authorization-architecture.md](authorization-architecture.md) |
| Secrets | Envelope encryption (AES-256-GCM), KEK from env/KMS | [security model](../security/security-model.md) |
| Egress | All outbound HTTP/DB through `platform/egress` with SSRF guard + per-env allowlist | [integration-architecture.md](integration-architecture.md) |
| Expressions | CEL (sandboxed, cost-limited) for conditions & advanced mapping | [ADR-0010](../adr/0010-expression-language-cel.md) |
| Observability | OpenTelemetry traces/metrics, `slog` JSON logs, Prometheus scrape | [operations](../operations/observability.md) |
| Contracts | OpenAPI-first; Go server stubs via oapi-codegen (chi), TS client via openapi-typescript | [ADR-0009](../adr/0009-frontend-architecture.md) |
