# Phase 2 — Data Platform, Analytics & BI (outline)

**Starts only after Phase 1 acceptance is verified.** Detailed slice plan will be written at Phase 2 kickoff, informed by
Phase 1 learnings.

Planned slices: ClickHouse foundation + query service (authz, limits, per-org databases) → data sources & schema
discovery (Postgres, MySQL, SQL Server, Oracle, REST, GraphQL, files, S3, Google Sheets) → ingestion (full,
incremental with watermarks, scheduled, file; checkpoints, schema drift detection) → SQL transformations (evaluate
dbt Core, ADR) → datasets (versioned, owned, lineage) → semantic layer (metrics defined once) → SQL Workbench
(Monaco, autocomplete, cancel, export streaming) → visual query builder → dashboard builder (ECharts, filters,
drill-down, cross-filter, sharing, versioning, scheduled reports) → governance (classification, sensitive fields,
dataset permissions incl. row/column obligations, query & export audit, retention) → external BI integration guides
(Metabase, Looker Studio, Power BI, Tableau) with documented per-provider limitations.

New resource types registered with authz: `datasource`, `dataset`, `semantic_model`, `metric`, `dashboard`, `query`.
Acceptance criteria: PRD §7.13.
