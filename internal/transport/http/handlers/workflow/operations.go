package workflow

import (
	"context"

	"github.com/google/uuid"

	"github.com/nuriansyah/hugimethara/internal/modules/workflow/model"
	wfdto "github.com/nuriansyah/hugimethara/internal/modules/workflow/model/dto"
	apperrors "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/transport/http/dto"
)

func apiTask(t model.ApprovalTask) (dto.ApprovalTask, error) {
	ctxVal, err := decode(t.Context)
	if err != nil {
		return dto.ApprovalTask{}, err
	}
	resp, err := decode(t.Response)
	if err != nil {
		return dto.ApprovalTask{}, err
	}
	out := dto.ApprovalTask{Id: t.ID, WorkspaceId: t.WorkspaceID, WorkflowId: t.WorkflowID,
		WorkflowName: t.WorkflowName, ExecutionId: t.ExecutionID, NodeId: t.NodeID, Kind: t.Kind, Title: t.Title,
		Instructions: t.Instructions, Context: ctxVal, Fields: []dto.ApprovalTaskField{},
		AssigneeUserIds: nonNilIDs(t.Users), AssigneeGroupIds: nonNilIDs(t.Groups),
		AllowSelfApproval: t.AllowSelfApproval, RequestedBy: t.RequestedBy, Status: string(t.Status),
		Response: resp, Comment: t.Comment, DecidedBy: t.DecidedBy, DecidedAt: t.DecidedAt, DueAt: t.DueAt,
		CreatedAt: t.CreatedAt, CanDecide: t.CanDecide}
	for _, f := range t.Fields {
		out.Fields = append(out.Fields, dto.ApprovalTaskField{Key: f.Key, Label: f.Label, Type: f.Type, Required: f.Required})
	}
	return out, nil
}

func nonNilIDs(ids []uuid.UUID) []uuid.UUID {
	if ids == nil {
		return []uuid.UUID{}
	}
	return ids
}

// @Summary Cancel an unfinished execution
// @Description Cancels open steps, pending approval tasks and timers. A step already running on a worker is fenced out: its result is discarded.
// @ID cancelWorkflowExecution
// @Tags workflows
// @Param execution_id path string true "Execution ID"
// @Success 200 {object} dto.WorkflowExecution
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/workflow-executions/{execution_id}/cancel [post]
func (h *HTTP) CancelWorkflowExecution(ctx context.Context, req dto.CancelWorkflowExecutionRequestObject) (dto.CancelWorkflowExecutionResponseObject, error) {
	e, err := h.svc.CancelExecution(ctx, req.ExecutionId)
	if err != nil {
		return nil, err
	}
	out, err := apiExecution(e)
	if err != nil {
		return nil, err
	}
	return dto.CancelWorkflowExecution200JSONResponse(out), nil
}

// @Summary Re-run a failed or timed-out execution from its failed step
// @Description Succeeded steps keep their outputs and are not re-run; the failed step and the steps cancelled because of it run again. The execution deadline restarts.
// @ID retryWorkflowExecution
// @Tags workflows
// @Param execution_id path string true "Execution ID"
// @Success 202 {object} dto.WorkflowExecution
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/workflow-executions/{execution_id}/retry [post]
func (h *HTTP) RetryWorkflowExecution(ctx context.Context, req dto.RetryWorkflowExecutionRequestObject) (dto.RetryWorkflowExecutionResponseObject, error) {
	e, err := h.svc.RerunExecution(ctx, req.ExecutionId)
	if err != nil {
		return nil, err
	}
	out, err := apiExecution(e)
	if err != nil {
		return nil, err
	}
	return dto.RetryWorkflowExecution202JSONResponse(out), nil
}

