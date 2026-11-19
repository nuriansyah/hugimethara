# ADR-0012: Data access with sqlx and plain .sql files (replaces sqlc)

**Status:** Accepted (2026-11-19, product owner decision) — supersedes the sqlc part of ADR-0002

## Context
S0–S1 used sqlc to generate Go from SQL. The product owner asked to drop code generation for queries and use
sqlx with ordinary `.sql` files.

## Decision
- `github.com/jmoiron/sqlx` over the pgx `database/sql` driver (`pgx/v5/stdlib`).
- Each domain keeps its statements in one plain `queries.sql` file next to its code, embedded with `go:embed`.
  Statements are named with a marker comment and use positional parameters:

  ```sql
  -- name: GetOrganization
  SELECT id, slug::text AS slug, name FROM organizations WHERE id = $1;
  ```
- `db.MustLoad` parses the file at package init (duplicate/empty names panic, so every test catches them) and
  `q.Get("Name")` returns the statement. Rows scan into structs with `db:"column"` tags.
- Scanning conventions that keep `database/sql` happy: cast `citext` to `text`, `inet` via `host(ip)`,
  arrays via `array_to_json(...)::text`, JSONB into `[]byte`, nullable columns into `sql.Null*` / `uuid.NullUUID`.
- Transactions stay explicit: `db.InTenantTx` / `db.InTx` take `*sqlx.Tx`; RLS binding is unchanged.

## Consequences
+ SQL stays readable and reviewable as SQL; no generation step.
− No compile-time checking of query/struct shapes — covered by integration tests that run every query against
  real PostgreSQL (`go test -tags integration`), which CI runs on every change.
