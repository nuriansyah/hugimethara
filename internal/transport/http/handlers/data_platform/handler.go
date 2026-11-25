// Package data_platform adapts the data platform use cases to the strict HTTP
// operation table. It holds no business rules.
package data_platform

import (
	"context"

	dtoin "github.com/nuriansyah/hugimethara/internal/modules/data_platform/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/data_platform/services"
	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/transport/http/dto"
)

type HTTP struct{ svc services.Service }

func NewHTTP(svc services.Service) *HTTP { return &HTTP{svc: svc} }

// @Summary list data source kinds
// @ID listDataSourceKinds
// @Tags data-platform
// @Success 200 {object} dto.ListDataSourceKinds200JSONResponse
// @Produce json
// @Router /v1/hugimethara/data-source-kinds [get]
func (h *HTTP) ListDataSourceKinds(context.Context, dto.ListDataSourceKindsRequestObject) (dto.ListDataSourceKindsResponseObject, error) {
	return dto.ListDataSourceKinds200JSONResponse{Data: dto.WireSlice[dto.DataSourceKind](h.svc.Kinds())}, nil
}

// @Summary list data sources
// @ID listDataSources
// @Tags data-platform
// @Success 200 {object} dto.ListDataSources200JSONResponse
// @Produce json
// @Param workspace_id query string true "workspace_id"
// @Router /v1/hugimethara/data-sources [get]
func (h *HTTP) ListDataSources(ctx context.Context, req dto.ListDataSourcesRequestObject) (dto.ListDataSourcesResponseObject, error) {
	rows, err := h.svc.ListDataSources(ctx, req.Params.WorkspaceID)
	if err != nil {
		return nil, err
	}
	return dto.ListDataSources200JSONResponse{Data: dto.WireSlice[dto.DataSource](rows)}, nil
}

// @Summary create data source
// @ID createDataSource
// @Tags data-platform
// @Success 201 {object} dto.CreateDataSource201JSONResponse
// @Produce json
// @Param request body dto.DataSourceCreate true "Request body"
// @Router /v1/hugimethara/data-sources [post]
func (h *HTTP) CreateDataSource(ctx context.Context, req dto.CreateDataSourceRequestObject) (dto.CreateDataSourceResponseObject, error) {
	if req.Body == nil {
		return nil, problem.BadRequest("A JSON body is required.")
	}
	row, err := h.svc.CreateDataSource(ctx, dtoin.CreateDataSourceInput{
		WorkspaceID: req.Body.WorkspaceID, EnvironmentID: req.Body.EnvironmentID, Name: req.Body.Name,
		Slug: req.Body.Slug, Kind: req.Body.Kind, Config: req.Body.Config, SecretID: req.Body.SecretID,
	})
	if err != nil {
		return nil, err
	}
	return dto.CreateDataSource201JSONResponse(dto.Wire[dto.DataSource](row)), nil
}

// @Summary get data source
// @ID getDataSource
// @Tags data-platform
// @Success 200 {object} dto.GetDataSource200JSONResponse
// @Produce json
// @Param data_source_id path string true "data_source_id"
// @Router /v1/hugimethara/data-sources/{data_source_id} [get]
func (h *HTTP) GetDataSource(ctx context.Context, req dto.GetDataSourceRequestObject) (dto.GetDataSourceResponseObject, error) {
	row, err := h.svc.GetDataSource(ctx, req.DataSourceId)
	if err != nil {
		return nil, err
	}
	return dto.GetDataSource200JSONResponse(dto.Wire[dto.DataSource](row)), nil
}

// @Summary update data source
// @ID updateDataSource
// @Tags data-platform
// @Success 200 {object} dto.UpdateDataSource200JSONResponse
// @Produce json
// @Param data_source_id path string true "data_source_id"
// @Param request body dto.DataSourceUpdate true "Request body"
// @Router /v1/hugimethara/data-sources/{data_source_id} [patch]
func (h *HTTP) UpdateDataSource(ctx context.Context, req dto.UpdateDataSourceRequestObject) (dto.UpdateDataSourceResponseObject, error) {
	if req.Body == nil {
		return nil, problem.BadRequest("A JSON body is required.")
	}
	row, err := h.svc.UpdateDataSource(ctx, req.DataSourceId, dtoin.UpdateDataSourceInput{
		Name: req.Body.Name, Slug: req.Body.Slug, Config: req.Body.Config, SecretID: req.Body.SecretID, Status: req.Body.Status,
	})
	if err != nil {
		return nil, err
	}
	return dto.UpdateDataSource200JSONResponse(dto.Wire[dto.DataSource](row)), nil
}

