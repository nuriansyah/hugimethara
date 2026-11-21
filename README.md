# Hugimethara

**Connect Systems. Orchestrate Operations. Unlock Intelligence.**

Hugimethara is a unified business operations & intelligence platform: integrations, API composition,
native workflow automation, webhooks, data platform and BI, under one multi-tenant, policy-governed control plane.

> Status: **Phase 1, slice S0 (foundation).** See [docs/IMPLEMENTATION_STATUS.md](docs/IMPLEMENTATION_STATUS.md).

## Quick start

Requires Docker, Go 1.26+, Node 24 + pnpm.

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
| [docs/adr/](docs/adr/) | Why each major technology/architecture choice was made (layout: [ADR-0014](docs/adr/0014-module-layout.md)) |
| [docs/database/phase-1-schema.md](docs/database/phase-1-schema.md) | Phase 1 data model + ERD |
| [docs/api/](docs/api/) | API conventions and Phase 1 endpoint plan |
| [docs/phases/phase-1.md](docs/phases/phase-1.md) | Phase 1 vertical slices + prioritized backlog |
| [docs/product/roadmap.md](docs/product/roadmap.md) | Three-phase roadmap |

## Repository layout

```
cmd/                         Go binaries: api, worker, migrate (incl. `hugi-migrate admin`)
internal/modules/<module>/   Bounded contexts (ADR-0014):
  model/ model/dto/            entities, invariants, use-case inputs (no I/O)
  repository/{commands,queries}/  rows + write-side / read-side SQL (CQRS)
  services/                    service.go = ports (interfaces), <module>_service.go = use cases
internal/transport/http/     routes.go, middleware/, handlers/<module>/ (OpenAPI adapters), dto/
internal/platform/           Domain-free primitives: config, database, logger, errors, clock, id, crypto, shutdown, …
internal/infrastructure/     External adapters: persistence/ (outbox, cache), messaging/, security/, observability/
internal/bootstrap/          api.go, worker.go, dependencies.go — composition roots
migrations/postgres/         Embedded PostgreSQL migrations
web/                         Next.js (App Router, TypeScript) app
packages/                    pnpm workspace: ui, api-client, shared-types, config
api/openapi/                 HTTP contract source of truth
deployments/ docs/ scripts/ examples/
```

## License

Apache License 2.0 — see [LICENSE](LICENSE).
