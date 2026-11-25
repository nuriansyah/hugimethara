// Package analytics adapts semantic models, metrics and query runs to HTTP.
package analytics

import (
	"context"

	analyticsmodel "github.com/nuriansyah/hugimethara/internal/modules/analytics/model"
	dtoin "github.com/nuriansyah/hugimethara/internal/modules/analytics/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/analytics/services"
	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/transport/http/dto"
)

type HTTP struct{ svc services.Service }

func NewHTTP(svc services.Service) *HTTP { return &HTTP{svc: svc} }

// @Summary list semantic models
// @ID listSemanticModels
// @Tags analytics
// @Success 200 {object} dto.ListSemanticModels200JSONResponse
// @Produce json
// @Param workspace_id query string true "workspace_id"
// @Router /v1/hugimethara/semantic-models [get]
func (h *HTTP) ListSemanticModels(ctx context.Context, req dto.ListSemanticModelsRequestObject) (dto.ListSemanticModelsResponseObject, error) {
	rows, err := h.svc.ListSemanticModels(ctx, req.Params.WorkspaceID)
	if err != nil {
		return nil, err
	}
	return dto.ListSemanticModels200JSONResponse{Data: dto.WireSlice[dto.SemanticModel](rows)}, nil
}

// @Summary create semantic model
// @ID createSemanticModel
// @Tags analytics
// @Success 201 {object} dto.CreateSemanticModel201JSONResponse
// @Produce json
// @Param request body dto.SemanticModelCreate true "Request body"
// @Router /v1/hugimethara/semantic-models [post]
func (h *HTTP) CreateSemanticModel(ctx context.Context, req dto.CreateSemanticModelRequestObject) (dto.CreateSemanticModelResponseObject, error) {
	if req.Body == nil {
		return nil, problem.BadRequest("A JSON body is required.")
	}
	row, err := h.svc.CreateSemanticModel(ctx, dtoin.CreateSemanticModelInput{
		WorkspaceID: req.Body.WorkspaceID, DatasetID: req.Body.DatasetID, Name: req.Body.Name,
		Slug: req.Body.Slug, Description: req.Body.Description, Dimensions: dto.Wire[[]analyticsmodel.Dimension](req.Body.Dimensions),
	})
	if err != nil {
		return nil, err
	}
	return dto.CreateSemanticModel201JSONResponse(dto.Wire[dto.SemanticModel](row)), nil
}

// @Summary get semantic model
// @ID getSemanticModel
// @Tags analytics
// @Success 200 {object} dto.GetSemanticModel200JSONResponse
// @Produce json
// @Param model_id path string true "model_id"
// @Router /v1/hugimethara/semantic-models/{model_id} [get]
func (h *HTTP) GetSemanticModel(ctx context.Context, req dto.GetSemanticModelRequestObject) (dto.GetSemanticModelResponseObject, error) {
	row, err := h.svc.GetSemanticModel(ctx, req.ModelId)
	if err != nil {
		return nil, err
	}
	return dto.GetSemanticModel200JSONResponse(dto.Wire[dto.SemanticModel](row)), nil
}

// @Summary update semantic model
// @ID updateSemanticModel
// @Tags analytics
// @Success 200 {object} dto.UpdateSemanticModel200JSONResponse
// @Produce json
// @Param model_id path string true "model_id"
// @Param request body dto.SemanticModelUpdate true "Request body"
// @Router /v1/hugimethara/semantic-models/{model_id} [patch]
func (h *HTTP) UpdateSemanticModel(ctx context.Context, req dto.UpdateSemanticModelRequestObject) (dto.UpdateSemanticModelResponseObject, error) {
	if req.Body == nil {
		return nil, problem.BadRequest("A JSON body is required.")
	}
	var dimensions *[]analyticsmodel.Dimension
	if req.Body.Dimensions != nil {
		converted := dto.Wire[[]analyticsmodel.Dimension](*req.Body.Dimensions)
		dimensions = &converted
	}
	row, err := h.svc.UpdateSemanticModel(ctx, req.ModelId, dtoin.UpdateSemanticModelInput{
		Name: req.Body.Name, Slug: req.Body.Slug, Description: req.Body.Description, Dimensions: dimensions,
	})
	if err != nil {
		return nil, err
	}
	return dto.UpdateSemanticModel200JSONResponse(dto.Wire[dto.SemanticModel](row)), nil
}

// @Summary delete semantic model
// @ID deleteSemanticModel
// @Tags analytics
// @Success 204 "No Content"
// @Produce json
// @Param model_id path string true "model_id"
// @Router /v1/hugimethara/semantic-models/{model_id} [delete]
func (h *HTTP) DeleteSemanticModel(ctx context.Context, req dto.DeleteSemanticModelRequestObject) (dto.DeleteSemanticModelResponseObject, error) {
	if err := h.svc.DeleteSemanticModel(ctx, req.ModelId); err != nil {
		return nil, err
	}
	return dto.DeleteSemanticModel204Response{}, nil
}

