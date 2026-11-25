-- dashboard repository — Write side.

-- name: InsertDashboard
INSERT INTO dashboards (org_id, id, workspace_id, name, slug, description, draft, status, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: UpdateDashboard
UPDATE dashboards
SET name = COALESCE($2, name),
    slug = COALESCE($3, slug),
    description = COALESCE($4, description),
    draft = COALESCE($5, draft),
    status = COALESCE($6, status)
WHERE id = $1;

-- name: DeleteDashboard
DELETE FROM dashboards WHERE id = $1;

-- name: SetShare
UPDATE dashboards
SET share_token_hash = $2, share_enabled = $3, share_expires_at = $4
WHERE id = $1;

-- name: InsertDashboardVersion
INSERT INTO dashboard_versions (org_id, id, dashboard_id, version, definition, checksum, published_by)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: InsertReport
INSERT INTO scheduled_reports (org_id, id, workspace_id, dashboard_id, name, schedule_cron, recipients, format, enabled, next_fire_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: UpdateReport
UPDATE scheduled_reports
SET name = COALESCE($2, name),
    schedule_cron = COALESCE($3, schedule_cron),
    recipients = COALESCE($4, recipients),
    format = COALESCE($5, format),
    enabled = COALESCE($6, enabled),
    next_fire_at = CASE WHEN $7 THEN $8 ELSE next_fire_at END
WHERE id = $1;

-- name: DeleteReport
DELETE FROM scheduled_reports WHERE id = $1;

-- name: ClaimReport
UPDATE scheduled_reports SET next_fire_at = $2 WHERE id = $1;

-- name: FinishReport
UPDATE scheduled_reports SET last_run_at = now(), next_fire_at = $2 WHERE id = $1;
