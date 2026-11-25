package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/infrastructure/analytics/clickhouse"
	model "github.com/nuriansyah/hugimethara/internal/modules/analytics/model"
	dto "github.com/nuriansyah/hugimethara/internal/modules/analytics/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/analytics/repository"
	"github.com/nuriansyah/hugimethara/internal/modules/analytics/repository/commands"
	"github.com/nuriansyah/hugimethara/internal/modules/analytics/repository/queries"
	authzmodel "github.com/nuriansyah/hugimethara/internal/modules/authorization/model"
	authzsvc "github.com/nuriansyah/hugimethara/internal/modules/authorization/services"
	datamodel "github.com/nuriansyah/hugimethara/internal/modules/data_platform/model"
	dataplatformsvc "github.com/nuriansyah/hugimethara/internal/modules/data_platform/services"
	auditmodel "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/model"
	auditsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/services"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
	"github.com/nuriansyah/hugimethara/internal/platform/sqlguard"
)

var _ Service = (*AnalyticsService)(nil)

type AnalyticsService struct {
	pool *sqlx.DB
	az   authzsvc.Authorizer
	ch   *clickhouse.Client
	dp   *dataplatformsvc.DataPlatformService
}

func NewService(pool *sqlx.DB, az authzsvc.Authorizer, ch *clickhouse.Client, dp *dataplatformsvc.DataPlatformService) (*AnalyticsService, error) {
	if pool == nil {
		return nil, errors.New("analytics: database pool is required")
	}
	if az == nil {
		return nil, errors.New("analytics: authorizer is required")
	}
	if dp == nil {
		return nil, errors.New("analytics: data platform service is required")
	}
	return &AnalyticsService{pool: pool, az: az, ch: ch, dp: dp}, nil
}

func (s *AnalyticsService) principal(ctx context.Context) (principal.Principal, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return principal.Principal{}, authzmodel.Unauthenticated()
	}
	return p, nil
}

func (s *AnalyticsService) tenantTx(ctx context.Context, p principal.Principal, fn func(*sqlx.Tx) error) error {
	return db.InTenantTx(ctx, s.pool, p.OrgID, fn)
}

func (s *AnalyticsService) require(ctx context.Context, p principal.Principal, action, resourceType string, workspaceID *uuid.UUID, id string) error {
	_, err := authzsvc.Require(ctx, s.az, action, authzmodel.Resource{Type: resourceType, ID: id, OrgID: p.OrgID, WorkspaceID: workspaceID})
	return err
}

func (s *AnalyticsService) loadSemanticModel(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (model.SemanticModel, error) {
	var row repository.SemanticModelRow
	if err := tx.GetContext(ctx, &row, queries.SQL.Get("GetSemanticModel"), id); err != nil {
		if db.IsNoRows(err) {
			return model.SemanticModel{}, problem.NotFound("The semantic model does not exist.")
		}
		return model.SemanticModel{}, err
	}
	return row.Model(), nil
}

func (s *AnalyticsService) loadMetric(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (model.Metric, error) {
	var row repository.MetricRow
	if err := tx.GetContext(ctx, &row, queries.SQL.Get("GetMetric"), id); err != nil {
		if db.IsNoRows(err) {
			return model.Metric{}, problem.NotFound("The metric does not exist.")
		}
		return model.Metric{}, err
	}
	return row.Model(), nil
}

func (s *AnalyticsService) ListSemanticModels(ctx context.Context, workspaceID uuid.UUID) ([]model.SemanticModel, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.require(ctx, p, "read", model.ResourceSemanticModel, &workspaceID, ""); err != nil {
		return nil, err
	}
	var rows []repository.SemanticModelRow
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, queries.SQL.Get("ListSemanticModels"), workspaceID)
	}); err != nil {
		return nil, err
	}
	out := make([]model.SemanticModel, len(rows))
	for i := range rows {
		out[i] = rows[i].Model()
	}
	return out, nil
}

