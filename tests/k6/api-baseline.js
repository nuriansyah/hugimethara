// Baseline for the read paths every page uses: session check, workspace list,
// workflow list, connector catalog and the overview KPIs.
//   k6 run tests/k6/api-baseline.js            (HUGI_BASE_URL defaults to the local stack)
import { group } from "k6";
import { bootstrapOrg, get, login, must } from "./lib.js";

export const options = {
  noCookiesReset: true, // keep each VU's session across iterations
  scenarios: {
    reads: { executor: "constant-vus", vus: Number(__ENV.VUS || 8), duration: __ENV.DURATION || "30s" },
  },
  thresholds: {
    http_req_failed: ["rate<0.01"],
    "http_req_duration{kind:read}": ["p(95)<150", "p(99)<400"],
    "http_req_duration{name:overview}": ["p(95)<250"],
  },
};

export function setup() {
  return bootstrapOrg("k6-read");
}

let signedIn = false;

export default function (org) {
  if (!signedIn) {
    login(org); // one session per VU, reused across iterations
    signedIn = true;
  }
  group("reads", () => {
    must(get("/me", { kind: "read", name: "me" }), 200, "me");
    must(get("/workspaces", { kind: "read", name: "workspaces" }), 200, "workspaces");
    must(get(`/workflows?workspace_id=${org.workspaceId}`, { kind: "read", name: "workflows" }), 200, "workflows");
    must(get("/connector-definitions", { kind: "read", name: "connector_definitions" }), 200, "catalog");
    must(get(`/overview?workspace_id=${org.workspaceId}&window=7d`, { kind: "read", name: "overview" }), 200, "overview");
  });
}
