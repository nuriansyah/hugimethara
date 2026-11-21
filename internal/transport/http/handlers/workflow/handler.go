// Package workflow adapts the workflow use cases to the HTTP API: it decodes
// wire DTOs, calls the module's services port and encodes responses. It holds
// no business rules.
package workflow

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nuriansyah/hugimethara/internal/modules/workflow/model"
	wfdto "github.com/nuriansyah/hugimethara/internal/modules/workflow/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/workflow/services"
	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/transport/http/dto"
)

// HTTP implements the workflow slice of the API.
type HTTP struct{ svc services.Service }

// NewHTTP returns the workflow HTTP adapter.
func NewHTTP(svc services.Service) *HTTP { return &HTTP{svc: svc} }

// definition converts between the wire and model definitions. They share one
// JSON shape by design, so a JSON round trip is the exact mapping.
func toModelDefinition(in dto.WorkflowDefinition) (model.Definition, error) {
	var out model.Definition
	raw, err := json.Marshal(in)
	if err == nil {
		err = json.Unmarshal(raw, &out)
	}
	if err != nil {
		return out, problem.Validation(problem.FieldError{Field: "draft", Code: "invalid", Message: "The definition is malformed."})
	}
	return out, nil
}

func toWireDefinition(in model.Definition) dto.WorkflowDefinition {
	var out dto.WorkflowDefinition
	raw, _ := json.Marshal(in)
	_ = json.Unmarshal(raw, &out)
	if out.Nodes == nil {
		out.Nodes = []dto.WorkflowNode{}
	}
	if out.Edges == nil {
		out.Edges = []dto.WorkflowEdge{}
	}
	return out
}

func apiWorkflow(w model.Workflow) dto.Workflow {
	return dto.Workflow{Id: w.ID, WorkspaceId: w.WorkspaceID, Name: w.Name, Slug: w.Slug,
		Description: w.Description, Draft: toWireDefinition(w.Draft), Status: w.Status,
		LatestVersion: w.LatestVersion, CreatedAt: w.CreatedAt, UpdatedAt: w.UpdatedAt}
}

func apiVersion(v model.Version) dto.WorkflowVersion {
	return dto.WorkflowVersion{Id: v.ID, WorkflowId: v.WorkflowID, Version: v.Version,
		Definition: toWireDefinition(v.Definition), Checksum: v.Checksum, PublishedBy: v.PublishedBy,
		PublishedAt: v.PublishedAt}
}

func apiDeployment(d model.Deployment) dto.WorkflowDeployment {
	return dto.WorkflowDeployment{WorkflowId: d.WorkflowID, EnvironmentId: d.EnvironmentID,
		VersionId: d.VersionID, Version: d.Version, Enabled: d.Enabled, UpdatedAt: d.UpdatedAt}
}

func apiError(e *model.StepError) *dto.WorkflowError {
	if e == nil {
		return nil
	}
	out := &dto.WorkflowError{Code: e.Code, Message: e.Message}
	if e.NodeID != "" {
		out.NodeId = &e.NodeID
	}
	if e.Retryable {
		out.Retryable = &e.Retryable
	}
	return out
}

func decode(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("workflow: decode stored payload: %w", err)
	}
	return v, nil
}

func apiExecution(e model.Execution) (dto.WorkflowExecution, error) {
	in, err := decode(e.Input)
	if err != nil {
		return dto.WorkflowExecution{}, err
	}
	out, err := decode(e.Output)
	if err != nil {
		return dto.WorkflowExecution{}, err
	}
	res := dto.WorkflowExecution{Id: e.ID, WorkspaceId: e.WorkspaceID, WorkflowId: e.WorkflowID,
		VersionId: e.VersionID, Version: e.Version, EnvironmentId: e.EnvironmentID, TriggerType: e.TriggerType,
		Status: string(e.Status), Input: in, Output: out, Error: apiError(e.Error), StepCount: e.StepCount,
		RerunCount: e.RerunCount,
		CreatedAt:  e.CreatedAt, StartedAt: e.StartedAt, FinishedAt: e.FinishedAt, DeadlineAt: e.DeadlineAt}
	for _, s := range e.Steps {
		sin, err := decode(s.Input)
		if err != nil {
			return dto.WorkflowExecution{}, err
		}
		sout, err := decode(s.Output)
		if err != nil {
			return dto.WorkflowExecution{}, err
		}
		step := dto.WorkflowStep{Id: s.ID, NodeId: s.NodeID, NodeType: s.NodeType, Attempt: s.Attempt,
			Status: string(s.Status), Input: sin, Output: sout, Error: apiError(s.Error), QueuedAt: s.QueuedAt,
			StartedAt: s.StartedAt, FinishedAt: s.FinishedAt}
		if s.Port != "" {
			port := s.Port
			step.Port = &port
		}
		res.Steps = append(res.Steps, step)
	}
	return res, nil
}

