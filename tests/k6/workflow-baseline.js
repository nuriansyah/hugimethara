// Baseline for the workflow engine: manual executions of a conditional +
// parallel workflow (if → parallel → transform ×2 → merge), end to end through
// the outbox relay, JetStream and hugi-worker. Measures accept latency and the
// time until the execution has finished. Each VU polls its execution, so one
// VU sustains ~0.5 executions/s; VUs stay below the per-account login limit.
import { check, sleep } from "k6";
import { Trend, Counter } from "k6/metrics";
import { bootstrapOrg, get, login, must, post, put } from "./lib.js";

const completion = new Trend("workflow_completion_ms", true);
const unfinished = new Counter("workflow_unfinished");

export const options = {
  noCookiesReset: true, // keep each VU's session across iterations
  scenarios: {
    executions: { executor: "constant-arrival-rate", rate: Number(__ENV.RATE || 4), timeUnit: "1s",
      duration: __ENV.DURATION || "30s", preAllocatedVUs: 8, maxVUs: 9 },
  },
  thresholds: {
    http_req_failed: ["rate<0.01"],
    "http_req_duration{name:execute}": ["p(95)<200"],
    // Dominated by the outbox relay's 500 ms idle poll: this graph has 4 sequential hops.
    workflow_completion_ms: ["p(95)<3000"],
    workflow_unfinished: ["count==0"],
  },
};

const draft = {
  schema_version: 1,
  trigger: { type: "trigger.manual" },
  nodes: [
    { id: "check", type: "logic.if", config: { condition: "input.amount > 100.0" } },
    { id: "fan", type: "logic.parallel" },
    { id: "a", type: "action.transform", config: { rules: [{ target: "approved", source: "$.amount" }] } },
    { id: "b", type: "action.transform", config: { rules: [{ target: "audited", source: "$.amount" }] } },
    { id: "join", type: "logic.merge" },
    { id: "small", type: "action.transform", config: { rules: [{ target: "auto", source: "$.amount" }] } },
  ],
  edges: [
    { from: "trigger", to: "check" },
    { from: "check", to: "fan", port: "true" },
    { from: "check", to: "small", port: "false" },
    { from: "fan", to: "a" }, { from: "fan", to: "b" },
    { from: "a", to: "join" }, { from: "b", to: "join" },
  ],
};

export function setup() {
  const org = bootstrapOrg("k6-wf");
  const wf = must(post("/workflows", { workspace_id: org.workspaceId, name: "Load flow", slug: "load-flow", draft }), 201, "create workflow");
  const version = must(post(`/workflows/${wf.id}/publish`, {}), 201, "publish");
  must(put(`/workflows/${wf.id}/deployments/${org.environmentId}`, { version_id: version.id, enabled: true }), 200, "deploy");
  return { ...org, workflowId: wf.id };
}

let signedIn = false;

export default function (s) {
  if (!signedIn) {
    login(s);
    signedIn = true;
  }
  const started = Date.now();
  const amount = Math.random() < 0.7 ? 250 : 5;
  const exec = must(post(`/workflows/${s.workflowId}/execute`, { environment_id: s.environmentId, input: { amount } },
    { name: "execute" }), 202, "execute");
  for (let i = 0; i < 50; i++) {
    sleep(0.1);
    const r = get(`/workflow-executions/${exec.id}`, { name: "poll" });
    const status = r.status === 200 ? r.json().status : "";
    if (["succeeded", "failed", "cancelled", "timed_out"].includes(status)) {
      check(status, { "execution succeeded": (st) => st === "succeeded" });
      completion.add(Date.now() - started);
      return;
    }
  }
  unfinished.add(1);
}
