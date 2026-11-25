// Package repository maps analytics rows to models.
package repository

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	model "github.com/nuriansyah/hugimethara/internal/modules/analytics/model"
)

type SemanticModelRow struct {
	ID          uuid.UUID `db:"id"`
	OrgID       uuid.UUID `db:"org_id"`
	WorkspaceID uuid.UUID `db:"workspace_id"`
	DatasetID   uuid.UUID `db:"dataset_id"`
	Name        string    `db:"name"`
	Slug        string    `db:"slug"`
	Description string    `db:"description"`
	Dimensions  []byte    `db:"dimensions"`
	Status      string    `db:"status"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

func (r SemanticModelRow) Model() model.SemanticModel {
	var dims []model.Dimension
	_ = json.Unmarshal(r.Dimensions, &dims)
	return model.SemanticModel{ID: r.ID, OrgID: r.OrgID, WorkspaceID: r.WorkspaceID, DatasetID: r.DatasetID,
		Name: r.Name, Slug: r.Slug, Description: r.Description, Dimensions: dims, Status: r.Status,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

type MetricRow struct {
	ID              uuid.UUID `db:"id"`
	OrgID           uuid.UUID `db:"org_id"`
	WorkspaceID     uuid.UUID `db:"workspace_id"`
	SemanticModelID uuid.UUID `db:"semantic_model_id"`
	Name            string    `db:"name"`
	Slug            string    `db:"slug"`
	Description     string    `db:"description"`
	Expression      string    `db:"expression"`
	Filters         []byte    `db:"filters"`
	Format          string    `db:"format"`
	Status          string    `db:"status"`
	CreatedAt       time.Time `db:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"`
}

func (r MetricRow) Model() model.Metric {
	var filters []string
	_ = json.Unmarshal(r.Filters, &filters)
	return model.Metric{ID: r.ID, OrgID: r.OrgID, WorkspaceID: r.WorkspaceID, SemanticModelID: r.SemanticModelID,
		Name: r.Name, Slug: r.Slug, Description: r.Description, Expression: r.Expression, Filters: filters,
		Format: r.Format, Status: r.Status, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

type QueryRunRow struct {
	ID            uuid.UUID  `db:"id"`
	OrgID         uuid.UUID  `db:"org_id"`
	WorkspaceID   uuid.UUID  `db:"workspace_id"`
	DatasetID     *uuid.UUID `db:"dataset_id"`
	MetricID      *uuid.UUID `db:"metric_id"`
	PrincipalID   uuid.UUID  `db:"principal_id"`
	PrincipalType string     `db:"principal_type"`
	SQL           string     `db:"sql"`
	ClickHouseID  string     `db:"clickhouse_id"`
	Status        string     `db:"status"`
	RowCount      int64      `db:"row_count"`
	BytesRead     int64      `db:"bytes_read"`
	DurationMS    int        `db:"duration_ms"`
	Columns       []byte     `db:"columns"`
	Preview       []byte     `db:"result_preview"`
	Error         string     `db:"error"`
	Exported      bool       `db:"exported"`
	CreatedAt     time.Time  `db:"created_at"`
	FinishedAt    *time.Time `db:"finished_at"`
}

func (r QueryRunRow) Model() model.QueryRun {
	var columns []string
	_ = json.Unmarshal(r.Columns, &columns)
	var preview any
	if len(r.Preview) > 0 {
		_ = json.Unmarshal(r.Preview, &preview)
	}
	return model.QueryRun{ID: r.ID, OrgID: r.OrgID, WorkspaceID: r.WorkspaceID, DatasetID: r.DatasetID,
		MetricID: r.MetricID, PrincipalID: r.PrincipalID, PrincipalType: r.PrincipalType, SQL: r.SQL,
		ClickHouseID: r.ClickHouseID, Status: r.Status, RowCount: r.RowCount, BytesRead: r.BytesRead,
		DurationMS: r.DurationMS, Columns: columns, ResultPreview: preview, Error: r.Error, Exported: r.Exported,
		CreatedAt: r.CreatedAt, FinishedAt: r.FinishedAt}
}