func apiNodeType(t model.NodeType) dto.NodeType {
	out := dto.NodeType{Type: t.Type, Kind: string(t.Kind), Label: t.Label, Description: t.Description,
		Ports: t.Ports, Available: t.Available, Fields: []dto.NodeTypeField{}}
	if out.Ports == nil {
		out.Ports = []string{}
	}
	if t.Unavailable != "" {
		r := t.Unavailable
		out.UnavailableReason = &r
	}
	for _, f := range t.Fields {
		field := dto.NodeTypeField{Key: f.Key, Label: f.Label, Kind: f.Kind, Required: f.Required, Options: f.Options}
		if f.Description != "" {
			d := f.Description
			field.Description = &d
		}
		out.Fields = append(out.Fields, field)
	}
	return out
}

// @Summary Workflow node-type catalog
// @ID listNodeTypes
// @Tags workflows
// @Success 200 {object} dto.NodeTypes
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/node-types [get]
func (h *HTTP) ListNodeTypes(ctx context.Context, _ dto.ListNodeTypesRequestObject) (dto.ListNodeTypesResponseObject, error) {
	types, err := h.svc.NodeTypes(ctx)
	if err != nil {
		return nil, err
	}
	out := dto.NodeTypes{Data: make([]dto.NodeType, len(types))}
	for i, t := range types {
		out.Data[i] = apiNodeType(t)
	}
	return dto.ListNodeTypes200JSONResponse(out), nil
}

// @Summary List workflows
// @ID listWorkflows
// @Tags workflows
// @Param workspace_id query string true "Workspace ID"
// @Success 200 {object} dto.Workflows
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/workflows [get]
func (h *HTTP) ListWorkflows(ctx context.Context, req dto.ListWorkflowsRequestObject) (dto.ListWorkflowsResponseObject, error) {
	rows, err := h.svc.List(ctx, req.Params.WorkspaceId)
	if err != nil {
		return nil, err
	}
	out := dto.Workflows{Data: make([]dto.Workflow, len(rows))}
	for i, w := range rows {
		out.Data[i] = apiWorkflow(w)
	}
	return dto.ListWorkflows200JSONResponse(out), nil
}

// @Summary Create a workflow draft
// @ID createWorkflow
// @Tags workflows
// @Param body body dto.WorkflowCreate true "Workflow"
// @Success 201 {object} dto.Workflow
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/workflows [post]
func (h *HTTP) CreateWorkflow(ctx context.Context, req dto.CreateWorkflowRequestObject) (dto.CreateWorkflowResponseObject, error) {
	if req.Body == nil {
		return nil, problem.BadRequest("A request body is required.")
	}
	in := wfdto.CreateInput{WorkspaceID: req.Body.WorkspaceId, Name: req.Body.Name, Slug: req.Body.Slug}
	if req.Body.Description != nil {
		in.Description = *req.Body.Description
	}
	if req.Body.Draft != nil {
		d, err := toModelDefinition(*req.Body.Draft)
		if err != nil {
			return nil, err
		}
		in.Draft = &d
	}
	w, err := h.svc.Create(ctx, in)
	if err != nil {
		return nil, err
	}
	return dto.CreateWorkflow201JSONResponse(apiWorkflow(w)), nil
}

// @Summary Get a workflow
// @ID getWorkflow
// @Tags workflows
// @Param workflow_id path string true "Workflow ID"
// @Success 200 {object} dto.Workflow
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/workflows/{workflow_id} [get]
func (h *HTTP) GetWorkflow(ctx context.Context, req dto.GetWorkflowRequestObject) (dto.GetWorkflowResponseObject, error) {
	w, err := h.svc.Get(ctx, req.WorkflowId)
	if err != nil {
		return nil, err
	}
	return dto.GetWorkflow200JSONResponse(apiWorkflow(w)), nil
}

