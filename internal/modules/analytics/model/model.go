// Package model contains the analytics semantic layer, metric and query-run
// entities. It performs no I/O (ADR-0014).
package model

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	ResourceSemanticModel = "semantic_model"
	ResourceMetric        = "metric"
	ResourceQuery         = "query"
)

var ErrNotFound = errors.New("analytics resource not found")

var dimensionRe = regexp.MustCompile(`^[a-z][a-z0-9_]{1,62}$`)

type Dimension struct {
	Name   string `json:"name"`
	Column string `json:"column"`
	Type   string `json:"type"`
}

type SemanticModel struct {
	ID          uuid.UUID   `db:"id" json:"id"`
	OrgID       uuid.UUID   `db:"org_id" json:"org_id"`
	WorkspaceID uuid.UUID   `db:"workspace_id" json:"workspace_id"`
	DatasetID   uuid.UUID   `db:"dataset_id" json:"dataset_id"`
	Name        string      `db:"name" json:"name"`
	Slug        string      `db:"slug" json:"slug"`
	Description string      `db:"description" json:"description"`
	Dimensions  []Dimension `db:"-" json:"dimensions"`
	Status      string      `db:"status" json:"status"`
	CreatedAt   time.Time   `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time   `db:"updated_at" json:"updated_at"`
}

type Metric struct {
	ID              uuid.UUID `db:"id" json:"id"`
	OrgID           uuid.UUID `db:"org_id" json:"org_id"`
	WorkspaceID     uuid.UUID `db:"workspace_id" json:"workspace_id"`
	SemanticModelID uuid.UUID `db:"semantic_model_id" json:"semantic_model_id"`
	Name            string    `db:"name" json:"name"`
	Slug            string    `db:"slug" json:"slug"`
	Description     string    `db:"description" json:"description"`
	Expression      string    `db:"expression" json:"expression"`
	Filters         []string  `db:"-" json:"filters"`
	Format          string    `db:"format" json:"format"`
	Status          string    `db:"status" json:"status"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time `db:"updated_at" json:"updated_at"`
}

type QueryRun struct {
	ID            uuid.UUID  `db:"id" json:"id"`
	OrgID         uuid.UUID  `db:"org_id" json:"org_id"`
	WorkspaceID   uuid.UUID  `db:"workspace_id" json:"workspace_id"`
	DatasetID     *uuid.UUID `db:"dataset_id" json:"dataset_id,omitempty"`
	MetricID      *uuid.UUID `db:"metric_id" json:"metric_id,omitempty"`
	PrincipalID   uuid.UUID  `db:"principal_id" json:"principal_id"`
	PrincipalType string     `db:"principal_type" json:"principal_type"`
	SQL           string     `db:"sql" json:"sql"`
	ClickHouseID  string     `db:"clickhouse_id" json:"clickhouse_id"`
	Status        string     `db:"status" json:"status"`
	RowCount      int64      `db:"row_count" json:"row_count"`
	BytesRead     int64      `db:"bytes_read" json:"bytes_read"`
	DurationMS    int        `db:"duration_ms" json:"duration_ms"`
	Columns       []string   `db:"-" json:"columns"`
	ResultPreview any        `db:"-" json:"result_preview,omitempty"`
	Error         string     `db:"error" json:"error,omitempty"`
	Exported      bool       `db:"exported" json:"exported"`
	CreatedAt     time.Time  `db:"created_at" json:"created_at"`
	FinishedAt    *time.Time `db:"finished_at" json:"finished_at,omitempty"`
}

type QueryResult struct {
	RunID      uuid.UUID `json:"run_id"`
	Columns    []string  `json:"columns"`
	Types      []string  `json:"types,omitempty"`
	Rows       [][]any   `json:"rows"`
	RowCount   int64     `json:"row_count"`
	BytesRead  int64     `json:"bytes_read"`
	DurationMS int       `json:"duration_ms"`
	Truncated  bool      `json:"truncated"`
}

type MetricValue struct {
	MetricID uuid.UUID `json:"metric_id"`
	Value    any       `json:"value"`
	SQL      string    `json:"sql"`
	RunID    uuid.UUID `json:"run_id"`
}

func ValidateDimension(d Dimension) error {
	if !dimensionRe.MatchString(d.Name) {
		return fmt.Errorf("dimension %q: use 2–63 lowercase letters, digits or underscores", d.Name)
	}
	if strings.TrimSpace(d.Column) == "" {
		return fmt.Errorf("dimension %q: a column is required", d.Name)
	}
	switch d.Type {
	case "string", "number", "timestamp", "boolean":
	default:
		return fmt.Errorf("dimension %q: type must be string, number, timestamp or boolean", d.Name)
	}
	return nil
}

func ValidateDimensions(dimensions []Dimension) error {
	seen := map[string]bool{}
	for _, d := range dimensions {
		if err := ValidateDimension(d); err != nil {
			return err
		}
		if seen[d.Name] {
			return fmt.Errorf("dimension %q is duplicated", d.Name)
		}
		seen[d.Name] = true
	}
	return nil
}

func ValidateMetricFormat(format string) error {
	switch format {
	case "number", "currency", "percent", "duration":
		return nil
	default:
		return errors.New("format must be number, currency, percent or duration")
	}
}
