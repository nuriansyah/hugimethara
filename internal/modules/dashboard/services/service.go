// Package services contains dashboard, share and scheduled-report use cases.
package services

import (
	"context"

	"github.com/google/uuid"

	model "github.com/nuriansyah/hugimethara/internal/modules/dashboard/model"
	dto "github.com/nuriansyah/hugimethara/internal/modules/dashboard/model/dto"
)

type WidgetData struct {
	WidgetID string `json:"widget_id"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	Data     any    `json:"data"`
}

type PublicDashboard struct {
	Dashboard model.Dashboard        `json:"dashboard"`
	Version   model.DashboardVersion `json:"version"`
}

type Service interface {
	ListDashboards(ctx context.Context, workspaceID uuid.UUID) ([]model.Dashboard, error)
	CreateDashboard(ctx context.Context, in dto.CreateDashboardInput) (model.Dashboard, error)
	GetDashboard(ctx context.Context, id uuid.UUID) (model.Dashboard, error)
	UpdateDashboard(ctx context.Context, id uuid.UUID, in dto.UpdateDashboardInput) (model.Dashboard, error)
	DeleteDashboard(ctx context.Context, id uuid.UUID) error
	PublishDashboard(ctx context.Context, id uuid.UUID) (model.DashboardVersion, error)
	ListVersions(ctx context.Context, id uuid.UUID) ([]model.DashboardVersion, error)
	Share(ctx context.Context, id uuid.UUID, in dto.ShareInput) (string, error)
	PublicView(ctx context.Context, token string) (PublicDashboard, error)
	WidgetData(ctx context.Context, dashboardID uuid.UUID, widgetID string, filters map[string]string) (WidgetData, error)

	ListReports(ctx context.Context, workspaceID uuid.UUID) ([]model.ScheduledReport, error)
	CreateReport(ctx context.Context, in dto.CreateReportInput) (model.ScheduledReport, error)
	UpdateReport(ctx context.Context, id uuid.UUID, in dto.UpdateReportInput) (model.ScheduledReport, error)
	DeleteReport(ctx context.Context, id uuid.UUID) error
	RunDueReports(ctx context.Context, limit int) (int, error)
}