// @Summary Update a workflow (name, description, draft, status)
// @ID updateWorkflow
// @Tags workflows
// @Param workflow_id path string true "Workflow ID"
// @Param body body dto.WorkflowUpdate true "Changes"
// @Success 200 {object} dto.Workflow
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/workflows/{workflow_id} [patch]
func (h *HTTP) UpdateWorkflow(ctx context.Context, req dto.UpdateWorkflowRequestObject) (dto.UpdateWorkflowResponseObject, error) {
	if req.Body == nil {
		return nil, problem.BadRequest("A request body is required.")
	}
	in := wfdto.UpdateInput{Name: req.Body.Name, Description: req.Body.Description, Status: req.Body.Status}
	if req.Body.Draft != nil {
		d, err := toModelDefinition(*req.Body.Draft)
		if err != nil {
			return nil, err
		}
		in.Draft = &d
	}
	w, err := h.svc.Update(ctx, req.WorkflowId, in)
	if err != nil {
		return nil, err
	}
	return dto.UpdateWorkflow200JSONResponse(apiWorkflow(w)), nil
}

// @Summary Delete a workflow and its history
// @ID deleteWorkflow
// @Tags workflows
// @Param workflow_id path string true "Workflow ID"
// @Success 204 "Deleted."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/workflows/{workflow_id} [delete]
func (h *HTTP) DeleteWorkflow(ctx context.Context, req dto.DeleteWorkflowRequestObject) (dto.DeleteWorkflowResponseObject, error) {
	if err := h.svc.Delete(ctx, req.WorkflowId); err != nil {
		return nil, err
	}
	return dto.DeleteWorkflow204Response{}, nil
}

// @Summary Validate the draft (returns every issue)
// @ID validateWorkflow
// @Tags workflows
// @Param workflow_id path string true "Workflow ID"
// @Success 200 {object} dto.WorkflowValidation
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/workflows/{workflow_id}/validate [post]
func (h *HTTP) ValidateWorkflow(ctx context.Context, req dto.ValidateWorkflowRequestObject) (dto.ValidateWorkflowResponseObject, error) {
	issues, err := h.svc.Validate(ctx, req.WorkflowId)
	if err != nil {
		return nil, err
	}
	out := dto.WorkflowValidation{Valid: len(issues) == 0, Issues: make([]dto.WorkflowIssue, len(issues))}
	for i, is := range issues {
		out.Issues[i] = dto.WorkflowIssue{Path: is.Path, Code: is.Code, Message: is.Message}
	}
	return dto.ValidateWorkflow200JSONResponse(out), nil
}

// @Summary Publish the draft as an immutable version
// @ID publishWorkflow
// @Tags workflows
// @Param workflow_id path string true "Workflow ID"
// @Success 201 {object} dto.WorkflowVersion
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/workflows/{workflow_id}/publish [post]
func (h *HTTP) PublishWorkflow(ctx context.Context, req dto.PublishWorkflowRequestObject) (dto.PublishWorkflowResponseObject, error) {
	v, err := h.svc.Publish(ctx, req.WorkflowId)
	if err != nil {
		return nil, err
	}
	return dto.PublishWorkflow201JSONResponse(apiVersion(v)), nil
}

// @Summary List published versions
// @ID listWorkflowVersions
// @Tags workflows
// @Param workflow_id path string true "Workflow ID"
// @Success 200 {object} dto.WorkflowVersions
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/workflows/{workflow_id}/versions [get]
func (h *HTTP) ListWorkflowVersions(ctx context.Context, req dto.ListWorkflowVersionsRequestObject) (dto.ListWorkflowVersionsResponseObject, error) {
	rows, err := h.svc.ListVersions(ctx, req.WorkflowId)
	if err != nil {
		return nil, err
	}
	out := dto.WorkflowVersions{Data: make([]dto.WorkflowVersion, len(rows))}
	for i, v := range rows {
		out.Data[i] = apiVersion(v)
	}
	return dto.ListWorkflowVersions200JSONResponse(out), nil
}

// @Summary List deployments per environment
// @ID listWorkflowDeployments
// @Tags workflows
// @Param workflow_id path string true "Workflow ID"
// @Success 200 {object} dto.WorkflowDeployments
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/workflows/{workflow_id}/deployments [get]
func (h *HTTP) ListWorkflowDeployments(ctx context.Context, req dto.ListWorkflowDeploymentsRequestObject) (dto.ListWorkflowDeploymentsResponseObject, error) {
	rows, err := h.svc.ListDeployments(ctx, req.WorkflowId)
	if err != nil {
		return nil, err
	}
	out := dto.WorkflowDeployments{Data: make([]dto.WorkflowDeployment, len(rows))}
	for i, d := range rows {
		out.Data[i] = apiDeployment(d)
	}
	return dto.ListWorkflowDeployments200JSONResponse(out), nil
}

