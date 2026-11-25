// Package dashboard adapts dashboards, shares and scheduled reports to HTTP.
package dashboard

import (
	"context"

	dashboardmodel "github.com/nuriansyah/hugimethara/internal/modules/dashboard/model"
	dtoin "github.com/nuriansyah/hugimethara/internal/modules/dashboard/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/dashboard/services"
	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/transport/http/dto"
)

type HTTP struct{ svc services.Service }

func NewHTTP(svc services.Service) *HTTP { return &HTTP{svc: svc} }

// @Summary list dashboards
// @ID listDashboards
// @Tags dashboards
// @Success 200 {object} dto.ListDashboards200JSONResponse
// @Produce json
// @Param workspace_id query string true "workspace_id"
// @Router /v1/hugimethara/dashboards [get]
func (h *HTTP) ListDashboards(ctx context.Context, req dto.ListDashboardsRequestObject) (dto.ListDashboardsResponseObject, error) {
	rows, err := h.svc.ListDashboards(ctx, req.Params.WorkspaceID)
	if err != nil {
		return nil, err
	}
	return dto.ListDashboards200JSONResponse{Data: dto.WireSlice[dto.Dashboard](rows)}, nil
}

// @Summary create dashboard
// @ID createDashboard
// @Tags dashboards
// @Success 201 {object} dto.CreateDashboard201JSONResponse
// @Produce json
// @Param request body dto.DashboardCreate true "Request body"
// @Router /v1/hugimethara/dashboards [post]
func (h *HTTP) CreateDashboard(ctx context.Context, req dto.CreateDashboardRequestObject) (dto.CreateDashboardResponseObject, error) {
	if req.Body == nil {
		return nil, problem.BadRequest("A JSON body is required.")
	}
	row, err := h.svc.CreateDashboard(ctx, dtoin.CreateDashboardInput{
		WorkspaceID: req.Body.WorkspaceID, Name: req.Body.Name, Slug: req.Body.Slug,
		Description: req.Body.Description, Definition: dto.Wire[dashboardmodel.Definition](req.Body.Definition),
	})
	if err != nil {
		return nil, err
	}
	return dto.CreateDashboard201JSONResponse(dto.Wire[dto.Dashboard](row)), nil
}

// @Summary get dashboard
// @ID getDashboard
// @Tags dashboards
// @Success 200 {object} dto.GetDashboard200JSONResponse
// @Produce json
// @Param dashboard_id path string true "dashboard_id"
// @Router /v1/hugimethara/dashboards/{dashboard_id} [get]
func (h *HTTP) GetDashboard(ctx context.Context, req dto.GetDashboardRequestObject) (dto.GetDashboardResponseObject, error) {
	row, err := h.svc.GetDashboard(ctx, req.DashboardId)
	if err != nil {
		return nil, err
	}
	return dto.GetDashboard200JSONResponse(dto.Wire[dto.Dashboard](row)), nil
}

// @Summary update dashboard
// @ID updateDashboard
// @Tags dashboards
// @Success 200 {object} dto.UpdateDashboard200JSONResponse
// @Produce json
// @Param dashboard_id path string true "dashboard_id"
// @Param request body dto.DashboardUpdate true "Request body"
// @Router /v1/hugimethara/dashboards/{dashboard_id} [patch]
func (h *HTTP) UpdateDashboard(ctx context.Context, req dto.UpdateDashboardRequestObject) (dto.UpdateDashboardResponseObject, error) {
	if req.Body == nil {
		return nil, problem.BadRequest("A JSON body is required.")
	}
	var definition *dashboardmodel.Definition
	if req.Body.Definition != nil {
		converted := dto.Wire[dashboardmodel.Definition](*req.Body.Definition)
		definition = &converted
	}
	row, err := h.svc.UpdateDashboard(ctx, req.DashboardId, dtoin.UpdateDashboardInput{
		Name: req.Body.Name, Slug: req.Body.Slug, Description: req.Body.Description, Definition: definition,
	})
	if err != nil {
		return nil, err
	}
	return dto.UpdateDashboard200JSONResponse(dto.Wire[dto.Dashboard](row)), nil
}

