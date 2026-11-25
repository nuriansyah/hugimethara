package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	analyticdto "github.com/nuriansyah/hugimethara/internal/modules/analytics/model/dto"
	analyticssvc "github.com/nuriansyah/hugimethara/internal/modules/analytics/services"
	authzmodel "github.com/nuriansyah/hugimethara/internal/modules/authorization/model"
	authzsvc "github.com/nuriansyah/hugimethara/internal/modules/authorization/services"
	model "github.com/nuriansyah/hugimethara/internal/modules/dashboard/model"
	dto "github.com/nuriansyah/hugimethara/internal/modules/dashboard/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/dashboard/repository"
	"github.com/nuriansyah/hugimethara/internal/modules/dashboard/repository/commands"
	"github.com/nuriansyah/hugimethara/internal/modules/dashboard/repository/queries"
	datamodel "github.com/nuriansyah/hugimethara/internal/modules/data_platform/model"
	auditmodel "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/model"
	auditsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/services"
	platformcron "github.com/nuriansyah/hugimethara/internal/platform/cron"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
	"github.com/nuriansyah/hugimethara/internal/platform/sqlguard"
)

var _ Service = (*DashboardService)(nil)

type DashboardService struct {
	pool      *sqlx.DB
	system    *sqlx.DB
	az        authzsvc.Authorizer
	analytics *analyticssvc.AnalyticsService
}

func NewService(pool, system *sqlx.DB, az authzsvc.Authorizer, analytics *analyticssvc.AnalyticsService) (*DashboardService, error) {
	if pool == nil {
		return nil, errors.New("dashboard: database pool is required")
	}
	if az == nil {
		return nil, errors.New("dashboard: authorizer is required")
	}
	if analytics == nil {
		return nil, errors.New("dashboard: analytics service is required")
	}
	return &DashboardService{pool: pool, system: system, az: az, analytics: analytics}, nil
}

func (s *DashboardService) principal(ctx context.Context) (principal.Principal, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return principal.Principal{}, authzmodel.Unauthenticated()
	}
	return p, nil
}

func (s *DashboardService) tenantTx(ctx context.Context, p principal.Principal, fn func(*sqlx.Tx) error) error {
	return db.InTenantTx(ctx, s.pool, p.OrgID, fn)
}

func (s *DashboardService) require(ctx context.Context, p principal.Principal, action string, workspaceID *uuid.UUID, id string) error {
	_, err := authzsvc.Require(ctx, s.az, action, authzmodel.Resource{Type: model.ResourceDashboard, ID: id, OrgID: p.OrgID, WorkspaceID: workspaceID})
	return err
}

func (s *DashboardService) loadDashboard(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (model.Dashboard, error) {
	var row repository.DashboardRow
	if err := tx.GetContext(ctx, &row, queries.SQL.Get("GetDashboard"), id); err != nil {
		if db.IsNoRows(err) {
			return model.Dashboard{}, problem.NotFound("The dashboard does not exist.")
		}
		return model.Dashboard{}, err
	}
	return row.Model(), nil
}

func (s *DashboardService) ListDashboards(ctx context.Context, workspaceID uuid.UUID) ([]model.Dashboard, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.require(ctx, p, "read", &workspaceID, ""); err != nil {
		return nil, err
	}
	var rows []repository.DashboardRow
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, queries.SQL.Get("ListDashboards"), workspaceID)
	}); err != nil {
		return nil, err
	}
	out := make([]model.Dashboard, len(rows))
	for i := range rows {
		out[i] = rows[i].Model()
	}
	return out, nil
}

