# Architecture Decision Records

Format: Context → Decision → Alternatives → Consequences. Status: Proposed | Accepted | Superseded.
All records below are **Proposed** until the blueprint review; accepting them is the gate to start Phase 1 code.

| # | Decision | Status |
|---|---|---|
| [0001](0001-modular-monolith.md) | Modular monolith with separately scaled binaries | Proposed |
| [0002](0002-postgresql-tenancy.md) | PostgreSQL shared schema, `org_id` + RLS tenancy | Proposed |
| [0003](0003-workflow-execution.md) | Native Postgres-state workflow engine; Temporal deferred | Proposed |
| [0004](0004-messaging-nats-jetstream.md) | NATS JetStream + transactional outbox | Proposed |
| [0005](0005-connector-isolation.md) | Connector trust tiers; custom code out-of-process | Proposed |
| [0006](0006-identity-federation.md) | Server-side sessions; OIDC federation; local auth | Proposed |
| [0007](0007-authorization-model.md) | Unified Authorizer; native dynamic RBAC first | Proposed |
| [0008](0008-analytical-store-clickhouse.md) | ClickHouse for analytics (Phase 2) | Proposed |
| [0009](0009-frontend-architecture.md) | Next.js client app, OpenAPI-first contracts | Proposed |
| [0010](0010-expression-language-cel.md) | CEL for conditions, mapping expressions, policies | Proposed |
| [0011](0011-secrets-envelope-encryption.md) | Envelope encryption for secrets | Proposed |
| [0012](0012-data-access-sqlx.md) | sqlx + plain .sql files (replaces sqlc) | Accepted |
| [0013](0013-entitlements.md) | Plans, paid features and quotas | Accepted |
