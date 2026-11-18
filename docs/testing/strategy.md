# Testing Strategy

| Layer | Tooling | Runs | Phase 1 focus |
|---|---|---|---|
| Unit | `go test`, testify; Vitest + Testing Library | every PR | state machine (exhaustive transition table), RBAC evaluation, mapping evaluator, CEL limits, SSRF guard, signature verification, retry/backoff |
| Integration | testcontainers-go (Postgres, Redis, NATS, MinIO) | every PR | repositories + RLS, outbox relay, step claim/lease/reaper, webhook ingress→execution, connector built-ins against containers |
| Contract | OpenAPI validation of handler responses (kin-openapi middleware in tests); generated TS client typecheck; connector operation schemas | every PR | spec ↔ implementation drift |
| E2E | Playwright against compose stack | main + nightly | PRD scenarios: order API composition, payment webhook → reconciliation workflow, finance approval, OIDC (Keycloak) login + group-based access |
| Security | dedicated `test/security` suite + ZAP baseline nightly | every PR / nightly | cross-tenant IDOR on every resource route (table-driven over the route list), privilege escalation via role bindings, secret exposure in responses/logs, SSRF matrix, webhook forgery/replay, CSRF |
| Resilience | chaos test: kill workers mid-step, drop NATS, restart Postgres | nightly | executions complete or fail cleanly; no duplicate side effects beyond idempotency guarantees |
| Load | k6 | pre-release | establish p50/p95/p99 baselines (documented, not promised) |

Rules: a slice is not done until its integration tests and the cross-tenant check for its routes pass.
Coverage is tracked but not gated on a number; state machine and authz packages require 100% branch coverage.