// @Summary delete dashboard
// @ID deleteDashboard
// @Tags dashboards
// @Success 204 "No Content"
// @Produce json
// @Param dashboard_id path string true "dashboard_id"
// @Router /v1/hugimethara/dashboards/{dashboard_id} [delete]
func (h *HTTP) DeleteDashboard(ctx context.Context, req dto.DeleteDashboardRequestObject) (dto.DeleteDashboardResponseObject, error) {
	if err := h.svc.DeleteDashboard(ctx, req.DashboardId); err != nil {
		return nil, err
	}
	return dto.DeleteDashboard204Response{}, nil
}

// @Summary publish dashboard
// @ID publishDashboard
// @Tags dashboards
// @Success 200 {object} dto.PublishDashboard200JSONResponse
// @Produce json
// @Param dashboard_id path string true "dashboard_id"
// @Router /v1/hugimethara/dashboards/{dashboard_id}/publish [post]
func (h *HTTP) PublishDashboard(ctx context.Context, req dto.PublishDashboardRequestObject) (dto.PublishDashboardResponseObject, error) {
	version, err := h.svc.PublishDashboard(ctx, req.DashboardId)
	if err != nil {
		return nil, err
	}
	return dto.PublishDashboard200JSONResponse(dto.Wire[dto.DashboardVersion](version)), nil
}

// @Summary list dashboard versions
// @ID listDashboardVersions
// @Tags dashboards
// @Success 200 {object} dto.ListDashboardVersions200JSONResponse
// @Produce json
// @Param dashboard_id path string true "dashboard_id"
// @Router /v1/hugimethara/dashboards/{dashboard_id}/versions [get]
func (h *HTTP) ListDashboardVersions(ctx context.Context, req dto.ListDashboardVersionsRequestObject) (dto.ListDashboardVersionsResponseObject, error) {
	rows, err := h.svc.ListVersions(ctx, req.DashboardId)
	if err != nil {
		return nil, err
	}
	return dto.ListDashboardVersions200JSONResponse{Data: dto.WireSlice[dto.DashboardVersion](rows)}, nil
}

// @Summary share dashboard
// @ID shareDashboard
// @Tags dashboards
// @Success 200 {object} dto.ShareDashboard200JSONResponse
// @Produce json
// @Param dashboard_id path string true "dashboard_id"
// @Param request body dto.ShareDashboardBody true "Request body"
// @Router /v1/hugimethara/dashboards/{dashboard_id}/share [post]
func (h *HTTP) ShareDashboard(ctx context.Context, req dto.ShareDashboardRequestObject) (dto.ShareDashboardResponseObject, error) {
	if req.Body == nil {
		return nil, problem.BadRequest("A JSON body is required.")
	}
	token, err := h.svc.Share(ctx, req.DashboardId, dtoin.ShareInput{Enabled: req.Body.Enabled, ExpiresAt: req.Body.ExpiresAt})
	if err != nil {
		return nil, err
	}
	return dto.ShareDashboard200JSONResponse(dto.ShareDashboardResult{
		Token: token, Enabled: req.Body.Enabled, ExpiresAt: req.Body.ExpiresAt,
	}), nil
}

// @Summary get dashboard widget data
// @ID getDashboardWidgetData
// @Tags dashboards
// @Success 200 {object} dto.GetDashboardWidgetData200JSONResponse
// @Produce json
// @Param dashboard_id path string true "dashboard_id"
// @Param widget_id path string true "widget_id"
// @Param request body dto.WidgetDataBody true "Request body"
// @Router /v1/hugimethara/dashboards/{dashboard_id}/widgets/{widget_id}/data [post]
func (h *HTTP) GetDashboardWidgetData(ctx context.Context, req dto.GetDashboardWidgetDataRequestObject) (dto.GetDashboardWidgetDataResponseObject, error) {
	filters := map[string]string{}
	if req.Body != nil {
		filters = req.Body.Filters
	}
	data, err := h.svc.WidgetData(ctx, req.DashboardId, req.WidgetId, filters)
	if err != nil {
		return nil, err
	}
	return dto.GetDashboardWidgetData200JSONResponse(dto.WidgetData{
		WidgetID: data.WidgetID, Type: data.Type, Title: data.Title, Data: data.Data,
	}), nil
}