func (s *AnalyticsService) CreateSemanticModel(ctx context.Context, in dto.CreateSemanticModelInput) (model.SemanticModel, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.SemanticModel{}, err
	}
	if in.WorkspaceID == uuid.Nil || in.DatasetID == uuid.Nil {
		return model.SemanticModel{}, problem.Validation(problem.FieldError{Field: "workspace_id", Code: "required", Message: "Workspace and dataset are required."})
	}
	if err := s.require(ctx, p, "create", model.ResourceSemanticModel, &in.WorkspaceID, ""); err != nil {
		return model.SemanticModel{}, err
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return model.SemanticModel{}, problem.Validation(problem.FieldError{Field: "name", Code: "required", Message: "A name is required."})
	}
	if in.Slug == "" {
		in.Slug = slugify(in.Name)
	}
	if err := datamodel.ValidateSlug(in.Slug); err != nil {
		return model.SemanticModel{}, problem.Validation(problem.FieldError{Field: "slug", Code: "invalid", Message: err.Error()})
	}
	if err := model.ValidateDimensions(in.Dimensions); err != nil {
		return model.SemanticModel{}, problem.Validation(problem.FieldError{Field: "dimensions", Code: "invalid", Message: err.Error()})
	}
	dataset, err := s.dp.GetDataset(ctx, in.DatasetID)
	if err != nil {
		return model.SemanticModel{}, err
	}
	if dataset.WorkspaceID != in.WorkspaceID {
		return model.SemanticModel{}, problem.Validation(problem.FieldError{Field: "dataset_id", Code: "invalid", Message: "The dataset is in a different workspace."})
	}
	id := uuid.New()
	var out model.SemanticModel
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		dims, _ := json.Marshal(in.Dimensions)
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertSemanticModel"), p.OrgID, id, in.WorkspaceID,
			in.DatasetID, in.Name, in.Slug, in.Description, dims, "active"); err != nil {
			if isUniqueViolation(err) {
				return problem.NewProblem(http.StatusConflict, "slug_taken", "Conflict", "A semantic model with that slug already exists.")
			}
			return err
		}
		row, err := s.loadSemanticModel(ctx, tx, id)
		if err != nil {
			return err
		}
		out = row
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "semantic_model.created",
			ResourceType: model.ResourceSemanticModel, ResourceID: id.String(), WorkspaceID: &in.WorkspaceID,
			Details: map[string]any{"dataset_id": in.DatasetID.String()}})
	}); err != nil {
		return model.SemanticModel{}, err
	}
	return out, nil
}

func (s *AnalyticsService) GetSemanticModel(ctx context.Context, id uuid.UUID) (model.SemanticModel, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.SemanticModel{}, err
	}
	var out model.SemanticModel
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadSemanticModel(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "read", model.ResourceSemanticModel, &row.WorkspaceID, id.String()); err != nil {
			return err
		}
		out = row
		return nil
	}); err != nil {
		return model.SemanticModel{}, err
	}
	return out, nil
}

func (s *AnalyticsService) UpdateSemanticModel(ctx context.Context, id uuid.UUID, in dto.UpdateSemanticModelInput) (model.SemanticModel, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.SemanticModel{}, err
	}
	if in.Dimensions != nil {
		if err := model.ValidateDimensions(*in.Dimensions); err != nil {
			return model.SemanticModel{}, problem.Validation(problem.FieldError{Field: "dimensions", Code: "invalid", Message: err.Error()})
		}
	}
	var out model.SemanticModel
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadSemanticModel(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "update", model.ResourceSemanticModel, &row.WorkspaceID, id.String()); err != nil {
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
		var dims any
		if in.Dimensions != nil {
			raw, _ := json.Marshal(*in.Dimensions)
			dims = json.RawMessage(raw)
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("UpdateSemanticModel"), id, in.Name, in.Slug, in.Description, dims, nil); err != nil {
			if isUniqueViolation(err) {
				return problem.NewProblem(http.StatusConflict, "slug_taken", "Conflict", "A semantic model with that slug already exists.")
			}
			return err
		}
		row, err = s.loadSemanticModel(ctx, tx, id)
		if err != nil {
			return err
		}
		out = row
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "semantic_model.updated",
			ResourceType: model.ResourceSemanticModel, ResourceID: id.String(), WorkspaceID: &row.WorkspaceID})
	}); err != nil {
		return model.SemanticModel{}, err
	}
	return out, nil
}

