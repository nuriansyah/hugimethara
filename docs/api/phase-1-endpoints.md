# Phase 1 API Contract Plan

All paths relative to `/v1/hugimethara`. "Perm" is the permission evaluated (resource type : action).
Slice = the vertical slice that delivers it ([phase-1.md](../phases/phase-1.md)).

| Method | Path | Perm | Slice |
|---|---|---|---|
| **Health** ||||
| GET | /healthz, /readyz (root, unversioned) | public | ✅ S0 |
| **Auth & self** ||||
| POST | /organizations (signup: org + owner) | public, rate-limited | ✅ S1 |
| POST | /auth/login · /auth/logout | public / session | ✅ S1 |
| GET | /auth/oidc/{idp_id}/start · /auth/oidc/callback | public | S5 |
| GET | /me · /me/sessions; DELETE /me/sessions/{id} · /me/permissions | session | ✅ S1/S3 |
| **Organization** ||||
| GET/PATCH | /organizations/current | organization:read / :update | ✅ S1 |
| CRUD | /departments | department:* | S2 |
| CRUD | /environments | environment:* | S2 |
| CRUD | /workspaces | workspace:create/read/update/archive | S2 |
| GET/POST/DELETE | /workspaces/{id}/members | workspace:manage_members | S2 |
| **Identity** ||||
| GET/PATCH | /users, /users/{id} (status, profile) | user:read / :update | S4 |
| POST/GET/DELETE | /invitations; POST /invitations/{token}/accept | user:invite | S4 |
| CRUD | /groups, /groups/{id}/members | group:* | S4 |
| CRUD | /service-accounts; POST/DELETE /service-accounts/{id}/keys | service_account:* | S4 |
| CRUD | /identity/providers; POST /identity/providers/{id}/test | identity_provider:* | S5 |
| CRUD | /identity/providers/{id}/group-mappings | identity_provider:update | S5 |
| **Authorization** ||||
| GET | /resource-types | role:read | ✅ S3 |
| CRUD | /roles (+ permissions in body; PUT replaces) | role:* | ✅ S3 |
| GET/POST/DELETE | /role-bindings | role:read / role:bind | ✅ S3 |
| POST | /authorization/evaluate (simulation, not logged) | authorization:simulate | ✅ S3 |
| GET | /authorization/decisions | authorization:read_decisions | ✅ S3 |
| **Audit** ||||
| GET | /audit-events (filters: actor, resource, action, time) | audit_event:read | ✅ S1 (API + UI) |
| **Secrets & connectors** ||||
| GET | /connector-definitions, /connector-definitions/{id}/versions | connector:read | S6 |
| POST | /connector-definitions (declarative, org-private) | connector:define | S6 (P1) |
| CRUD | /connectors (instances; credentials write-only) | connector:* | S6 |
| POST | /connectors/{id}/test | connector:test | S6 |
| POST | /connectors/{id}/operations/{op}/invoke (dry-run/test) | connector:invoke | S6 |
| GET | /connectors/{id}/health | connector:read | S6 |
| CRUD | /credentials (standalone secrets: write-only values) | secret:* | S6 |
| **Service registry** ||||
| CRUD | /services, /services/{id}/environments | service:* | S7 |
| POST | /services/{id}/import-openapi (upload or url; `?dry_run=true` returns diff) | service:update | S7 |
| GET | /services/{id}/endpoints, /service-endpoints/{id} | service:read | S7 |
| POST | /service-endpoints/{id}/try (API explorer call) | service:invoke | S7 |
| **Mapping** ||||
| CRUD | /mappings; POST /mappings/{id}/publish; GET /mappings/{id}/versions | mapping:* | S8 |
| POST | /mappings/{id}/test (ad-hoc input or stored tests) | mapping:read | S8 |
| CRUD | /mappings/{id}/tests | mapping:update | S8 |
| **API composition** ||||
| CRUD | /api-compositions; POST /{id}/validate · /{id}/publish; GET /{id}/versions | api_composition:* | S9 |
| POST | /api-compositions/{id}/execute | api_composition:execute | S9 |
| GET | /api-composition-executions?composition_id= | api_composition:read | S9 |
| * | /gw/{org}/{workspace}/{path} (exposed compositions, separate listener) | api_composition:execute | S9 (P1) |
| **Workflow** ||||
| CRUD | /workflows; POST /{id}/validate · /{id}/publish; GET /{id}/versions | workflow:* | S10 |
| PUT | /workflows/{id}/deployments/{environment_id} (version, enabled) | workflow:deploy | S10 |
| POST | /workflows/{id}/execute (manual; Idempotency-Key) | workflow:execute | S10 |
| POST | /workflows/{id}/nodes/{node_id}/test (single-node test with sample input) | workflow:update | S10 |
| GET | /workflow-executions?workflow_id=&status=; GET /workflow-executions/{id} (+ steps) | workflow:read | S10 |
| POST | /workflow-executions/{id}/cancel · /retry (re-run from failed step) | workflow:execute | S11 |
| GET | /approval-tasks?assigned_to=me; POST /approval-tasks/{id}/decide | (assignee rule) | S11 |
| GET | /node-types (catalog + config schemas) | workflow:read | S10 |
| **Webhook** ||||
| CRUD | /webhooks (inbound endpoints; secret shown once on create/rotate); POST /webhooks/{id}/rotate-secret | webhook:* | S12 |
| POST | /hooks/{public_id} (ingress, root path, unauthenticated + signature) | signature | S12 |
| GET | /webhook-events?endpoint_id= | webhook:read | S12 |
| CRUD | /webhook-subscriptions (outbound) | webhook:* | S12 |
| GET | /webhook-deliveries?subscription_id=&status=; POST /webhook-deliveries/{id}/replay | webhook:read / :replay | S12 |
| GET | /event-types (platform event catalog) | webhook:read | S12 |
