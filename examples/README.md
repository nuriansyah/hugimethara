# Examples

| Path | What it shows |
|---|---|
| [workflows/order-approval.json](workflows/order-approval.json) | S10 workflow definition: `logic.if` routing, `logic.parallel` fan-out to an HTTP call and a transform, joined by `logic.merge`; the untaken branch is dead-path skipped. |
| [workflows/finance-approval.json](workflows/finance-approval.json) | S11 finance approval: `logic.switch` sends payments of 1000+ to a `human.approval` for a finance group (12 h timeout); approved → pay over HTTP, rejected → record, expired → escalate; small payments are auto-approved. Replace the group id with one of yours. |

Create a workflow with the example as its draft (`POST /v1/hugimethara/workflows` with `"draft": <file>`),
then publish it, deploy the version to an environment (`PUT /workflows/{id}/deployments/{environment_id}`)
and run it (`POST /workflows/{id}/execute`). The HTTP node only reaches hosts allowed by the environment's
egress policy; private and loopback addresses are denied unless added to the environment allowlist.