// @Summary list semantic model metrics
// @ID listSemanticModelMetrics
// @Tags analytics
// @Success 200 {object} dto.ListSemanticModelMetrics200JSONResponse
// @Produce json
// @Param model_id path string true "model_id"
// @Router /v1/hugimethara/semantic-models/{model_id}/metrics [get]
func (h *HTTP) ListSemanticModelMetrics(ctx context.Context, req dto.ListSemanticModelMetricsRequestObject) (dto.ListSemanticModelMetricsResponseObject, error) {
	rows, err := h.svc.ListMetricsForModel(ctx, req.ModelId)
	if err != nil {
		return nil, err
	}
	return dto.ListSemanticModelMetrics200JSONResponse{Data: dto.WireSlice[dto.Metric](rows)}, nil
}

// @Summary list metrics
// @ID listMetrics
// @Tags analytics
// @Success 200 {object} dto.ListMetrics200JSONResponse
// @Produce json
// @Param workspace_id query string true "workspace_id"
// @Router /v1/hugimethara/metrics [get]
func (h *HTTP) ListMetrics(ctx context.Context, req dto.ListMetricsRequestObject) (dto.ListMetricsResponseObject, error) {
	rows, err := h.svc.ListMetrics(ctx, req.Params.WorkspaceID)
	if err != nil {
		return nil, err
	}
	return dto.ListMetrics200JSONResponse{Data: dto.WireSlice[dto.Metric](rows)}, nil
}

// @Summary create metric
// @ID createMetric
// @Tags analytics
// @Success 201 {object} dto.CreateMetric201JSONResponse
// @Produce json
// @Param request body dto.MetricCreate true "Request body"
// @Router /v1/hugimethara/metrics [post]
func (h *HTTP) CreateMetric(ctx context.Context, req dto.CreateMetricRequestObject) (dto.CreateMetricResponseObject, error) {
	if req.Body == nil {
		return nil, problem.BadRequest("A JSON body is required.")
	}
	row, err := h.svc.CreateMetric(ctx, dtoin.CreateMetricInput{
		WorkspaceID: req.Body.WorkspaceID, SemanticModelID: req.Body.SemanticModelID, Name: req.Body.Name,
		Slug: req.Body.Slug, Description: req.Body.Description, Expression: req.Body.Expression,
		Filters: req.Body.Filters, Format: req.Body.Format,
	})
	if err != nil {
		return nil, err
	}
	return dto.CreateMetric201JSONResponse(dto.Wire[dto.Metric](row)), nil
}

// @Summary get metric
// @ID getMetric
// @Tags analytics
// @Success 200 {object} dto.GetMetric200JSONResponse
// @Produce json
// @Param metric_id path string true "metric_id"
// @Router /v1/hugimethara/metrics/{metric_id} [get]
func (h *HTTP) GetMetric(ctx context.Context, req dto.GetMetricRequestObject) (dto.GetMetricResponseObject, error) {
	row, err := h.svc.GetMetric(ctx, req.MetricId)
	if err != nil {
		return nil, err
	}
	return dto.GetMetric200JSONResponse(dto.Wire[dto.Metric](row)), nil
}

// @Summary update metric
// @ID updateMetric
// @Tags analytics
// @Success 200 {object} dto.UpdateMetric200JSONResponse
// @Produce json
// @Param metric_id path string true "metric_id"
// @Param request body dto.MetricUpdate true "Request body"
// @Router /v1/hugimethara/metrics/{metric_id} [patch]
func (h *HTTP) UpdateMetric(ctx context.Context, req dto.UpdateMetricRequestObject) (dto.UpdateMetricResponseObject, error) {
	if req.Body == nil {
		return nil, problem.BadRequest("A JSON body is required.")
	}
	row, err := h.svc.UpdateMetric(ctx, req.MetricId, dtoin.UpdateMetricInput{
		Name: req.Body.Name, Slug: req.Body.Slug, Description: req.Body.Description,
		Expression: req.Body.Expression, Filters: req.Body.Filters, Format: req.Body.Format,
	})
	if err != nil {
		return nil, err
	}
	return dto.UpdateMetric200JSONResponse(dto.Wire[dto.Metric](row)), nil
}

