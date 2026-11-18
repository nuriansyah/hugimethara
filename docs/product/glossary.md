# Glossary

| Term | Meaning |
|---|---|
| Organization | Tenant. Root of all data isolation. |
| Workspace | Team/department space inside an org; scope for resources and memberships. |
| Department | Org-level label grouping workspaces and users (used for ABAC in Phase 3). |
| Environment | Tenant-level deployment stage (development/staging/production) that scopes connector instances, secrets, deployments. |
| Principal | Authenticated actor: user or service account. |
| Resource type | Registered kind of thing permissions apply to (`workflow`, `connector`, …). |
| Permission | `<resource_type>:<action>` string. |
| Role binding | Grants a role to a principal/group at a scope (org, workspace, or single resource). |
| Connector definition / version | Catalog entry describing how to talk to a system, versioned. |
| Connector instance | A configured connection (config + credentials) in a workspace/environment. |
| Service | Internal system registered in the Service Registry, with endpoints (often from OpenAPI). |
| Mapping | Versioned transformation from an input JSON to an output JSON. |
| API composition | Versioned DAG of service calls producing one unified response. |
| Workflow | Versioned graph of trigger + nodes; executed asynchronously. |
| Execution / Step | One run of a workflow version / one node attempt within it. |
| Deployment | Binding of a workflow version to an environment (enabled triggers). |
| Outbox | Table used to publish events atomically with DB changes. |
| CEL | Common Expression Language — sandboxed expressions used in conditions, mappings, policies. |
