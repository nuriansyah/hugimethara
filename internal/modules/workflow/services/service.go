package services

import (
	"context"

	"github.com/google/uuid"

	"github.com/nuriansyah/hugimethara/internal/modules/workflow/model"
	"github.com/nuriansyah/hugimethara/internal/modules/workflow/model/dto"
)

// DefinitionUseCases manage workflows and their drafts.
type DefinitionUseCases interface {
	NodeTypes(ctx context.Context) ([]model.NodeType, error)
	List(ctx context.Context, workspaceID uuid.UUID) ([]model.Workflow, error)
	Get(ctx context.Context, id uuid.UUID) (model.Workflow, error)
	Create(ctx context.Context, in dto.CreateInput) (model.Workflow, error)
	Update(ctx context.Context, id uuid.UUID, in dto.UpdateInput) (model.Workflow, error)
	Delete(ctx context.Context, id uuid.UUID) error
	// Validate returns every issue in the draft; empty means publishable.
	Validate(ctx context.Context, id uuid.UUID) ([]model.Issue, error)
}

// ReleaseUseCases freeze drafts into versions and pin them to environments.
type ReleaseUseCases interface {
	Publish(ctx context.Context, id uuid.UUID) (model.Version, error)
	ListVersions(ctx context.Context, id uuid.UUID) ([]model.Version, error)
	Deploy(ctx context.Context, id, environmentID uuid.UUID, in dto.DeployInput) (model.Deployment, error)
	ListDeployments(ctx context.Context, id uuid.UUID) ([]model.Deployment, error)
}

// ExecutionUseCases start executions and inspect their history.
type ExecutionUseCases interface {
	Execute(ctx context.Context, id uuid.UUID, in dto.ExecuteInput) (model.Execution, error)
	ListExecutions(ctx context.Context, f dto.ExecutionFilter) ([]model.Execution, error)
	GetExecution(ctx context.Context, executionID uuid.UUID) (model.Execution, error)
	// CancelExecution stops an unfinished execution: open steps, pending tasks
	// and timers are cancelled; a running step's worker is fenced out.
	CancelExecution(ctx context.Context, executionID uuid.UUID) (model.Execution, error)
	// RerunExecution re-runs a failed or timed-out execution from its failed
	// step(s), keeping every succeeded step's output.
	RerunExecution(ctx context.Context, executionID uuid.UUID) (model.Execution, error)
	ListSchedules(ctx context.Context, workspaceID uuid.UUID) ([]model.Schedule, error)
}

// TaskUseCases are the approvals inbox: human tasks and their decisions.
type TaskUseCases interface {
	ListTasks(ctx context.Context, f dto.TaskFilter) ([]model.ApprovalTask, error)
	GetTask(ctx context.Context, id uuid.UUID) (model.ApprovalTask, error)
	DecideTask(ctx context.Context, id uuid.UUID, in dto.DecideInput) (model.ApprovalTask, error)
}

// Service is the complete control-plane surface of the workflow module.
type Service interface {
	DefinitionUseCases
	ReleaseUseCases
	ExecutionUseCases
	TaskUseCases
}

// StepHandler is the data-plane port the worker's message consumer drives.
type StepHandler interface {
	HandleStep(ctx context.Context, msg StepMessage) error
}

var (
	_ Service     = (*WorkflowService)(nil)
	_ StepHandler = (*Engine)(nil)
)
