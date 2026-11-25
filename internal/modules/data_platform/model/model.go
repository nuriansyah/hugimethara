// Package model contains the data platform entities and pure invariants. It
// performs no I/O (ADR-0014).
package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/nuriansyah/hugimethara/internal/platform/sqlguard"
)

// Resource types registered in migration 00017.
const (
	ResourceDataSource = "datasource"
	ResourceDataset    = "dataset"
)

var ErrNotFound = errors.New("data platform resource not found")

var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
var tableRe = regexp.MustCompile(`^[a-z][a-z0-9_]{1,62}$`)
var columnRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,62}$`)

// Kind describes one source adapter.
type Kind struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	Category  string `json:"category"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

// Kinds is the source catalog shown by the API and the UI.
var Kinds = []Kind{
	{Key: "postgresql", Name: "PostgreSQL", Category: "database", Available: true},
	{Key: "rest", Name: "REST / JSON", Category: "api", Available: true},
	{Key: "file", Name: "CSV / JSON file", Category: "file", Available: true},
	{Key: "mysql", Name: "MySQL", Category: "database", Reason: "Adapter ships in S22."},
	{Key: "sqlserver", Name: "SQL Server", Category: "database", Reason: "Adapter ships in S22."},
	{Key: "oracle", Name: "Oracle", Category: "database", Reason: "Adapter ships in S22."},
	{Key: "graphql", Name: "GraphQL", Category: "api", Reason: "Adapter ships in S22."},
	{Key: "s3", Name: "S3-compatible storage", Category: "storage", Reason: "Adapter ships in S22."},
	{Key: "google_sheets", Name: "Google Sheets", Category: "api", Reason: "Adapter ships in S22."},
}

// KindInfo returns the catalog entry for key.
func KindInfo(key string) (Kind, bool) {
	for _, k := range Kinds {
		if k.Key == key {
			return k, true
		}
	}
	return Kind{}, false
}

// Available reports whether the adapter can test, discover and ingest today.
func Available(key string) bool {
	k, ok := KindInfo(key)
	return ok && k.Available
}

// ValidateSlug checks the resource slug rule shared by sources, pipelines,
// datasets, models and dashboards.
func ValidateSlug(slug string) error {
	if !slugRe.MatchString(slug) {
		return errors.New("use 1–63 lowercase letters, digits, or hyphens; begin with a letter or digit")
	}
	return nil
}

// ValidateTable checks an ingestion target table name.
func ValidateTable(name string) error {
	if !tableRe.MatchString(name) {
		return errors.New("use 2–63 lowercase letters, digits, or underscores; begin with a letter")
	}
	return nil
}

// ValidateColumn checks a column name used in policies and watermarks.
func ValidateColumn(name string) error {
	if !columnRe.MatchString(name) {
		return errors.New("invalid column name")
	}
	return nil
}

// ValidateSourceConfig checks the required connection fields for an available
// source kind. Secrets are never part of config.
func ValidateSourceConfig(kind string, config map[string]any) error {
	switch kind {
	case "postgresql":
		for _, field := range []string{"host", "database"} {
			if s, _ := config[field].(string); strings.TrimSpace(s) == "" {
				return fmt.Errorf("config.%s is required", field)
			}
		}
		if port, ok := config["port"].(float64); ok && (port < 1 || port > 65535) {
			return errors.New("config.port must be between 1 and 65535")
		}
	case "rest":
		base, _ := config["base_url"].(string)
		u, err := url.Parse(base)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return errors.New("config.base_url must be an http(s) URL")
		}
	case "file":
		format, _ := config["format"].(string)
		if format != "csv" && format != "json" {
			return errors.New("config.format must be csv or json")
		}
	}
	return nil
}

// DataSource is an external system registered by a workspace.
type DataSource struct {
	ID            uuid.UUID      `db:"id" json:"id"`
	OrgID         uuid.UUID      `db:"org_id" json:"org_id"`
	WorkspaceID   uuid.UUID      `db:"workspace_id" json:"workspace_id"`
	EnvironmentID uuid.UUID      `db:"environment_id" json:"environment_id"`
	Name          string         `db:"name" json:"name"`
	Slug          string         `db:"slug" json:"slug"`
	Kind          string         `db:"kind" json:"kind"`
	Config        map[string]any `db:"-" json:"config"`
	SecretID      *uuid.UUID     `db:"secret_id" json:"secret_id,omitempty"`
	Status        string         `db:"status" json:"status"`
	CreatedAt     time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time      `db:"updated_at" json:"updated_at"`
}