func (s *DashboardService) CreateDashboard(ctx context.Context, in dto.CreateDashboardInput) (model.Dashboard, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Dashboard{}, err
	}
	if in.WorkspaceID == uuid.Nil {
		return model.Dashboard{}, problem.Validation(problem.FieldError{Field: "workspace_id", Code: "required", Message: "A workspace is required."})
	}
	if err := s.require(ctx, p, "create", &in.WorkspaceID, ""); err != nil {
		return model.Dashboard{}, err
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return model.Dashboard{}, problem.Validation(problem.FieldError{Field: "name", Code: "required", Message: "A name is required."})
	}
	if in.Slug == "" {
		in.Slug = slugify(in.Name)
	}
	if err := datamodel.ValidateSlug(in.Slug); err != nil {
		return model.Dashboard{}, problem.Validation(problem.FieldError{Field: "slug", Code: "invalid", Message: err.Error()})
	}
	if in.Definition.SchemaVersion == 0 {
		in.Definition = model.Definition{SchemaVersion: 1, Widgets: []model.Widget{}, Filters: []model.Filter{}}
	}
	if err := model.ValidateDefinition(in.Definition); err != nil {
		return model.Dashboard{}, problem.Validation(problem.FieldError{Field: "definition", Code: "invalid", Message: err.Error()})
	}
	id := uuid.New()
	var out model.Dashboard
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		draft, _ := json.Marshal(in.Definition)
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertDashboard"), p.OrgID, id, in.WorkspaceID, in.Name,
			in.Slug, in.Description, draft, "draft", p.ID); err != nil {
			if isUniqueViolation(err) {
				return problem.NewProblem(http.StatusConflict, "slug_taken", "Conflict", "A dashboard with that slug already exists.")
			}
			return err
		}
		row, err := s.loadDashboard(ctx, tx, id)
		if err != nil {
			return err
		}
		out = row
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "dashboard.created",
			ResourceType: model.ResourceDashboard, ResourceID: id.String(), WorkspaceID: &in.WorkspaceID})
	}); err != nil {
		return model.Dashboard{}, err
	}
	return out, nil
}

func (s *DashboardService) GetDashboard(ctx context.Context, id uuid.UUID) (model.Dashboard, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Dashboard{}, err
	}
	var out model.Dashboard
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadDashboard(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "read", &row.WorkspaceID, id.String()); err != nil {
			return err
		}
		out = row
		return nil
	}); err != nil {
		return model.Dashboard{}, err
	}
	return out, nil
}

func (s *DashboardService) UpdateDashboard(ctx context.Context, id uuid.UUID, in dto.UpdateDashboardInput) (model.Dashboard, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Dashboard{}, err
	}
	if in.Definition != nil {
		if err := model.ValidateDefinition(*in.Definition); err != nil {
			return model.Dashboard{}, problem.Validation(problem.FieldError{Field: "definition", Code: "invalid", Message: err.Error()})
		}
	}
	var out model.Dashboard
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadDashboard(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "update", &row.WorkspaceID, id.String()); err != nil {
			return err
		}
		if in.Name != nil {
			name := strings.TrimSpace(*in.Name)
			if name == "" {
				return problem.Validation(problem.FieldError{Field: "name", Code: "required", Message: "A name is required."})
			}
			in.Name = &name
		}
		if in.Slug != nil {
			if err := datamodel.ValidateSlug(*in.Slug); err != nil {
				return problem.Validation(problem.FieldError{Field: "slug", Code: "invalid", Message: err.Error()})
			}
		}
		var draft any
		if in.Definition != nil {
			raw, _ := json.Marshal(*in.Definition)
			draft = json.RawMessage(raw)
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("UpdateDashboard"), id, in.Name, in.Slug, in.Description, draft, nil); err != nil {
			if isUniqueViolation(err) {
				return problem.NewProblem(http.StatusConflict, "slug_taken", "Conflict", "A dashboard with that slug already exists.")
			}
			return err
		}
		row, err = s.loadDashboard(ctx, tx, id)
		if err != nil {
			return err
		}
		out = row
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "dashboard.updated",
			ResourceType: model.ResourceDashboard, ResourceID: id.String(), WorkspaceID: &row.WorkspaceID})
	}); err != nil {
		return model.Dashboard{}, err
	}
	return out, nil
}

func (s *DashboardService) DeleteDashboard(ctx context.Context, id uuid.UUID) error {
	p, err := s.principal(ctx)
	if err != nil {
		return err
	}
	return s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadDashboard(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "delete", &row.WorkspaceID, id.String()); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("DeleteDashboard"), id); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "dashboard.deleted",
			ResourceType: model.ResourceDashboard, ResourceID: id.String(), WorkspaceID: &row.WorkspaceID})
	})
}