// @Summary delete data source
// @ID deleteDataSource
// @Tags data-platform
// @Success 204 "No Content"
// @Produce json
// @Param data_source_id path string true "data_source_id"
// @Router /v1/hugimethara/data-sources/{data_source_id} [delete]
func (h *HTTP) DeleteDataSource(ctx context.Context, req dto.DeleteDataSourceRequestObject) (dto.DeleteDataSourceResponseObject, error) {
	if err := h.svc.DeleteDataSource(ctx, req.DataSourceId); err != nil {
		return nil, err
	}
	return dto.DeleteDataSource204Response{}, nil
}

// @Summary test data source
// @ID testDataSource
// @Tags data-platform
// @Success 200 {object} dto.TestDataSource200JSONResponse
// @Produce json
// @Param data_source_id path string true "data_source_id"
// @Router /v1/hugimethara/data-sources/{data_source_id}/test [post]
func (h *HTTP) TestDataSource(ctx context.Context, req dto.TestDataSourceRequestObject) (dto.TestDataSourceResponseObject, error) {
	result, err := h.svc.TestDataSource(ctx, req.DataSourceId)
	if err != nil {
		return nil, err
	}
	return dto.TestDataSource200JSONResponse(dto.DataSourceTestResult{
		OK: result.OK, LatencyMS: result.LatencyMS, Message: result.Message, ErrorCode: result.ErrorCode,
	}), nil
}

// @Summary discover data source
// @ID discoverDataSource
// @Tags data-platform
// @Success 200 {object} dto.DiscoverDataSource200JSONResponse
// @Produce json
// @Param data_source_id path string true "data_source_id"
// @Router /v1/hugimethara/data-sources/{data_source_id}/discover [post]
func (h *HTTP) DiscoverDataSource(ctx context.Context, req dto.DiscoverDataSourceRequestObject) (dto.DiscoverDataSourceResponseObject, error) {
	discovery, err := h.svc.DiscoverDataSource(ctx, req.DataSourceId)
	if err != nil {
		return nil, err
	}
	return dto.DiscoverDataSource200JSONResponse(dto.DataSourceDiscovery{
		Schema: dto.Wire[dto.Schema](discovery.Schema), Checksum: discovery.Checksum, Drift: discovery.Drift, CapturedAt: discovery.CapturedAt,
	}), nil
}

// @Summary list data source schemas
// @ID listDataSourceSchemas
// @Tags data-platform
// @Success 200 {object} dto.ListDataSourceSchemas200JSONResponse
// @Produce json
// @Param data_source_id path string true "data_source_id"
// @Param limit query integer false "limit"
// @Router /v1/hugimethara/data-sources/{data_source_id}/schemas [get]
func (h *HTTP) ListDataSourceSchemas(ctx context.Context, req dto.ListDataSourceSchemasRequestObject) (dto.ListDataSourceSchemasResponseObject, error) {
	limit := 0
	if req.Params.Limit != nil {
		limit = *req.Params.Limit
	}
	rows, err := h.svc.ListSchemas(ctx, req.DataSourceId, limit)
	if err != nil {
		return nil, err
	}
	return dto.ListDataSourceSchemas200JSONResponse{Data: dto.WireSlice[dto.DataSourceSchema](rows)}, nil
}

// @Summary list pipelines
// @ID listPipelines
// @Tags data-platform
// @Success 200 {object} dto.ListPipelines200JSONResponse
// @Produce json
// @Param workspace_id query string true "workspace_id"
// @Router /v1/hugimethara/pipelines [get]
func (h *HTTP) ListPipelines(ctx context.Context, req dto.ListPipelinesRequestObject) (dto.ListPipelinesResponseObject, error) {
	rows, err := h.svc.ListPipelines(ctx, req.Params.WorkspaceID)
	if err != nil {
		return nil, err
	}
	return dto.ListPipelines200JSONResponse{Data: dto.WireSlice[dto.IngestionPipeline](rows)}, nil
}