// @Summary delete metric
// @ID deleteMetric
// @Tags analytics
// @Success 204 "No Content"
// @Produce json
// @Param metric_id path string true "metric_id"
// @Router /v1/hugimethara/metrics/{metric_id} [delete]
func (h *HTTP) DeleteMetric(ctx context.Context, req dto.DeleteMetricRequestObject) (dto.DeleteMetricResponseObject, error) {
	if err := h.svc.DeleteMetric(ctx, req.MetricId); err != nil {
		return nil, err
	}
	return dto.DeleteMetric204Response{}, nil
}

// @Summary query metric
// @ID queryMetric
// @Tags analytics
// @Success 200 {object} dto.QueryMetric200JSONResponse
// @Produce json
// @Param metric_id path string true "metric_id"
// @Param request body dto.MetricQueryBody true "Request body"
// @Router /v1/hugimethara/metrics/{metric_id}/query [post]
func (h *HTTP) QueryMetric(ctx context.Context, req dto.QueryMetricRequestObject) (dto.QueryMetricResponseObject, error) {
	filters := []string(nil)
	limit := 0
	if req.Body != nil {
		filters = req.Body.Filters
		limit = req.Body.Limit
	}
	value, err := h.svc.EvaluateMetric(ctx, req.MetricId, dtoin.MetricQueryInput{Filters: filters, Limit: limit})
	if err != nil {
		return nil, err
	}
	return dto.QueryMetric200JSONResponse(dto.Wire[dto.MetricValue](value)), nil
}

// @Summary execute query
// @ID executeQuery
// @Tags analytics
// @Success 200 {object} dto.ExecuteQuery200JSONResponse
// @Produce json
// @Param request body dto.QueryBody true "Request body"
// @Router /v1/hugimethara/queries [post]
func (h *HTTP) ExecuteQuery(ctx context.Context, req dto.ExecuteQueryRequestObject) (dto.ExecuteQueryResponseObject, error) {
	if req.Body == nil {
		return nil, problem.BadRequest("A JSON body is required.")
	}
	result, err := h.svc.Execute(ctx, dtoin.QueryInput{
		WorkspaceID: req.Body.WorkspaceID, DatasetID: req.Body.DatasetID, SQL: req.Body.SQL,
		Limit: req.Body.Limit, Export: req.Body.Export,
	})
	if err != nil {
		return nil, err
	}
	return dto.ExecuteQuery200JSONResponse(dto.Wire[dto.QueryResult](result)), nil
}

// @Summary list queries
// @ID listQueries
// @Tags analytics
// @Success 200 {object} dto.ListQueries200JSONResponse
// @Produce json
// @Param workspace_id query string true "workspace_id"
// @Router /v1/hugimethara/queries [get]
func (h *HTTP) ListQueries(ctx context.Context, req dto.ListQueriesRequestObject) (dto.ListQueriesResponseObject, error) {
	rows, err := h.svc.ListQueryRuns(ctx, req.Params.WorkspaceID, 50)
	if err != nil {
		return nil, err
	}
	return dto.ListQueries200JSONResponse{Data: dto.WireSlice[dto.QueryRun](rows)}, nil
}

// @Summary get query
// @ID getQuery
// @Tags analytics
// @Success 200 {object} dto.GetQuery200JSONResponse
// @Produce json
// @Param query_id path string true "query_id"
// @Router /v1/hugimethara/queries/{query_id} [get]
func (h *HTTP) GetQuery(ctx context.Context, req dto.GetQueryRequestObject) (dto.GetQueryResponseObject, error) {
	row, err := h.svc.GetQueryRun(ctx, req.QueryId)
	if err != nil {
		return nil, err
	}
	return dto.GetQuery200JSONResponse(dto.Wire[dto.QueryRun](row)), nil
}

// @Summary cancel query
// @ID cancelQuery
// @Tags analytics
// @Success 204 "No Content"
// @Produce json
// @Param query_id path string true "query_id"
// @Router /v1/hugimethara/queries/{query_id}/cancel [post]
func (h *HTTP) CancelQuery(ctx context.Context, req dto.CancelQueryRequestObject) (dto.CancelQueryResponseObject, error) {
	if err := h.svc.Cancel(ctx, req.QueryId); err != nil {
		return nil, err
	}
	return dto.CancelQuery204Response{}, nil
}

// @Summary export query
// @ID exportQuery
// @Tags analytics
// @Success 200 {string} string "CSV export"
// @Produce json
// @Param query_id path string true "query_id"
// @Router /v1/hugimethara/queries/{query_id}/export [get]
func (h *HTTP) ExportQuery(ctx context.Context, req dto.ExportQueryRequestObject) (dto.ExportQueryResponseObject, error) {
	body, err := h.svc.ExportCSV(ctx, req.QueryId)
	if err != nil {
		return nil, err
	}
	return dto.ExportQuery200TextResponse(body), nil
}