func (s *DashboardService) PublishDashboard(ctx context.Context, id uuid.UUID) (model.DashboardVersion, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.DashboardVersion{}, err
	}
	var dashboard model.Dashboard
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadDashboard(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "publish", &row.WorkspaceID, id.String()); err != nil {
			return err
		}
		dashboard = row
		return nil
	}); err != nil {
		return model.DashboardVersion{}, err
	}
	if err := model.ValidateDefinition(dashboard.Draft); err != nil {
		return model.DashboardVersion{}, problem.Validation(problem.FieldError{Field: "definition", Code: "invalid", Message: err.Error()})
	}
	var version model.DashboardVersion
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		number := 1
		var latest repository.DashboardVersionRow
		if err := tx.GetContext(ctx, &latest, queries.SQL.Get("LatestDashboardVersion"), id); err == nil {
			number = latest.Version + 1
		} else if !db.IsNoRows(err) {
			return err
		}
		defRaw, _ := json.Marshal(dashboard.Draft)
		checksum := sqlguard.Checksum(map[string]any{"definition": dashboard.Draft})
		versionID := uuid.New()
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertDashboardVersion"), p.OrgID, versionID, id, number,
			defRaw, checksum, p.ID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("UpdateDashboard"), id, nil, nil, nil, nil, "published"); err != nil {
			return err
		}
		var row repository.DashboardVersionRow
		if err := tx.GetContext(ctx, &row, queries.SQL.Get("LatestDashboardVersion"), id); err != nil {
			return err
		}
		version = row.Model()
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "dashboard.published",
			ResourceType: model.ResourceDashboard, ResourceID: id.String(), WorkspaceID: &dashboard.WorkspaceID,
			Details: map[string]any{"version": number}})
	}); err != nil {
		return model.DashboardVersion{}, err
	}
	return version, nil
}

func (s *DashboardService) ListVersions(ctx context.Context, id uuid.UUID) ([]model.DashboardVersion, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	var out []model.DashboardVersion
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadDashboard(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "read", &row.WorkspaceID, id.String()); err != nil {
			return err
		}
		var rows []repository.DashboardVersionRow
		if err := tx.SelectContext(ctx, &rows, queries.SQL.Get("ListDashboardVersions"), id); err != nil {
			return err
		}
		out = make([]model.DashboardVersion, len(rows))
		for i := range rows {
			out[i] = rows[i].Model()
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *DashboardService) Share(ctx context.Context, id uuid.UUID, in dto.ShareInput) (string, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return "", err
	}
	var token string
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadDashboard(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "share", &row.WorkspaceID, id.String()); err != nil {
			return err
		}
		if !in.Enabled {
			if _, err := tx.ExecContext(ctx, commands.SQL.Get("SetShare"), id, "", false, nil); err != nil {
				return err
			}
			return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "dashboard.share_disabled",
				ResourceType: model.ResourceDashboard, ResourceID: id.String(), WorkspaceID: &row.WorkspaceID})
		}
		random := make([]byte, 32)
		if _, err := rand.Read(random); err != nil {
			return err
		}
		token = base64.RawURLEncoding.EncodeToString([]byte(p.OrgID.String() + "." + id.String() + "." + base64.RawURLEncoding.EncodeToString(random)))
		hash := sha256.Sum256([]byte(token))
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("SetShare"), id, hex.EncodeToString(hash[:]), true, in.ExpiresAt); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "dashboard.shared",
			ResourceType: model.ResourceDashboard, ResourceID: id.String(), WorkspaceID: &row.WorkspaceID,
			Details: map[string]any{"expires_at": in.ExpiresAt}})
	}); err != nil {
		return "", err
	}
	return token, nil
}