func (s *AnalyticsService) DeleteSemanticModel(ctx context.Context, id uuid.UUID) error {
	p, err := s.principal(ctx)
	if err != nil {
		return err
	}
	return s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadSemanticModel(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "delete", model.ResourceSemanticModel, &row.WorkspaceID, id.String()); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("DeleteSemanticModel"), id); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "semantic_model.deleted",
			ResourceType: model.ResourceSemanticModel, ResourceID: id.String(), WorkspaceID: &row.WorkspaceID})
	})
}

func (s *AnalyticsService) ListMetrics(ctx context.Context, workspaceID uuid.UUID) ([]model.Metric, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.require(ctx, p, "read", model.ResourceMetric, &workspaceID, ""); err != nil {
		return nil, err
	}
	var rows []repository.MetricRow
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, queries.SQL.Get("ListMetrics"), workspaceID)
	}); err != nil {
		return nil, err
	}
	out := make([]model.Metric, len(rows))
	for i := range rows {
		out[i] = rows[i].Model()
	}
	return out, nil
}

func (s *AnalyticsService) ListMetricsForModel(ctx context.Context, modelID uuid.UUID) ([]model.Metric, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	var out []model.Metric
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		semantic, err := s.loadSemanticModel(ctx, tx, modelID)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "read", model.ResourceSemanticModel, &semantic.WorkspaceID, modelID.String()); err != nil {
			return err
		}
		var rows []repository.MetricRow
		if err := tx.SelectContext(ctx, &rows, queries.SQL.Get("ListMetricsForModel"), modelID); err != nil {
			return err
		}
		out = make([]model.Metric, len(rows))
		for i := range rows {
			out[i] = rows[i].Model()
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *AnalyticsService) CreateMetric(ctx context.Context, in dto.CreateMetricInput) (model.Metric, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Metric{}, err
	}
	if in.WorkspaceID == uuid.Nil || in.SemanticModelID == uuid.Nil {
		return model.Metric{}, problem.Validation(problem.FieldError{Field: "workspace_id", Code: "required", Message: "Workspace and semantic model are required."})
	}
	if err := s.require(ctx, p, "create", model.ResourceMetric, &in.WorkspaceID, ""); err != nil {
		return model.Metric{}, err
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return model.Metric{}, problem.Validation(problem.FieldError{Field: "name", Code: "required", Message: "A name is required."})
	}
	if in.Slug == "" {
		in.Slug = slugify(in.Name)
	}
	if err := datamodel.ValidateSlug(in.Slug); err != nil {
		return model.Metric{}, problem.Validation(problem.FieldError{Field: "slug", Code: "invalid", Message: err.Error()})
	}
	if in.Format == "" {
		in.Format = "number"
	}
	if err := model.ValidateMetricFormat(in.Format); err != nil {
		return model.Metric{}, problem.Validation(problem.FieldError{Field: "format", Code: "invalid", Message: err.Error()})
	}
	if err := s.validateMetric(ctx, in.WorkspaceID, in.SemanticModelID, in.Expression, in.Filters); err != nil {
		return model.Metric{}, err
	}
	id := uuid.New()
	var out model.Metric
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		filters, _ := json.Marshal(in.Filters)
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertMetric"), p.OrgID, id, in.WorkspaceID,
			in.SemanticModelID, in.Name, in.Slug, in.Description, in.Expression, filters, in.Format, "active"); err != nil {
			if isUniqueViolation(err) {
				return problem.NewProblem(http.StatusConflict, "slug_taken", "Conflict", "A metric with that slug already exists.")
			}
			return err
		}
		row, err := s.loadMetric(ctx, tx, id)
		if err != nil {
			return err
		}
		out = row
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "metric.created",
			ResourceType: model.ResourceMetric, ResourceID: id.String(), WorkspaceID: &in.WorkspaceID,
			Details: map[string]any{"semantic_model_id": in.SemanticModelID.String()}})
	}); err != nil {
		return model.Metric{}, err
	}
	return out, nil
}

