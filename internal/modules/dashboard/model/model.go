// Package model contains dashboard definitions, versions, shares and
// scheduled reports. It performs no I/O (ADR-0014).
package model

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
)

const ResourceDashboard = "dashboard"

var ErrNotFound = errors.New("dashboard not found")

var widgetIDRe = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,62}$`)

type Widget struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Title     string         `json:"title"`
	MetricID  *uuid.UUID     `json:"metric_id,omitempty"`
	DatasetID *uuid.UUID     `json:"dataset_id,omitempty"`
	SQL       string         `json:"sql,omitempty"`
	Width     int            `json:"width,omitempty"`
	Height    int            `json:"height,omitempty"`
	Config    map[string]any `json:"config,omitempty"`
}

type Filter struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Column  string `json:"column"`
	Type    string `json:"type"`
	Default any    `json:"default,omitempty"`
}

type Definition struct {
	SchemaVersion int      `json:"schema_version"`
	Widgets       []Widget `json:"widgets"`
	Filters       []Filter `json:"filters"`
}

type Dashboard struct {
	ID             uuid.UUID  `db:"id" json:"id"`
	OrgID          uuid.UUID  `db:"org_id" json:"org_id"`
	WorkspaceID    uuid.UUID  `db:"workspace_id" json:"workspace_id"`
	Name           string     `db:"name" json:"name"`
	Slug           string     `db:"slug" json:"slug"`
	Description    string     `db:"description" json:"description"`
	Draft          Definition `db:"-" json:"draft"`
	Status         string     `db:"status" json:"status"`
	ShareEnabled   bool       `db:"share_enabled" json:"share_enabled"`
	ShareExpiresAt *time.Time `db:"share_expires_at" json:"share_expires_at,omitempty"`
	CreatedAt      time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at" json:"updated_at"`
}

type DashboardVersion struct {
	ID          uuid.UUID  `db:"id" json:"id"`
	DashboardID uuid.UUID  `db:"dashboard_id" json:"dashboard_id"`
	Version     int        `db:"version" json:"version"`
	Definition  Definition `db:"-" json:"definition"`
	Checksum    string     `db:"checksum" json:"checksum"`
	PublishedBy uuid.UUID  `db:"published_by" json:"published_by"`
	PublishedAt time.Time  `db:"published_at" json:"published_at"`
}

type ScheduledReport struct {
	ID           uuid.UUID  `db:"id" json:"id"`
	OrgID        uuid.UUID  `db:"org_id" json:"org_id"`
	WorkspaceID  uuid.UUID  `db:"workspace_id" json:"workspace_id"`
	DashboardID  uuid.UUID  `db:"dashboard_id" json:"dashboard_id"`
	Name         string     `db:"name" json:"name"`
	ScheduleCron string     `db:"schedule_cron" json:"schedule_cron"`
	Recipients   []string   `db:"-" json:"recipients"`
	Format       string     `db:"format" json:"format"`
	Enabled      bool       `db:"enabled" json:"enabled"`
	NextFireAt   *time.Time `db:"next_fire_at" json:"next_fire_at,omitempty"`
	LastRunAt    *time.Time `db:"last_run_at" json:"last_run_at,omitempty"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at" json:"updated_at"`
}

func ValidateDefinition(d Definition) error {
	if d.SchemaVersion == 0 {
		d.SchemaVersion = 1
	}
	if d.SchemaVersion != 1 {
		return errors.New("schema_version must be 1")
	}
	seen := map[string]bool{}
	for _, w := range d.Widgets {
		if !widgetIDRe.MatchString(w.ID) {
			return fmt.Errorf("widget %q: use 1–63 lowercase letters, digits, _ or -", w.ID)
		}
		if seen[w.ID] {
			return fmt.Errorf("widget %q is duplicated", w.ID)
		}
		seen[w.ID] = true
		switch w.Type {
		case "metric":
			if w.MetricID == nil {
				return fmt.Errorf("widget %q: metric widgets need metric_id", w.ID)
			}
		case "table", "chart":
			if w.DatasetID == nil || w.SQL == "" {
				return fmt.Errorf("widget %q: table and chart widgets need dataset_id and sql", w.ID)
			}
		case "text":
		default:
			return fmt.Errorf("widget %q: type must be metric, table, chart or text", w.ID)
		}
	}
	for _, f := range d.Filters {
		if !widgetIDRe.MatchString(f.ID) {
			return fmt.Errorf("filter %q has an invalid id", f.ID)
		}
	}
	return nil
}
