// Package overview adapts the workspace overview use case to the HTTP API.
package overview

import (
	"context"

	"github.com/nuriansyah/hugimethara/internal/modules/governance/overview/model"
	"github.com/nuriansyah/hugimethara/internal/modules/governance/overview/services"
	"github.com/nuriansyah/hugimethara/internal/transport/http/dto"
)

// HTTP implements the overview slice of the API.
type HTTP struct{ svc services.Service }

// NewHTTP returns the overview HTTP adapter.
func NewHTTP(svc services.Service) *HTTP { return &HTTP{svc: svc} }

// @Summary Workspace overview (operational KPIs)
// @Description Executions, failures and duration percentiles in the window, connector health from the latest connection test of each instance, and webhook traffic. Each section is present only when the caller can read workflows, connectors or webhooks in the workspace; with none of them the request is refused.
// @ID getOverview
// @Tags overview
// @Param workspace_id query string true "Workspace ID"
// @Param window query string false "24h (default), 7d or 30d"
// @Success 200 {object} dto.Overview
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/overview [get]
func (h *HTTP) GetOverview(ctx context.Context, req dto.GetOverviewRequestObject) (dto.GetOverviewResponseObject, error) {
	var window model.Window
	if req.Params.Window != nil {
		window = model.Window(*req.Params.Window)
	}
	o, err := h.svc.Get(ctx, req.Params.WorkspaceId, window)
	if err != nil {
		return nil, err
	}
	out := dto.Overview{WorkspaceId: o.WorkspaceID, Window: string(o.Window), From: o.From, To: o.To}
	if w := o.Workflows; w != nil {
		ow := &dto.OverviewWorkflows{Total: w.Total, Succeeded: w.Succeeded, Failed: w.Failed, Cancelled: w.Cancelled,
			Running: w.Running, P50Ms: w.P50Ms, P95Ms: w.P95Ms, PendingApprovals: w.PendingApprovals,
			ActiveWorkflows: w.ActiveWorkflows, Series: make([]dto.OverviewSeriesPoint, len(w.Series)),
			TopFailing: make([]dto.OverviewFailingWorkflow, len(w.TopFailing))}
		if finished := w.Succeeded + w.Failed; finished > 0 {
			rate := float64(w.Succeeded) / float64(finished)
			ow.SuccessRate = &rate
		}
		for i, p := range w.Series {
			ow.Series[i] = dto.OverviewSeriesPoint{Bucket: p.Bucket, Succeeded: p.Succeeded, Failed: p.Failed}
		}
		for i, f := range w.TopFailing {
			ow.TopFailing[i] = dto.OverviewFailingWorkflow{WorkflowId: f.WorkflowID, Name: f.Name, Failures: f.Failures,
				LastErrorCode: f.LastErrorCode, LastFailedAt: f.LastFailedAt}
		}
		out.Workflows = ow
	}
	if c := o.Connectors; c != nil {
		oc := &dto.OverviewConnectors{Total: c.Total, Enabled: c.Enabled, Healthy: c.Healthy, Unhealthy: c.Unhealthy,
			Unchecked: c.Unchecked, Failing: make([]dto.OverviewUnhealthyConnector, len(c.Failing))}
		for i, f := range c.Failing {
			oc.Failing[i] = dto.OverviewUnhealthyConnector{ConnectorId: f.ConnectorID, RefKey: f.RefKey,
				DefinitionKey: f.DefinitionKey, EnvironmentId: f.EnvironmentID, ErrorCode: f.ErrorCode, CheckedAt: f.CheckedAt}
		}
		out.Connectors = oc
	}
	if h := o.Webhooks; h != nil {
		out.Webhooks = &dto.OverviewWebhooks{Endpoints: h.Endpoints, Received: h.Received, RejectedTotal: h.RejectedTotal,
			Subscriptions: h.Subscriptions, DeliveriesSucceeded: h.DeliveriesSucceeded,
			DeliveriesPending: h.DeliveriesPending, DeliveriesDeadLettered: h.DeliveriesDeadLetters}
	}
	return dto.GetOverview200JSONResponse(out), nil
}
