# Implementation Status

_Last updated: 2026-11-19_

## Current stage: Phase 1 — S6 connector runtime (implementation in progress)

| Phase | Status | Completion |
|---|---|---|
| Blueprint | Drafted; ADRs still **Proposed** (not formally accepted) | docs complete |
| Phase 1 | In progress | S0–S7 implementation underway; S6 execution runtime and S8–S14 remain |
| Phase 2 | Not started | 0% |
| Phase 3 | Not started | 0% |

## Milestone log

### 2026-11-19 — S6 connector management and S7 service registry
- **S6 implemented:** connector catalog/version and instance APIs, schema-validated configuration, credential
  references, health history, and egress-guarded reachability checks using the selected environment's allowlist.
  The vault and connector management UI are available.
- **S6 still incomplete:** there is no connector invocation API/runtime, shared timeout/retry/rate-limit/circuit-breaker
  wrapper, or working protocol adapter suite for the seeded built-ins. Do not treat catalog entries as runtimes.
- **S7 implemented:** service and environment-binding management, OpenAPI import/diff, endpoint catalog, and guarded
  explorer requests using the selected environment's egress policy.
- **Verification:** the complete backend integration suite passed with `-race` against disposable Postgres/Redis;
  targeted connector egress and service import/explorer tests passed; the web production build, backend binary build,
  Compose configuration validation, and `git diff --check` passed.

### 2026-11-19 — S5 OIDC federation
- **Implemented:** OIDC Authorization Code + PKCE sign-in with Entra ID, Google Workspace, Keycloak and generic
  presets. Per-organization providers with write-only client secrets sealed AES-256-GCM (`HUGI_MASTER_KEY`,
  dev fallback), issuer discovery cached per process for 15 minutes, exact redirect URI. Public provider lookup by
  org slug, SSO start and callback, sealed state cookie (state/nonce/verifier, 10 min) and stable `sso_error`
  codes. Linking by `(provider, subject)`, JIT provisioning or pre-invited members only; verified-email and
  allowed-domain checks; an existing account in another tenant is never auto-linked (`account_exists`).
  Email/name/groups claim mapping; IdP groups sync exactly to mapped groups (mapping omitted `group_id` creates
  an `idp`-sourced group), escalation-checked via `CheckCanHandOut`, manual edits blocked on IdP groups.
  Suspended/disabled members are refused; JIT joins are audited (`user.joined`, outbox event).
- **Database:** `00008_identity_providers` — `identity_providers`, `identity_links`, `group_mappings`, RLS,
  pre-tenant `SECURITY DEFINER` lookups (`hugi_auth_oidc_provider(s)`, `hugi_auth_user_by_email`,
  `hugi_auth_identity_link`, `hugi_auth_membership_status`), `identity_provider` resource type and `org_admin`
  grant. Down migration written.
- **API:** public `GET /auth/oidc/providers`, `GET /auth/oidc/{id}/start`, `GET /auth/oidc/callback`;
  `GET/POST /identity/providers`, `GET/PATCH/DELETE /identity/providers/{id}`,
  `POST /identity/providers/{id}/test`, `GET/POST /identity/providers/{id}/group-mappings`,
  `DELETE /identity/providers/{id}/group-mappings/{mapping_id}`.
- **UI:** Settings → Single sign-on provider manager (CRUD, enable/disable, discovery test, redirect URI,
  claim map, domains, JIT, group mappings) and a "Continue with SSO" flow on the sign-in page with friendly
  `sso_error` messages.
- **Tests:** 4 new integration tests with an in-process mock IdP covering provider security/lifecycle, public
  discovery scoping, JIT login + group sync + suspension + invalid signature, and linking rules (existing member,
  no account, cross-tenant conflict, email-domain allowlist). Full integration suite (83) and unit suite green;
  web lint/typecheck green. Keycloak browser E2E verified login and group-mapped workspace access.
- **Known limitations:** discovery cache is per process; SAML is reserved but not implemented; SSO-only enforcement
  and domain-based provider discovery are not built; IdP group sync applies on sign-in, not as a background job.

### 2026-11-19 — S5 Keycloak E2E fixture and S6 vault foundation
- **S5:** Added an opt-in Compose `sso` profile with an imported Keycloak realm, confidential OIDC client, test
  user, and `engineering` group claim. The callback URL matches the exact public redirect URI. Started the profile,
  verified the imported realm's OIDC discovery document, and completed browser login with group-mapped workspace
  access.
- **S6 vault:** Added `00009_secrets_vault` with RLS-protected organization DEKs wrapped by the configured
  AES-256-GCM master key and secret ciphertext authenticated with `org_id|secret_id` AAD. Added audited,
  permission-gated `GET/POST/PATCH/DELETE /credentials`; plaintext is accepted for create/rotation, never returned,
  and only the final-four-character hint is exposed. `secret:use`-authorized `Resolve` is restricted to backend
  callers. Added Settings → Credentials with write-only create/rotate UX.
