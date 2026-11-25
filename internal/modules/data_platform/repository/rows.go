// Package repository maps data platform rows to models.
package repository

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	model "github.com/nuriansyah/hugimethara/internal/modules/data_platform/model"
)

type DataSourceRow struct {
	ID            uuid.UUID  `db:"id"`
	OrgID         uuid.UUID  `db:"org_id"`
	WorkspaceID   uuid.UUID  `db:"workspace_id"`
	EnvironmentID uuid.UUID  `db:"environment_id"`
	Name          string     `db:"name"`
	Slug          string     `db:"slug"`
	Kind          string     `db:"kind"`
	ConfigRaw     []byte     `db:"config"`
	SecretID      *uuid.UUID `db:"secret_id"`
	Status        string     `db:"status"`
	CreatedAt     time.Time  `db:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at"`
}

func (r DataSourceRow) Model() model.DataSource {
	return model.DataSource{ID: r.ID, OrgID: r.OrgID, WorkspaceID: r.WorkspaceID, EnvironmentID: r.EnvironmentID,
		Name: r.Name, Slug: r.Slug, Kind: r.Kind, Config: model.RawObject(r.ConfigRaw), SecretID: r.SecretID,
		Status: r.Status, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

type SchemaRow struct {
	ID           uuid.UUID `db:"id"`
	DataSourceID uuid.UUID `db:"data_source_id"`
	SchemaRaw    []byte    `db:"schema"`
	Checksum     string    `db:"checksum"`
	CapturedAt   time.Time `db:"captured_at"`
}

func (r SchemaRow) Model() model.DataSourceSchema {
	var s model.Schema
	_ = json.Unmarshal(r.SchemaRaw, &s)
	return model.DataSourceSchema{ID: r.ID, DataSourceID: r.DataSourceID, Schema: s, Checksum: r.Checksum, CapturedAt: r.CapturedAt}
}

type PipelineRow struct {
	ID              uuid.UUID  `db:"id"`
	OrgID           uuid.UUID  `db:"org_id"`
	WorkspaceID     uuid.UUID  `db:"workspace_id"`
	DataSourceID    uuid.UUID  `db:"data_source_id"`
	Name            string     `db:"name"`
	Slug            string     `db:"slug"`
	TargetTable     string     `db:"target_table"`
	Mode            string     `db:"mode"`
	SourceObject    string     `db:"source_object"`
	WatermarkColumn string     `db:"watermark_column"`
	WatermarkType   string     `db:"watermark_type"`
	ScheduleCron    string     `db:"schedule_cron"`
	ConfigRaw       []byte     `db:"config"`
	Enabled         bool       `db:"enabled"`
	LastRunAt       *time.Time `db:"last_run_at"`
	NextRunAt       *time.Time `db:"next_run_at"`
	CreatedAt       time.Time  `db:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at"`
}

func (r PipelineRow) Model() model.Pipeline {
	return model.Pipeline{ID: r.ID, OrgID: r.OrgID, WorkspaceID: r.WorkspaceID, DataSourceID: r.DataSourceID,
		Name: r.Name, Slug: r.Slug, TargetTable: r.TargetTable, Mode: r.Mode, SourceObject: r.SourceObject,
		WatermarkColumn: r.WatermarkColumn, WatermarkType: r.WatermarkType, ScheduleCron: r.ScheduleCron,
		Config: model.RawObject(r.ConfigRaw), Enabled: r.Enabled, LastRunAt: r.LastRunAt, NextRunAt: r.NextRunAt,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

type RunRow struct {
	ID              uuid.UUID  `db:"id"`
	PipelineID      uuid.UUID  `db:"pipeline_id"`
	Status          string     `db:"status"`
	Mode            string     `db:"mode"`
	WatermarkBefore string     `db:"watermark_before"`
	WatermarkAfter  string     `db:"watermark_after"`
	RowsRead        int64      `db:"rows_read"`
	RowsWritten     int64      `db:"rows_written"`
	SchemaDriftRaw  []byte     `db:"schema_drift"`
	ErrorRaw        []byte     `db:"error"`
	StartedAt       *time.Time `db:"started_at"`
	FinishedAt      *time.Time `db:"finished_at"`
	CreatedAt       time.Time  `db:"created_at"`
}

func (r RunRow) Model() model.IngestionRun {
	out := model.IngestionRun{ID: r.ID, PipelineID: r.PipelineID, Status: r.Status, Mode: r.Mode,
		WatermarkBefore: r.WatermarkBefore, WatermarkAfter: r.WatermarkAfter, RowsRead: r.RowsRead,
		RowsWritten: r.RowsWritten, StartedAt: r.StartedAt, FinishedAt: r.FinishedAt, CreatedAt: r.CreatedAt}
	if len(r.SchemaDriftRaw) > 0 {
		_ = json.Unmarshal(r.SchemaDriftRaw, &out.SchemaDrift)
	}
	if len(r.ErrorRaw) > 0 {
		_ = json.Unmarshal(r.ErrorRaw, &out.Error)
	}
	return out
}

type DatasetRow struct {
	ID          uuid.UUID `db:"id"`
	OrgID       uuid.UUID `db:"org_id"`
	WorkspaceID uuid.UUID `db:"workspace_id"`
	Name        string    `db:"name"`
	Slug        string    `db:"slug"`
	Description string    `db:"description"`
	Owner       string    `db:"owner"`
	DraftSQL    string    `db:"draft_sql"`
	Status      string    `db:"status"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

func (r DatasetRow) Model() model.Dataset {
	return model.Dataset{ID: r.ID, OrgID: r.OrgID, WorkspaceID: r.WorkspaceID, Name: r.Name, Slug: r.Slug,
		Description: r.Description, Owner: r.Owner, DraftSQL: r.DraftSQL, Status: r.Status,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

type DatasetVersionRow struct {
	ID          uuid.UUID `db:"id"`
	DatasetID   uuid.UUID `db:"dataset_id"`
	Version     int       `db:"version"`
	SQL         string    `db:"sql"`
	ColumnsRaw  []byte    `db:"columns"`
	Checksum    string    `db:"checksum"`
	PublishedBy uuid.UUID `db:"published_by"`
	PublishedAt time.Time `db:"published_at"`
}

func (r DatasetVersionRow) Model() model.DatasetVersion {
	var cols []model.SchemaColumn
	_ = json.Unmarshal(r.ColumnsRaw, &cols)
	return model.DatasetVersion{ID: r.ID, DatasetID: r.DatasetID, Version: r.Version, SQL: r.SQL, Columns: cols,
		Checksum: r.Checksum, PublishedBy: r.PublishedBy, PublishedAt: r.PublishedAt}
}

type LineageRow struct {
	DatasetID    uuid.UUID `db:"dataset_id"`
	UpstreamKind string    `db:"upstream_kind"`
	UpstreamID   uuid.UUID `db:"upstream_id"`
	ColumnsRaw   []byte    `db:"columns"`
}

func (r LineageRow) Model() model.Lineage {
	var cols []string
	_ = json.Unmarshal(r.ColumnsRaw, &cols)
	return model.Lineage{DatasetID: r.DatasetID, UpstreamKind: r.UpstreamKind, UpstreamID: r.UpstreamID, Columns: cols}
}

type ColumnPolicyRow struct {
	ColumnName     string `db:"column_name"`
	Classification string `db:"classification"`
	Mask           string `db:"mask"`
}

func (r ColumnPolicyRow) Model() model.ColumnPolicy {
	return model.ColumnPolicy{ColumnName: r.ColumnName, Classification: r.Classification, Mask: r.Mask}
}

type RowPolicyRow struct {
	RoleKey        string `db:"role_key"`
	RowFilter      string `db:"row_filter"`
	ColumnMasksRaw []byte `db:"column_masks"`
}

func (r RowPolicyRow) Model() model.RowPolicy {
	var masks map[string]string
	_ = json.Unmarshal(r.ColumnMasksRaw, &masks)
	return model.RowPolicy{RoleKey: r.RoleKey, RowFilter: r.RowFilter, ColumnMasks: masks}
}
