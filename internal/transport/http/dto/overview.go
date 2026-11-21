package dto

import (
	"net/http"
	"time"

	"github.com/google/uuid"
)

// ---- S14: workspace overview (operational KPIs) ----

type OverviewSeriesPoint struct {
	Bucket    time.Time `json:"bucket" binding:"required"`
	Succeeded int       `json:"succeeded" binding:"required"`
	Failed    int       `json:"failed" binding:"required"`
}

type OverviewFailingWorkflow struct {
	WorkflowId    uuid.UUID `json:"workflow_id" binding:"required"`
	Name          string    `json:"name" binding:"required"`
	Failures      int       `json:"failures" binding:"required"`
	LastErrorCode string    `json:"last_error_code" binding:"required"`
	LastFailedAt  time.Time `json:"last_failed_at" binding:"required"`
}

type OverviewWorkflows struct {
	Total            int                       `json:"total" binding:"required"`
	Succeeded        int                       `json:"succeeded" binding:"required"`
	Failed           int                       `json:"failed" binding:"required"`
	Cancelled        int                       `json:"cancelled" binding:"required"`
	Running          int                       `json:"running" binding:"required"`
	SuccessRate      *float64                  `json:"success_rate,omitempty"`
	P50Ms            *int                      `json:"p50_ms,omitempty"`
	P95Ms            *int                      `json:"p95_ms,omitempty"`
	PendingApprovals int                       `json:"pending_approvals" binding:"required"`
	ActiveWorkflows  int                       `json:"active_workflows" binding:"required"`
	Series           []OverviewSeriesPoint     `json:"series" binding:"required"`
	TopFailing       []OverviewFailingWorkflow `json:"top_failing" binding:"required"`
}

type OverviewUnhealthyConnector struct {
	ConnectorId   uuid.UUID `json:"connector_id" binding:"required"`
	RefKey        string    `json:"ref_key" binding:"required"`
	DefinitionKey string    `json:"definition_key" binding:"required"`
	EnvironmentId uuid.UUID `json:"environment_id" binding:"required"`
	ErrorCode     string    `json:"error_code" binding:"required"`
	CheckedAt     time.Time `json:"checked_at" binding:"required"`
}

type OverviewConnectors struct {
	Total     int                          `json:"total" binding:"required"`
	Enabled   int                          `json:"enabled" binding:"required"`
	Healthy   int                          `json:"healthy" binding:"required"`
	Unhealthy int                          `json:"unhealthy" binding:"required"`
	Unchecked int                          `json:"unchecked" binding:"required"`
	Failing   []OverviewUnhealthyConnector `json:"failing" binding:"required"`
}

type OverviewWebhooks struct {
	Endpoints              int `json:"endpoints" binding:"required"`
	Received               int `json:"received" binding:"required"`
	RejectedTotal          int `json:"rejected_total" binding:"required"`
	Subscriptions          int `json:"subscriptions" binding:"required"`
	DeliveriesSucceeded    int `json:"deliveries_succeeded" binding:"required"`
	DeliveriesPending      int `json:"deliveries_pending" binding:"required"`
	DeliveriesDeadLettered int `json:"deliveries_dead_lettered" binding:"required"`
}

// Overview is a workspace KPI snapshot. A section is omitted when the caller
// cannot read that resource type in the workspace.
type Overview struct {
	WorkspaceId uuid.UUID           `json:"workspace_id" binding:"required"`
	Window      string              `json:"window" binding:"required" enums:"24h,7d,30d"`
	From        time.Time           `json:"from" binding:"required"`
	To          time.Time           `json:"to" binding:"required"`
	Workflows   *OverviewWorkflows  `json:"workflows,omitempty"`
	Connectors  *OverviewConnectors `json:"connectors,omitempty"`
	Webhooks    *OverviewWebhooks   `json:"webhooks,omitempty"`
}

type GetOverviewParams struct {
	WorkspaceId uuid.UUID `form:"workspace_id" json:"workspace_id"`
	Window      *string   `form:"window,omitempty" json:"window,omitempty"`
}
type GetOverviewRequestObject struct{ Params GetOverviewParams }

type GetOverviewResponseObject interface {
	VisitGetOverviewResponse(http.ResponseWriter) error
}

type GetOverview200JSONResponse Overview

func (r GetOverview200JSONResponse) VisitGetOverviewResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 200, r)
}