// SchemaColumn is one discovered column.
type SchemaColumn struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
}

// SchemaObject is one discovered table or object.
type SchemaObject struct {
	Name    string         `json:"name"`
	Type    string         `json:"type"`
	Columns []SchemaColumn `json:"columns"`
}

// Schema is a discovered source schema snapshot.
type Schema struct {
	Objects []SchemaObject `json:"objects"`
}

// DataSourceSchema is one snapshot.
type DataSourceSchema struct {
	ID           uuid.UUID `db:"id" json:"id"`
	DataSourceID uuid.UUID `db:"data_source_id" json:"data_source_id"`
	Schema       Schema    `db:"-" json:"schema"`
	Checksum     string    `db:"checksum" json:"checksum"`
	CapturedAt   time.Time `db:"captured_at" json:"captured_at"`
}

// Pipeline is a full or incremental ingestion definition.
type Pipeline struct {
	ID              uuid.UUID      `db:"id" json:"id"`
	OrgID           uuid.UUID      `db:"org_id" json:"org_id"`
	WorkspaceID     uuid.UUID      `db:"workspace_id" json:"workspace_id"`
	DataSourceID    uuid.UUID      `db:"data_source_id" json:"data_source_id"`
	Name            string         `db:"name" json:"name"`
	Slug            string         `db:"slug" json:"slug"`
	TargetTable     string         `db:"target_table" json:"target_table"`
	Mode            string         `db:"mode" json:"mode"`
	SourceObject    string         `db:"source_object" json:"source_object"`
	WatermarkColumn string         `db:"watermark_column" json:"watermark_column"`
	WatermarkType   string         `db:"watermark_type" json:"watermark_type"`
	ScheduleCron    string         `db:"schedule_cron" json:"schedule_cron"`
	Config          map[string]any `db:"-" json:"config"`
	Enabled         bool           `db:"enabled" json:"enabled"`
	LastRunAt       *time.Time     `db:"last_run_at" json:"last_run_at,omitempty"`
	NextRunAt       *time.Time     `db:"next_run_at" json:"next_run_at,omitempty"`
	CreatedAt       time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time      `db:"updated_at" json:"updated_at"`
}

// IngestionRun is one pipeline execution.
type IngestionRun struct {
	ID              uuid.UUID      `db:"id" json:"id"`
	PipelineID      uuid.UUID      `db:"pipeline_id" json:"pipeline_id"`
	Status          string         `db:"status" json:"status"`
	Mode            string         `db:"mode" json:"mode"`
	WatermarkBefore string         `db:"watermark_before" json:"watermark_before"`
	WatermarkAfter  string         `db:"watermark_after" json:"watermark_after"`
	RowsRead        int64          `db:"rows_read" json:"rows_read"`
	RowsWritten     int64          `db:"rows_written" json:"rows_written"`
	SchemaDrift     []string       `db:"-" json:"schema_drift,omitempty"`
	Error           map[string]any `db:"-" json:"error,omitempty"`
	StartedAt       *time.Time     `db:"started_at" json:"started_at,omitempty"`
	FinishedAt      *time.Time     `db:"finished_at" json:"finished_at,omitempty"`
	CreatedAt       time.Time      `db:"created_at" json:"created_at"`
}