// @Summary get shared dashboard
// @ID getSharedDashboard
// @Tags dashboards
// @Success 200 {object} dto.GetSharedDashboard200JSONResponse
// @Produce json
// @Param token path string true "token"
// @Router /v1/hugimethara/shared-dashboards/{token} [get]
func (h *HTTP) GetSharedDashboard(ctx context.Context, req dto.GetSharedDashboardRequestObject) (dto.GetSharedDashboardResponseObject, error) {
	view, err := h.svc.PublicView(ctx, req.Token)
	if err != nil {
		return nil, err
	}
	return dto.GetSharedDashboard200JSONResponse(dto.PublicDashboard{
		Dashboard: dto.Wire[dto.Dashboard](view.Dashboard), Version: dto.Wire[dto.DashboardVersion](view.Version),
	}), nil
}

// @Summary list scheduled reports
// @ID listScheduledReports
// @Tags dashboards
// @Success 200 {object} dto.ListScheduledReports200JSONResponse
// @Produce json
// @Param workspace_id query string true "workspace_id"
// @Router /v1/hugimethara/scheduled-reports [get]
func (h *HTTP) ListScheduledReports(ctx context.Context, req dto.ListScheduledReportsRequestObject) (dto.ListScheduledReportsResponseObject, error) {
	rows, err := h.svc.ListReports(ctx, req.Params.WorkspaceID)
	if err != nil {
		return nil, err
	}
	return dto.ListScheduledReports200JSONResponse{Data: dto.WireSlice[dto.ScheduledReport](rows)}, nil
}

// @Summary create scheduled report
// @ID createScheduledReport
// @Tags dashboards
// @Success 201 {object} dto.CreateScheduledReport201JSONResponse
// @Produce json
// @Param request body dto.ScheduledReportCreate true "Request body"
// @Router /v1/hugimethara/scheduled-reports [post]
func (h *HTTP) CreateScheduledReport(ctx context.Context, req dto.CreateScheduledReportRequestObject) (dto.CreateScheduledReportResponseObject, error) {
	if req.Body == nil {
		return nil, problem.BadRequest("A JSON body is required.")
	}
	row, err := h.svc.CreateReport(ctx, dtoin.CreateReportInput{
		WorkspaceID: req.Body.WorkspaceID, DashboardID: req.Body.DashboardID, Name: req.Body.Name,
		ScheduleCron: req.Body.ScheduleCron, Recipients: req.Body.Recipients, Format: req.Body.Format, Enabled: req.Body.Enabled,
	})
	if err != nil {
		return nil, err
	}
	return dto.CreateScheduledReport201JSONResponse(dto.Wire[dto.ScheduledReport](row)), nil
}

// @Summary update scheduled report
// @ID updateScheduledReport
// @Tags dashboards
// @Success 200 {object} dto.UpdateScheduledReport200JSONResponse
// @Produce json
// @Param report_id path string true "report_id"
// @Param request body dto.ScheduledReportUpdate true "Request body"
// @Router /v1/hugimethara/scheduled-reports/{report_id} [patch]
func (h *HTTP) UpdateScheduledReport(ctx context.Context, req dto.UpdateScheduledReportRequestObject) (dto.UpdateScheduledReportResponseObject, error) {
	if req.Body == nil {
		return nil, problem.BadRequest("A JSON body is required.")
	}
	row, err := h.svc.UpdateReport(ctx, req.ReportId, dtoin.UpdateReportInput{
		Name: req.Body.Name, ScheduleCron: req.Body.ScheduleCron, Recipients: req.Body.Recipients,
		Format: req.Body.Format, Enabled: req.Body.Enabled,
	})
	if err != nil {
		return nil, err
	}
	return dto.UpdateScheduledReport200JSONResponse(dto.Wire[dto.ScheduledReport](row)), nil
}

// @Summary delete scheduled report
// @ID deleteScheduledReport
// @Tags dashboards
// @Success 204 "No Content"
// @Produce json
// @Param report_id path string true "report_id"
// @Router /v1/hugimethara/scheduled-reports/{report_id} [delete]
func (h *HTTP) DeleteScheduledReport(ctx context.Context, req dto.DeleteScheduledReportRequestObject) (dto.DeleteScheduledReportResponseObject, error) {
	if err := h.svc.DeleteReport(ctx, req.ReportId); err != nil {
		return nil, err
	}
	return dto.DeleteScheduledReport204Response{}, nil
}
