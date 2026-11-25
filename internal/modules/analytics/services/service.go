// Package services contains analytics use cases: the semantic layer, metric
// evaluation and the governed SQL workbench.
package services

import (
	"context"

	"github.com/google/uuid"

	model "github.com/nuriansyah/hugimethara/internal/modules/analytics/model"
	dto "github.com/nuriansyah/hugimethara/internal/modules/analytics/model/dto"
)

type Service interface {
	ListSemanticModels(ctx context.Context, workspaceID uuid.UUID) ([]model.SemanticModel, error)
	CreateSemanticModel(ctx context.Context, in dto.CreateSemanticModelInput) (model.SemanticModel, error)
	GetSemanticModel(ctx context.Context, id uuid.UUID) (model.SemanticModel, error)
	UpdateSemanticModel(ctx context.Context, id uuid.UUID, in dto.UpdateSemanticModelInput) (model.SemanticModel, error)
	DeleteSemanticModel(ctx context.Context, id uuid.UUID) error

	ListMetrics(ctx context.Context, workspaceID uuid.UUID) ([]model.Metric, error)
	ListMetricsForModel(ctx context.Context, modelID uuid.UUID) ([]model.Metric, error)
	CreateMetric(ctx context.Context, in dto.CreateMetricInput) (model.Metric, error)
	GetMetric(ctx context.Context, id uuid.UUID) (model.Metric, error)
	UpdateMetric(ctx context.Context, id uuid.UUID, in dto.UpdateMetricInput) (model.Metric, error)
	DeleteMetric(ctx context.Context, id uuid.UUID) error
	EvaluateMetric(ctx context.Context, id uuid.UUID, in dto.MetricQueryInput) (model.MetricValue, error)

	Execute(ctx context.Context, in dto.QueryInput) (model.QueryResult, error)
	Cancel(ctx context.Context, runID uuid.UUID) error
	ExportCSV(ctx context.Context, runID uuid.UUID) ([]byte, error)
	ListQueryRuns(ctx context.Context, workspaceID uuid.UUID, limit int) ([]model.QueryRun, error)
	GetQueryRun(ctx context.Context, runID uuid.UUID) (model.QueryRun, error)
}