// @Summary create pipeline
// @ID createPipeline
// @Tags data-platform
// @Success 201 {object} dto.CreatePipeline201JSONResponse
// @Produce json
// @Param request body dto.PipelineCreate true "Request body"
// @Router /v1/hugimethara/pipelines [post]
func (h *HTTP) CreatePipeline(ctx context.Context, req dto.CreatePipelineRequestObject) (dto.CreatePipelineResponseObject, error) {
	if req.Body == nil {
		return nil, problem.BadRequest("A JSON body is required.")
	}
	row, err := h.svc.CreatePipeline(ctx, dtoin.CreatePipelineInput{
		WorkspaceID: req.Body.WorkspaceID, DataSourceID: req.Body.DataSourceID, Name: req.Body.Name,
		Slug: req.Body.Slug, TargetTable: req.Body.TargetTable, Mode: req.Body.Mode,
		SourceObject: req.Body.SourceObject, WatermarkColumn: req.Body.WatermarkColumn,
		WatermarkType: req.Body.WatermarkType, ScheduleCron: req.Body.ScheduleCron,
		Config: req.Body.Config, Enabled: req.Body.Enabled,
	})
	if err != nil {
		return nil, err
	}
	return dto.CreatePipeline201JSONResponse(dto.Wire[dto.IngestionPipeline](row)), nil
}

// @Summary get pipeline
// @ID getPipeline
// @Tags data-platform
// @Success 200 {object} dto.GetPipeline200JSONResponse
// @Produce json
// @Param pipeline_id path string true "pipeline_id"
// @Router /v1/hugimethara/pipelines/{pipeline_id} [get]
func (h *HTTP) GetPipeline(ctx context.Context, req dto.GetPipelineRequestObject) (dto.GetPipelineResponseObject, error) {
	row, err := h.svc.GetPipeline(ctx, req.PipelineId)
	if err != nil {
		return nil, err
	}
	return dto.GetPipeline200JSONResponse(dto.Wire[dto.IngestionPipeline](row)), nil
}

// @Summary update pipeline
// @ID updatePipeline
// @Tags data-platform
// @Success 200 {object} dto.UpdatePipeline200JSONResponse
// @Produce json
// @Param pipeline_id path string true "pipeline_id"
// @Param request body dto.PipelineUpdate true "Request body"
// @Router /v1/hugimethara/pipelines/{pipeline_id} [patch]
func (h *HTTP) UpdatePipeline(ctx context.Context, req dto.UpdatePipelineRequestObject) (dto.UpdatePipelineResponseObject, error) {
	if req.Body == nil {
		return nil, problem.BadRequest("A JSON body is required.")
	}
	row, err := h.svc.UpdatePipeline(ctx, req.PipelineId, dtoin.UpdatePipelineInput{
		Name: req.Body.Name, TargetTable: req.Body.TargetTable, Mode: req.Body.Mode,
		SourceObject: req.Body.SourceObject, WatermarkColumn: req.Body.WatermarkColumn,
		WatermarkType: req.Body.WatermarkType, ScheduleCron: req.Body.ScheduleCron,
		Config: req.Body.Config, Enabled: req.Body.Enabled,
	})
	if err != nil {
		return nil, err
	}
	return dto.UpdatePipeline200JSONResponse(dto.Wire[dto.IngestionPipeline](row)), nil
}

// @Summary delete pipeline
// @ID deletePipeline
// @Tags data-platform
// @Success 204 "No Content"
// @Produce json
// @Param pipeline_id path string true "pipeline_id"
// @Router /v1/hugimethara/pipelines/{pipeline_id} [delete]
func (h *HTTP) DeletePipeline(ctx context.Context, req dto.DeletePipelineRequestObject) (dto.DeletePipelineResponseObject, error) {
	if err := h.svc.DeletePipeline(ctx, req.PipelineId); err != nil {
		return nil, err
	}
	return dto.DeletePipeline204Response{}, nil
}

// @Summary run pipeline
// @ID runPipeline
// @Tags data-platform
// @Success 200 {object} dto.RunPipeline200JSONResponse
// @Produce json
// @Param pipeline_id path string true "pipeline_id"
// @Router /v1/hugimethara/pipelines/{pipeline_id}/run [post]
func (h *HTTP) RunPipeline(ctx context.Context, req dto.RunPipelineRequestObject) (dto.RunPipelineResponseObject, error) {
	run, err := h.svc.RunPipeline(ctx, req.PipelineId)
	if err != nil {
		return nil, err
	}
	return dto.RunPipeline200JSONResponse(dto.Wire[dto.IngestionRun](run)), nil
}

