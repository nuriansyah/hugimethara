// Shared helpers for the Phase 1 k6 baselines (S14).
import http from "k6/http";
import { check, fail } from "k6";

export const BASE = __ENV.HUGI_BASE_URL || "http://127.0.0.1:58080";
export const API = `${BASE}/v1/hugimethara`;

const JSON_HEADERS = { "Content-Type": "application/json" };

/** Reads the CSRF cookie the API set on this VU's jar. */
export function csrf() {
  const jar = http.cookieJar();
  const cookies = jar.cookiesForURL(BASE);
  const v = cookies["__Host-hugi_csrf"] || cookies["hugi_csrf"];
  return v ? v[0] : "";
}

export function post(path, body, tags = {}) {
  return http.post(`${API}${path}`, JSON.stringify(body), {
    headers: { ...JSON_HEADERS, "X-CSRF-Token": csrf() },
    tags,
  });
}

export function put(path, body, tags = {}) {
  return http.put(`${API}${path}`, JSON.stringify(body), {
    headers: { ...JSON_HEADERS, "X-CSRF-Token": csrf() },
    tags,
  });
}

export function get(path, tags = {}) {
  return http.get(`${API}${path}`, { tags });
}

export function must(res, status, what) {
  if (!check(res, { [`${what} → ${status}`]: (r) => r.status === status })) {
    fail(`${what}: expected ${status}, got ${res.status}: ${res.body}`);
  }
  return res.json();
}

const PASSWORD = "correct horse battery staple";

/** Signs the owner in on this VU (setup()'s session is not shared with VUs). */
export function login(org) {
  must(http.post(`${API}/auth/login`, JSON.stringify({ email: org.email, password: PASSWORD, organization_slug: org.slug }),
    { headers: JSON_HEADERS }), 200, "login");
}

/** Creates a fresh organization (owner session in this VU's jar) with a workspace. */
export function bootstrapOrg(prefix) {
  const slug = `${prefix}-${Date.now().toString(36)}-${Math.floor(Math.random() * 1e6).toString(36)}`;
  must(
    http.post(`${API}/organizations`, JSON.stringify({
      organization_name: `Load ${slug}`, organization_slug: slug, display_name: "Load owner",
      email: `owner@${slug}.test`, password: PASSWORD,
    }), { headers: JSON_HEADERS }),
    201, "signup",
  );
  const ws = must(post("/workspaces", { slug: "load", name: "Load" }), 201, "create workspace");
  const envs = must(get("/environments"), 200, "list environments").data;
  const env = envs.find((e) => e.slug === "development") || envs[0];
  return { slug, email: `owner@${slug}.test`, workspaceId: ws.id, environmentId: env.id };
}