// @Summary List deployed workflow schedules
// @ID listWorkflowSchedules
// @Tags workflows
// @Param workspace_id query string true "Workspace ID"
// @Success 200 {object} dto.WorkflowSchedules
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/workflow-schedules [get]
func (h *HTTP) ListWorkflowSchedules(ctx context.Context, req dto.ListWorkflowSchedulesRequestObject) (dto.ListWorkflowSchedulesResponseObject, error) {
	rows, err := h.svc.ListSchedules(ctx, req.Params.WorkspaceId)
	if err != nil {
		return nil, err
	}
	out := dto.WorkflowSchedules{Data: make([]dto.WorkflowSchedule, len(rows))}
	for i, s := range rows {
		out.Data[i] = dto.WorkflowSchedule{WorkflowId: s.WorkflowID, WorkflowName: s.WorkflowName,
			WorkflowStatus: s.WorkflowStatus, EnvironmentId: s.EnvironmentID, EnvironmentName: s.EnvironmentName,
			Version: s.Version, Enabled: s.Enabled, Cron: s.Cron, Timezone: s.Timezone, NextFireAt: s.NextFireAt}
	}
	return dto.ListWorkflowSchedules200JSONResponse(out), nil
}

// @Summary Approvals inbox
// @Description assigned_to=me (the default) lists tasks assigned to the caller directly or through a group, in every workspace. assigned_to=any lists a workspace's tasks and needs workflow:read there.
// @ID listApprovalTasks
// @Tags workflows
// @Param assigned_to query string false "me (default) or any"
// @Param workspace_id query string false "Workspace ID (required with assigned_to=any)"
// @Param execution_id query string false "Execution ID"
// @Param status query string false "Task status"
// @Param limit query int false "Page size (max 200)"
// @Success 200 {object} dto.ApprovalTasks
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/approval-tasks [get]
func (h *HTTP) ListApprovalTasks(ctx context.Context, req dto.ListApprovalTasksRequestObject) (dto.ListApprovalTasksResponseObject, error) {
	f := wfdto.TaskFilter{WorkspaceID: req.Params.WorkspaceId, ExecutionID: req.Params.ExecutionId, Mine: true}
	if a := req.Params.AssignedTo; a != nil {
		switch *a {
		case "me":
		case "any":
			f.Mine = false
		default:
			return nil, apperrors.Validation(apperrors.FieldError{Field: "assigned_to", Code: "invalid",
				Message: "assigned_to must be me or any."})
		}
	}
	if req.Params.Status != nil {
		f.Status = *req.Params.Status
	}
	if req.Params.Limit != nil {
		f.Limit = *req.Params.Limit
	}
	rows, err := h.svc.ListTasks(ctx, f)
	if err != nil {
		return nil, err
	}
	out := dto.ApprovalTasks{Data: make([]dto.ApprovalTask, 0, len(rows))}
	for _, t := range rows {
		x, err := apiTask(t)
		if err != nil {
			return nil, err
		}
		out.Data = append(out.Data, x)
	}
	return dto.ListApprovalTasks200JSONResponse(out), nil
}

// @Summary Get an approval task
// @ID getApprovalTask
// @Tags workflows
// @Param task_id path string true "Task ID"
// @Success 200 {object} dto.ApprovalTask
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/approval-tasks/{task_id} [get]
func (h *HTTP) GetApprovalTask(ctx context.Context, req dto.GetApprovalTaskRequestObject) (dto.GetApprovalTaskResponseObject, error) {
	t, err := h.svc.GetTask(ctx, req.TaskId)
	if err != nil {
		return nil, err
	}
	out, err := apiTask(t)
	if err != nil {
		return nil, err
	}
	return dto.GetApprovalTask200JSONResponse(out), nil
}

// @Summary Approve, reject or submit a task
// @Description Only assignees may decide. Unless the node allows it, whoever started the execution cannot approve it (self_approval_forbidden). The waiting step resumes on the approved, rejected or submitted branch.
// @ID decideApprovalTask
// @Tags workflows
// @Param task_id path string true "Task ID"
// @Param body body dto.ApprovalDecision true "Decision"
// @Success 200 {object} dto.ApprovalTask
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/approval-tasks/{task_id}/decide [post]
func (h *HTTP) DecideApprovalTask(ctx context.Context, req dto.DecideApprovalTaskRequestObject) (dto.DecideApprovalTaskResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest("A request body is required.")
	}
	in := wfdto.DecideInput{Decision: req.Body.Decision, Data: req.Body.Data}
	if req.Body.Comment != nil {
		in.Comment = *req.Body.Comment
	}
	t, err := h.svc.DecideTask(ctx, req.TaskId, in)
	if err != nil {
		return nil, err
	}
	out, err := apiTask(t)
	if err != nil {
		return nil, err
	}
	return dto.DecideApprovalTask200JSONResponse(out), nil
}
