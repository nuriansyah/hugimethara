# ADR-0009: Next.js client app and OpenAPI-first contracts

**Status:** Proposed

## Decision
- OpenAPI 3.0.3 in `api/openapi/` is the source of truth. Go handler interfaces generated with `oapi-codegen`
  (chi, strict server); TS types/client with `openapi-typescript`/`openapi-fetch`. CI fails on drift.
- Next.js App Router, feature-oriented folders, TanStack Query for server state; Next.js holds no business logic and
  no secrets. See [frontend-architecture.md](../architecture/frontend-architecture.md).

## Alternatives
Code-first spec generation (swaggo annotations drift and under-specify), tRPC/GraphQL (PRD mandates REST, and external
consumers need OpenAPI anyway), Next.js as BFF (duplicates authz surface).
