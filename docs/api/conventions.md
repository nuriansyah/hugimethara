# API Conventions

Base path: `/v1/hugimethara`. Spec: `api/openapi/` (OpenAPI 3.0.3), served at `/v1/hugimethara/openapi.json`,
rendered in the Developer Platform UI.

## Tenancy in URLs
The organization comes from the authenticated principal, not the URL — a user operating in org A cannot address
org B by changing a path segment. Workspace-scoped collections take `workspace_id` as a query param on list and a body
field on create (as in the PRD's flat paths, e.g. `POST /workflows`). A resource fetched by id outside the
principal's org returns **404** (never 403 — no existence oracle).

## Authentication
- Browser: session cookie + `X-CSRF-Token` header on unsafe methods.
- Machines: `Authorization: Bearer hgm_…` (service-account API key).
- `X-Hugi-Org` header only for users belonging to multiple orgs (session is bound to one active org; switching
  creates a new session-org binding).

## Errors (RFC 9457 Problem Details)
```json
{
  "type": "https://docs.hugimethara.dev/errors/validation",
  "title": "Validation failed",
  "status": 422,
  "code": "validation_failed",
  "detail": "2 fields are invalid.",
  "request_id": "01HZX…",
  "errors": [ { "field": "config.base_url", "code": "invalid_url", "message": "Must be an https URL." } ]
}
```
Codes are stable identifiers documented per endpoint. 5xx never leak internals; details go to logs under `request_id`.

## Pagination, filtering, sorting
- Cursor pagination: `?limit=50&cursor=…` → `{ "data": [...], "next_cursor": "…" }`. Max limit 200.
- Filters as explicit params (`?status=failed&workflow_id=…`); free text `?q=`.
- `?sort=-created_at,name` against a per-endpoint allowlist.

## Idempotency
`Idempotency-Key` header accepted on all `POST` that create or execute; stored 24h per (org, principal, key, route)
with request hash; replay returns the original response; same key + different body → 409.

## Correlation
`X-Request-Id` accepted (validated) or generated; echoed in responses; propagated as trace attribute and to
outbound calls (`traceparent` + `X-Request-Id`).

## Rate limiting
Redis token buckets per principal and per org (and per webhook endpoint). `429` with `Retry-After` and
`RateLimit-*` headers.

## Versioning
Additive changes within `v1`. Breaking changes → `v2` path. Deprecations announced via `Deprecation`/`Sunset` headers.

## Async operations
Execute endpoints return `202` + resource (`{ "execution_id": …, "status": "pending" }`); clients poll the execution
resource. (SSE stream for execution progress is P1.)
