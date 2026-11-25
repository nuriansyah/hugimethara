# Phase 2 — Data Platform, Analytics & BI

Goal: an organization can connect its operational systems, land their data in an analytical store, model it as
versioned datasets, define metrics once, explore it in a SQL workbench, and publish shared dashboards — with every
query, export and dataset change authorized, limited and audited.

Phase 1 acceptance is verified (see [IMPLEMENTATION_STATUS.md](../IMPLEMENTATION_STATUS.md)). This document is the
Phase 2 slice plan written at kickoff. Each slice is vertical: migration + backend (model, repository, service,
handler) + OpenAPI + frontend + tests + docs. A slice is done only against the Definition of Done (PRD §19).

## 1. Architecture decisions confirmed at kickoff

- **Analytical store:** ClickHouse, per ADR-0008. Postgres remains the control plane. Hugimethara is the only
  component with ClickHouse credentials; clients never connect directly. Each organization gets one ClickHouse
  database (`org_<uuid hex>`), created on first use. Raw ingestion tables, dataset views and semantic queries all
  live there.
- **Query service:** every query goes through one guard: read-only statement shape, single statement, allowed table
  references, `max_execution_time`, `max_memory_usage`, `max_result_rows`, and an explicit row/column policy pass.
  Query runs are stored in Postgres (`query_runs`) and audited (`query.executed`, `query.exported`).
- **Transformation language:** native SQL models first (draft → validate → publish → immutable version). dbt Core
  is evaluated in S21: if adopted, it compiles to the same versioned SQL artifacts; no dbt runtime is required for
  the first three slices.
- **Source adapters:** PostgreSQL, REST/JSON, CSV/file and S3-compatible object storage ship first. MySQL, SQL
  Server, Oracle, GraphQL and Google Sheets are catalogued with stable `source_kind_unavailable` errors until their
  adapters land (S22). This keeps the trust boundary explicit instead of shipping untested drivers.
- **Ingestion:** full and incremental (watermark) modes, manual and scheduled runs. Incremental runs persist the
  watermark and a schema snapshot; a changed source schema is recorded as drift on the run and surfaced in the UI.
  Ingestion writes append-only raw tables with `_hugi_ingested_at`, `_hugi_run_id` and `_hugi_source`.
- **Row/column governance:** datasets own column classifications and per-role obligations (`row_filter`, column
  masks). Published dataset views embed the row filter for the querying role where possible; result masking is the
  final backstop for ad-hoc workbench queries.
- **Resource types:** `datasource`, `dataset`, `semantic_model`, `metric`, `dashboard`, `query`, registered with
  authz and granted to the standard role templates.

## 2. Vertical slices

| # | Slice | Delivers (DB · API · UI) | Exit check |
|---|---|---|---|
| S15 | **Analytical store & query service** | ClickHouse config/compose/health, per-org databases, query guard, `query_runs` · `POST /queries`, `GET /queries`, `GET /queries/{id}`, `POST /queries/{id}/cancel`, `GET /queries/{id}/export` · SQL Workbench (SQL editor, dataset picker, run/cancel, result grid, CSV export) | Read-only guard and limit unit tests; query lifecycle + cancel + export + tenant isolation integration test; compose health green |
| S16 | **Data sources & schema discovery** | `data_sources`, `data_source_schemas` · CRUD, test connection, discover schema · Data sources page | PostgreSQL/REST/CSV discovery tests; secrets never echoed; egress policy honoured |
| S17 | **Ingestion** | `ingestion_pipelines`, `ingestion_runs` · CRUD, run now, run history, schedule · pipeline editor + runs | Full and incremental Postgres/REST ingestion E2E; watermark checkpoint; schema drift recorded; scheduled worker run |
| S18 | **Datasets & semantic layer** | `datasets`, `dataset_versions`, `dataset_lineage`, `semantic_models`, `metrics` · CRUD, validate/preview, publish, lineage, metric query · Datasets page, semantic model/metric editor | Publish creates an immutable version and ClickHouse view; preview honours row filter; metric query returns one aggregate |
| S19 | **Governance** | `dataset_column_policies`, `dataset_row_policies` · classifications, per-role row filters and column masks, query/export audit · policy editor | Sensitive columns masked for unauthorized roles; row filter applied; every export audited |
| S20 | **Dashboards** | `dashboards`, `dashboard_versions`, `scheduled_reports` · CRUD, publish, share link, widgets, schedule · dashboard builder and viewer | Dashboard publish versioning; share link read-only; widget queries go through the query service; scheduled report fires from the worker |
| S21 | **Transformations & BI interop** | dbt Core evaluation (ADR), external BI guides (Metabase, Looker Studio, Power BI, Tableau), dataset lineage graph | Decision recorded; guides validated against the read-only query service; lineage graph renders |
| S22 | **Source breadth & hardening** | MySQL, SQL Server, Oracle, GraphQL, S3, Google Sheets adapters; autocomplete; visual query builder; retention; k6 data-plane baselines; acceptance review | Adapter matrix tests; every §7.13 criterion verified with evidence |

This session ships S15–S20 end to end (with the source breadth and transformation caveats above) and records the
remaining work in the P2 backlog.

## 3. Acceptance criteria → slices

| PRD §7.13 criterion | Slice(s) |
|---|---|
| ClickHouse foundation with per-org isolation and guarded queries | S15 |
| Data sources connect and schema is discovered | S16, S22 |
| Ingestion is full, incremental, scheduled, checkpointed, drift-aware | S17 |
| Datasets are versioned, owned and traced to lineage | S18 |
| Metrics are defined once in a semantic layer | S18 |
| SQL workbench supports run, cancel and streaming export | S15 |
| Dashboards support filters, sharing and versioning | S20 |
| Sensitive data is classified and row/column policies are enforced | S19 |
| Queries and exports are audited and retention-bound | S19, S22 |
| External BI tools can connect through the governed query service | S21 |
| New resource types are registered with authz | every slice, migration 00017 |
| Integration tests pass | every slice |
| Docker Compose deployment works (including ClickHouse) | S15, S22 |

## 4. Prioritized backlog after S20

### P0
- MySQL/SQL Server/Oracle/GraphQL/S3/Google Sheets adapters with a shared connection-test matrix.
- Autocomplete (schema-aware) and a visual query builder in the workbench.
- Scheduled report delivery (email/Slack) through the connector runtime.

### P1
- dbt Core project import/compile and dataset lineage graph.
- ClickHouse row policies as a second enforcement layer (defence in depth).
- Query result spooling to object storage for large exports.
- Data quality checks and freshness SLAs.

### P2
- Anomaly detection, NL→SQL, cost management and per-org analytical quotas.
- External BI connection guides validated against each provider.
- Retention jobs and long-term audit/decision-log export.

## 5. Explicit deviations in this session

- The web workbench uses a plain SQL editor, not Monaco (Monaco is an editor dependency, not a correctness gate);
  the API already exposes schema metadata for autocomplete.
- Dashboard widgets render as accessible tables and inline SVG charts; ECharts is deferred with the visual builder.
- Row filters are embedded into published dataset views and applied to previews; ad-hoc workbench queries against
  raw tables are restricted to principals holding `dataset:export`, and sensitive result columns are masked.
- MySQL, SQL Server, Oracle, GraphQL, S3 and Google Sheets are catalogued and validated but return
  `source_kind_unavailable` from test/discover/ingest until S22.
