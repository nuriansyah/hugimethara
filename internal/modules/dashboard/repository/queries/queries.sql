-- dashboard repository — Read side.

-- name: ListDashboards
SELECT id, org_id, workspace_id, name, slug, description, draft, status, share_enabled, share_expires_at,
       created_at, updated_at
FROM dashboards WHERE workspace_id = $1 AND status <> 'archived' ORDER BY name;

-- name: GetDashboard
SELECT id, org_id, workspace_id, name, slug, description, draft, status, share_enabled, share_expires_at,
       created_at, updated_at
FROM dashboards WHERE id = $1;

-- name: LatestDashboardVersion
SELECT id, dashboard_id, version, definition, checksum, published_by, published_at
FROM dashboard_versions WHERE dashboard_id = $1 ORDER BY version DESC LIMIT 1;

-- name: ListDashboardVersions
SELECT id, dashboard_id, version, definition, checksum, published_by, published_at
FROM dashboard_versions WHERE dashboard_id = $1 ORDER BY version DESC;

-- name: ListReports
SELECT id, org_id, workspace_id, dashboard_id, name, schedule_cron, recipients, format, enabled,
       next_fire_at, last_run_at, created_at, updated_at
FROM scheduled_reports WHERE workspace_id = $1 ORDER BY name;

-- name: GetReport
SELECT id, org_id, workspace_id, dashboard_id, name, schedule_cron, recipients, format, enabled,
       next_fire_at, last_run_at, created_at, updated_at
FROM scheduled_reports WHERE id = $1;

-- name: DueReports
SELECT id, org_id, workspace_id, dashboard_id, name, schedule_cron, recipients, format, enabled,
       next_fire_at, last_run_at, created_at, updated_at
FROM scheduled_reports
WHERE enabled AND next_fire_at IS NOT NULL AND next_fire_at <= now()
ORDER BY next_fire_at LIMIT $1 FOR UPDATE SKIP LOCKED;