// Dataset is a versioned SQL model.
type Dataset struct {
	ID          uuid.UUID `db:"id" json:"id"`
	OrgID       uuid.UUID `db:"org_id" json:"org_id"`
	WorkspaceID uuid.UUID `db:"workspace_id" json:"workspace_id"`
	Name        string    `db:"name" json:"name"`
	Slug        string    `db:"slug" json:"slug"`
	Description string    `db:"description" json:"description"`
	Owner       string    `db:"owner" json:"owner"`
	DraftSQL    string    `db:"draft_sql" json:"draft_sql"`
	Status      string    `db:"status" json:"status"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// DatasetVersion is an immutable published model.
type DatasetVersion struct {
	ID          uuid.UUID      `db:"id" json:"id"`
	DatasetID   uuid.UUID      `db:"dataset_id" json:"dataset_id"`
	Version     int            `db:"version" json:"version"`
	SQL         string         `db:"sql" json:"sql"`
	Columns     []SchemaColumn `db:"-" json:"columns"`
	Checksum    string         `db:"checksum" json:"checksum"`
	PublishedBy uuid.UUID      `db:"published_by" json:"published_by"`
	PublishedAt time.Time      `db:"published_at" json:"published_at"`
}

// Lineage is one upstream edge of a dataset.
type Lineage struct {
	DatasetID    uuid.UUID `db:"dataset_id" json:"dataset_id"`
	UpstreamKind string    `db:"upstream_kind" json:"upstream_kind"`
	UpstreamID   uuid.UUID `db:"upstream_id" json:"upstream_id"`
	Columns      []string  `db:"-" json:"columns"`
}

// ColumnPolicy classifies a dataset column.
type ColumnPolicy struct {
	ColumnName     string `db:"column_name" json:"column_name"`
	Classification string `db:"classification" json:"classification"`
	Mask           string `db:"mask" json:"mask"`
}

// RowPolicy is a role-specific obligation.
type RowPolicy struct {
	RoleKey     string            `db:"role_key" json:"role_key"`
	RowFilter   string            `db:"row_filter" json:"row_filter"`
	ColumnMasks map[string]string `db:"-" json:"column_masks"`
}

// DatasetView is the ClickHouse view name for a published dataset. The id
// suffix keeps two workspaces in one organization from colliding on a slug.
func DatasetView(id uuid.UUID, slug string) string {
	return "ds_" + strings.ReplaceAll(slug, "-", "_") + "_" + strings.ReplaceAll(id.String(), "-", "")[:8]
}

// RawObject decodes a JSONB object, tolerating null.
func RawObject(raw []byte) map[string]any {
	if len(raw) == 0 || string(raw) == "null" {
		return map[string]any{}
	}
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	if out == nil {
		out = map[string]any{}
	}
	return out
}

// RawArray decodes a JSONB array, tolerating null.
func RawArray(raw []byte) []any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var out []any
	_ = json.Unmarshal(raw, &out)
	return out
}

// JSONArg marshals a value for a jsonb parameter.
func JSONArg(v any) json.RawMessage {
	if v == nil {
		return json.RawMessage("{}")
	}
	raw, _ := json.Marshal(v)
	return raw
}

// DiffSchema describes changes from old to new in stable, human-readable lines.
func DiffSchema(oldSchema, newSchema Schema) []string {
	oldObjects := map[string]SchemaObject{}
	for _, o := range oldSchema.Objects {
		oldObjects[o.Name] = o
	}
	newObjects := map[string]SchemaObject{}
	for _, o := range newSchema.Objects {
		newObjects[o.Name] = o
	}
	var out []string
	var oldNames, newNames []string
	for name := range oldObjects {
		oldNames = append(oldNames, name)
	}
	for name := range newObjects {
		newNames = append(newNames, name)
	}
	sort.Strings(oldNames)
	sort.Strings(newNames)
	for _, name := range newNames {
		o := newObjects[name]
		prev, ok := oldObjects[name]
		if !ok {
			out = append(out, "object added: "+name)
			continue
		}
		oldCols := map[string]SchemaColumn{}
		for _, c := range prev.Columns {
			oldCols[c.Name] = c
		}
		newCols := map[string]SchemaColumn{}
		for _, c := range o.Columns {
			newCols[c.Name] = c
		}
		for _, c := range o.Columns {
			before, ok := oldCols[c.Name]
			if !ok {
				out = append(out, fmt.Sprintf("column added: %s.%s", name, c.Name))
				continue
			}
			if before.Type != c.Type {
				out = append(out, fmt.Sprintf("column type changed: %s.%s %s -> %s", name, c.Name, before.Type, c.Type))
			}
			if before.Nullable != c.Nullable {
				out = append(out, fmt.Sprintf("column nullability changed: %s.%s", name, c.Name))
			}
		}
		for _, c := range prev.Columns {
			if _, ok := newCols[c.Name]; !ok {
				out = append(out, fmt.Sprintf("column removed: %s.%s", name, c.Name))
			}
		}
	}
	for _, name := range oldNames {
		if _, ok := newObjects[name]; !ok {
			out = append(out, "object removed: "+name)
		}
	}
	return out
}

// ValidateDatasetSQL delegates to the shared guard with the allowed table set.
func ValidateDatasetSQL(sql string, allowed []string) error {
	return sqlguard.ValidateDatasetSQL(sql, allowed)
}
