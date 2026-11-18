# Personas → Phase 1 capabilities and default roles

| Persona | Primary Phase 1 use | Default mode | Suggested role template |
|---|---|---|---|
| Organization Administrator | Workspaces, users, IdP, roles, audit | Simple | `org_admin` |
| Marketing | Lead-sync workflows from webhook forms to CRM, Slack notifications | Simple | `editor` in Marketing workspace |
| Sales | Lead follow-up workflows, CRM sync | Simple | `editor` / `operator` |
| Finance | Payment webhook → reconciliation workflow, approval workflows | Simple | `operator` + approver group |
| Operations | Approval workflows, notifications, execution monitoring | Simple | `operator` |
| Engineering | Service registry, OpenAPI import, compositions, custom connectors, debugging executions | Advanced | `workspace_admin` in Engineering |
| Data team | (Phase 2) data sources, datasets; Phase 1: DB query nodes | Advanced | `editor` |
| Executives | (Phase 2) dashboards; Phase 1: overview page | Simple | `viewer` |

Role templates (`org_owner`, `org_admin`, `workspace_admin`, `editor`, `operator`, `viewer`) are starting points copied
into each org and freely editable (except `org_owner`).
