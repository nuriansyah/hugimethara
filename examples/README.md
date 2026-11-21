# Examples

| Path | What it shows |
|---|---|
| [workflows/order-approval.json](workflows/order-approval.json) | S10 workflow definition: `logic.if` routing, `logic.parallel` fan-out to an HTTP call and a transform, joined by `logic.merge`; the untaken branch is dead-path skipped. |

Create a workflow with the example as its draft (`POST /v1/hugimethara/workflows` with `"draft": <file>`),
then publish it, deploy the version to an environment (`PUT /workflows/{id}/deployments/{environment_id}`)
and run it (`POST /workflows/{id}/execute`). The HTTP node only reaches hosts allowed by the environment's
egress policy; private and loopback addresses are denied unless added to the environment allowlist.
