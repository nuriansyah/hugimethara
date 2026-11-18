# Control Plane

The control plane is the set of domains that **define** things: organizations, identity, authorization, connector
instances, services, mappings, compositions, workflow definitions, webhook endpoints, audit.

## Properties

- **Store:** PostgreSQL only (Phase 1). Redis is a cache, never the source of truth.
- **Versioning:** executable artifacts (mapping, composition, workflow) have a mutable *draft* and immutable
  *published versions* (`version` integer, `checksum` sha256 of canonical JSON). The data plane only ever executes
  a published version id → executions are reproducible and version-pinned.
- **Validation at publish time:** structural (JSON Schema), semantic (graph acyclicity, reachable nodes, referenced
  connectors/secrets exist and are in the same org+environment), limits (node count, fan-out). Invalid drafts can be
  saved; they cannot be published.
- **Audit:** every mutation writes an `audit_events` row in the same transaction.
- **Events:** state changes the data plane cares about (e.g. `workflow.version.published`, `connector.instance.updated`)
  go through the transactional outbox so caches can be invalidated.

## Environments

Each workspace inherits the org's environments (default: `development`, `staging`, `production`). Connector instances,
secrets, webhook endpoints and workflow *deployments* are environment-scoped. A workflow version is environment-agnostic;
it is **deployed** to an environment, and connector references resolve per environment (`connector_ref: "crm"` →
the `crm` instance in the target environment). This gives promotion without editing the definition.

Phase 1 ships environment scoping on data; promotion workflows/approvals arrive in Phase 3.