func (s *AnalyticsService) GetMetric(ctx context.Context, id uuid.UUID) (model.Metric, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Metric{}, err
	}
	var out model.Metric
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadMetric(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "read", model.ResourceMetric, &row.WorkspaceID, id.String()); err != nil {
			return err
		}
		out = row
		return nil
	}); err != nil {
		return model.Metric{}, err
	}
	return out, nil
}

func (s *AnalyticsService) UpdateMetric(ctx context.Context, id uuid.UUID, in dto.UpdateMetricInput) (model.Metric, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Metric{}, err
	}
	if in.Format != nil {
		if err := model.ValidateMetricFormat(*in.Format); err != nil {
			return model.Metric{}, problem.Validation(problem.FieldError{Field: "format", Code: "invalid", Message: err.Error()})
		}
	}
	var out model.Metric
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadMetric(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "update", model.ResourceMetric, &row.WorkspaceID, id.String()); err != nil {
			return err
		}
		expression := row.Expression
		if in.Expression != nil {
			expression = *in.Expression
		}
		filters := row.Filters
		if in.Filters != nil {
			filters = *in.Filters
		}
		if in.Expression != nil || in.Filters != nil {
			if err := s.validateMetric(ctx, row.WorkspaceID, row.SemanticModelID, expression, filters); err != nil {
				return err
			}
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
		var filterArg any
		if in.Filters != nil {
			raw, _ := json.Marshal(*in.Filters)
			filterArg = json.RawMessage(raw)
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("UpdateMetric"), id, in.Name, in.Slug, in.Description,
			in.Expression, filterArg, in.Format, nil); err != nil {
			if isUniqueViolation(err) {
				return problem.NewProblem(http.StatusConflict, "slug_taken", "Conflict", "A metric with that slug already exists.")
			}
			return err
		}
		row, err = s.loadMetric(ctx, tx, id)
		if err != nil {
			return err
		}
		out = row
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "metric.updated",
			ResourceType: model.ResourceMetric, ResourceID: id.String(), WorkspaceID: &row.WorkspaceID})
	}); err != nil {
		return model.Metric{}, err
	}
	return out, nil
}

func (s *AnalyticsService) DeleteMetric(ctx context.Context, id uuid.UUID) error {
	p, err := s.principal(ctx)
	if err != nil {
		return err
	}
	return s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadMetric(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "delete", model.ResourceMetric, &row.WorkspaceID, id.String()); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("DeleteMetric"), id); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "metric.deleted",
			ResourceType: model.ResourceMetric, ResourceID: id.String(), WorkspaceID: &row.WorkspaceID})
	})
}

func (s *AnalyticsService) validateMetric(ctx context.Context, workspaceID, modelID uuid.UUID, expression string, filters []string) error {
	var semantic model.SemanticModel
	if err := s.tenantTx(ctx, s.mustPrincipal(ctx), func(tx *sqlx.Tx) error {
		row, err := s.loadSemanticModel(ctx, tx, modelID)
		if err != nil {
			return err
		}
		semantic = row
		return nil
	}); err != nil {
		return err
	}
	if semantic.WorkspaceID != workspaceID {
		return problem.Validation(problem.FieldError{Field: "semantic_model_id", Code: "invalid", Message: "The semantic model is in a different workspace."})
	}
	dataset, err := s.dp.GetDataset(ctx, semantic.DatasetID)
	if err != nil {
		return err
	}
	view := datamodel.DatasetView(dataset.ID, dataset.Slug)
	for _, filter := range filters {
		if err := sqlguard.ValidateReadOnly("SELECT 1 WHERE " + filter); err != nil {
			return problem.Validation(problem.FieldError{Field: "filters", Code: "invalid", Message: err.Error()})
		}
	}
	if _, err := sqlguard.MetricQuery(view, expression, filters); err != nil {
		return problem.Validation(problem.FieldError{Field: "expression", Code: "invalid", Message: err.Error()})
	}
	return nil
}

func (s *AnalyticsService) mustPrincipal(ctx context.Context) principal.Principal {
	p, _ := principal.From(ctx)
	return p
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
