# Data Architecture

## Stores and what lives where

| Store | Phase | Holds | Never holds |
|---|---|---|---|
| PostgreSQL | 1 | All metadata, config, identity, authz, execution state, webhook events (≤256 KB), audit (hot, 90d) | Analytical fact tables, large blobs |
| Redis | 1 | Session cache, rate-limit buckets, authz version stamps, short-lived locks | Anything that must survive a flush |
| NATS JetStream | 1 | Dispatch messages (step, delivery, events), short retention | Source-of-truth state |
| S3 / MinIO | 1 | Large step payloads, OpenAPI specs, webhook payloads >256 KB, exports | Secrets (unless encrypted envelope) |
| ClickHouse | 2 | Raw ingested data, transformed models, curated datasets, long-term audit/decision logs, execution analytics | Transactional metadata |

## Phase 2 layering (preview)

```
sources ──ingest──▶ raw_<org>.<source>_<table>   (ClickHouse, append-only, ingestion metadata columns)
                     │ SQL transforms / dbt Core (evaluated, ADR in Phase 2)
                     ▼
                    stg_ / int_ models ──▶ curated datasets (versioned, owned, permissioned)
                                            │
                              semantic layer (metrics/dimensions defined once)
                                            ▼
                         query service (authz + row/column policy + cost limits)
                                            ▼
                         dashboards · SQL workbench · visual builder · exports · external BI
```

Tenant isolation in ClickHouse: **database per organization** plus a query service that is the only component holding
ClickHouse credentials and that rewrites/guards every query (row policies + `max_execution_time`,
`max_memory_usage`, `max_result_rows`). Decision recorded when Phase 2 starts.
