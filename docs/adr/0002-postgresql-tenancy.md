# ADR-0002: PostgreSQL shared schema, `org_id` + Row-Level Security

**Status:** Proposed

## Context
Must support SMB to enterprise tenants; "never allow one tenant to access another tenant's data".

## Decision
- Shared database, shared schema. Every tenant-owned table has `org_id uuid NOT NULL`.
- Primary keys are UUIDv7 (time-ordered, index-friendly). Child tables use **composite FKs** `(org_id, parent_id)`
  referencing `UNIQUE (org_id, id)` on the parent, making cross-tenant references impossible at the schema level.
- **RLS enabled on every tenant table**: `USING (org_id = current_setting('app.org_id')::uuid)`. The app connects as
  `hugi_app` (no `BYPASSRLS`, not table owner); every tenant-scoped transaction runs `SET LOCAL app.org_id`.
  Migrations run as `hugi_owner`. Platform-level jobs that legitimately span tenants (reaper, outbox relay) use a
  narrowly-scoped `hugi_system` role on specific tables only.
- Repositories still filter by `org_id` explicitly — RLS is defense-in-depth, not the primary mechanism.
- pgx/v5 + sqlc; goose migrations (SQL, embedded in `hugi-migrate`).

## Alternatives
- Schema-per-tenant: migration fan-out and connection/catalog bloat at thousands of tenants. Rejected for default;
  **database-per-tenant** remains an enterprise deployment option because nothing in the code assumes co-location.
- No RLS: one forgotten `WHERE` is a cross-tenant breach. Rejected.

## Consequences
+ Schema-enforced isolation, cheap tenants. − `SET LOCAL` per tx and RLS planning overhead (measured in load tests);
− cross-tenant admin tooling must use the system role deliberately.
