-- +goose Up
-- Foundation: extensions, runtime roles, RLS helper.
-- Runtime roles are NOLOGIN group roles created here if absent; deployments
-- create LOGIN users that are members of them (see deploy/compose/postgres-init.sh).

CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- +goose StatementBegin
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'hugi_app') THEN
    CREATE ROLE hugi_app NOLOGIN;      -- request path; subject to RLS
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'hugi_system') THEN
    CREATE ROLE hugi_system NOLOGIN;   -- cross-tenant platform jobs (outbox relay, reaper); narrow grants only
  END IF;
END
$$;
-- +goose StatementEnd

-- Runtime roles must never bypass RLS.
ALTER ROLE hugi_app NOBYPASSRLS;
ALTER ROLE hugi_system NOBYPASSRLS;

GRANT USAGE ON SCHEMA public TO hugi_app, hugi_system;
-- Readiness checks compare the applied migration version with the binary's.
GRANT SELECT ON goose_db_version TO hugi_app, hugi_system;

-- hugi_current_org() returns the tenant bound by db.InTenantTx, or NULL when
-- the transaction is not tenant-bound (which makes every RLS predicate false).
-- +goose StatementBegin
CREATE FUNCTION hugi_current_org() RETURNS uuid
LANGUAGE sql STABLE PARALLEL SAFE AS $$
  SELECT NULLIF(current_setting('app.org_id', true), '')::uuid
$$;
-- +goose StatementEnd

-- updated_at maintenance for tables that opt in.
-- +goose StatementBegin
CREATE FUNCTION hugi_touch_updated_at() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  NEW.updated_at := now();
  RETURN NEW;
END
$$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION IF EXISTS hugi_touch_updated_at();
DROP FUNCTION IF EXISTS hugi_current_org();
REVOKE SELECT ON goose_db_version FROM hugi_app, hugi_system;
REVOKE USAGE ON SCHEMA public FROM hugi_app, hugi_system;
