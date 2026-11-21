# ADR-0014: Layered module layout and repository CQRS split

Status: Accepted (2026-11-21)

## Context

Each domain module was a single flat Go package mixing entities, SQL, use cases and HTTP adapters
(`organization/http.go` next to `organization/store.go`). As modules grew (identity ≈ 4k lines) this made
dependencies implicit, let HTTP concerns leak into business code, and gave reviewers no structural signal about
which code may perform I/O. The web app likewise kept its design system and API client inside `apps/web`.

## Decision

Every bounded context under `internal/modules/<module>` uses the same layers. Dependencies only point down:

```
transport/http/handlers/<module>   HTTP adapter (decode wire DTO → call port → encode)        L4
modules/<module>/services          use cases; service.go = ports (interfaces), <module>_service.go = impl   L3
modules/<module>/repository        row shapes + mapping to the model                         L2
  ├── commands/                    write-side SQL (INSERT/UPDATE/DELETE, SELECT … FOR UPDATE)
  └── queries/                     read-side SQL (side-effect free)
modules/<module>/model/dto         use-case inputs (commands), transport-agnostic             L1
modules/<module>/model             entities, value objects, invariants, validation — no I/O   L0
```

- **SRP / SoC** — one reason to change per package: the model changes with business rules, repository with the
  schema, services with use cases, handlers with the wire contract.
- **DIP / ISP** — handlers and other modules depend on `services.Service` (or one narrow `…UseCases`
  interface embedded in it), never on the concrete struct. `var _ Service = (*XService)(nil)` keeps the
  contract compiler-checked.
- **CQRS at the statement level** — reads and writes live in separate registries, so a reviewer can see from the
  import alone whether a code path mutates state; read paths can later move to a replica without touching writes.
- Sub-contexts that are independent bounded contexts keep the same layout one level down
  (`integration/connector`, `governance/audit`, …).
- `internal/platform` holds domain-free primitives (config, database, logger, errors (RFC 9457), clock, id
  (UUIDv7, RFC 9562), crypto, shutdown, principal, pagination). `internal/infrastructure` holds adapters to
  external systems: `persistence` (outbox, cache, migrations), `messaging` (NATS JetStream), `security`
  (egress policy, rate limiting), `observability` (telemetry, health).
- `cmd/` ships exactly three binaries: `api`, `worker`, `migrate` (operator commands are `hugi-migrate admin …`).
- Web: `web/` is the Next.js app; reusable code is in pnpm workspace packages — `packages/ui` (design system),
  `packages/api-client` (typed fetch client), `packages/shared-types` (types generated from
  `api/openapi/openapi.yaml`), `packages/config` (shared tsconfig).

The boundaries are enforced in CI by `depguard` rules in `.golangci.yml`.

## Alternatives

- **Keep flat packages, enforce with lint only** — cheap, but provides no structure for new contributors and
  cannot express "model has no I/O".
- **Hexagonal ports/adapters per module (`domain/`, `app/`, `adapters/`)** — equivalent intent; rejected only to
  match the team's agreed vocabulary (`model`, `repository`, `services`).
- **Repository interfaces with one method per query** — the strongest isolation, but a large rewrite of
  ~230 call sites for little immediate benefit. The statement registries are the seam where typed repository
  methods can be introduced incrementally (new modules such as `workflow` start that way).

## Consequences

- Cross-layer identifiers had to be exported (e.g. row types, small validation helpers). Exported fields that
  were previously unexported carry `json:"-"` so serialization is unchanged.
- Imports are longer and aliased by convention: `<short>model`, `<short>dto`, `<short>repo`, `<short>svc`,
  `<short>handler` when used outside the owning module.
- New modules (`workflow`, `data_platform`, `analytics`, `dashboard`) start in this layout from day one.
