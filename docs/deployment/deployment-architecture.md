# Deployment Architecture

## Local / Docker Compose (Phase 1 acceptance target)

`deployments/compose/docker-compose.yml` services:

| Service | Image | Notes |
|---|---|---|
| postgres | postgres (current stable major) | volumes, init creates roles |
| redis | redis / valkey | |
| nats | nats with `-js` | JetStream file store |
| migrate | hugi (same image) `hugi-migrate up && seed` | runs to completion first |
| api | hugi `hugi-api` | :8080 API, :8081 gateway, :9090 metrics |
| worker | hugi `hugi-worker` | `deploy.replicas: 2` to exercise concurrency |
| web | hugi-web | :3000 |
| keycloak (profile `idp`) | keycloak | realm import for OIDC E2E tests |
| mock-services (profile `demo`) | wiremock | order/customer/payment/shipment for composition demos |
| otel-collector, prometheus, grafana (profile `obs`) | | optional |

One Go image (distroless, non-root) contains all binaries; command selects the role.

**Object storage:** not in the S0 stack because no code uses it yet. MinIO stopped publishing community container
images in 2025, so the S3-compatible dev server is chosen when the first slice needs it (S7: OpenAPI spec storage).
Candidates: SeaweedFS, Garage, RustFS. Application code targets the S3 API only.

**Database roles** (ADR-0002): `hugi_owner` (migrations, table owner), `hugi_app` (API, RLS-enforced),
`hugi_system` (worker platform jobs, narrow cross-tenant grants), `hugi_auth` (NOLOGIN; owns the pre-tenant
authentication functions, read-only except lockout counters; created by migration 00003). Compose creates the login roles in
`deployments/compose/postgres-init.sh`; in other environments create them before running `hugi-migrate up`.

| Variable | Used by | Purpose |
|---|---|---|
| `HUGI_ENV` | all | development / staging / production / test |
| `HUGI_DATABASE_URL` | api | `hugi_app` connection |
| `HUGI_SYSTEM_DATABASE_URL` | worker | `hugi_system` connection |
| `HUGI_MIGRATE_DATABASE_URL` | migrate | owner connection |
| `HUGI_REDIS_URL`, `HUGI_NATS_URL` | api, worker | dependencies |
| `HUGI_HTTP_ADDR` / `HUGI_METRICS_ADDR` | api, worker | listeners (default `:8080` / `:9090`) |
| `HUGI_CORS_ALLOWED_ORIGINS` | api | trusted browser origins (CORS + state-changing request Origin check) |
| `HUGI_COOKIE_SECURE` | api | default `true` (`__Host-` cookies); `false` only for plain-HTTP local dev; refused in production |
| `HUGI_TRUSTED_PROXIES` | api | CIDRs whose `X-Forwarded-For` is trusted for client IPs (audit, rate limits) |
| `HUGI_LOG_LEVEL`, `HUGI_LOG_FORMAT` | all | `info`/`json` by default |
| `OTEL_EXPORTER_OTLP_ENDPOINT`, `HUGI_TRACE_SAMPLE_RATIO` | api, worker | trace export (optional) |
| `HUGI_API_URL` | web | where `/api/*` is proxied (read at runtime) |

## Kubernetes (Helm, P1)
Deployments: `api` (HPA on CPU/RPS), `worker` (HPA on JetStream pending messages via KEDA), `web`. Job: `migrate`
(pre-upgrade hook). Postgres/Redis/NATS/MinIO are external dependencies (managed services or operators), not bundled
in production values.

## Configuration
Typed config struct loaded from env (`HUGI_*`), validated at startup (fail fast). Secrets via env/file mounts
(`HUGI_KEK_FILE`). Per-environment values files. No config in the database that is needed before DB connect.

## Environments
`dev` (compose), `staging`, `production` — distinct from *tenant* environments (which are data inside an org).
