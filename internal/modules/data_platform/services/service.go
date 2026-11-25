// Package services contains the data platform use cases: data sources, schema
// discovery, ingestion pipelines, versioned datasets and their governance.
package services

import (
	"context"

	"github.com/google/uuid"

	model "github.com/nuriansyah/hugimethara/internal/modules/data_platform/model"
	dto "github.com/nuriansyah/hugimethara/internal/modules/data_platform/model/dto"
)

// Service is the complete use-case surface of the data platform.
type Service interface {
	Kinds() []model.Kind

	ListDataSources(ctx context.Context, workspaceID uuid.UUID) ([]model.DataSource, error)
	CreateDataSource(ctx context.Context, in dto.CreateDataSourceInput) (model.DataSource, error)
	GetDataSource(ctx context.Context, id uuid.UUID) (model.DataSource, error)
	UpdateDataSource(ctx context.Context, id uuid.UUID, in dto.UpdateDataSourceInput) (model.DataSource, error)
	DeleteDataSource(ctx context.Context, id uuid.UUID) error
	TestDataSource(ctx context.Context, id uuid.UUID) (TestResult, error)
	DiscoverDataSource(ctx context.Context, id uuid.UUID) (Discovery, error)
	ListSchemas(ctx context.Context, id uuid.UUID, limit int) ([]model.DataSourceSchema, error)

	ListPipelines(ctx context.Context, workspaceID uuid.UUID) ([]model.Pipeline, error)
	CreatePipeline(ctx context.Context, in dto.CreatePipelineInput) (model.Pipeline, error)
	GetPipeline(ctx context.Context, id uuid.UUID) (model.Pipeline, error)
	UpdatePipeline(ctx context.Context, id uuid.UUID, in dto.UpdatePipelineInput) (model.Pipeline, error)
	DeletePipeline(ctx context.Context, id uuid.UUID) error
	RunPipeline(ctx context.Context, id uuid.UUID) (model.IngestionRun, error)
	ListRuns(ctx context.Context, pipelineID uuid.UUID, limit int) ([]model.IngestionRun, error)

	ListDatasets(ctx context.Context, workspaceID uuid.UUID) ([]model.Dataset, error)
	CreateDataset(ctx context.Context, in dto.CreateDatasetInput) (model.Dataset, error)
	GetDataset(ctx context.Context, id uuid.UUID) (model.Dataset, error)
	UpdateDataset(ctx context.Context, id uuid.UUID, in dto.UpdateDatasetInput) (model.Dataset, error)
	DeleteDataset(ctx context.Context, id uuid.UUID) error
	PublishDataset(ctx context.Context, id uuid.UUID) (model.DatasetVersion, error)
	ListDatasetVersions(ctx context.Context, id uuid.UUID) ([]model.DatasetVersion, error)
	PreviewDataset(ctx context.Context, id uuid.UUID, sql string) (Preview, error)
	Lineage(ctx context.Context, id uuid.UUID) ([]model.Lineage, error)

	ColumnPolicies(ctx context.Context, datasetID uuid.UUID) ([]model.ColumnPolicy, error)
	PutColumnPolicy(ctx context.Context, datasetID uuid.UUID, in dto.ColumnPolicyInput) (model.ColumnPolicy, error)
	DeleteColumnPolicy(ctx context.Context, datasetID uuid.UUID, column string) error
	RowPolicies(ctx context.Context, datasetID uuid.UUID) ([]model.RowPolicy, error)
	PutRowPolicy(ctx context.Context, datasetID uuid.UUID, in dto.RowPolicyInput) (model.RowPolicy, error)
	DeleteRowPolicy(ctx context.Context, datasetID uuid.UUID, roleKey string) error

	// Query context used by the analytics module.
	AllowedTables(ctx context.Context) ([]string, error)
	PublishedViews(ctx context.Context) (map[string]uuid.UUID, error)
	DatasetForQuery(ctx context.Context, id uuid.UUID) (model.Dataset, error)
	RowFilterFor(ctx context.Context, datasetID uuid.UUID) (string, map[string]string, error)
}

// TestResult is the outcome of a source connection test.
type TestResult struct {
	OK        bool   `json:"ok"`
	LatencyMS int    `json:"latency_ms"`
	Message   string `json:"message"`
	ErrorCode string `json:"error_code,omitempty"`
}

// Discovery is a schema discovery outcome with drift from the previous snapshot.
type Discovery struct {
	Schema     model.Schema `json:"schema"`
	Checksum   string       `json:"checksum"`
	Drift      []string     `json:"drift"`
	CapturedAt string       `json:"captured_at"`
}

// Preview is a dataset preview result.
type Preview struct {
	Columns  []string `json:"columns"`
	Rows     [][]any  `json:"rows"`
	RowCount int64    `json:"row_count"`
}

var _ Service = (*DataPlatformService)(nil)