func (s *DashboardService) PublicView(ctx context.Context, token string) (PublicDashboard, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return PublicDashboard{}, problem.NotFound("The share link is invalid or expired.")
	}
	parts := strings.Split(string(raw), ".")
	if len(parts) != 3 {
		return PublicDashboard{}, problem.NotFound("The share link is invalid or expired.")
	}
	orgID, err := uuid.Parse(parts[0])
	if err != nil {
		return PublicDashboard{}, problem.NotFound("The share link is invalid or expired.")
	}
	dashboardID, err := uuid.Parse(parts[1])
	if err != nil {
		return PublicDashboard{}, problem.NotFound("The share link is invalid or expired.")
	}
	hash := sha256.Sum256([]byte(token))
	var out PublicDashboard
	if err := db.InTenantTx(ctx, s.pool, orgID, func(tx *sqlx.Tx) error {
		row, err := s.loadDashboard(ctx, tx, dashboardID)
		if err != nil {
			return err
		}
		if !row.ShareEnabled {
			return problem.NotFound("The share link is invalid or expired.")
		}
		if row.ShareExpiresAt != nil && row.ShareExpiresAt.Before(time.Now()) {
			return problem.NotFound("The share link is invalid or expired.")
		}
		var stored string
		if err := tx.GetContext(ctx, &stored, "SELECT share_token_hash FROM dashboards WHERE id = $1", dashboardID); err != nil {
			return err
		}
		if stored == "" || subtle.ConstantTimeCompare([]byte(stored), []byte(hex.EncodeToString(hash[:]))) != 1 {
			return problem.NotFound("The share link is invalid or expired.")
		}
		var version repository.DashboardVersionRow
		if err := tx.GetContext(ctx, &version, queries.SQL.Get("LatestDashboardVersion"), dashboardID); err != nil {
			if db.IsNoRows(err) {
				return problem.NotFound("The dashboard has not been published.")
			}
			return err
		}
		out = PublicDashboard{Dashboard: row, Version: version.Model()}
		return nil
	}); err != nil {
		return PublicDashboard{}, err
	}
	out.Dashboard.ShareEnabled = false
	out.Dashboard.ShareExpiresAt = nil
	return out, nil
}

func (s *DashboardService) WidgetData(ctx context.Context, dashboardID uuid.UUID, widgetID string, filters map[string]string) (WidgetData, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return WidgetData{}, err
	}
	var dashboard model.Dashboard
	var definition model.Definition
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadDashboard(ctx, tx, dashboardID)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "read", &row.WorkspaceID, dashboardID.String()); err != nil {
			return err
		}
		dashboard = row
		var version repository.DashboardVersionRow
		if err := tx.GetContext(ctx, &version, queries.SQL.Get("LatestDashboardVersion"), dashboardID); err == nil {
			definition = version.Model().Definition
		} else if db.IsNoRows(err) {
			definition = row.Draft
		} else {
			return err
		}
		return nil
	}); err != nil {
		return WidgetData{}, err
	}
	var widget *model.Widget
	for i := range definition.Widgets {
		if definition.Widgets[i].ID == widgetID {
			widget = &definition.Widgets[i]
			break
		}
	}
	if widget == nil {
		return WidgetData{}, problem.NotFound("The widget does not exist.")
	}
	out := WidgetData{WidgetID: widget.ID, Type: widget.Type, Title: widget.Title}
	switch widget.Type {
	case "metric":
		value, err := s.analytics.EvaluateMetric(ctx, *widget.MetricID, analyticdto.MetricQueryInput{Filters: filterExpressions(definition, filters)})
		if err != nil {
			return WidgetData{}, err
		}
		out.Data = value
	case "table", "chart":
		sqlText := widget.SQL
		if conditions := filterExpressions(definition, filters); len(conditions) > 0 {
			sqlText = "SELECT * FROM (" + strings.TrimSuffix(strings.TrimSpace(sqlText), ";") + ") AS w WHERE " + strings.Join(conditions, " AND ")
		}
		result, err := s.analytics.Execute(ctx, analyticdto.QueryInput{WorkspaceID: dashboard.WorkspaceID, DatasetID: widget.DatasetID, SQL: sqlText, Limit: 500})
		if err != nil {
			return WidgetData{}, err
		}
		out.Data = result
	case "text":
		out.Data = widget.Config
	}
	return out, nil
}

