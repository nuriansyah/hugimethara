# Phase 1 — Integration, API Orchestration & Workflow Foundation

Goal: an organization can onboard (local or OIDC), structure itself into workspaces with dynamic RBAC, connect
systems, compose APIs, map data, and run native workflows triggered manually, on schedule, or by webhook —
with secrets encrypted, everything audited, and the whole stack running from Docker Compose.

## 1. Vertical slices (in order)

Each slice = migration + backend (service, handlers, authz) + OpenAPI + frontend screens + tests + docs update.
A slice is "done" only against the Definition of Done (PRD §19).

| # | Slice | Delivers (DB · API · UI) | Exit check |
|---|---|---|---|
| S0 | **Foundation** | Monorepo, Go module, config, slog, OTel, health/ready, pgx+RLS session helper, goose, sqlx + plain .sql (ADR-0012), oapi-codegen pipeline, error/pagination/request-id middleware, egress client skeleton, compose (pg, redis, nats, minio), CI (lint, test, vuln scan, arch-lint), Next.js shell + design system + app layout + generated client | `docker compose up` → web shell + `/readyz` green; CI green |
| S1 | **Org bootstrap, sessions, audit** | organizations, users, memberships, credentials, sessions, audit_events, outbox · signup, login/logout, `/me`, org settings · login, signup, org settings, audit log list | Integration: signup → login → session revoke; audit rows written in-tx |
| S2 ✅ | **Workspaces & environments** (+ plans/paid features, ADR-0013) | workspaces, members, departments, environments · CRUD · workspace switcher, workspace mgmt, environments page | Org creates 5 workspaces; members assigned |
| S3 ✅ | **Dynamic RBAC** | resource_types, roles, role_permissions, role_bindings, authz_decisions · Authorizer + ScopeResolver, version-stamped cache, `/authorization/evaluate`, `/me/permissions` · roles & permissions editor, bindings, "check access" simulator | Exhaustive RBAC unit tests; cross-tenant + escalation security tests for S1–S3 routes; **all existing handlers now authorized** |
| S4 ✅ | **Users, groups, invitations, service accounts** | groups, invitations, service_accounts, api_keys · CRUD + invite accept · users, groups, service accounts screens | Invited user joins with workspace role; API key auth works |
| S5 ◐ | **OIDC federation** | identity_providers, identity_links, group_mappings · OIDC start/callback (PKCE), presets Entra/Google/Keycloak/generic, JIT, group mapping · IdP settings page, SSO button | E2E against Keycloak in compose: login + group-based access |
| S6 | **Secrets & connectors** | data_keys, secrets, connector_* · envelope encryption, connector runtime wrapper (validate, timeout, retry, rate-limit, breaker), built-ins: HTTP/REST, PostgreSQL, MySQL, SMTP, Slack, MS Teams, S3/MinIO · catalog, connection config forms generated from JSON Schema, test connection, health | Secrets never in responses/logs (test); SSRF matrix tests; connection test works for each built-in |
| S7 | **Service registry & API explorer** | services, service_environments, service_specs, service_endpoints · OpenAPI import with diff, try-endpoint · service registry, import wizard, API explorer | Import petstore + mock order service; call endpoint via explorer |
| S8 | **Mapping engine & studio** | mappings, versions, tests · IR compiler/evaluator (JSONPath subset + transform ops + CEL), publish gated by tests · Mapping Studio (visual field mapper + JSON/CEL advanced) | PRD §6.7 example passes; property tests for evaluator limits |
| S9 ✅ | **API composition** | api_compositions, versions, executions · DAG runtime (parallel, `when`, on_error, timeouts, retries, breaker, limits), execute endpoint · composition editor (step graph + mapping), run & inspect | PRD §6.6 order-overview scenario E2E against mock services |
| S10 | **Workflow engine core** | workflows, versions, deployments, triggers, executions, steps · validator, state machine, outbox relay, step dispatcher/claim/lease/reaper, nodes: manual trigger, http, connector, transform, composition, if, parallel, merge, filter · Workflow Designer (React Flow), publish/deploy, execution history with step I/O | Conditional + parallel workflow executes; worker kill mid-run recovers; failures inspectable |
| S11 ◐ | **Workflow engine complete** | timers, approval_tasks · schedule trigger, webhook/event triggers, delay, switch, loop, db_query, notify, record create/update, approval, manual input, retry/timeout/cancel, re-run from failed step · schedules page, approvals inbox, cancel/retry controls | Finance approval E2E; limits enforced with clear reason codes |
| S12 ✅ | **Webhook engine** | webhook_* · ingress w/ verifiers (Standard Webhooks, HMAC, Stripe/GitHub/Slack presets), dedupe, outbound subscriptions, retry schedule, DLQ, replay, event catalog · webhooks management, delivery logs, replay | Payment webhook → reconciliation workflow E2E; forgery/replay rejected |
| S13 ✅ | **External workflow tools** | n8n / Make / Zapier connectors (trigger + inbound status presets) | Platform works with them unreachable (test) |
| S14 ✅ | **Phase review & hardening** | OpenAPI docs page, overview dashboard (org KPIs: executions, failures, connector health), full security suite, k6 baselines, docs pass, acceptance review | Every §6.12 criterion verified with evidence in IMPLEMENTATION_STATUS.md |

