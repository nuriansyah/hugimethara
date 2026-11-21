// Baseline for webhook ingress: Standard Webhooks signature verification,
// dedupe and the transactional insert + outbox event, at a constant rate.
import crypto from "k6/crypto";
import encoding from "k6/encoding";
import http from "k6/http";
import { check } from "k6";
import { BASE, bootstrapOrg, must, post } from "./lib.js";

export const options = {
  noCookiesReset: true, // keep each VU's session across iterations
  scenarios: {
    ingress: { executor: "constant-arrival-rate", rate: Number(__ENV.RATE || 50), timeUnit: "1s",
      duration: __ENV.DURATION || "30s", preAllocatedVUs: 20, maxVUs: 100 },
  },
  thresholds: {
    http_req_failed: ["rate<0.01"],
    "http_req_duration{name:ingress}": ["p(95)<100", "p(99)<250"],
  },
};

export function setup() {
  const org = bootstrapOrg("k6-hook");
  const ep = must(post("/webhooks", { workspace_id: org.workspaceId, environment_id: org.environmentId,
    name: "Load hook", verifier: "standard_webhooks", rate_limit_per_minute: 10000 }), 201, "create endpoint");
  return { publicId: ep.public_id, secret: ep.secret };
}

export default function (s) {
  const id = `msg_${__VU}_${__ITER}_${Date.now()}`;
  const ts = Math.floor(Date.now() / 1000).toString();
  const body = JSON.stringify({ type: "load.test", data: { vu: __VU, iter: __ITER } });
  const key = encoding.b64decode(s.secret.slice("whsec_".length), "std");
  const sig = crypto.hmac("sha256", key, `${id}.${ts}.${body}`, "base64");
  const res = http.post(`${BASE}/hooks/${s.publicId}`, body, {
    headers: { "Content-Type": "application/json", "webhook-id": id, "webhook-timestamp": ts, "webhook-signature": `v1,${sig}` },
    tags: { name: "ingress" },
  });
  check(res, { "accepted": (r) => r.status === 202 });
}
