package dto

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// ---- workflow definition (mirrors docs/architecture/workflow-runtime.md §1) ----

type WorkflowDefinition struct {
	SchemaVersion int             `json:"schema_version" binding:"required"`
	Trigger       WorkflowTrigger `json:"trigger" binding:"required"`
	Nodes         []WorkflowNode  `json:"nodes" binding:"required"`
	Edges         []WorkflowEdge  `json:"edges" binding:"required"`
	Limits        *WorkflowLimits `json:"limits,omitempty"`
}

type WorkflowTrigger struct {
	Type   string                 `json:"type" binding:"required"`
	Config map[string]interface{} `json:"config,omitempty"`
}

type WorkflowNode struct {
	Id        string                 `json:"id" binding:"required"`
	Type      string                 `json:"type" binding:"required"`
	Name      *string                `json:"name,omitempty"`
	Config    map[string]interface{} `json:"config,omitempty"`
	Retry     *WorkflowRetry         `json:"retry,omitempty"`
	TimeoutMs *int                   `json:"timeout_ms,omitempty"`
	Position  *WorkflowPosition      `json:"position,omitempty"`
}

type WorkflowRetry struct {
	MaxAttempts int     `json:"max_attempts" binding:"required"`
	Backoff     *string `json:"backoff,omitempty"`
	InitialMs   *int    `json:"initial_ms,omitempty"`
}

type WorkflowPosition struct {
	X float64 `json:"x" binding:"required"`
	Y float64 `json:"y" binding:"required"`
}

type WorkflowEdge struct {
	From string  `json:"from" binding:"required"`
	To   string  `json:"to" binding:"required"`
	Port *string `json:"port,omitempty"`
}

type WorkflowLimits struct {
	MaxSteps                *int `json:"max_steps,omitempty"`
	MaxDurationSeconds      *int `json:"max_duration_seconds,omitempty"`
	MaxConcurrentExecutions *int `json:"max_concurrent_executions,omitempty"`
}

// ---- resources ----

type Workflow struct {
	Id            uuid.UUID          `json:"id" binding:"required"`
	WorkspaceId   uuid.UUID          `json:"workspace_id" binding:"required"`
	Name          string             `json:"name" binding:"required"`
	Slug          string             `json:"slug" binding:"required"`
	Description   string             `json:"description" binding:"required"`
	Draft         WorkflowDefinition `json:"draft" binding:"required"`
	Status        string             `json:"status" binding:"required" enums:"draft,active,paused,archived"`
	LatestVersion *int               `json:"latest_version,omitempty"`
	CreatedAt     time.Time          `json:"created_at" binding:"required"`
	UpdatedAt     time.Time          `json:"updated_at" binding:"required"`
}

type WorkflowCreate struct {
	WorkspaceId uuid.UUID           `json:"workspace_id" binding:"required"`
	Name        string              `json:"name" binding:"required"`
	Slug        string              `json:"slug" binding:"required"`
	Description *string             `json:"description,omitempty"`
	Draft       *WorkflowDefinition `json:"draft,omitempty"`
}

type WorkflowUpdate struct {
	Name        *string             `json:"name,omitempty"`
	Description *string             `json:"description,omitempty"`
	Draft       *WorkflowDefinition `json:"draft,omitempty"`
	Status      *string             `json:"status,omitempty" enums:"active,paused,archived"`
}

type WorkflowIssue struct {
	Path    string `json:"path" binding:"required"`
	Code    string `json:"code" binding:"required"`
	Message string `json:"message" binding:"required"`
}

type WorkflowValidation struct {
	Valid  bool            `json:"valid" binding:"required"`
	Issues []WorkflowIssue `json:"issues" binding:"required"`
}

type WorkflowVersion struct {
	Id          uuid.UUID          `json:"id" binding:"required"`
	WorkflowId  uuid.UUID          `json:"workflow_id" binding:"required"`
	Version     int                `json:"version" binding:"required"`
	Definition  WorkflowDefinition `json:"definition" binding:"required"`
	Checksum    string             `json:"checksum" binding:"required"`
	PublishedBy uuid.UUID          `json:"published_by" binding:"required"`
	PublishedAt time.Time          `json:"published_at" binding:"required"`
}

type WorkflowDeployment struct {
	WorkflowId    uuid.UUID `json:"workflow_id" binding:"required"`
	EnvironmentId uuid.UUID `json:"environment_id" binding:"required"`
	VersionId     uuid.UUID `json:"version_id" binding:"required"`
	Version       int       `json:"version" binding:"required"`
	Enabled       bool      `json:"enabled" binding:"required"`
	UpdatedAt     time.Time `json:"updated_at" binding:"required"`
}

type WorkflowDeploy struct {
	VersionId uuid.UUID `json:"version_id" binding:"required"`
	Enabled   bool      `json:"enabled" binding:"required"`
}

type WorkflowExecute struct {
	EnvironmentId  uuid.UUID              `json:"environment_id" binding:"required"`
	Input          map[string]interface{} `json:"input,omitempty"`
	IdempotencyKey *string                `json:"idempotency_key,omitempty"`
}

