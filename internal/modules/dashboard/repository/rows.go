// Package repository maps dashboard rows to models.
package repository

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	model "github.com/nuriansyah/hugimethara/internal/modules/dashboard/model"
)

type DashboardRow struct {
	ID             uuid.UUID  `db:"id"`
	OrgID          uuid.UUID  `db:"org_id"`
	WorkspaceID    uuid.UUID  `db:"workspace_id"`
	Name           string     `db:"name"`
	Slug           string     `db:"slug"`
	Description    string     `db:"description"`
	DraftRaw       []byte     `db:"draft"`
	Status         string     `db:"status"`
	ShareEnabled   bool       `db:"share_enabled"`
	ShareExpiresAt *time.Time `db:"share_expires_at"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
}

func (r DashboardRow) Model() model.Dashboard {
	var def model.Definition
	_ = json.Unmarshal(r.DraftRaw, &def)
	return model.Dashboard{ID: r.ID, OrgID: r.OrgID, WorkspaceID: r.WorkspaceID, Name: r.Name, Slug: r.Slug,
		Description: r.Description, Draft: def, Status: r.Status, ShareEnabled: r.ShareEnabled,
		ShareExpiresAt: r.ShareExpiresAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

type DashboardVersionRow struct {
	ID          uuid.UUID `db:"id"`
	DashboardID uuid.UUID `db:"dashboard_id"`
	Version     int       `db:"version"`
	Definition  []byte    `db:"definition"`
	Checksum    string    `db:"checksum"`
	PublishedBy uuid.UUID `db:"published_by"`
	PublishedAt time.Time `db:"published_at"`
}

func (r DashboardVersionRow) Model() model.DashboardVersion {
	var def model.Definition
	_ = json.Unmarshal(r.Definition, &def)
	return model.DashboardVersion{ID: r.ID, DashboardID: r.DashboardID, Version: r.Version, Definition: def,
		Checksum: r.Checksum, PublishedBy: r.PublishedBy, PublishedAt: r.PublishedAt}
}

type ReportRow struct {
	ID           uuid.UUID  `db:"id"`
	OrgID        uuid.UUID  `db:"org_id"`
	WorkspaceID  uuid.UUID  `db:"workspace_id"`
	DashboardID  uuid.UUID  `db:"dashboard_id"`
	Name         string     `db:"name"`
	ScheduleCron string     `db:"schedule_cron"`
	Recipients   []byte     `db:"recipients"`
	Format       string     `db:"format"`
	Enabled      bool       `db:"enabled"`
	NextFireAt   *time.Time `db:"next_fire_at"`
	LastRunAt    *time.Time `db:"last_run_at"`
	CreatedAt    time.Time  `db:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at"`
}

func (r ReportRow) Model() model.ScheduledReport {
	var recipients []string
	_ = json.Unmarshal(r.Recipients, &recipients)
	return model.ScheduledReport{ID: r.ID, OrgID: r.OrgID, WorkspaceID: r.WorkspaceID, DashboardID: r.DashboardID,
		Name: r.Name, ScheduleCron: r.ScheduleCron, Recipients: recipients, Format: r.Format, Enabled: r.Enabled,
		NextFireAt: r.NextFireAt, LastRunAt: r.LastRunAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}