- **Tests:** Secretbox additional-authenticated-data binding test; fresh-Postgres integration tests for credential
  create/list/get/rotate/delete, successful authorized resolution, secret absence from API/DB ciphertext,
  tenant isolation, and permission/input rejection. Targeted backend tests, web typecheck/lint, and Compose config
  validation pass.
- **Still pending in S6:** connector invocation/runtime, timeout/retry/rate-limit/circuit-breaker controls, and
  protocol implementations and execution tests for the seeded built-ins. Existing S5 IdP client secrets still use
  the legacy single-key format; migration to the vault is not yet implemented.

### 2026-11-19 — S4 Users, groups, invitations, service accounts
- **Implemented:** invitations by email with an optional org role and per-workspace roles (authorized and
  escalation-checked at invite time, applied on accept), public invitation preview + accept (session for the
  invited address, existing password, or new account; single use, expiring, revocable, rate limited); member
  management (suspend/reactivate ends sessions and access; remove from org; last-owner and self guards); groups
  with members and role bindings (grants flow to members; adding members requires being able to grant the group's
  roles); service accounts with API keys (`Authorization: Bearer hugi_…`, hashed, prefix lookup, expiry,
  revocation, disable) authenticated as `service_account` principals; role bindings, simulation, decision log and
  audit trail understand all three principal kinds. Workspaces created by a service account make it their admin.
- **Database:** `00007_groups_invitations_service_accounts` — new tables, `role_bindings.principal_type` +
  `group_id`/`service_account_id` with a one-principal check, membership views restricted to direct user bindings,
  `authz_decisions.principal_type`, catalog types `group`/`service_account` and `user:invite|update|remove`
  (org_admin template and existing system admin roles extended), `hugi_auth_resolve_api_key`,
  `hugi_auth_invitation_lookup`. Down migration written.
- **API:** `/users/{id}` (GET/PATCH/DELETE), `/invitations` (+ revoke), `/invitation-tokens/{token}` (+ `/accept`,
  public), `/groups` (+ members), `/service-accounts` (+ keys); `/role-bindings` takes `group_id` /
  `service_account_id` and returns `principal`; `/authorization/evaluate` takes `service_account_id`.
- **UI:** People → Members (status, suspend/remove, invite dialog with workspace roles, one-time link, pending
  invitations), Groups (CRUD, members), Service accounts (CRUD, enable/disable, keys with one-time secret);
  `/invite/[token]` accept page; Access assigns roles to members, groups or service accounts.
- **Tests:** 5 new integration tests (invite → join with workspace role, existing-account and signed-in accept,
  single use / revoke / expiry / validation, invite escalation; API key auth incl. roles, CSRF-free writes, bad
  keys, disable/revoke/expiry, cross-tenant, deletion; group-granted access, via-group simulation, group escalation;
  suspend/remove with last-owner guard and re-invite). Full suite (73) green with `-race`; golangci-lint 0 issues;
  web lint, typecheck, build green. Browser-checked against the real API: invite → accept as a new account →
  workspace visible; group membership; key creation with one-time secret; Access table with group assignment.
- **Known limitations:** invitation links are copied by the inviter (no email until the S6 SMTP connector);
  workspace member lists show direct members only (group-derived access shows in Access and simulation); the
  "check access" UI simulates members only (the API also accepts service accounts); no per-plan member quota yet;
  no self-service profile/password change yet.

### 2026-11-19 — S3 Dynamic RBAC
- **Implemented:** roles/permissions as data (catalog of 9 resource types), per-org role templates (owner, admin,
  member, workspace admin, editor, viewer), custom roles, role bindings at org/workspace/resource scope with
  optional expiry, workspace membership as workspace-scope bindings (per-workspace roles), no-escalation rule,
  last-owner protection, version-stamped grant cache (Redis + 30s cap), decision log (all denials + sensitive
  allows), access simulation, `/me/permissions` for UI gating. Interim authorizer and `is_owner` removed.
- **Database:** `00006_rbac` with backfill of existing orgs (verified on the dev database with real data).
- **API:** `/me/permissions`, `/resource-types`, `/roles` (CRUD), `/role-bindings`, `/authorization/evaluate`,
  `/authorization/decisions`; workspace member add takes `role_key`; member lists include roles.
- **UI:** Governance → Roles (permission matrix editor), Access (assignments, assign dialog, check access,
  recent denials); every control gated by permission instead of owner flag; workspace member role picker.
- **Tests:** grant matching/scope/escalation unit tests; 7 RBAC integration tests (templates & effective perms,
  custom role grant/revoke immediacy, escalation attempts, last owner, workspace-admin scoping, simulation &
  decision log, tenant-scoped roles) + cross-instance cache propagation via Redis; whole suite green with `-race`;
  lint 0 issues; web build green; browser-checked Roles and Access pages on migrated dev data.
- **Known limitations:** groups and service-account principals arrive in S4 (bindings are user-only); resource-
  scope bindings are supported by the evaluator but no resource type uses them yet (datasets etc. in Phase 2).