func filterExpressions(definition model.Definition, values map[string]string) []string {
	var out []string
	for _, filter := range definition.Filters {
		value, ok := values[filter.ID]
		if !ok || value == "" {
			continue
		}
		if err := datamodel.ValidateColumn(filter.Column); err != nil {
			continue
		}
		out = append(out, sqlguard.QuoteIdent(filter.Column)+" = "+sqlguard.QuoteLiteral(value))
	}
	return out
}

func (s *DashboardService) ListReports(ctx context.Context, workspaceID uuid.UUID) ([]model.ScheduledReport, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.require(ctx, p, "read", &workspaceID, ""); err != nil {
		return nil, err
	}
	var rows []repository.ReportRow
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, queries.SQL.Get("ListReports"), workspaceID)
	}); err != nil {
		return nil, err
	}
	out := make([]model.ScheduledReport, len(rows))
	for i := range rows {
		out[i] = rows[i].Model()
	}
	return out, nil
}

func (s *DashboardService) CreateReport(ctx context.Context, in dto.CreateReportInput) (model.ScheduledReport, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.ScheduledReport{}, err
	}
	if in.WorkspaceID == uuid.Nil || in.DashboardID == uuid.Nil {
		return model.ScheduledReport{}, problem.Validation(problem.FieldError{Field: "workspace_id", Code: "required", Message: "Workspace and dashboard are required."})
	}
	if err := s.require(ctx, p, "update", &in.WorkspaceID, in.DashboardID.String()); err != nil {
		return model.ScheduledReport{}, err
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return model.ScheduledReport{}, problem.Validation(problem.FieldError{Field: "name", Code: "required", Message: "A name is required."})
	}
	if _, err := platformcron.Parse(in.ScheduleCron); err != nil {
		return model.ScheduledReport{}, problem.Validation(problem.FieldError{Field: "schedule_cron", Code: "invalid", Message: err.Error()})
	}
	if in.Format == "" {
		in.Format = "csv"
	}
	if in.Format != "csv" && in.Format != "json" {
		return model.ScheduledReport{}, problem.Validation(problem.FieldError{Field: "format", Code: "invalid", Message: "Format must be csv or json."})
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	next := nextRun(in.ScheduleCron, enabled)
	id := uuid.New()
	var out model.ScheduledReport
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		recipients, _ := json.Marshal(in.Recipients)
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertReport"), p.OrgID, id, in.WorkspaceID, in.DashboardID,
			in.Name, in.ScheduleCron, recipients, in.Format, enabled, next); err != nil {
			return err
		}
		row, err := s.loadReport(ctx, tx, id)
		if err != nil {
			return err
		}
		out = row
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "report.created",
			ResourceType: model.ResourceDashboard, ResourceID: id.String(), WorkspaceID: &in.WorkspaceID})
	}); err != nil {
		return model.ScheduledReport{}, err
	}
	return out, nil
}

func (s *DashboardService) UpdateReport(ctx context.Context, id uuid.UUID, in dto.UpdateReportInput) (model.ScheduledReport, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.ScheduledReport{}, err
	}
	var out model.ScheduledReport
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadReport(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "update", &row.WorkspaceID, row.DashboardID.String()); err != nil {
			return err
		}
		if in.Name != nil {
			name := strings.TrimSpace(*in.Name)
			if name == "" {
				return problem.Validation(problem.FieldError{Field: "name", Code: "required", Message: "A name is required."})
			}
			in.Name = &name
		}
		schedule := row.ScheduleCron
		if in.ScheduleCron != nil {
			schedule = *in.ScheduleCron
			if _, err := platformcron.Parse(schedule); err != nil {
				return problem.Validation(problem.FieldError{Field: "schedule_cron", Code: "invalid", Message: err.Error()})
			}
		}
		if in.Format != nil && *in.Format != "csv" && *in.Format != "json" {
			return problem.Validation(problem.FieldError{Field: "format", Code: "invalid", Message: "Format must be csv or json."})
		}
		enabled := row.Enabled
		if in.Enabled != nil {
			enabled = *in.Enabled
		}
		var recipients any
		if in.Recipients != nil {
			raw, _ := json.Marshal(*in.Recipients)
			recipients = json.RawMessage(raw)
		}
		next := nextRun(schedule, enabled)
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("UpdateReport"), id, in.Name, in.ScheduleCron, recipients,
			in.Format, in.Enabled, true, next); err != nil {
			return err
		}
		row, err = s.loadReport(ctx, tx, id)
		if err != nil {
			return err
		}
		out = row
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "report.updated",
			ResourceType: model.ResourceDashboard, ResourceID: id.String(), WorkspaceID: &row.WorkspaceID})
	}); err != nil {
		return model.ScheduledReport{}, err
	}
	return out, nil
}

