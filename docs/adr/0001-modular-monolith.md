# ADR-0001: Modular monolith with separately scaled binaries

**Status:** Proposed

## Context
~20 domains, one team, strict tenancy and authorization that must be applied uniformly, and a requirement that
execution workers scale independently of the control plane.

## Decision
One Go codebase organized into domain packages with enforced dependency rules, built into `hugi-api`,
`hugi-worker` and `hugi-migrate` binaries. Domains talk through in-process service interfaces; async work goes through
the outbox + JetStream. A domain may be extracted into a service later only when a measured need appears
(independent release cadence, isolation, or scaling profile the binary split cannot meet).

## Alternatives
- **Microservices from day one** — distributed transactions for audit/outbox, N× auth plumbing, network hops in the
  authz path, heavy ops burden before product-market fit. Rejected.
- **Single binary** — cannot scale workers independently (PRD §4.2). Rejected.

## Consequences
+ Single transaction can cover state change + audit + outbox. + One authz/tenancy implementation.
− Discipline needed to keep boundaries; mitigated with `go-arch-lint` in CI and per-domain table ownership.
