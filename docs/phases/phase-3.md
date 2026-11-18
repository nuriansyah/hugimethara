# Phase 3 — Enterprise Governance, Advanced Orchestration & Extensibility (outline)

**Starts only after Phase 2 acceptance is verified.**

Planned workstreams:
- **Identity:** SAML 2.0, Okta/Auth0 presets, LDAP/AD, SCIM provisioning/deprovisioning, attribute & group sync.
- **Authorization:** `CompositeAuthorizer` with ABAC (CEL policy documents, schema-validated), ReBAC (native tuples or
  OpenFGA adapter), OPA/Cedar/Casbin/HTTP adapters; policy versioning, simulation, testing, approval, staged rollout,
  rollback; field- and row-level obligations enforced by the query service.
- **Workflow:** Temporal evaluation behind `Executor`; templates, sub-workflows, compensation, dependency graphs,
  advanced scheduling, version migration, multi-environment promotion with approvals.
- **Plugins:** manifest, permissions, lifecycle, isolated runtimes per trust level (out-of-process containers; WASM
  for pure functions — to be evaluated).
- **Governance:** environment separation controls, audit export, retention, data classification, secret rotation,
  access reviews, lifecycle automation, compliance reports (SOC 2 / ISO 27001-aligned controls; no certification claims).
- **Observability & cost:** unified ops UI, alerts, incident history, replay; usage metering and budgets.
- **Intelligence & MCP:** NL→SQL / NL→workflow (always validated + permission-checked), anomaly detection; MCP server
  exposing authorized tools through the same Authorizer.

Acceptance criteria: PRD §8.10.
