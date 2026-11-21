package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	compositionmodel "github.com/nuriansyah/hugimethara/internal/modules/api_composition/model"
)

type CompositionRow struct {
	ID          uuid.UUID       `db:"id"`
	WorkspaceID uuid.UUID       `db:"workspace_id"`
	Name        string          `db:"name"`
	Slug        string          `db:"slug"`
	Draft       json.RawMessage `db:"draft"`
	Exposed     bool            `db:"exposed"`
	CreatedAt   time.Time       `db:"created_at"`
	UpdatedAt   time.Time       `db:"updated_at"`
}

type VersionRow struct {
	ID            uuid.UUID       `db:"id"`
	CompositionID uuid.UUID       `db:"composition_id"`
	Version       int             `db:"version"`
	Definition    json.RawMessage `db:"definition"`
	Checksum      string          `db:"checksum"`
	PublishedBy   uuid.UUID       `db:"published_by"`
	PublishedAt   time.Time       `db:"published_at"`
}

type ExecutionRow struct {
	ID            uuid.UUID       `db:"id"`
	CompositionID uuid.UUID       `db:"composition_id"`
	VersionID     uuid.UUID       `db:"version_id"`
	EnvironmentID uuid.UUID       `db:"environment_id"`
	StartedAt     time.Time       `db:"started_at"`
	DurationMs    int             `db:"duration_ms"`
	Status        string          `db:"status"`
	StepSummary   json.RawMessage `db:"step_summary"`
	Output        []byte          `db:"output"`
	Error         *string         `db:"error"`
	RequestID     string          `db:"request_id"`
	WorkspaceID   uuid.UUID       `db:"workspace_id"`
}

func (r CompositionRow) Model() (compositionmodel.Composition, error) {
	var draft map[string]any
	if err := json.Unmarshal(r.Draft, &draft); err != nil {
		return compositionmodel.Composition{}, fmt.Errorf("composition: decode draft: %w", err)
	}
	if draft == nil {
		return compositionmodel.Composition{}, errors.New("composition: stored draft is not a JSON object")
	}
	return compositionmodel.Composition{ID: r.ID, WorkspaceID: r.WorkspaceID, Name: r.Name, Slug: r.Slug,
		Draft: draft, Exposed: r.Exposed, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}, nil
}

func (r VersionRow) Model() (compositionmodel.Version, error) {
	var definition map[string]any
	if err := json.Unmarshal(r.Definition, &definition); err != nil {
		return compositionmodel.Version{}, fmt.Errorf("composition: decode version: %w", err)
	}
	if definition == nil {
		return compositionmodel.Version{}, errors.New("composition: stored version is not a JSON object")
	}
	return compositionmodel.Version{ID: r.ID, CompositionID: r.CompositionID, Version: r.Version,
		Definition: definition, Checksum: r.Checksum, PublishedBy: r.PublishedBy, PublishedAt: r.PublishedAt}, nil
}

func (r ExecutionRow) Model() (compositionmodel.Execution, error) {
	output, err := compositionmodel.RawJSONValue(r.Output)
	if err != nil {
		return compositionmodel.Execution{}, err
	}
	return compositionmodel.Execution{ID: r.ID, CompositionID: r.CompositionID, VersionID: r.VersionID,
		EnvironmentID: r.EnvironmentID, StartedAt: r.StartedAt, DurationMs: r.DurationMs,
		Status: r.Status, StepSummary: r.StepSummary, Output: output,
		Error: r.Error, RequestID: r.RequestID}, nil
}
