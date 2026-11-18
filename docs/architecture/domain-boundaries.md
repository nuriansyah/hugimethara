# Domain Boundaries

## Rules

1. **A domain owns its tables.** No other package reads or writes them. Cross-domain access goes through the owning
   domain's exported `Service` interface.
2. **Dependencies point downward** in the layering below. Cycles are a build failure (`go-arch-lint`).
3. **`platform/` knows no domain.** It must not import any `internal/<domain>` package.
4. **Cross-domain reactions use events**, not calls, when the caller does not need the result synchronously
   (e.g. `organization.created` → `authz` seeds default roles is a *synchronous* call inside the same tx because
   the org is unusable without it; `workflow.execution.failed` → notifications is an *event*).
5. **Every service method takes a `Principal`-bearing `context.Context`** and every store query is tenant-scoped.

## Layering (Phase 1)

```
L4  workflow · composition · webhook               (orchestration; consume L3)
L3  connector · servicereg · mapping                (integration building blocks)
L2  secret · authz                                  (governance primitives)
L1  identity · organization · audit · event         (tenancy & identity foundation)
L0  platform/*                                      (no domain knowledge)
```

`authz` sits above `identity`/`organization` because evaluation needs group membership and workspace hierarchy;
those two expose read interfaces (`MembershipReader`, `WorkspaceHierarchy`) and never call `authz` themselves —
HTTP handlers in every domain call `authz` before invoking their service.

## Domain catalogue

| Domain | Phase | Owns | Exposes |
|---|---|---|---|
| organization | 1 | organizations, organization_settings, workspaces, workspace_members, departments, environments | org/workspace lifecycle, `WorkspaceHierarchy` |
| identity | 1 | users, user_credentials, sessions, identity_providers, identity_links, groups, group_members, group_mappings, invitations, service_accounts, api_keys | `Authenticate`, `MembershipReader`, user/group CRUD |
| authz | 1 (RBAC) / 3 (ABAC, ReBAC, adapters) | resource_types, permissions, roles, role_permissions, role_bindings, authz_decisions | `Authorizer`, `ResourceRegistry`, `EffectivePermissions` |
| audit | 1 | audit_events | `Record(ctx, Event)` (tx-aware) |
| event | 1 | outbox | `Publish(tx, Event)`, event catalog |
| secret | 1 | secrets, data_keys | `Put`, `Resolve` (data plane only), never `Reveal` over HTTP |
| connector | 1 | connector_definitions, connector_versions, connector_instances, connector_health | definition catalog, `Invoke(op, input)` |
| servicereg | 1 | services, service_endpoints, service_specs | endpoint lookup, OpenAPI import |
| mapping | 1 | mappings, mapping_versions, mapping_tests | `Compile`, `Apply` |
| composition | 1 | api_compositions, api_composition_versions, api_composition_executions | sync `Execute` |
| workflow | 1 | workflows, workflow_versions, workflow_triggers, workflow_executions, workflow_execution_steps, workflow_timers, approval_tasks | publish, trigger, engine |
| webhook | 1 | webhook_endpoints, webhook_events, webhook_subscriptions, webhook_deliveries | ingress, delivery |
| datasource, ingestion, dataset, semantic, dashboard | 2 | — | — |
| plugin, observability, billing | 3 | — | — |

## Shared execution primitives

API composition and workflows both need "call an endpoint/connector op with mapped input, apply retry/timeout,
map the output". This lives in **`connector.Invoke`** + **`mapping.Apply`**, not in either orchestrator, so both
engines behave identically and are tested once.
