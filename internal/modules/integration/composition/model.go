package composition

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Composition struct {
	ID          uuid.UUID      `json:"id"`
	WorkspaceID uuid.UUID      `json:"workspace_id"`
	Name        string         `json:"name"`
	Slug        string         `json:"slug"`
	Draft       map[string]any `json:"draft"`
	Exposed     bool           `json:"exposed"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type Version struct {
	ID            uuid.UUID      `json:"id"`
	CompositionID uuid.UUID      `json:"composition_id"`
	Version       int            `json:"version"`
	Definition    map[string]any `json:"definition"`
	Checksum      string         `json:"checksum"`
	PublishedBy   uuid.UUID      `json:"published_by"`
	PublishedAt   time.Time      `json:"published_at"`
}

type Execution struct {
	ID            uuid.UUID       `json:"id"`
	CompositionID uuid.UUID       `json:"composition_id"`
	VersionID     uuid.UUID       `json:"version_id"`
	EnvironmentID uuid.UUID       `json:"environment_id"`
	StartedAt     time.Time       `json:"started_at"`
	DurationMs    int             `json:"duration_ms"`
	Status        string          `json:"status"`
	StepSummary   json.RawMessage `json:"step_summary"`
	Output        any             `json:"output,omitempty"`
	Error         *string         `json:"error,omitempty"`
	RequestID     string          `json:"request_id"`
}

type CreateInput struct {
	WorkspaceID uuid.UUID
	Name        string
	Slug        string
	Draft       map[string]any
}

type UpdateInput struct {
	Name    *string
	Slug    *string
	Draft   *map[string]any
	Exposed *bool
}

type ExecuteInput struct {
	EnvironmentID uuid.UUID      `json:"environment_id"`
	Request       map[string]any `json:"request"`
	RequestID     string
}

type StepResult struct {
	Status     int    `json:"status,omitempty"`
	Body       any    `json:"body,omitempty"`
	DurationMs int    `json:"duration_ms,omitempty"`
	Skipped    bool   `json:"skipped,omitempty"`
	Error      string `json:"error,omitempty"`
	Attempts   int    `json:"attempts,omitempty"`
}
