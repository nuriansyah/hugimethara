package dto

import (
	"encoding/json"

	"github.com/google/uuid"

	"github.com/nuriansyah/hugimethara/internal/modules/workflow/model"
)

// CreateInput creates a workflow draft.
type CreateInput struct {
	WorkspaceID uuid.UUID
	Name        string
	Slug        string
	Description string
	Draft       *model.Definition // nil: an empty manual-trigger draft
}

// UpdateInput is a partial update of a workflow; nil fields are unchanged.
type UpdateInput struct {
	Name        *string
	Description *string
	Draft       *model.Definition
	Status      *string // active | paused | archived
}

// DeployInput pins a published version to an environment.
type DeployInput struct {
	VersionID uuid.UUID
	Enabled   bool
}

// ExecuteInput starts a manual execution.
type ExecuteInput struct {
	EnvironmentID  uuid.UUID
	Input          json.RawMessage
	IdempotencyKey string // optional; repeats return the original execution
}

// ExecutionFilter narrows the execution history.
type ExecutionFilter struct {
	WorkspaceID uuid.UUID
	WorkflowID  *uuid.UUID
	Status      string
	Limit       int
}
