-- governance/overview repository — Read side. All statements run in a
-- tenant-bound transaction; RLS scopes them to the caller's organization.

-- name: WorkflowTotals
-- $1 workspace, $2 window start.
SELECT
  count(*) FILTER (WHERE e.created_at >= $2)                                              AS total,
  count(*) FILTER (WHERE e.created_at >= $2 AND e.status = 'succeeded')                   AS succeeded,
  count(*) FILTER (WHERE e.created_at >= $2 AND e.status IN ('failed', 'timed_out'))      AS failed,
  count(*) FILTER (WHERE e.created_at >= $2 AND e.status = 'cancelled')                   AS cancelled,
  count(*) FILTER (WHERE e.status IN ('pending', 'running'))                              AS running,
  (percentile_cont(0.5) WITHIN GROUP (ORDER BY extract(epoch FROM e.finished_at - e.started_at) * 1000)
     FILTER (WHERE e.created_at >= $2 AND e.finished_at IS NOT NULL AND e.started_at IS NOT NULL))::int AS p50_ms,
  (percentile_cont(0.95) WITHIN GROUP (ORDER BY extract(epoch FROM e.finished_at - e.started_at) * 1000)
     FILTER (WHERE e.created_at >= $2 AND e.finished_at IS NOT NULL AND e.started_at IS NOT NULL))::int AS p95_ms,
  (SELECT count(*) FROM approval_tasks t WHERE t.workspace_id = $1 AND t.status = 'pending') AS pending_approvals,
  (SELECT count(*) FROM workflows w WHERE w.workspace_id = $1 AND w.status = 'active')     AS active_workflows
FROM workflow_executions e
WHERE e.workspace_id = $1 AND (e.created_at >= $2 OR e.status IN ('pending', 'running'));

-- name: WorkflowSeries
-- $1 workspace, $2 window start, $3 bucket ('hour' | 'day').
SELECT date_trunc($3, e.created_at, 'UTC') AS bucket,
       count(*) FILTER (WHERE e.status = 'succeeded')               AS succeeded,
       count(*) FILTER (WHERE e.status IN ('failed', 'timed_out'))  AS failed
FROM workflow_executions e
WHERE e.workspace_id = $1 AND e.created_at >= $2
GROUP BY 1 ORDER BY 1;

-- name: TopFailingWorkflows
SELECT w.id AS workflow_id, w.name, count(*) AS failures,
       coalesce((array_agg(e.error ->> 'code' ORDER BY e.created_at DESC))[1], '') AS last_error_code,
       max(e.created_at) AS last_failed_at
FROM workflow_executions e
JOIN workflows w ON w.id = e.workflow_id
WHERE e.workspace_id = $1 AND e.created_at >= $2 AND e.status IN ('failed', 'timed_out')
GROUP BY w.id, w.name
ORDER BY failures DESC, last_failed_at DESC
LIMIT 5;

-- name: ConnectorHealth
-- Latest health check per instance of the workspace.
WITH latest AS (
  SELECT DISTINCT ON (h.instance_id) h.instance_id, h.ok, h.error_code, h.checked_at
  FROM connector_health h
  JOIN connector_instances i ON i.id = h.instance_id
  WHERE i.workspace_id = $1
  ORDER BY h.instance_id, h.checked_at DESC
)
SELECT count(*)                                           AS total,
       count(*) FILTER (WHERE i.status = 'enabled')       AS enabled,
       count(*) FILTER (WHERE l.ok)                       AS healthy,
       count(*) FILTER (WHERE l.ok = false)               AS unhealthy,
       count(*) FILTER (WHERE l.instance_id IS NULL)      AS unchecked
FROM connector_instances i
LEFT JOIN latest l ON l.instance_id = i.id
WHERE i.workspace_id = $1;

-- name: UnhealthyConnectors
WITH latest AS (
  SELECT DISTINCT ON (h.instance_id) h.instance_id, h.ok, h.error_code, h.checked_at
  FROM connector_health h
  JOIN connector_instances i ON i.id = h.instance_id
  WHERE i.workspace_id = $1
  ORDER BY h.instance_id, h.checked_at DESC
)
SELECT i.id, i.ref_key, d.key AS definition_key, i.environment_id, l.error_code, l.checked_at
FROM latest l
JOIN connector_instances i ON i.id = l.instance_id
JOIN connector_versions v ON v.id = i.connector_version_id
JOIN connector_definitions d ON d.id = v.definition_id
WHERE l.ok = false
ORDER BY l.checked_at DESC
LIMIT 10;

-- name: WebhookTotals
-- $1 workspace, $2 window start.
SELECT
  (SELECT count(*) FROM webhook_endpoints p WHERE p.workspace_id = $1)                         AS endpoints,
  (SELECT count(*) FROM webhook_events v WHERE v.workspace_id = $1 AND v.received_at >= $2)    AS received,
  (SELECT coalesce(sum(p.rejected_count), 0) FROM webhook_endpoints p WHERE p.workspace_id = $1) AS rejected_total,
  (SELECT count(*) FROM webhook_subscriptions s WHERE s.workspace_id = $1)                     AS subscriptions,
  count(*) FILTER (WHERE d.status = 'succeeded')                                               AS deliveries_succeeded,
  count(*) FILTER (WHERE d.status = 'pending')                                                 AS deliveries_pending,
  count(*) FILTER (WHERE d.status = 'dead_lettered')                                           AS deliveries_dead_lettered
FROM webhook_deliveries d
WHERE d.workspace_id = $1 AND d.created_at >= $2;