type WorkflowError struct {
	Code      string  `json:"code" binding:"required"`
	Message   string  `json:"message" binding:"required"`
	NodeId    *string `json:"node_id,omitempty"`
	Retryable *bool   `json:"retryable,omitempty"`
}

type WorkflowStep struct {
	Id         uuid.UUID      `json:"id" binding:"required"`
	NodeId     string         `json:"node_id" binding:"required"`
	NodeType   string         `json:"node_type" binding:"required"`
	Attempt    int            `json:"attempt" binding:"required"`
	Status     string         `json:"status" binding:"required" enums:"pending,queued,running,waiting,succeeded,failed,skipped,cancelled,retry_scheduled"`
	Input      interface{}    `json:"input,omitempty"`
	Output     interface{}    `json:"output,omitempty"`
	Port       *string        `json:"port,omitempty"`
	Error      *WorkflowError `json:"error,omitempty"`
	QueuedAt   *time.Time     `json:"queued_at,omitempty"`
	StartedAt  *time.Time     `json:"started_at,omitempty"`
	FinishedAt *time.Time     `json:"finished_at,omitempty"`
}

type WorkflowExecution struct {
	Id            uuid.UUID      `json:"id" binding:"required"`
	WorkspaceId   uuid.UUID      `json:"workspace_id" binding:"required"`
	WorkflowId    uuid.UUID      `json:"workflow_id" binding:"required"`
	VersionId     uuid.UUID      `json:"version_id" binding:"required"`
	Version       int            `json:"version" binding:"required"`
	EnvironmentId uuid.UUID      `json:"environment_id" binding:"required"`
	TriggerType   string         `json:"trigger_type" binding:"required"`
	Status        string         `json:"status" binding:"required" enums:"pending,running,succeeded,failed,cancelled,timed_out"`
	Input         interface{}    `json:"input,omitempty"`
	Output        interface{}    `json:"output,omitempty"`
	Error         *WorkflowError `json:"error,omitempty"`
	StepCount     int            `json:"step_count" binding:"required"`
	RerunCount    int            `json:"rerun_count" binding:"required"`
	CreatedAt     time.Time      `json:"created_at" binding:"required"`
	StartedAt     *time.Time     `json:"started_at,omitempty"`
	FinishedAt    *time.Time     `json:"finished_at,omitempty"`
	DeadlineAt    time.Time      `json:"deadline_at" binding:"required"`
	Steps         []WorkflowStep `json:"steps,omitempty"`
}

type NodeTypeField struct {
	Key         string   `json:"key" binding:"required"`
	Label       string   `json:"label" binding:"required"`
	Kind        string   `json:"kind" binding:"required" enums:"string,text,expression,json,select,number,boolean"`
	Required    bool     `json:"required" binding:"required"`
	Options     []string `json:"options,omitempty"`
	Description *string  `json:"description,omitempty"`
}

type NodeType struct {
	Type              string          `json:"type" binding:"required"`
	Kind              string          `json:"kind" binding:"required" enums:"trigger,action,logic,human"`
	Label             string          `json:"label" binding:"required"`
	Description       string          `json:"description" binding:"required"`
	Ports             []string        `json:"ports" binding:"required"`
	Available         bool            `json:"available" binding:"required"`
	UnavailableReason *string         `json:"unavailable_reason,omitempty"`
	Fields            []NodeTypeField `json:"fields" binding:"required"`
}

type Workflows struct {
	Data []Workflow `json:"data" binding:"required"`
}
type WorkflowVersions struct {
	Data []WorkflowVersion `json:"data" binding:"required"`
}
type WorkflowDeployments struct {
	Data []WorkflowDeployment `json:"data" binding:"required"`
}
type WorkflowExecutions struct {
	Data []WorkflowExecution `json:"data" binding:"required"`
}
type NodeTypes struct {
	Data []NodeType `json:"data" binding:"required"`
}

// ---- request objects ----

type ListNodeTypesRequestObject struct{}

type ListWorkflowsParams struct {
	WorkspaceId uuid.UUID `form:"workspace_id" json:"workspace_id"`
}
type ListWorkflowsRequestObject struct{ Params ListWorkflowsParams }
type CreateWorkflowRequestObject struct{ Body *WorkflowCreate }
type GetWorkflowRequestObject struct{ WorkflowId uuid.UUID }
type UpdateWorkflowRequestObject struct {
	WorkflowId uuid.UUID
	Body       *WorkflowUpdate
}
type DeleteWorkflowRequestObject struct{ WorkflowId uuid.UUID }
type ValidateWorkflowRequestObject struct{ WorkflowId uuid.UUID }
type PublishWorkflowRequestObject struct{ WorkflowId uuid.UUID }
type ListWorkflowVersionsRequestObject struct{ WorkflowId uuid.UUID }
type ListWorkflowDeploymentsRequestObject struct{ WorkflowId uuid.UUID }
type DeployWorkflowRequestObject struct {
	WorkflowId    uuid.UUID
	EnvironmentId uuid.UUID
	Body          *WorkflowDeploy
}
type ExecuteWorkflowRequestObject struct {
	WorkflowId uuid.UUID
	Body       *WorkflowExecute
}
type ListWorkflowExecutionsParams struct {
	WorkspaceId uuid.UUID  `form:"workspace_id" json:"workspace_id"`
	WorkflowId  *uuid.UUID `form:"workflow_id,omitempty" json:"workflow_id,omitempty"`
	Status      *string    `form:"status,omitempty" json:"status,omitempty"`
	Limit       *int       `form:"limit,omitempty" json:"limit,omitempty"`
}
type ListWorkflowExecutionsRequestObject struct{ Params ListWorkflowExecutionsParams }
type GetWorkflowExecutionRequestObject struct{ ExecutionId uuid.UUID }

