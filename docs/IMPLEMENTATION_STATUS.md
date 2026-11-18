# Implementation Status

_Last updated: 2026-11-18_

## Current stage: Phase 1 — S1 Org bootstrap, sessions, audit

| Phase | Status | Completion |
|---|---|---|
| Blueprint | Drafted; ADRs still **Proposed** (not formally accepted) | docs complete |
| Phase 1 | In progress | ~12% — S0 and S1 code complete; compose E2E not yet run (see below) |
| Phase 2 | Not started | 0% |
| Phase 3 | Not started | 0% |

## Milestone log

### 2026-11-18 — S1 Org bootstrap, sessions, audit
- **Implemented**
  - Domains: `organization` (create, get, update name + session settings), `identity` (signup, login with
    org choice, logout, session resolution, CSRF, list/revoke own sessions, argon2id, lockout), `audit` (in-tx
    recording with redaction, filtered keyset listing), `principal` (request actor), `authz` (contract +
    deny-by-default `InterimAuthorizer`: owner = all, member = `organization:read`; S3 replaces it).
  - Platform: `ratelimit` (Redis fixed window, fails open), client IP/UA middleware honouring trusted proxies,
    Origin guard for state-changing requests, problem extensions + `Retry-After`.
  - Router: every operation not marked `security: []` in the spec requires a session (deny by default, derived from
    the embedded spec); session middleware enforces the CSRF token on state-changing requests.
  - Web: `/signup`, `/login` (with organization picker), auth gate, shell with org name + user menu + sign out,
    `/settings/organization` (owner-editable, read-only for members), `/settings/sessions`, `/governance/audit`
    (filters, load-older pagination). All with loading / empty / error (request id + retry) states and API field
    errors mapped onto form fields. Toasts via sonner.
- **Database changes:** `00003_org_identity_audit` — organizations, users, org_memberships, user_credentials,
  sessions, audit_events (append-only), RLS on all, `hugi_auth` role + 4 `SECURITY DEFINER` auth functions,
  outbox → organizations FK.
- **API endpoints:** `POST /organizations`, `POST /auth/login`, `POST /auth/logout`, `GET /me`, `GET /me/sessions`,
  `DELETE /me/sessions/{id}`, `GET|PATCH /organizations/current`, `GET /audit-events`.
