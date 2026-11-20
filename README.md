# Hugimethara

**Connect Systems. Orchestrate Operations. Unlock Intelligence.**

Hugimethara is a unified business operations & intelligence platform: integrations, API composition,
native workflow automation, webhooks, data platform and BI, under one multi-tenant, policy-governed control plane.

> Status: **Phase 1, slice S0 (foundation).** See [docs/IMPLEMENTATION_STATUS.md](docs/IMPLEMENTATION_STATUS.md).

## Quick start

Requires Docker, Go 1.27+ (auto-downloaded by `go` via `toolchain`), Node 24 + pnpm.

```bash
make up                  # postgres, redis, nats, migrate, api (:8080), worker x2, web (:3000)
open http://localhost:3000
make test                # backend unit tests
make test-integration    # backend integration tests (needs HUGI_TEST_POSTGRES_URL + HUGI_TEST_REDIS_URL; see Makefile)
make lint web-check      # golangci-lint, eslint, typecheck, next build
make generate            # regenerate oapi-codegen server + TS client after OpenAPI changes
```

## Where to start

| Read | For |
|---|---|
| [docs/architecture/system-overview.md](docs/architecture/system-overview.md) | Architecture, repo layout, components |
| [docs/architecture/domain-boundaries.md](docs/architecture/domain-boundaries.md) | Domains and their dependency rules |
| [docs/adr/](docs/adr/) | Why each major technology/architecture choice was made |
| [docs/database/phase-1-schema.md](docs/database/phase-1-schema.md) | Phase 1 data model + ERD |
| [docs/api/](docs/api/) | API conventions and Phase 1 endpoint plan |
| [docs/phases/phase-1.md](docs/phases/phase-1.md) | Phase 1 vertical slices + prioritized backlog |
| [docs/product/roadmap.md](docs/product/roadmap.md) | Three-phase roadmap |

## Repository layout

```
cmd/                    Go binaries (hugi-api, hugi-worker, hugi-migrate, hugi-admin)
internal/modules/       Domain modules
internal/platform/      Cross-cutting services
internal/transport/http/ HTTP routes and generated OpenAPI server
internal/bootstrap/     API and worker composition roots
migrations/postgres/    Embedded PostgreSQL migrations
apps/web/               Next.js (App Router, TypeScript) frontend
deployments/            Docker Compose and deployment assets
api/openapi/            HTTP contract source of truth
docs/                   Product, architecture, ADRs, database, API, security, ops docs
```

## License

Apache License 2.0 — see [LICENSE](LICENSE).