// ---- response objects ----

type ListNodeTypesResponseObject interface {
	VisitListNodeTypesResponse(http.ResponseWriter) error
}
type ListWorkflowsResponseObject interface {
	VisitListWorkflowsResponse(http.ResponseWriter) error
}
type CreateWorkflowResponseObject interface {
	VisitCreateWorkflowResponse(http.ResponseWriter) error
}
type GetWorkflowResponseObject interface {
	VisitGetWorkflowResponse(http.ResponseWriter) error
}
type UpdateWorkflowResponseObject interface {
	VisitUpdateWorkflowResponse(http.ResponseWriter) error
}
type DeleteWorkflowResponseObject interface {
	VisitDeleteWorkflowResponse(http.ResponseWriter) error
}
type ValidateWorkflowResponseObject interface {
	VisitValidateWorkflowResponse(http.ResponseWriter) error
}
type PublishWorkflowResponseObject interface {
	VisitPublishWorkflowResponse(http.ResponseWriter) error
}
type ListWorkflowVersionsResponseObject interface {
	VisitListWorkflowVersionsResponse(http.ResponseWriter) error
}
type ListWorkflowDeploymentsResponseObject interface {
	VisitListWorkflowDeploymentsResponse(http.ResponseWriter) error
}
type DeployWorkflowResponseObject interface {
	VisitDeployWorkflowResponse(http.ResponseWriter) error
}
type ExecuteWorkflowResponseObject interface {
	VisitExecuteWorkflowResponse(http.ResponseWriter) error
}
type ListWorkflowExecutionsResponseObject interface {
	VisitListWorkflowExecutionsResponse(http.ResponseWriter) error
}
type GetWorkflowExecutionResponseObject interface {
	VisitGetWorkflowExecutionResponse(http.ResponseWriter) error
}

type ListNodeTypes200JSONResponse NodeTypes

func (r ListNodeTypes200JSONResponse) VisitListNodeTypesResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 200, r)
}

type ListWorkflows200JSONResponse Workflows

func (r ListWorkflows200JSONResponse) VisitListWorkflowsResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 200, r)
}

type CreateWorkflow201JSONResponse Workflow

func (r CreateWorkflow201JSONResponse) VisitCreateWorkflowResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 201, r)
}

type GetWorkflow200JSONResponse Workflow

func (r GetWorkflow200JSONResponse) VisitGetWorkflowResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 200, r)
}

type UpdateWorkflow200JSONResponse Workflow

func (r UpdateWorkflow200JSONResponse) VisitUpdateWorkflowResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 200, r)
}

type DeleteWorkflow204Response struct{}

func (DeleteWorkflow204Response) VisitDeleteWorkflowResponse(w http.ResponseWriter) error {
	w.WriteHeader(http.StatusNoContent)
	return nil
}

type ValidateWorkflow200JSONResponse WorkflowValidation

func (r ValidateWorkflow200JSONResponse) VisitValidateWorkflowResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 200, r)
}

type PublishWorkflow201JSONResponse WorkflowVersion

func (r PublishWorkflow201JSONResponse) VisitPublishWorkflowResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 201, r)
}

type ListWorkflowVersions200JSONResponse WorkflowVersions

func (r ListWorkflowVersions200JSONResponse) VisitListWorkflowVersionsResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 200, r)
}

type ListWorkflowDeployments200JSONResponse WorkflowDeployments

func (r ListWorkflowDeployments200JSONResponse) VisitListWorkflowDeploymentsResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 200, r)
}

type DeployWorkflow200JSONResponse WorkflowDeployment

func (r DeployWorkflow200JSONResponse) VisitDeployWorkflowResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 200, r)
}

type ExecuteWorkflow202JSONResponse WorkflowExecution

func (r ExecuteWorkflow202JSONResponse) VisitExecuteWorkflowResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 202, r)
}

type ListWorkflowExecutions200JSONResponse WorkflowExecutions

func (r ListWorkflowExecutions200JSONResponse) VisitListWorkflowExecutionsResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 200, r)
}

type GetWorkflowExecution200JSONResponse WorkflowExecution

func (r GetWorkflowExecution200JSONResponse) VisitGetWorkflowExecutionResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 200, r)
}

func writeWorkflowJSON(w http.ResponseWriter, status int, value any) error {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(value); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err := body.WriteTo(w)
	return err
}
