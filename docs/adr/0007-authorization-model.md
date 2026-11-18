# ADR-0007: Unified Authorizer; native dynamic RBAC first

**Status:** Proposed

## Decision
Every domain calls one `authz.Authorizer` (+ `ScopeResolver` for list filtering). Phase 1 implementation is native
RBAC with data-defined resource types, permissions, roles and scoped bindings (org/workspace/resource), version-stamped
caching, and decision auditing. Phase 3 adds ABAC/ReBAC and external adapters as composable implementations behind the
same interface. Details: [authorization-architecture.md](../architecture/authorization-architecture.md).

## Alternatives
OpenFGA/OPA from day one: extra mandatory infra and a second source of truth for memberships before we need
relationship graphs. Casbin embedded: workable, but its model file is a second policy language to expose to tenants.
Both remain Phase 3 adapters.

## Consequences
Tenant check is a separate non-bypassable stage, so no adapter can ever grant cross-tenant access.
