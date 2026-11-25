package services

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/infrastructure/analytics/clickhouse"
	model "github.com/nuriansyah/hugimethara/internal/modules/analytics/model"
	dto "github.com/nuriansyah/hugimethara/internal/modules/analytics/model/dto"
	datamodel "github.com/nuriansyah/hugimethara/internal/modules/data_platform/model"
	auditmodel "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/model"
	auditsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/services"
	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/platform/sqlguard"
)

func (s *AnalyticsService) EvaluateMetric(ctx context.Context, id uuid.UUID, in dto.MetricQueryInput) (model.MetricValue, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.MetricValue{}, err
	}
	var metric model.Metric
	var semantic model.SemanticModel
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadMetric(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "read", model.ResourceMetric, &row.WorkspaceID, id.String()); err != nil {
			return err
		}
		parent, err := s.loadSemanticModel(ctx, tx, row.SemanticModelID)
		if err != nil {
			return err
		}
		metric, semantic = row, parent
		return nil
	}); err != nil {
		return model.MetricValue{}, err
	}
	dataset, err := s.dp.GetDataset(ctx, semantic.DatasetID)
	if err != nil {
		return model.MetricValue{}, err
	}
	if dataset.Status != "published" {
		return model.MetricValue{}, problem.Validation(problem.FieldError{Field: "metric_id", Code: "not_published", Message: "The metric's dataset is not published."})
	}
	if !s.ch.Enabled() {
		return model.MetricValue{}, problem.NewProblem(http.StatusServiceUnavailable, "analytics_unavailable", "Analytics unavailable", "The analytical store is not configured.")
	}
	filters := append([]string{}, metric.Filters...)
	for _, filter := range in.Filters {
		if err := sqlguard.ValidateReadOnly("SELECT 1 WHERE " + filter); err != nil {
			return model.MetricValue{}, problem.Validation(problem.FieldError{Field: "filters", Code: "invalid", Message: err.Error()})
		}
		filters = append(filters, filter)
	}
	rowFilter, _, err := s.dp.RowFilterFor(ctx, dataset.ID)
	if err != nil {
		return model.MetricValue{}, err
	}
	if rowFilter != "" {
		filters = append(filters, rowFilter)
	}
	view := datamodel.DatasetView(dataset.ID, dataset.Slug)
	query, err := sqlguard.MetricQuery(view, metric.Expression, filters)
	if err != nil {
		return model.MetricValue{}, problem.Validation(problem.FieldError{Field: "expression", Code: "invalid", Message: err.Error()})
	}
	dbName, err := s.ch.EnsureOrgDatabase(ctx, p.OrgID)
	if err != nil {
		return model.MetricValue{}, analyticsError(err)
	}
	runID := uuid.New()
	chID := uuid.NewString()
	if err := s.insertRun(ctx, p, runID, metric.WorkspaceID, &dataset.ID, &metric.ID, query, chID, false); err != nil {
		return model.MetricValue{}, err
	}
	start := time.Now()
	result, err := s.ch.Query(ctx, dbName, chID, query, clickhouse.Limits{MaxExecutionTime: 20 * time.Second, MaxMemoryBytes: 256 << 20, MaxResultRows: 1})
	if err != nil {
		_ = s.finishRun(ctx, p, runID, "failed", 0, 0, 0, nil, nil, err.Error())
		return model.MetricValue{}, problem.Validation(problem.FieldError{Field: "metric", Code: "invalid", Message: err.Error()})
	}
	duration := int(time.Since(start).Milliseconds())
	var value any
	if len(result.Rows) > 0 && len(result.Rows[0]) > 0 {
		value = result.Rows[0][0]
	}
	if err := s.finishRun(ctx, p, runID, "succeeded", result.RowCount, result.Bytes, duration, nil, result.Columns, ""); err != nil {
		return model.MetricValue{}, err
	}
	_ = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "metric.queried",
			ResourceType: model.ResourceMetric, ResourceID: metric.ID.String(), WorkspaceID: &metric.WorkspaceID,
			Details: map[string]any{"run_id": runID.String()}})
	})
	return model.MetricValue{MetricID: metric.ID, Value: value, SQL: query, RunID: runID}, nil
}
