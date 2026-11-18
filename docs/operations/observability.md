# Observability

- **Health:** `/healthz` (process alive), `/readyz` (Postgres, Redis, NATS reachable; migrations at expected version).
- **Logs:** `log/slog` JSON; fields `request_id`, `trace_id`, `org_id`, `principal_id`, `domain`; redacting handler.
- **Traces:** OpenTelemetry SDK, OTLP exporter (optional collector). Spans: HTTP server, pgx, Redis, NATS publish/consume
  (context propagated via message headers), egress HTTP client, connector invoke, workflow step.
- **Metrics (Prometheus, `/metrics` on separate port):**
  `hugi_http_requests_total{route,method,status}`, `hugi_http_request_duration_seconds`,
  `hugi_workflow_executions_total{status}`, `hugi_workflow_step_duration_seconds{node_type}`,
  `hugi_workflow_queue_lag_seconds`, `hugi_connector_requests_total{connector,status}`,
  `hugi_connector_request_duration_seconds`, `hugi_webhook_deliveries_total{status}`, `hugi_outbox_pending`,
  `hugi_authz_decisions_total{allowed}`, pgxpool stats.
- Org id is a **trace/log attribute, not a metric label** (cardinality), except per-org quota gauges sampled for top-N.