- **Tests executed (2026-11-18)**
  - Unit (`go test -race ./...`): pass — password hashing/verification and malformed hashes, password policy,
    tokens + CSRF verification, org choice, email validation, interim authorizer decision table and HTTP mapping,
    slug/name/settings validation, audit redaction, public-operation set derived from spec, session requirement for
    protected ops, cross-site Origin rejection.
  - `golangci-lint`: 0 issues. Web lint, typecheck, build: pass (7 routes).
  - Integration suite written (`internal/api/auth_integration_test.go`, `ratelimit_integration_test.go`):
    **not yet run** — Docker Desktop is stuck mid-shutdown after the earlier disk-full event,
    so testcontainers cannot start. Covers: signup → me → CSRF-guarded update → audit trail → logout → login;
    validation + slug/email conflicts with no partial rows left; wrong password / unknown email indistinguishable,
    lockout and its expiry, failure audit; multi-org choice (list only after correct password); member vs owner
    permissions; cross-tenant isolation (cannot revoke another org's session); session revoke and idle expiry;
    Redis rate limits and fail-open. The S0 integration tests also need a re-run (new outbox FK).
- **Bugs caught by tests:** the generated strict middleware passes Go-cased operation names (`Login`), so the
  public-operation allowlist matched nothing and every public endpoint required a session (failed closed). Fixed
  with case-insensitive matching + regression test.
- **Known limitations**
  - Signup reveals whether an email/slug is taken (409). Rate limited; email verification comes with invitations (S4).
  - A user can belong to several orgs only via invitation (S4); S1 tests create the second membership as a fixture.
  - No MFA yet (P1), no password reset yet (needs email delivery: S6 SMTP connector).
  - `audit_events` not yet partitioned.
  - Compose stack still unverified end to end (skipped at your request).

### 2026-11-18 — S0 Foundation
- **Implemented**
  - Go module `github.com/nuriansyah/hugimethara/backend` (Go 1.27.4 via `toolchain`), binaries `hugi-api`,
    `hugi-worker`, `hugi-migrate`.
  - `internal/platform`: typed env config with fail-fast validation (never echoes credential URLs); slog JSON logger
    with secret-key redaction + request/trace id enrichment; OpenTelemetry tracing (OTLP export optional) + Prometheus
    registry; `httpx` (RFC 9457 problems with opaque 500s, cursor pagination, sort allowlist, request-id, recover,
    trace/metrics/access-log middleware labelled by route pattern, CORS allowlist, security headers, body cap);
    `db` (pgx pool, `InTenantTx` binding `app.org_id` per transaction for RLS); `egress` (SSRF-guarded HTTP client:
    dial-time IP check after DNS, allowlist, redirect re-check, no proxy, body limit); `health` (/healthz, /readyz with
    concurrent checks); `bus` (NATS JetStream + stream provisioning `HUGI_WF`, `HUGI_WEBHOOK`, `HUGI_EVENTS`);
    `cache` (Redis); `migrate` (embedded goose, readiness = schema at least the binary's version); `lifecycle`.
  - `internal/event`: transactional outbox `Publish` (captures traceparent + request id) and `Relay`
    (`FOR UPDATE SKIP LOCKED`, JetStream msg-id dedupe, metrics). The worker runs the relay.
  - OpenAPI-first pipeline: `api/openapi/openapi.yaml` → oapi-codegen strict chi server (`internal/api/apigen`) and
    openapi-typescript client (`web/src/lib/api/schema.d.ts`). Endpoints: `/healthz`, `/readyz`,
    `/v1/hugimethara/system/info`.
  - sqlc v1.31.1 and oapi-codegen v2.8.2 pinned as Go `tool` dependencies (Homebrew sqlc was v1.8.0).
  - Web: Next.js 16.4.6, React 19.3, Tailwind v4, shadcn/ui (Radix), TanStack Query, openapi-fetch; app shell with
    permission-ready navigation registry (only shipped pages listed); Overview page showing deployment info and live
    dependency readiness with loading / error (request id + retry) / empty states; runtime `/api` proxy.
  - Docker: distroless non-root backend image, Next standalone web image, compose stack (postgres 18.7, redis 8.8.4,
    nats 2.15.3, migrate, api, worker ×2, web; optional `obs` profile).
  - CI (`.github/workflows/ci.yml`): generated-code drift check, vet, golangci-lint (incl. depguard boundary rule),
    unit + integration tests with `-race`, govulncheck, web lint/typecheck/build/audit, image build + Trivy scan.
  - Makefile: `up`, `down`, `test`, `test-integration`, `lint`, `web-check`, `generate`, `check-generated`.
- **Database changes:** `00001_platform_foundation` (extensions, `hugi_app`/`hugi_system` roles, NOBYPASSRLS,
  `hugi_current_org()`), `00002_outbox` (RLS: tenant policy for `hugi_app`, relay policy for `hugi_system`;
  `hugi_app` has INSERT/SELECT only).
- **API endpoints:** `GET /healthz`, `GET /readyz`, `GET /v1/hugimethara/system/info`.
- **Tests executed (local, 2026-11-18)**
  - `go test -race ./...` — pass (config, logging, httpx, egress, health, api router).
  - `go test -tags integration ./internal/platform/db ./internal/event` — pass against postgres:18.7 + nats:2.15.3
    containers: cross-tenant insert rejected and rows invisible across tenants; unbound tx sees nothing; tenant
    binding does not leak across pooled transactions; runtime roles are not superuser/BYPASSRLS; `hugi_app` cannot
    UPDATE outbox; migrations down-to-0/up round trip and readiness flips; two concurrent relays publish 120
    committed rows exactly once each, rolled-back row never published, headers carry org + request id.
  - `golangci-lint run` — 0 issues. Web `pnpm lint`, `pnpm typecheck`, `pnpm build` — pass.
- **Not verified yet**
  - `docker compose up` end-to-end (images building, /readyz green through the stack, web page against live API).
    Docker Desktop shut down mid-build because the host disk was 99% full (3.2 GiB free).
  - CI workflow has not run on GitHub yet (nothing pushed).
- **Known limitations / decisions**
  - Object storage omitted from compose until S7 (MinIO community images discontinued); replacement TBD.
  - Outbox moved from S1 into S0 (platform infrastructure).
  - `hugi-worker` currently runs only the outbox relay and stream provisioning; execution workloads arrive in S10–S12.
  - No authentication yet (S1). The only exposed endpoints are public metadata/health.

### 2026-11-18 — Blueprint
- **Implemented:** no runtime code. Repository inspected (contained only `LICENSE`, Apache-2.0).
- **Architecture decisions (proposed):** ADR-0001…0011 — modular monolith with `hugi-api`/`hugi-worker` binaries;
  Postgres shared schema + RLS; native Postgres-state workflow engine with JetStream dispatch and outbox; connector
  trust tiers; server-side sessions + OIDC; unified Authorizer with native RBAC; ClickHouse (Phase 2);
  OpenAPI-first contracts; CEL expressions; envelope-encrypted secrets.
- **Files created:** `README.md`, `docs/**` (product, architecture, adr, database, api, security, deployment,
  testing, operations, phases, this file).
- **Database changes:** none applied; Phase 1 schema designed in `docs/database/phase-1-schema.md`.
- **API endpoints:** none implemented; Phase 1 contract plan in `docs/api/phase-1-endpoints.md`.
- **Tests executed:** none (no code).
- **Known limitations / deviations from PRD:** workflow graph stored as a validated JSONB document per version rather
  than `workflow_nodes`/`workflow_edges` tables (rationale in workflow-runtime.md); PRD §9 authz tables
  `policies`/`policy_sets`/`policy_bindings`/`principals` deferred to Phase 3.
- **Remaining:** answer open questions (phase-1.md §4), accept ADRs, start S0.

## Phase 1 slice tracker

| Slice | Status | Evidence |
|---|---|---|
| S0 Foundation | ◐ code complete; compose E2E pending | unit + integration + lint + web build pass (see log) |
| S1 Org bootstrap, sessions, audit | ◐ code complete; integration run pending Docker | unit + lint + web build pass |
| S2 Workspaces & environments | ☐ | |
| S3 Dynamic RBAC | ☐ | |
| S4 Users, groups, invitations, service accounts | ☐ | |
| S5 OIDC federation | ☐ | |
| S6 Secrets & connectors | ☐ | |
| S7 Service registry & API explorer | ☐ | |
| S8 Mapping engine & studio | ☐ | |
| S9 API composition | ☐ | |
| S10 Workflow engine core | ☐ | |
| S11 Workflow engine complete | ☐ | |
| S12 Webhook engine | ☐ | |
| S13 External workflow tools | ☐ | |
| S14 Phase review & hardening | ☐ | |