### 2026-11-19 — sqlx migration, UUIDv7, S2, paid features
- **Data access:** sqlc removed; sqlx + plain embedded `queries.sql` per domain (ADR-0012). All PKs `DEFAULT uuidv7()`.
- **S2:** departments, environments (egress allowlist, CIDR-validated), workspaces (archive/restore, membership),
  org member list; members see only their workspaces (404 otherwise); owners manage.
- **Paid features (ADR-0013):** plans Free/Pro/Enterprise, feature flags + quotas, per-org overrides, 402 errors,
  advisory-locked quota checks, audit-log retention by plan, `hugi-admin` CLI for plan changes (audited).
- **Merchant isolation:** integration test proves org A cannot read/modify org B's workspaces, departments,
  environments, users, plan or audit trail, even with identical slugs or guessed ids.
- **Tests:** full unit + integration suite green with `-race` (3 consecutive runs) against local Postgres 18 + Redis
  (`HUGI_TEST_POSTGRES_URL`/`HUGI_TEST_REDIS_URL`), in-process NATS. Found and fixed: down-migration 00003 order,
  email `format` causing 400 instead of field error, test-harness race on cluster-wide roles (the earlier flake).
- **Browser-verified (real hugi-api + web):** signup → overview, workspace create, quota message at limit. Fixed a
  dialog that closed on outside focus after a 402.
- **Not done:** docker compose E2E; CI not yet run on GitHub; workspace switcher deferred to S6; nothing committed.

### 2026-11-18 — S1 Org bootstrap, sessions, audit
- **Implemented**
  - Modules: `organization` (create, get, update name + session settings), `identity` (signup, login with
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
  - Integration suite written (`internal/transport/http/auth_integration_test.go`,
    `internal/platform/ratelimit/ratelimit_integration_test.go`):
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
  - Root Go module `github.com/nuriansyah/hugimethara` (Go 1.27.4 via `toolchain`), binaries `hugi-api`,
    `hugi-worker`, `hugi-migrate`, and `hugi-admin`.
  - `internal/platform`: typed env config with fail-fast validation (never echoes credential URLs); slog JSON logger
    with secret-key redaction + request/trace id enrichment; OpenTelemetry tracing (OTLP export optional) + Prometheus
    registry; `httpx` (RFC 9457 problems with opaque 500s, cursor pagination, sort allowlist, request-id, recover,
    trace/metrics/access-log middleware labelled by route pattern, CORS allowlist, security headers, body cap);
    `database` (pgx pool, `InTenantTx` binding `app.org_id` per transaction for RLS); `egress` (SSRF-guarded HTTP client:
    dial-time IP check after DNS, allowlist, redirect re-check, no proxy, body limit); `health` (/healthz, /readyz with
    concurrent checks); `bus` (NATS JetStream + stream provisioning `HUGI_WF`, `HUGI_WEBHOOK`, `HUGI_EVENTS`);
    `cache` (Redis); `migrate` (embedded goose, readiness = schema at least the binary's version); `shutdown`.
  - `internal/modules/workflow/event`: transactional outbox `Publish` (captures traceparent + request id) and `Relay`
    (`FOR UPDATE SKIP LOCKED`, JetStream msg-id dedupe, metrics). The worker runs the relay.
  - OpenAPI-first pipeline: `api/openapi/openapi.yaml` → oapi-codegen strict chi server
    (`internal/transport/http/apigen`) and openapi-typescript client (`apps/web/src/lib/api/schema.d.ts`).
    Endpoints: `/healthz`, `/readyz`,
    `/v1/hugimethara/system/info`.
  - oapi-codegen v2.8.2 pinned as a Go `tool` dependency; database access uses sqlx and handwritten SQL.
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
  - `go test -tags integration ./internal/platform/database ./internal/modules/workflow/event` — pass against
    postgres:18.7 + nats:2.15.3
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
| S1 Org bootstrap, sessions, audit | ✅ | integration suite green (local Postgres/Redis) |
| S2 Workspaces & environments | ✅ (+ paid features) | integration + browser smoke |
| S3 Dynamic RBAC | ✅ | 8 RBAC integration tests + unit tests; browser check |
| S4 Users, groups, invitations, service accounts | ✅ | 5 integration tests + browser check |
| S5 OIDC federation | ✅ | 4 mock-IdP integration tests; Keycloak browser E2E verified group-based workspace access |
| S6 Secrets & connectors | ◐ vault and connector management implemented; runtime incomplete | full integration suite and targeted egress tests pass; no invocation/runtime or protocol adapters |
| S7 Service registry & API explorer | ✅ | OpenAPI import/diff and guarded explorer integration tests pass |
| S8 Mapping engine & studio | ✅ | Mapping drafts, test cases, publish gates, immutable versions, JSONPath/transform/CEL evaluator, and Mapping Studio |
| S9 API composition | ☐ | |
| S10 Workflow engine core | ☐ | |
| S11 Workflow engine complete | ☐ | |
| S12 Webhook engine | ☐ | |
| S13 External workflow tools | ☐ | |
| S14 Phase review & hardening | ☐ | |