// @Summary list pipeline runs
// @ID listPipelineRuns
// @Tags data-platform
// @Success 200 {object} dto.ListPipelineRuns200JSONResponse
// @Produce json
// @Param pipeline_id path string true "pipeline_id"
// @Param limit query integer false "limit"
// @Router /v1/hugimethara/pipelines/{pipeline_id}/runs [get]
func (h *HTTP) ListPipelineRuns(ctx context.Context, req dto.ListPipelineRunsRequestObject) (dto.ListPipelineRunsResponseObject, error) {
	limit := 0
	if req.Params.Limit != nil {
		limit = *req.Params.Limit
	}
	rows, err := h.svc.ListRuns(ctx, req.PipelineId, limit)
	if err != nil {
		return nil, err
	}
	return dto.ListPipelineRuns200JSONResponse{Data: dto.WireSlice[dto.IngestionRun](rows)}, nil
}

// @Summary list datasets
// @ID listDatasets
// @Tags data-platform
// @Success 200 {object} dto.ListDatasets200JSONResponse
// @Produce json
// @Param workspace_id query string true "workspace_id"
// @Router /v1/hugimethara/datasets [get]
func (h *HTTP) ListDatasets(ctx context.Context, req dto.ListDatasetsRequestObject) (dto.ListDatasetsResponseObject, error) {
	rows, err := h.svc.ListDatasets(ctx, req.Params.WorkspaceID)
	if err != nil {
		return nil, err
	}
	return dto.ListDatasets200JSONResponse{Data: dto.WireSlice[dto.Dataset](rows)}, nil
}

// @Summary create dataset
// @ID createDataset
// @Tags data-platform
// @Success 201 {object} dto.CreateDataset201JSONResponse
// @Produce json
// @Param request body dto.DatasetCreate true "Request body"
// @Router /v1/hugimethara/datasets [post]
func (h *HTTP) CreateDataset(ctx context.Context, req dto.CreateDatasetRequestObject) (dto.CreateDatasetResponseObject, error) {
	if req.Body == nil {
		return nil, problem.BadRequest("A JSON body is required.")
	}
	row, err := h.svc.CreateDataset(ctx, dtoin.CreateDatasetInput{
		WorkspaceID: req.Body.WorkspaceID, Name: req.Body.Name, Slug: req.Body.Slug,
		Description: req.Body.Description, Owner: req.Body.Owner, SQL: req.Body.SQL,
	})
	if err != nil {
		return nil, err
	}
	return dto.CreateDataset201JSONResponse(dto.Wire[dto.Dataset](row)), nil
}

// @Summary get dataset
// @ID getDataset
// @Tags data-platform
// @Success 200 {object} dto.GetDataset200JSONResponse
// @Produce json
// @Param dataset_id path string true "dataset_id"
// @Router /v1/hugimethara/datasets/{dataset_id} [get]
func (h *HTTP) GetDataset(ctx context.Context, req dto.GetDatasetRequestObject) (dto.GetDatasetResponseObject, error) {
	row, err := h.svc.GetDataset(ctx, req.DatasetId)
	if err != nil {
		return nil, err
	}
	return dto.GetDataset200JSONResponse(dto.Wire[dto.Dataset](row)), nil
}

// @Summary update dataset
// @ID updateDataset
// @Tags data-platform
// @Success 200 {object} dto.UpdateDataset200JSONResponse
// @Produce json
// @Param dataset_id path string true "dataset_id"
// @Param request body dto.DatasetUpdate true "Request body"
// @Router /v1/hugimethara/datasets/{dataset_id} [patch]
func (h *HTTP) UpdateDataset(ctx context.Context, req dto.UpdateDatasetRequestObject) (dto.UpdateDatasetResponseObject, error) {
	if req.Body == nil {
		return nil, problem.BadRequest("A JSON body is required.")
	}
	row, err := h.svc.UpdateDataset(ctx, req.DatasetId, dtoin.UpdateDatasetInput{
		Name: req.Body.Name, Slug: req.Body.Slug, Description: req.Body.Description, Owner: req.Body.Owner, SQL: req.Body.SQL,
	})
	if err != nil {
		return nil, err
	}
	return dto.UpdateDataset200JSONResponse(dto.Wire[dto.Dataset](row)), nil
}