// @Summary Deploy a version to an environment
// @ID deployWorkflow
// @Tags workflows
// @Param workflow_id path string true "Workflow ID"
// @Param environment_id path string true "Environment ID"
// @Param body body dto.WorkflowDeploy true "Deployment"
// @Success 200 {object} dto.WorkflowDeployment
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/workflows/{workflow_id}/deployments/{environment_id} [put]
func (h *HTTP) DeployWorkflow(ctx context.Context, req dto.DeployWorkflowRequestObject) (dto.DeployWorkflowResponseObject, error) {
	if req.Body == nil {
		return nil, problem.BadRequest("A request body is required.")
	}
	d, err := h.svc.Deploy(ctx, req.WorkflowId, req.EnvironmentId,
		wfdto.DeployInput{VersionID: req.Body.VersionId, Enabled: req.Body.Enabled})
	if err != nil {
		return nil, err
	}
	return dto.DeployWorkflow200JSONResponse(apiDeployment(d)), nil
}

// @Summary Start a manual execution (asynchronous)
// @ID executeWorkflow
// @Tags workflows
// @Param workflow_id path string true "Workflow ID"
// @Param body body dto.WorkflowExecute true "Execution request"
// @Success 202 {object} dto.WorkflowExecution
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/workflows/{workflow_id}/execute [post]
func (h *HTTP) ExecuteWorkflow(ctx context.Context, req dto.ExecuteWorkflowRequestObject) (dto.ExecuteWorkflowResponseObject, error) {
	if req.Body == nil {
		return nil, problem.BadRequest("A request body is required.")
	}
	in := wfdto.ExecuteInput{EnvironmentID: req.Body.EnvironmentId}
	if req.Body.Input != nil {
		raw, err := json.Marshal(req.Body.Input)
		if err != nil {
			return nil, problem.BadRequest("input must be a JSON object.")
		}
		in.Input = raw
	}
	if req.Body.IdempotencyKey != nil {
		in.IdempotencyKey = *req.Body.IdempotencyKey
	}
	e, err := h.svc.Execute(ctx, req.WorkflowId, in)
	if err != nil {
		return nil, err
	}
	out, err := apiExecution(e)
	if err != nil {
		return nil, err
	}
	return dto.ExecuteWorkflow202JSONResponse(out), nil
}

// @Summary Workflow execution history
// @ID listWorkflowExecutions
// @Tags workflows
// @Param workspace_id query string true "Workspace ID"
// @Param workflow_id query string false "Workflow ID"
// @Param status query string false "Execution status"
// @Param limit query int false "Page size (max 200)"
// @Success 200 {object} dto.WorkflowExecutions
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/workflow-executions [get]
func (h *HTTP) ListWorkflowExecutions(ctx context.Context, req dto.ListWorkflowExecutionsRequestObject) (dto.ListWorkflowExecutionsResponseObject, error) {
	f := wfdto.ExecutionFilter{WorkspaceID: req.Params.WorkspaceId, WorkflowID: req.Params.WorkflowId}
	if req.Params.Status != nil {
		f.Status = *req.Params.Status
	}
	if req.Params.Limit != nil {
		f.Limit = *req.Params.Limit
	}
	rows, err := h.svc.ListExecutions(ctx, f)
	if err != nil {
		return nil, err
	}
	out := dto.WorkflowExecutions{Data: make([]dto.WorkflowExecution, 0, len(rows))}
	for _, e := range rows {
		x, err := apiExecution(e)
		if err != nil {
			return nil, err
		}
		out.Data = append(out.Data, x)
	}
	return dto.ListWorkflowExecutions200JSONResponse(out), nil
}

// @Summary Get an execution with every step's input, output and error
// @ID getWorkflowExecution
// @Tags workflows
// @Param execution_id path string true "Execution ID"
// @Success 200 {object} dto.WorkflowExecution
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/workflow-executions/{execution_id} [get]
func (h *HTTP) GetWorkflowExecution(ctx context.Context, req dto.GetWorkflowExecutionRequestObject) (dto.GetWorkflowExecutionResponseObject, error) {
	e, err := h.svc.GetExecution(ctx, req.ExecutionId)
	if err != nil {
		return nil, err
	}
	out, err := apiExecution(e)
	if err != nil {
		return nil, err
	}
	return dto.GetWorkflowExecution200JSONResponse(out), nil
}
