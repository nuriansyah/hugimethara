package model

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/nuriansyah/hugimethara/internal/platform/errors"
)

// ResourceType is the authorization resource owned by this module.
const ResourceType = "workflow"

// Workflow is an editable workflow and its draft definition.
type Workflow struct {
	ID            uuid.UUID
	WorkspaceID   uuid.UUID
	Name          string
	Slug          string
	Description   string
	Draft         Definition
	Status        string
	LatestVersion *int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Version is an immutable published definition.
type Version struct {
	ID          uuid.UUID
	WorkflowID  uuid.UUID
	Version     int
	Definition  Definition
	Checksum    string
	PublishedBy uuid.UUID
	PublishedAt time.Time
}

// Deployment pins a version to an environment.
type Deployment struct {
	WorkflowID    uuid.UUID
	EnvironmentID uuid.UUID
	VersionID     uuid.UUID
	Version       int
	Enabled       bool
	UpdatedAt     time.Time
}

// Execution is one run of a pinned version.
type Execution struct {
	ID            uuid.UUID
	WorkspaceID   uuid.UUID
	WorkflowID    uuid.UUID
	VersionID     uuid.UUID
	Version       int
	EnvironmentID uuid.UUID
	TriggerType   string
	Status        ExecutionStatus
	Input         json.RawMessage
	Output        json.RawMessage
	Error         *StepError
	StepCount     int
	CreatedAt     time.Time
	StartedAt     *time.Time
	FinishedAt    *time.Time
	DeadlineAt    time.Time
	Steps         []Step
}

// Step is one node run inside an execution, with its I/O for inspection.
type Step struct {
	ID         uuid.UUID
	NodeID     string
	NodeType   string
	Attempt    int
	Status     StepStatus
	Input      json.RawMessage
	Output     json.RawMessage
	Port       string
	Error      *StepError
	QueuedAt   *time.Time
	StartedAt  *time.Time
	FinishedAt *time.Time
}

// EmptyDefinition is the draft of a new workflow.
func EmptyDefinition() Definition {
	return Definition{SchemaVersion: SchemaVersion, Trigger: Trigger{Type: TypeTriggerManual},
		Nodes: []Node{}, Edges: []Edge{}}
}

// Problems returned by the workflow use cases.
var (
	ErrWorkflowNotFound  = apperrors.NotFound("Workflow not found.")
	ErrExecutionNotFound = apperrors.NotFound("Workflow execution not found.")
	ErrNotDeployed       = apperrors.NewProblem(http.StatusConflict, "workflow_not_deployed", "Workflow not deployed",
		"This workflow has no enabled deployment in the selected environment.")
	ErrNothingToPublish = apperrors.NewProblem(http.StatusConflict, "no_changes", "Nothing to publish",
		"The draft is identical to the latest published version.")
	ErrSlugTaken = apperrors.NewProblem(http.StatusConflict, "slug_taken", "Address unavailable",
		"Another workflow in this workspace already uses this slug.")
)

// InvalidDefinition wraps validation issues in an RFC 9457 problem.
func InvalidDefinition(issues []Issue) *apperrors.Problem {
	fields := make([]apperrors.FieldError, len(issues))
	for i, is := range issues {
		fields[i] = apperrors.FieldError{Field: is.Path, Code: is.Code, Message: is.Message}
	}
	p := apperrors.NewProblem(http.StatusUnprocessableEntity, "invalid_workflow", "Workflow definition is invalid",
		"Fix the listed issues before publishing.")
	p.Errors = fields
	return p
}