func (s *DashboardService) DeleteReport(ctx context.Context, id uuid.UUID) error {
	p, err := s.principal(ctx)
	if err != nil {
		return err
	}
	return s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadReport(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "update", &row.WorkspaceID, row.DashboardID.String()); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("DeleteReport"), id); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "report.deleted",
			ResourceType: model.ResourceDashboard, ResourceID: id.String(), WorkspaceID: &row.WorkspaceID})
	})
}

func (s *DashboardService) RunDueReports(ctx context.Context, limit int) (int, error) {
	if s.system == nil {
		return 0, errors.New("dashboard: system pool is required for scheduled reports")
	}
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	var reports []repository.ReportRow
	claimUntil := time.Now().UTC().Add(2 * time.Minute)
	if err := db.InTx(ctx, s.system, func(tx *sqlx.Tx) error {
		if err := tx.SelectContext(ctx, &reports, queries.SQL.Get("DueReports"), limit); err != nil {
			return err
		}
		for _, report := range reports {
			if _, err := tx.ExecContext(ctx, commands.SQL.Get("ClaimReport"), report.ID, claimUntil); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return 0, err
	}
	fired := 0
	for _, row := range reports {
		next := nextRun(row.ScheduleCron, row.Enabled)
		if err := db.InTenantTx(ctx, s.pool, row.OrgID, func(tx *sqlx.Tx) error {
			if _, err := tx.ExecContext(ctx, commands.SQL.Get("FinishReport"), row.ID, next); err != nil {
				return err
			}
			return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: row.OrgID, Action: "report.fired",
				ResourceType: model.ResourceDashboard, ResourceID: row.ID.String(), WorkspaceID: &row.WorkspaceID,
				Actor: &auditmodel.Actor{Type: "system"},
				Details: map[string]any{"dashboard_id": row.DashboardID.String(), "format": row.Format,
					"recipients": len(row.Model().Recipients)}})
		}); err != nil {
			continue
		}
		fired++
	}
	return fired, nil
}

func (s *DashboardService) loadReport(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (model.ScheduledReport, error) {
	var row repository.ReportRow
	if err := tx.GetContext(ctx, &row, queries.SQL.Get("GetReport"), id); err != nil {
		if db.IsNoRows(err) {
			return model.ScheduledReport{}, problem.NotFound("The scheduled report does not exist.")
		}
		return model.ScheduledReport{}, err
	}
	return row.Model(), nil
}

func nextRun(expr string, enabled bool) *time.Time {
	if !enabled || strings.TrimSpace(expr) == "" {
		return nil
	}
	c, err := platformcron.Parse(expr)
	if err != nil {
		return nil
	}
	n := c.Next(time.Now().UTC(), time.UTC)
	if n.IsZero() {
		return nil
	}
	return &n
}

func slugify(name string) string {
	var b strings.Builder
	lastDash := true
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case !lastDash:
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 63 {
		out = out[:63]
	}
	return out
}

func isUniqueViolation(err error) bool {
	return strings.Contains(strings.ToLower(fmt.Sprint(err)), "unique") ||
		strings.Contains(strings.ToLower(fmt.Sprint(err)), "duplicate key")
}
