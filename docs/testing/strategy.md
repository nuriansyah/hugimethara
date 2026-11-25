# Testing Strategy

As built for Phase 1 (S0–S14). A slice is not done until its integration tests and the route-table security suite
pass for its routes.

| Layer | Tooling | Runs | Phase 1 coverage |
|---|---|---|---|
| Unit | `go test -race`, testify | every PR | workflow and step state machines, publish validation, cron (DST and half-hour zones), RBAC evaluation/escalation, mapping evaluator limits, CEL, SSRF guard, webhook verifiers and signing, retry schedules, connector runtime and n8n/Make/Zapier presets |
| Integration | real PostgreSQL + Redis from `HUGI_TEST_POSTGRES_URL` / `HUGI_TEST_REDIS_URL` (fresh database per test, RLS roles), in-process NATS JetStream | every PR | repositories + RLS, outbox relay, step claim/lease/reaper, and the HTTP end-to-end scenarios: signup → login → session revoke, invitations and API keys, RBAC, OIDC against an in-process IdP, connectors and egress, OpenAPI import and explorer, mapping publish gates, the order-overview composition, conditional/parallel workflows, crash recovery, approvals, schedules/events, webhook ingress → workflow and outbound delivery/retry/DLQ/replay, overview KPIs |
| Security | route-table-driven `TestSecuritySuite` (`internal/transport/http/security_integration_test.go`) plus slice-specific tests | every PR | every operation in the route table: anonymous → 401, mutation without CSRF → 403, cross-tenant object references refused without leaking ids or secrets, collections never list another tenant's rows, security headers on every response, no secret in any response; plus webhook forgery/replay, egress/SSRF matrices and escalation checks |
| Contract | embedded OpenAPI contract served at `/v1/hugimethara/openapi.{json,yaml}`; generated TS client; `make check-generated` / CI drift check | every PR | spec ↔ implementation drift, generated client typecheck, the web API reference page |
| Resilience | integration tests that kill a worker mid-step and let a second worker finish, webhook retries/dead letters, execution deadlines and reapers | every PR | no duplicate side effects beyond the idempotency guarantees; executions complete or fail cleanly |
| Load | k6 scripts in [tests/k6](../../tests/k6) with failing thresholds | pre-release | API reads, workflow execution end to end, webhook ingress; recorded numbers in [performance-baseline.md](performance-baseline.md) |

Rules: coverage is tracked but not gated on a number; state machine and authz packages require 100 % branch coverage.

**Not automated yet:** a Playwright browser suite (Phase 1 slices were browser-checked manually against a live
stack and those checks are recorded in [IMPLEMENTATION_STATUS.md](../IMPLEMENTATION_STATUS.md)); a ZAP baseline
scan; a scheduled chaos run that drops NATS or Postgres. These are Phase 2 hardening candidates, not Phase 1
acceptance criteria.