// @Summary delete dataset
// @ID deleteDataset
// @Tags data-platform
// @Success 204 "No Content"
// @Produce json
// @Param dataset_id path string true "dataset_id"
// @Router /v1/hugimethara/datasets/{dataset_id} [delete]
func (h *HTTP) DeleteDataset(ctx context.Context, req dto.DeleteDatasetRequestObject) (dto.DeleteDatasetResponseObject, error) {
	if err := h.svc.DeleteDataset(ctx, req.DatasetId); err != nil {
		return nil, err
	}
	return dto.DeleteDataset204Response{}, nil
}

// @Summary publish dataset
// @ID publishDataset
// @Tags data-platform
// @Success 200 {object} dto.PublishDataset200JSONResponse
// @Produce json
// @Param dataset_id path string true "dataset_id"
// @Router /v1/hugimethara/datasets/{dataset_id}/publish [post]
func (h *HTTP) PublishDataset(ctx context.Context, req dto.PublishDatasetRequestObject) (dto.PublishDatasetResponseObject, error) {
	version, err := h.svc.PublishDataset(ctx, req.DatasetId)
	if err != nil {
		return nil, err
	}
	return dto.PublishDataset200JSONResponse(dto.Wire[dto.DatasetVersion](version)), nil
}

// @Summary list dataset versions
// @ID listDatasetVersions
// @Tags data-platform
// @Success 200 {object} dto.ListDatasetVersions200JSONResponse
// @Produce json
// @Param dataset_id path string true "dataset_id"
// @Router /v1/hugimethara/datasets/{dataset_id}/versions [get]
func (h *HTTP) ListDatasetVersions(ctx context.Context, req dto.ListDatasetVersionsRequestObject) (dto.ListDatasetVersionsResponseObject, error) {
	rows, err := h.svc.ListDatasetVersions(ctx, req.DatasetId)
	if err != nil {
		return nil, err
	}
	return dto.ListDatasetVersions200JSONResponse{Data: dto.WireSlice[dto.DatasetVersion](rows)}, nil
}

// @Summary preview dataset
// @ID previewDataset
// @Tags data-platform
// @Success 200 {object} dto.PreviewDataset200JSONResponse
// @Produce json
// @Param dataset_id path string true "dataset_id"
// @Param request body dto.DatasetPreviewBody true "Request body"
// @Router /v1/hugimethara/datasets/{dataset_id}/preview [post]
func (h *HTTP) PreviewDataset(ctx context.Context, req dto.PreviewDatasetRequestObject) (dto.PreviewDatasetResponseObject, error) {
	if req.Body == nil {
		return nil, problem.BadRequest("A JSON body is required.")
	}
	preview, err := h.svc.PreviewDataset(ctx, req.DatasetId, req.Body.SQL)
	if err != nil {
		return nil, err
	}
	return dto.PreviewDataset200JSONResponse(dto.DatasetPreview{
		Columns: preview.Columns, Rows: preview.Rows, RowCount: preview.RowCount,
	}), nil
}

// @Summary list dataset lineage
// @ID listDatasetLineage
// @Tags data-platform
// @Success 200 {object} dto.ListDatasetLineage200JSONResponse
// @Produce json
// @Param dataset_id path string true "dataset_id"
// @Router /v1/hugimethara/datasets/{dataset_id}/lineage [get]
func (h *HTTP) ListDatasetLineage(ctx context.Context, req dto.ListDatasetLineageRequestObject) (dto.ListDatasetLineageResponseObject, error) {
	rows, err := h.svc.Lineage(ctx, req.DatasetId)
	if err != nil {
		return nil, err
	}
	return dto.ListDatasetLineage200JSONResponse{Data: dto.WireSlice[dto.DatasetLineage](rows)}, nil
}

// @Summary list column policies
// @ID listColumnPolicies
// @Tags data-platform
// @Success 200 {object} dto.ListColumnPolicies200JSONResponse
// @Produce json
// @Param dataset_id path string true "dataset_id"
// @Router /v1/hugimethara/datasets/{dataset_id}/column-policies [get]
func (h *HTTP) ListColumnPolicies(ctx context.Context, req dto.ListColumnPoliciesRequestObject) (dto.ListColumnPoliciesResponseObject, error) {
	rows, err := h.svc.ColumnPolicies(ctx, req.DatasetId)
	if err != nil {
		return nil, err
	}
	return dto.ListColumnPolicies200JSONResponse{Data: dto.WireSlice[dto.ColumnPolicy](rows)}, nil
}

