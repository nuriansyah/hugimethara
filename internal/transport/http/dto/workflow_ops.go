package dto

import (
	"net/http"
	"time"

	"github.com/google/uuid"
)

// ---- S11: approvals inbox, schedules, cancel / re-run ----

type ApprovalTaskField struct {
	Key      string `json:"key" binding:"required"`
	Label    string `json:"label" binding:"required"`
	Type     string `json:"type" binding:"required" enums:"string,text,number,boolean"`
	Required bool   `json:"required" binding:"required"`
}

type ApprovalTask struct {
	Id                uuid.UUID           `json:"id" binding:"required"`
	WorkspaceId       uuid.UUID           `json:"workspace_id" binding:"required"`
	WorkflowId        uuid.UUID           `json:"workflow_id" binding:"required"`
	WorkflowName      string              `json:"workflow_name" binding:"required"`
	ExecutionId       uuid.UUID           `json:"execution_id" binding:"required"`
	NodeId            string              `json:"node_id" binding:"required"`
	Kind              string              `json:"kind" binding:"required" enums:"approval,input"`
	Title             string              `json:"title" binding:"required"`
	Instructions      string              `json:"instructions" binding:"required"`
	Context           interface{}         `json:"context,omitempty"`
	Fields            []ApprovalTaskField `json:"fields" binding:"required"`
	AssigneeUserIds   []uuid.UUID         `json:"assignee_user_ids" binding:"required"`
	AssigneeGroupIds  []uuid.UUID         `json:"assignee_group_ids" binding:"required"`
	AllowSelfApproval bool                `json:"allow_self_approval" binding:"required"`
	RequestedBy       *uuid.UUID          `json:"requested_by,omitempty"`
	Status            string              `json:"status" binding:"required" enums:"pending,approved,rejected,submitted,expired,cancelled"`
	Response          interface{}         `json:"response,omitempty"`
	Comment           string              `json:"comment" binding:"required"`
	DecidedBy         *uuid.UUID          `json:"decided_by,omitempty"`
	DecidedAt         *time.Time          `json:"decided_at,omitempty"`
	DueAt             *time.Time          `json:"due_at,omitempty"`
	CreatedAt         time.Time           `json:"created_at" binding:"required"`
	CanDecide         bool                `json:"can_decide" binding:"required"`
}

type ApprovalTasks struct {
	Data []ApprovalTask `json:"data" binding:"required"`
}

type ApprovalDecision struct {
	Decision string                 `json:"decision" binding:"required" enums:"approve,reject,submit"`
	Comment  *string                `json:"comment,omitempty"`
	Data     map[string]interface{} `json:"data,omitempty"`
}

type WorkflowSchedule struct {
	WorkflowId      uuid.UUID  `json:"workflow_id" binding:"required"`
	WorkflowName    string     `json:"workflow_name" binding:"required"`
	WorkflowStatus  string     `json:"workflow_status" binding:"required"`
	EnvironmentId   uuid.UUID  `json:"environment_id" binding:"required"`
	EnvironmentName string     `json:"environment_name" binding:"required"`
	Version         int        `json:"version" binding:"required"`
	Enabled         bool       `json:"enabled" binding:"required"`
	Cron            string     `json:"cron" binding:"required"`
	Timezone        string     `json:"timezone" binding:"required"`
	NextFireAt      *time.Time `json:"next_fire_at,omitempty"`
}

type WorkflowSchedules struct {
	Data []WorkflowSchedule `json:"data" binding:"required"`
}

type CancelWorkflowExecutionRequestObject struct{ ExecutionId uuid.UUID }
type RetryWorkflowExecutionRequestObject struct{ ExecutionId uuid.UUID }

type ListWorkflowSchedulesParams struct {
	WorkspaceId uuid.UUID `form:"workspace_id" json:"workspace_id"`
}
type ListWorkflowSchedulesRequestObject struct{ Params ListWorkflowSchedulesParams }

type ListApprovalTasksParams struct {
	AssignedTo  *string    `form:"assigned_to,omitempty" json:"assigned_to,omitempty"`
	WorkspaceId *uuid.UUID `form:"workspace_id,omitempty" json:"workspace_id,omitempty"`
	ExecutionId *uuid.UUID `form:"execution_id,omitempty" json:"execution_id,omitempty"`
	Status      *string    `form:"status,omitempty" json:"status,omitempty"`
	Limit       *int       `form:"limit,omitempty" json:"limit,omitempty"`
}
type ListApprovalTasksRequestObject struct{ Params ListApprovalTasksParams }
type GetApprovalTaskRequestObject struct{ TaskId uuid.UUID }
type DecideApprovalTaskRequestObject struct {
	TaskId uuid.UUID
	Body   *ApprovalDecision
}

type CancelWorkflowExecutionResponseObject interface {
	VisitCancelWorkflowExecutionResponse(http.ResponseWriter) error
}
type RetryWorkflowExecutionResponseObject interface {
	VisitRetryWorkflowExecutionResponse(http.ResponseWriter) error
}
type ListWorkflowSchedulesResponseObject interface {
	VisitListWorkflowSchedulesResponse(http.ResponseWriter) error
}
type ListApprovalTasksResponseObject interface {
	VisitListApprovalTasksResponse(http.ResponseWriter) error
}
type GetApprovalTaskResponseObject interface {
	VisitGetApprovalTaskResponse(http.ResponseWriter) error
}
type DecideApprovalTaskResponseObject interface {
	VisitDecideApprovalTaskResponse(http.ResponseWriter) error
}

type CancelWorkflowExecution200JSONResponse WorkflowExecution

func (r CancelWorkflowExecution200JSONResponse) VisitCancelWorkflowExecutionResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 200, r)
}

type RetryWorkflowExecution202JSONResponse WorkflowExecution

func (r RetryWorkflowExecution202JSONResponse) VisitRetryWorkflowExecutionResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 202, r)
}

type ListWorkflowSchedules200JSONResponse WorkflowSchedules

func (r ListWorkflowSchedules200JSONResponse) VisitListWorkflowSchedulesResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 200, r)
}

type ListApprovalTasks200JSONResponse ApprovalTasks

func (r ListApprovalTasks200JSONResponse) VisitListApprovalTasksResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 200, r)
}

type GetApprovalTask200JSONResponse ApprovalTask

func (r GetApprovalTask200JSONResponse) VisitGetApprovalTaskResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 200, r)
}

type DecideApprovalTask200JSONResponse ApprovalTask

func (r DecideApprovalTask200JSONResponse) VisitDecideApprovalTaskResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 200, r)
}
