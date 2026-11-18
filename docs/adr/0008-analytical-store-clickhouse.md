# ADR-0008: ClickHouse as analytical store (Phase 2)

**Status:** Proposed (decision confirmed at Phase 2 kickoff)

## Decision
ClickHouse for raw/transformed/curated analytical data and high-volume logs (audit, authz decisions, execution
analytics). Postgres stays transactional. Access only through a Hugimethara query service that enforces authz,
row/column policies and resource limits. Tenancy: database per org.

## Alternatives
Postgres + Timescale/Citus (simpler ops, weaker for wide scans at volume), DuckDB/MotherDuck (great single-node,
weaker multi-tenant server story), BigQuery/Snowflake (customer-cloud lock-in; can be added as external sources).

## Consequences
Not part of Phase 1 compose file; Phase 1 must not depend on it.