// @Summary put column policy
// @ID putColumnPolicy
// @Tags data-platform
// @Success 200 {object} dto.PutColumnPolicy200JSONResponse
// @Produce json
// @Param dataset_id path string true "dataset_id"
// @Param column path string true "column"
// @Param request body dto.ColumnPolicyBody true "Request body"
// @Router /v1/hugimethara/datasets/{dataset_id}/column-policies/{column} [put]
func (h *HTTP) PutColumnPolicy(ctx context.Context, req dto.PutColumnPolicyRequestObject) (dto.PutColumnPolicyResponseObject, error) {
	if req.Body == nil {
		return nil, problem.BadRequest("A JSON body is required.")
	}
	row, err := h.svc.PutColumnPolicy(ctx, req.DatasetId, dtoin.ColumnPolicyInput{
		ColumnName: req.Column, Classification: req.Body.Classification, Mask: req.Body.Mask,
	})
	if err != nil {
		return nil, err
	}
	return dto.PutColumnPolicy200JSONResponse(dto.Wire[dto.ColumnPolicy](row)), nil
}

// @Summary delete column policy
// @ID deleteColumnPolicy
// @Tags data-platform
// @Success 204 "No Content"
// @Produce json
// @Param dataset_id path string true "dataset_id"
// @Param column path string true "column"
// @Router /v1/hugimethara/datasets/{dataset_id}/column-policies/{column} [delete]
func (h *HTTP) DeleteColumnPolicy(ctx context.Context, req dto.DeleteColumnPolicyRequestObject) (dto.DeleteColumnPolicyResponseObject, error) {
	if err := h.svc.DeleteColumnPolicy(ctx, req.DatasetId, req.Column); err != nil {
		return nil, err
	}
	return dto.DeleteColumnPolicy204Response{}, nil
}

// @Summary list row policies
// @ID listRowPolicies
// @Tags data-platform
// @Success 200 {object} dto.ListRowPolicies200JSONResponse
// @Produce json
// @Param dataset_id path string true "dataset_id"
// @Router /v1/hugimethara/datasets/{dataset_id}/row-policies [get]
func (h *HTTP) ListRowPolicies(ctx context.Context, req dto.ListRowPoliciesRequestObject) (dto.ListRowPoliciesResponseObject, error) {
	rows, err := h.svc.RowPolicies(ctx, req.DatasetId)
	if err != nil {
		return nil, err
	}
	return dto.ListRowPolicies200JSONResponse{Data: dto.WireSlice[dto.RowPolicy](rows)}, nil
}

// @Summary put row policy
// @ID putRowPolicy
// @Tags data-platform
// @Success 200 {object} dto.PutRowPolicy200JSONResponse
// @Produce json
// @Param dataset_id path string true "dataset_id"
// @Param role_key path string true "role_key"
// @Param request body dto.RowPolicyBody true "Request body"
// @Router /v1/hugimethara/datasets/{dataset_id}/row-policies/{role_key} [put]
func (h *HTTP) PutRowPolicy(ctx context.Context, req dto.PutRowPolicyRequestObject) (dto.PutRowPolicyResponseObject, error) {
	if req.Body == nil {
		return nil, problem.BadRequest("A JSON body is required.")
	}
	row, err := h.svc.PutRowPolicy(ctx, req.DatasetId, dtoin.RowPolicyInput{
		RoleKey: req.RoleKey, RowFilter: req.Body.RowFilter, ColumnMasks: req.Body.ColumnMasks,
	})
	if err != nil {
		return nil, err
	}
	return dto.PutRowPolicy200JSONResponse(dto.Wire[dto.RowPolicy](row)), nil
}

// @Summary delete row policy
// @ID deleteRowPolicy
// @Tags data-platform
// @Success 204 "No Content"
// @Produce json
// @Param dataset_id path string true "dataset_id"
// @Param role_key path string true "role_key"
// @Router /v1/hugimethara/datasets/{dataset_id}/row-policies/{role_key} [delete]
func (h *HTTP) DeleteRowPolicy(ctx context.Context, req dto.DeleteRowPolicyRequestObject) (dto.DeleteRowPolicyResponseObject, error) {
	if err := h.svc.DeleteRowPolicy(ctx, req.DatasetId, req.RoleKey); err != nil {
		return nil, err
	}
	return dto.DeleteRowPolicy204Response{}, nil
}