## 2. Acceptance criteria → slices

| PRD §6.12 criterion | Slice(s) |
|---|---|
| Org can create multiple workspaces | S2 |
| Users can be assigned to workspaces | S2, S4 |
| Dynamic RBAC works | S3 |
| OIDC login works | S5 |
| Connector registered/configured; connection tests | S6 |
| OpenAPI import | S7 |
| Multiple APIs composed; response mapping | S8, S9 |
| Native workflow without n8n; conditions & parallel | S10 |
| Workflow failures inspectable | S10, S11 |
| Webhooks received and delivered | S12 |
| Secrets encrypted | S6 |
| Audit events recorded | S1 onward |
| Backend APIs documented | every slice (OpenAPI), S14 |
| Integration tests pass | every slice |
| Docker Compose deployment works | S0, S14 |

## 3. Prioritized backlog

### P0 — foundational (Phase 1 cannot pass acceptance without these)
- S0 foundation in full (incl. CI, RLS helper, outbox table, egress guard, OpenAPI pipeline).
- Org signup, local login, sessions, CSRF, logout, session revocation.
- Audit recording (tx-bound) + audit list API.
- Workspaces, membership, environments.
- Resource registry, roles, permissions, bindings, Authorizer, ScopeResolver, decision audit, `/me/permissions`.
- Cross-tenant & escalation security test harness (route-table driven).
- Invitations; groups.
- OIDC login with Keycloak + generic preset; claim → group mapping.
- Secret vault (envelope encryption, local KEK).
- Connector runtime wrapper + HTTP/REST, PostgreSQL, SMTP, Slack built-ins; connection test.
- Service registry + OpenAPI import (no diff UI yet) + API explorer try-call.
- Mapping IR + evaluator + tests + publish.
- Composition runtime + execute endpoint + editor.
- Workflow: definition/validator/publish/deploy, state machine, relay, dispatcher, lease/reaper, manual + webhook + schedule triggers, http/connector/transform/if/switch/parallel/merge/delay/filter/approval nodes, retries, timeouts, cancellation, limits, execution history UI.
- Inbound webhooks with HMAC + Standard Webhooks; outbound subscriptions with retry, DLQ, replay, delivery logs.
- Docker Compose full stack; OpenAPI published.

### P1 — usable first release
- Service accounts + API keys.
- Entra ID & Google Workspace presets; "SSO only" enforcement; TOTP MFA for local accounts.
- MySQL, MS Teams, S3/MinIO, Redis built-in connectors; OAuth2 auth-code flow for SaaS connectors (HubSpot, Google Sheets, Salesforce).
- Declarative (org-defined) REST connectors.
- OpenAPI re-import diff with breaking-change detection.
- Composition gateway exposure (`/gw/...`).
- Loop node, manual-input node, db_query node, record create/update, notify node, composition node, single-node test.
- Re-run execution from failed step; SSE progress stream.
- Event trigger (platform events) + provider verifier presets (Stripe, GitHub, Slack).
- n8n/Make/Zapier connectors.
- Simple/Advanced mode toggle; onboarding checklist; contextual docs.
- Overview dashboard (operational KPIs); Helm chart; k6 baselines.

### P2 — important improvements
- Connector SDK + gRPC Connector Protocol + reference out-of-process runner.
- Workflow templates gallery (marketing lead sync, payment reconciliation, approval).
- Connector version compatibility checks on upgrade.
- Per-org quotas UI; usage counters.
- GraphQL, SOAP, Oracle, WhatsApp-provider connectors.
- KMS-backed `KeyProvider` (AWS/GCP/Vault).
- Audit export (CSV/JSON stream).
- Webhook endpoint auto-disable + notifications.

### P3 — future (Phase 2/3)
- SAML, SCIM, LDAP; ABAC/ReBAC; OPA/Cedar/OpenFGA/Casbin adapters; policy simulation/rollout.
- Temporal executor evaluation; sub-workflows; compensation; workflow migration.
- Plugin ecosystem & sandboxed runtime.
- Data platform, ClickHouse, semantic layer, dashboards (Phase 2).
- MCP server, NL→SQL/workflow, anomaly detection, cost management.

## 4. Open questions for blueprint review

Defaults applied when S0 started (change any of them by saying so):
module path `github.com/nuriansyah/hugimethara` (from the git remote); Go 1.26+;
local password login kept for bootstrap/SMB; deployment model and first SaaS connectors still open.

1. **Deployment model:** multi-tenant SaaS, self-hosted single org, or both? (Affects signup flow, billing, and whether `organizations` signup is public.)
2. **Go module path / GitHub org** for `go.mod` and image names.
3. **Local password auth:** keep for bootstrap/SMB (proposed), or SSO-only from day one?
4. **Go version:** Go 1.26.0 is the minimum required by the current dependency graph. The workspace must use Go 1.26+.
5. **First built-in SaaS connectors** after HTTP/Postgres/SMTP/Slack: which 3 matter most (HubSpot, Salesforce, Google Sheets, WhatsApp provider)?
