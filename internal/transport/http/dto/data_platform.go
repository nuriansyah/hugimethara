package dto

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
)

func writeJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

// Wire converts a model value to its wire shape. The model and wire types
// share JSON tags by design, so a JSON round trip is the exact mapping.
func Wire[T any](in any) T {
	var out T
	raw, _ := json.Marshal(in)
	_ = json.Unmarshal(raw, &out)
	return out
}

// WireSlice converts a model slice to its wire shape, always returning a
// non-nil slice so list responses encode as [] rather than null.
func WireSlice[T any](in any) []T {
	out := Wire[[]T](in)
	if out == nil {
		out = []T{}
	}
	return out
}

// Data platform wire types.
type DataSourceKind struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	Category  string `json:"category"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

type DataSource struct {
	ID            uuid.UUID      `json:"id"`
	OrgID         uuid.UUID      `json:"org_id"`
	WorkspaceID   uuid.UUID      `json:"workspace_id"`
	EnvironmentID uuid.UUID      `json:"environment_id"`
	Name          string         `json:"name"`
	Slug          string         `json:"slug"`
	Kind          string         `json:"kind"`
	Config        map[string]any `json:"config"`
	SecretID      *uuid.UUID     `json:"secret_id,omitempty"`
	Status        string         `json:"status"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

type SchemaColumn struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
}

type SchemaObject struct {
	Name    string         `json:"name"`
	Type    string         `json:"type"`
	Columns []SchemaColumn `json:"columns"`
}

type Schema struct {
	Objects []SchemaObject `json:"objects"`
}

type DataSourceSchema struct {
	ID           uuid.UUID `json:"id"`
	DataSourceID uuid.UUID `json:"data_source_id"`
	Schema       Schema    `json:"schema"`
	Checksum     string    `json:"checksum"`
	CapturedAt   time.Time `json:"captured_at"`
}

type IngestionPipeline struct {
	ID              uuid.UUID      `json:"id"`
	OrgID           uuid.UUID      `json:"org_id"`
	WorkspaceID     uuid.UUID      `json:"workspace_id"`
	DataSourceID    uuid.UUID      `json:"data_source_id"`
	Name            string         `json:"name"`
	Slug            string         `json:"slug"`
	TargetTable     string         `json:"target_table"`
	Mode            string         `json:"mode"`
	SourceObject    string         `json:"source_object"`
	WatermarkColumn string         `json:"watermark_column"`
	WatermarkType   string         `json:"watermark_type"`
	ScheduleCron    string         `json:"schedule_cron"`
	Config          map[string]any `json:"config"`
	Enabled         bool           `json:"enabled"`
	LastRunAt       *time.Time     `json:"last_run_at,omitempty"`
	NextRunAt       *time.Time     `json:"next_run_at,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

type IngestionRun struct {
	ID              uuid.UUID      `json:"id"`
	PipelineID      uuid.UUID      `json:"pipeline_id"`
	Status          string         `json:"status"`
	Mode            string         `json:"mode"`
	WatermarkBefore string         `json:"watermark_before"`
	WatermarkAfter  string         `json:"watermark_after"`
	RowsRead        int64          `json:"rows_read"`
	RowsWritten     int64          `json:"rows_written"`
	SchemaDrift     []string       `json:"schema_drift,omitempty"`
	Error           map[string]any `json:"error,omitempty"`
	StartedAt       *time.Time     `json:"started_at,omitempty"`
	FinishedAt      *time.Time     `json:"finished_at,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
}

type Dataset struct {
	ID          uuid.UUID `json:"id"`
	OrgID       uuid.UUID `json:"org_id"`
	WorkspaceID uuid.UUID `json:"workspace_id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	Owner       string    `json:"owner"`
	DraftSQL    string    `json:"draft_sql"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type DatasetVersion struct {
	ID          uuid.UUID      `json:"id"`
	DatasetID   uuid.UUID      `json:"dataset_id"`
	Version     int            `json:"version"`
	SQL         string         `json:"sql"`
	Columns     []SchemaColumn `json:"columns"`
	Checksum    string         `json:"checksum"`
	PublishedBy uuid.UUID      `json:"published_by"`
	PublishedAt time.Time      `json:"published_at"`
}

type DatasetLineage struct {
	DatasetID    uuid.UUID `json:"dataset_id"`
	UpstreamKind string    `json:"upstream_kind"`
	UpstreamID   uuid.UUID `json:"upstream_id"`
	Columns      []string  `json:"columns"`
}

type ColumnPolicy struct {
	ColumnName     string `json:"column_name"`
	Classification string `json:"classification"`
	Mask           string `json:"mask"`
}

type RowPolicy struct {
	RoleKey     string            `json:"role_key"`
	RowFilter   string            `json:"row_filter"`
	ColumnMasks map[string]string `json:"column_masks"`
}

type DataSourceTestResult struct {
	OK        bool   `json:"ok"`
	LatencyMS int    `json:"latency_ms"`
	Message   string `json:"message"`
	ErrorCode string `json:"error_code,omitempty"`
}

type DataSourceDiscovery struct {
	Schema     Schema   `json:"schema"`
	Checksum   string   `json:"checksum"`
	Drift      []string `json:"drift"`
	CapturedAt string   `json:"captured_at"`
}

type DatasetPreview struct {
	Columns  []string `json:"columns"`
	Rows     [][]any  `json:"rows"`
	RowCount int64    `json:"row_count"`
}

type DataSourceCreate struct {
	WorkspaceID   uuid.UUID      `json:"workspace_id" binding:"required"`
	EnvironmentID uuid.UUID      `json:"environment_id" binding:"required"`
	Name          string         `json:"name" binding:"required"`
	Slug          string         `json:"slug,omitempty"`
	Kind          string         `json:"kind" binding:"required"`
	Config        map[string]any `json:"config,omitempty"`
	SecretID      *uuid.UUID     `json:"secret_id,omitempty"`
}

type DataSourceUpdate struct {
	Name     *string         `json:"name,omitempty"`
	Slug     *string         `json:"slug,omitempty"`
	Config   *map[string]any `json:"config,omitempty"`
	SecretID *uuid.UUID      `json:"secret_id,omitempty"`
	Status   *string         `json:"status,omitempty"`
}

type PipelineCreate struct {
	WorkspaceID     uuid.UUID      `json:"workspace_id" binding:"required"`
	DataSourceID    uuid.UUID      `json:"data_source_id" binding:"required"`
	Name            string         `json:"name" binding:"required"`
	Slug            string         `json:"slug,omitempty"`
	TargetTable     string         `json:"target_table" binding:"required"`
	Mode            string         `json:"mode,omitempty"`
	SourceObject    string         `json:"source_object" binding:"required"`
	WatermarkColumn string         `json:"watermark_column,omitempty"`
	WatermarkType   string         `json:"watermark_type,omitempty"`
	ScheduleCron    string         `json:"schedule_cron,omitempty"`
	Config          map[string]any `json:"config,omitempty"`
	Enabled         *bool          `json:"enabled,omitempty"`
}

type PipelineUpdate struct {
	Name            *string         `json:"name,omitempty"`
	TargetTable     *string         `json:"target_table,omitempty"`
	Mode            *string         `json:"mode,omitempty"`
	SourceObject    *string         `json:"source_object,omitempty"`
	WatermarkColumn *string         `json:"watermark_column,omitempty"`
	WatermarkType   *string         `json:"watermark_type,omitempty"`
	ScheduleCron    *string         `json:"schedule_cron,omitempty"`
	Config          *map[string]any `json:"config,omitempty"`
	Enabled         *bool           `json:"enabled,omitempty"`
}

type DatasetCreate struct {
	WorkspaceID uuid.UUID `json:"workspace_id" binding:"required"`
	Name        string    `json:"name" binding:"required"`
	Slug        string    `json:"slug,omitempty"`
	Description string    `json:"description,omitempty"`
	Owner       string    `json:"owner,omitempty"`
	SQL         string    `json:"sql,omitempty"`
}

type DatasetUpdate struct {
	Name        *string `json:"name,omitempty"`
	Slug        *string `json:"slug,omitempty"`
	Description *string `json:"description,omitempty"`
	Owner       *string `json:"owner,omitempty"`
	SQL         *string `json:"sql,omitempty"`
}

type DatasetPreviewBody struct {
	SQL string `json:"sql,omitempty"`
}

type ColumnPolicyBody struct {
	Classification string `json:"classification" binding:"required"`
	Mask           string `json:"mask,omitempty"`
}

type RowPolicyBody struct {
	RowFilter   string            `json:"row_filter,omitempty"`
	ColumnMasks map[string]string `json:"column_masks,omitempty"`
}

// Analytics wire types.
type Dimension struct {
	Name   string `json:"name"`
	Column string `json:"column"`
	Type   string `json:"type"`
}

type SemanticModel struct {
	ID          uuid.UUID   `json:"id"`
	OrgID       uuid.UUID   `json:"org_id"`
	WorkspaceID uuid.UUID   `json:"workspace_id"`
	DatasetID   uuid.UUID   `json:"dataset_id"`
	Name        string      `json:"name"`
	Slug        string      `json:"slug"`
	Description string      `json:"description"`
	Dimensions  []Dimension `json:"dimensions"`
	Status      string      `json:"status"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

type Metric struct {
	ID              uuid.UUID `json:"id"`
	OrgID           uuid.UUID `json:"org_id"`
	WorkspaceID     uuid.UUID `json:"workspace_id"`
	SemanticModelID uuid.UUID `json:"semantic_model_id"`
	Name            string    `json:"name"`
	Slug            string    `json:"slug"`
	Description     string    `json:"description"`
	Expression      string    `json:"expression"`
	Filters         []string  `json:"filters"`
	Format          string    `json:"format"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type QueryRun struct {
	ID            uuid.UUID  `json:"id"`
	OrgID         uuid.UUID  `json:"org_id"`
	WorkspaceID   uuid.UUID  `json:"workspace_id"`
	DatasetID     *uuid.UUID `json:"dataset_id,omitempty"`
	MetricID      *uuid.UUID `json:"metric_id,omitempty"`
	PrincipalID   uuid.UUID  `json:"principal_id"`
	PrincipalType string     `json:"principal_type"`
	SQL           string     `json:"sql"`
	ClickHouseID  string     `json:"clickhouse_id"`
	Status        string     `json:"status"`
	RowCount      int64      `json:"row_count"`
	BytesRead     int64      `json:"bytes_read"`
	DurationMS    int        `json:"duration_ms"`
	Columns       []string   `json:"columns"`
	ResultPreview any        `json:"result_preview,omitempty"`
	Error         string     `json:"error,omitempty"`
	Exported      bool       `json:"exported"`
	CreatedAt     time.Time  `json:"created_at"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
}

type QueryResult struct {
	RunID      uuid.UUID `json:"run_id"`
	Columns    []string  `json:"columns"`
	Types      []string  `json:"types,omitempty"`
	Rows       [][]any   `json:"rows"`
	RowCount   int64     `json:"row_count"`
	BytesRead  int64     `json:"bytes_read"`
	DurationMS int       `json:"duration_ms"`
	Truncated  bool      `json:"truncated"`
}

type MetricValue struct {
	MetricID uuid.UUID `json:"metric_id"`
	Value    any       `json:"value"`
	SQL      string    `json:"sql"`
	RunID    uuid.UUID `json:"run_id"`
}

type SemanticModelCreate struct {
	WorkspaceID uuid.UUID   `json:"workspace_id" binding:"required"`
	DatasetID   uuid.UUID   `json:"dataset_id" binding:"required"`
	Name        string      `json:"name" binding:"required"`
	Slug        string      `json:"slug,omitempty"`
	Description string      `json:"description,omitempty"`
	Dimensions  []Dimension `json:"dimensions,omitempty"`
}

type SemanticModelUpdate struct {
	Name        *string      `json:"name,omitempty"`
	Slug        *string      `json:"slug,omitempty"`
	Description *string      `json:"description,omitempty"`
	Dimensions  *[]Dimension `json:"dimensions,omitempty"`
}

type MetricCreate struct {
	WorkspaceID     uuid.UUID `json:"workspace_id" binding:"required"`
	SemanticModelID uuid.UUID `json:"semantic_model_id" binding:"required"`
	Name            string    `json:"name" binding:"required"`
	Slug            string    `json:"slug,omitempty"`
	Description     string    `json:"description,omitempty"`
	Expression      string    `json:"expression" binding:"required"`
	Filters         []string  `json:"filters,omitempty"`
	Format          string    `json:"format,omitempty"`
}

type MetricUpdate struct {
	Name        *string   `json:"name,omitempty"`
	Slug        *string   `json:"slug,omitempty"`
	Description *string   `json:"description,omitempty"`
	Expression  *string   `json:"expression,omitempty"`
	Filters     *[]string `json:"filters,omitempty"`
	Format      *string   `json:"format,omitempty"`
}

type MetricQueryBody struct {
	Filters []string `json:"filters,omitempty"`
	Limit   int      `json:"limit,omitempty"`
}

type QueryBody struct {
	WorkspaceID uuid.UUID  `json:"workspace_id" binding:"required"`
	DatasetID   *uuid.UUID `json:"dataset_id,omitempty"`
	SQL         string     `json:"sql" binding:"required"`
	Limit       int        `json:"limit,omitempty"`
	Export      bool       `json:"export,omitempty"`
}

// Dashboard wire types.
type Widget struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Title     string         `json:"title"`
	MetricID  *uuid.UUID     `json:"metric_id,omitempty"`
	DatasetID *uuid.UUID     `json:"dataset_id,omitempty"`
	SQL       string         `json:"sql,omitempty"`
	Width     int            `json:"width,omitempty"`
	Height    int            `json:"height,omitempty"`
	Config    map[string]any `json:"config,omitempty"`
}

type DashboardFilter struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Column  string `json:"column"`
	Type    string `json:"type"`
	Default any    `json:"default,omitempty"`
}

type DashboardDefinition struct {
	SchemaVersion int               `json:"schema_version"`
	Widgets       []Widget          `json:"widgets"`
	Filters       []DashboardFilter `json:"filters"`
}

type Dashboard struct {
	ID             uuid.UUID           `json:"id"`
	OrgID          uuid.UUID           `json:"org_id"`
	WorkspaceID    uuid.UUID           `json:"workspace_id"`
	Name           string              `json:"name"`
	Slug           string              `json:"slug"`
	Description    string              `json:"description"`
	Draft          DashboardDefinition `json:"draft"`
	Status         string              `json:"status"`
	ShareEnabled   bool                `json:"share_enabled"`
	ShareExpiresAt *time.Time          `json:"share_expires_at,omitempty"`
	CreatedAt      time.Time           `json:"created_at"`
	UpdatedAt      time.Time           `json:"updated_at"`
}

type DashboardVersion struct {
	ID          uuid.UUID           `json:"id"`
	DashboardID uuid.UUID           `json:"dashboard_id"`
	Version     int                 `json:"version"`
	Definition  DashboardDefinition `json:"definition"`
	Checksum    string              `json:"checksum"`
	PublishedBy uuid.UUID           `json:"published_by"`
	PublishedAt time.Time           `json:"published_at"`
}

type ScheduledReport struct {
	ID           uuid.UUID  `json:"id"`
	OrgID        uuid.UUID  `json:"org_id"`
	WorkspaceID  uuid.UUID  `json:"workspace_id"`
	DashboardID  uuid.UUID  `json:"dashboard_id"`
	Name         string     `json:"name"`
	ScheduleCron string     `json:"schedule_cron"`
	Recipients   []string   `json:"recipients"`
	Format       string     `json:"format"`
	Enabled      bool       `json:"enabled"`
	NextFireAt   *time.Time `json:"next_fire_at,omitempty"`
	LastRunAt    *time.Time `json:"last_run_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type DashboardCreate struct {
	WorkspaceID uuid.UUID           `json:"workspace_id" binding:"required"`
	Name        string              `json:"name" binding:"required"`
	Slug        string              `json:"slug,omitempty"`
	Description string              `json:"description,omitempty"`
	Definition  DashboardDefinition `json:"definition,omitempty"`
}

type DashboardUpdate struct {
	Name        *string              `json:"name,omitempty"`
	Slug        *string              `json:"slug,omitempty"`
	Description *string              `json:"description,omitempty"`
	Definition  *DashboardDefinition `json:"definition,omitempty"`
}

type ShareDashboardBody struct {
	Enabled   bool       `json:"enabled"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type ShareDashboardResult struct {
	Token     string     `json:"token"`
	Enabled   bool       `json:"enabled"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type WidgetDataBody struct {
	Filters map[string]string `json:"filters,omitempty"`
}

type WidgetData struct {
	WidgetID string `json:"widget_id"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	Data     any    `json:"data"`
}

type PublicDashboard struct {
	Dashboard Dashboard        `json:"dashboard"`
	Version   DashboardVersion `json:"version"`
}

type ScheduledReportCreate struct {
	WorkspaceID  uuid.UUID `json:"workspace_id" binding:"required"`
	DashboardID  uuid.UUID `json:"dashboard_id" binding:"required"`
	Name         string    `json:"name" binding:"required"`
	ScheduleCron string    `json:"schedule_cron" binding:"required"`
	Recipients   []string  `json:"recipients,omitempty"`
	Format       string    `json:"format,omitempty"`
	Enabled      *bool     `json:"enabled,omitempty"`
}

type ScheduledReportUpdate struct {
	Name         *string   `json:"name,omitempty"`
	ScheduleCron *string   `json:"schedule_cron,omitempty"`
	Recipients   *[]string `json:"recipients,omitempty"`
	Format       *string   `json:"format,omitempty"`
	Enabled      *bool     `json:"enabled,omitempty"`
}

type ListDataSourceKindsRequestObject struct{}
type ListDataSourceKindsResponseObject interface {
	VisitListDataSourceKindsResponse(w http.ResponseWriter) error
}
type ListDataSourceKinds200JSONResponse struct {
	Data []DataSourceKind `json:"data"`
}

func (response ListDataSourceKinds200JSONResponse) VisitListDataSourceKindsResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type ListDataSourcesParams struct {
	WorkspaceID uuid.UUID `form:"workspace_id"`
}
type ListDataSourcesRequestObject struct{ Params ListDataSourcesParams }
type ListDataSourcesResponseObject interface {
	VisitListDataSourcesResponse(w http.ResponseWriter) error
}
type ListDataSources200JSONResponse struct {
	Data []DataSource `json:"data"`
}

func (response ListDataSources200JSONResponse) VisitListDataSourcesResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type ListPipelinesParams struct {
	WorkspaceID uuid.UUID `form:"workspace_id"`
}
type ListPipelinesRequestObject struct{ Params ListPipelinesParams }
type ListPipelinesResponseObject interface {
	VisitListPipelinesResponse(w http.ResponseWriter) error
}
type ListPipelines200JSONResponse struct {
	Data []IngestionPipeline `json:"data"`
}

func (response ListPipelines200JSONResponse) VisitListPipelinesResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type ListPipelineRunsRequestObject struct {
	PipelineId uuid.UUID
	Params     ListPipelineRunsParams
}
type ListPipelineRunsParams struct {
	Limit *int `form:"limit,omitempty"`
}
type ListPipelineRunsResponseObject interface {
	VisitListPipelineRunsResponse(w http.ResponseWriter) error
}
type ListPipelineRuns200JSONResponse struct {
	Data []IngestionRun `json:"data"`
}

func (response ListPipelineRuns200JSONResponse) VisitListPipelineRunsResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type ListDatasetsParams struct {
	WorkspaceID uuid.UUID `form:"workspace_id"`
}
type ListDatasetsRequestObject struct{ Params ListDatasetsParams }
type ListDatasetsResponseObject interface {
	VisitListDatasetsResponse(w http.ResponseWriter) error
}
type ListDatasets200JSONResponse struct {
	Data []Dataset `json:"data"`
}

func (response ListDatasets200JSONResponse) VisitListDatasetsResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type ListDatasetVersionsRequestObject struct{ DatasetId uuid.UUID }
type ListDatasetVersionsResponseObject interface {
	VisitListDatasetVersionsResponse(w http.ResponseWriter) error
}
type ListDatasetVersions200JSONResponse struct {
	Data []DatasetVersion `json:"data"`
}

func (response ListDatasetVersions200JSONResponse) VisitListDatasetVersionsResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type ListDatasetLineageRequestObject struct{ DatasetId uuid.UUID }
type ListDatasetLineageResponseObject interface {
	VisitListDatasetLineageResponse(w http.ResponseWriter) error
}
type ListDatasetLineage200JSONResponse struct {
	Data []DatasetLineage `json:"data"`
}

func (response ListDatasetLineage200JSONResponse) VisitListDatasetLineageResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type ListColumnPoliciesRequestObject struct{ DatasetId uuid.UUID }
type ListColumnPoliciesResponseObject interface {
	VisitListColumnPoliciesResponse(w http.ResponseWriter) error
}
type ListColumnPolicies200JSONResponse struct {
	Data []ColumnPolicy `json:"data"`
}

func (response ListColumnPolicies200JSONResponse) VisitListColumnPoliciesResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type ListRowPoliciesRequestObject struct{ DatasetId uuid.UUID }
type ListRowPoliciesResponseObject interface {
	VisitListRowPoliciesResponse(w http.ResponseWriter) error
}
type ListRowPolicies200JSONResponse struct {
	Data []RowPolicy `json:"data"`
}

func (response ListRowPolicies200JSONResponse) VisitListRowPoliciesResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type ListSemanticModelMetricsRequestObject struct{ ModelId uuid.UUID }
type ListSemanticModelMetricsResponseObject interface {
	VisitListSemanticModelMetricsResponse(w http.ResponseWriter) error
}
type ListSemanticModelMetrics200JSONResponse struct {
	Data []Metric `json:"data"`
}

func (response ListSemanticModelMetrics200JSONResponse) VisitListSemanticModelMetricsResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type ListDashboardVersionsRequestObject struct{ DashboardId uuid.UUID }
type ListDashboardVersionsResponseObject interface {
	VisitListDashboardVersionsResponse(w http.ResponseWriter) error
}
type ListDashboardVersions200JSONResponse struct {
	Data []DashboardVersion `json:"data"`
}

func (response ListDashboardVersions200JSONResponse) VisitListDashboardVersionsResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type ListSemanticModelsParams struct {
	WorkspaceID uuid.UUID `form:"workspace_id"`
}
type ListSemanticModelsRequestObject struct{ Params ListSemanticModelsParams }
type ListSemanticModelsResponseObject interface {
	VisitListSemanticModelsResponse(w http.ResponseWriter) error
}
type ListSemanticModels200JSONResponse struct {
	Data []SemanticModel `json:"data"`
}

func (response ListSemanticModels200JSONResponse) VisitListSemanticModelsResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type ListMetricsParams struct {
	WorkspaceID uuid.UUID `form:"workspace_id"`
}
type ListMetricsRequestObject struct{ Params ListMetricsParams }
type ListMetricsResponseObject interface {
	VisitListMetricsResponse(w http.ResponseWriter) error
}
type ListMetrics200JSONResponse struct {
	Data []Metric `json:"data"`
}

func (response ListMetrics200JSONResponse) VisitListMetricsResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type ListQueriesParams struct {
	WorkspaceID uuid.UUID `form:"workspace_id"`
}
type ListQueriesRequestObject struct{ Params ListQueriesParams }
type ListQueriesResponseObject interface {
	VisitListQueriesResponse(w http.ResponseWriter) error
}
type ListQueries200JSONResponse struct {
	Data []QueryRun `json:"data"`
}

func (response ListQueries200JSONResponse) VisitListQueriesResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type ListDashboardsParams struct {
	WorkspaceID uuid.UUID `form:"workspace_id"`
}
type ListDashboardsRequestObject struct{ Params ListDashboardsParams }
type ListDashboardsResponseObject interface {
	VisitListDashboardsResponse(w http.ResponseWriter) error
}
type ListDashboards200JSONResponse struct {
	Data []Dashboard `json:"data"`
}

func (response ListDashboards200JSONResponse) VisitListDashboardsResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type ListScheduledReportsParams struct {
	WorkspaceID uuid.UUID `form:"workspace_id"`
}
type ListScheduledReportsRequestObject struct{ Params ListScheduledReportsParams }
type ListScheduledReportsResponseObject interface {
	VisitListScheduledReportsResponse(w http.ResponseWriter) error
}
type ListScheduledReports200JSONResponse struct {
	Data []ScheduledReport `json:"data"`
}

func (response ListScheduledReports200JSONResponse) VisitListScheduledReportsResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type GetDataSourceRequestObject struct{ DataSourceId uuid.UUID }
type GetDataSourceResponseObject interface {
	VisitGetDataSourceResponse(w http.ResponseWriter) error
}
type GetDataSource200JSONResponse DataSource

func (response GetDataSource200JSONResponse) VisitGetDataSourceResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type GetPipelineRequestObject struct{ PipelineId uuid.UUID }
type GetPipelineResponseObject interface {
	VisitGetPipelineResponse(w http.ResponseWriter) error
}
type GetPipeline200JSONResponse IngestionPipeline

func (response GetPipeline200JSONResponse) VisitGetPipelineResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type GetDatasetRequestObject struct{ DatasetId uuid.UUID }
type GetDatasetResponseObject interface {
	VisitGetDatasetResponse(w http.ResponseWriter) error
}
type GetDataset200JSONResponse Dataset

func (response GetDataset200JSONResponse) VisitGetDatasetResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type GetSemanticModelRequestObject struct{ ModelId uuid.UUID }
type GetSemanticModelResponseObject interface {
	VisitGetSemanticModelResponse(w http.ResponseWriter) error
}
type GetSemanticModel200JSONResponse SemanticModel

func (response GetSemanticModel200JSONResponse) VisitGetSemanticModelResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type GetMetricRequestObject struct{ MetricId uuid.UUID }
type GetMetricResponseObject interface {
	VisitGetMetricResponse(w http.ResponseWriter) error
}
type GetMetric200JSONResponse Metric

func (response GetMetric200JSONResponse) VisitGetMetricResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type GetQueryRequestObject struct{ QueryId uuid.UUID }
type GetQueryResponseObject interface {
	VisitGetQueryResponse(w http.ResponseWriter) error
}
type GetQuery200JSONResponse QueryRun

func (response GetQuery200JSONResponse) VisitGetQueryResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type GetDashboardRequestObject struct{ DashboardId uuid.UUID }
type GetDashboardResponseObject interface {
	VisitGetDashboardResponse(w http.ResponseWriter) error
}
type GetDashboard200JSONResponse Dashboard

func (response GetDashboard200JSONResponse) VisitGetDashboardResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type GetSharedDashboardRequestObject struct{ Token string }
type GetSharedDashboardResponseObject interface {
	VisitGetSharedDashboardResponse(w http.ResponseWriter) error
}
type GetSharedDashboard200JSONResponse PublicDashboard

func (response GetSharedDashboard200JSONResponse) VisitGetSharedDashboardResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type CreateDataSourceRequestObject struct{ Body *DataSourceCreate }
type CreateDataSourceResponseObject interface {
	VisitCreateDataSourceResponse(w http.ResponseWriter) error
}
type CreateDataSource201JSONResponse DataSource

func (response CreateDataSource201JSONResponse) VisitCreateDataSourceResponse(w http.ResponseWriter) error {
	return writeJSON(w, 201, response)
}

type CreatePipelineRequestObject struct{ Body *PipelineCreate }
type CreatePipelineResponseObject interface {
	VisitCreatePipelineResponse(w http.ResponseWriter) error
}
type CreatePipeline201JSONResponse IngestionPipeline

func (response CreatePipeline201JSONResponse) VisitCreatePipelineResponse(w http.ResponseWriter) error {
	return writeJSON(w, 201, response)
}

type CreateDatasetRequestObject struct{ Body *DatasetCreate }
type CreateDatasetResponseObject interface {
	VisitCreateDatasetResponse(w http.ResponseWriter) error
}
type CreateDataset201JSONResponse Dataset

func (response CreateDataset201JSONResponse) VisitCreateDatasetResponse(w http.ResponseWriter) error {
	return writeJSON(w, 201, response)
}

type CreateSemanticModelRequestObject struct{ Body *SemanticModelCreate }
type CreateSemanticModelResponseObject interface {
	VisitCreateSemanticModelResponse(w http.ResponseWriter) error
}
type CreateSemanticModel201JSONResponse SemanticModel

func (response CreateSemanticModel201JSONResponse) VisitCreateSemanticModelResponse(w http.ResponseWriter) error {
	return writeJSON(w, 201, response)
}

type CreateMetricRequestObject struct{ Body *MetricCreate }
type CreateMetricResponseObject interface {
	VisitCreateMetricResponse(w http.ResponseWriter) error
}
type CreateMetric201JSONResponse Metric

func (response CreateMetric201JSONResponse) VisitCreateMetricResponse(w http.ResponseWriter) error {
	return writeJSON(w, 201, response)
}

type CreateDashboardRequestObject struct{ Body *DashboardCreate }
type CreateDashboardResponseObject interface {
	VisitCreateDashboardResponse(w http.ResponseWriter) error
}
type CreateDashboard201JSONResponse Dashboard

func (response CreateDashboard201JSONResponse) VisitCreateDashboardResponse(w http.ResponseWriter) error {
	return writeJSON(w, 201, response)
}

type CreateScheduledReportRequestObject struct{ Body *ScheduledReportCreate }
type CreateScheduledReportResponseObject interface {
	VisitCreateScheduledReportResponse(w http.ResponseWriter) error
}
type CreateScheduledReport201JSONResponse ScheduledReport

func (response CreateScheduledReport201JSONResponse) VisitCreateScheduledReportResponse(w http.ResponseWriter) error {
	return writeJSON(w, 201, response)
}

type UpdateDataSourceRequestObject struct {
	DataSourceId uuid.UUID
	Body         *DataSourceUpdate
}
type UpdateDataSourceResponseObject interface {
	VisitUpdateDataSourceResponse(w http.ResponseWriter) error
}
type UpdateDataSource200JSONResponse DataSource

func (response UpdateDataSource200JSONResponse) VisitUpdateDataSourceResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type UpdatePipelineRequestObject struct {
	PipelineId uuid.UUID
	Body       *PipelineUpdate
}
type UpdatePipelineResponseObject interface {
	VisitUpdatePipelineResponse(w http.ResponseWriter) error
}
type UpdatePipeline200JSONResponse IngestionPipeline

func (response UpdatePipeline200JSONResponse) VisitUpdatePipelineResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type UpdateDatasetRequestObject struct {
	DatasetId uuid.UUID
	Body      *DatasetUpdate
}
type UpdateDatasetResponseObject interface {
	VisitUpdateDatasetResponse(w http.ResponseWriter) error
}
type UpdateDataset200JSONResponse Dataset

func (response UpdateDataset200JSONResponse) VisitUpdateDatasetResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type UpdateSemanticModelRequestObject struct {
	ModelId uuid.UUID
	Body    *SemanticModelUpdate
}
type UpdateSemanticModelResponseObject interface {
	VisitUpdateSemanticModelResponse(w http.ResponseWriter) error
}
type UpdateSemanticModel200JSONResponse SemanticModel

func (response UpdateSemanticModel200JSONResponse) VisitUpdateSemanticModelResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type UpdateMetricRequestObject struct {
	MetricId uuid.UUID
	Body     *MetricUpdate
}
type UpdateMetricResponseObject interface {
	VisitUpdateMetricResponse(w http.ResponseWriter) error
}
type UpdateMetric200JSONResponse Metric

func (response UpdateMetric200JSONResponse) VisitUpdateMetricResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type UpdateDashboardRequestObject struct {
	DashboardId uuid.UUID
	Body        *DashboardUpdate
}
type UpdateDashboardResponseObject interface {
	VisitUpdateDashboardResponse(w http.ResponseWriter) error
}
type UpdateDashboard200JSONResponse Dashboard

func (response UpdateDashboard200JSONResponse) VisitUpdateDashboardResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type DeleteDataSourceRequestObject struct{ DataSourceId uuid.UUID }
type DeleteDataSourceResponseObject interface {
	VisitDeleteDataSourceResponse(w http.ResponseWriter) error
}
type DeleteDataSource204Response struct{}

func (response DeleteDataSource204Response) VisitDeleteDataSourceResponse(w http.ResponseWriter) error {
	w.WriteHeader(204)
	return nil
}

type DeletePipelineRequestObject struct{ PipelineId uuid.UUID }
type DeletePipelineResponseObject interface {
	VisitDeletePipelineResponse(w http.ResponseWriter) error
}
type DeletePipeline204Response struct{}

func (response DeletePipeline204Response) VisitDeletePipelineResponse(w http.ResponseWriter) error {
	w.WriteHeader(204)
	return nil
}

type DeleteDatasetRequestObject struct{ DatasetId uuid.UUID }
type DeleteDatasetResponseObject interface {
	VisitDeleteDatasetResponse(w http.ResponseWriter) error
}
type DeleteDataset204Response struct{}

func (response DeleteDataset204Response) VisitDeleteDatasetResponse(w http.ResponseWriter) error {
	w.WriteHeader(204)
	return nil
}

type DeleteSemanticModelRequestObject struct{ ModelId uuid.UUID }
type DeleteSemanticModelResponseObject interface {
	VisitDeleteSemanticModelResponse(w http.ResponseWriter) error
}
type DeleteSemanticModel204Response struct{}

func (response DeleteSemanticModel204Response) VisitDeleteSemanticModelResponse(w http.ResponseWriter) error {
	w.WriteHeader(204)
	return nil
}

type DeleteMetricRequestObject struct{ MetricId uuid.UUID }
type DeleteMetricResponseObject interface {
	VisitDeleteMetricResponse(w http.ResponseWriter) error
}
type DeleteMetric204Response struct{}

func (response DeleteMetric204Response) VisitDeleteMetricResponse(w http.ResponseWriter) error {
	w.WriteHeader(204)
	return nil
}

type DeleteDashboardRequestObject struct{ DashboardId uuid.UUID }
type DeleteDashboardResponseObject interface {
	VisitDeleteDashboardResponse(w http.ResponseWriter) error
}
type DeleteDashboard204Response struct{}

func (response DeleteDashboard204Response) VisitDeleteDashboardResponse(w http.ResponseWriter) error {
	w.WriteHeader(204)
	return nil
}

type TestDataSourceRequestObject struct{ DataSourceId uuid.UUID }
type TestDataSourceResponseObject interface {
	VisitTestDataSourceResponse(w http.ResponseWriter) error
}
type TestDataSource200JSONResponse DataSourceTestResult

func (response TestDataSource200JSONResponse) VisitTestDataSourceResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type DiscoverDataSourceRequestObject struct{ DataSourceId uuid.UUID }
type DiscoverDataSourceResponseObject interface {
	VisitDiscoverDataSourceResponse(w http.ResponseWriter) error
}
type DiscoverDataSource200JSONResponse DataSourceDiscovery

func (response DiscoverDataSource200JSONResponse) VisitDiscoverDataSourceResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type RunPipelineRequestObject struct{ PipelineId uuid.UUID }
type RunPipelineResponseObject interface {
	VisitRunPipelineResponse(w http.ResponseWriter) error
}
type RunPipeline200JSONResponse IngestionRun

func (response RunPipeline200JSONResponse) VisitRunPipelineResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type PublishDatasetRequestObject struct{ DatasetId uuid.UUID }
type PublishDatasetResponseObject interface {
	VisitPublishDatasetResponse(w http.ResponseWriter) error
}
type PublishDataset200JSONResponse DatasetVersion

func (response PublishDataset200JSONResponse) VisitPublishDatasetResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type PreviewDatasetRequestObject struct {
	DatasetId uuid.UUID
	Body      *DatasetPreviewBody
}
type PreviewDatasetResponseObject interface {
	VisitPreviewDatasetResponse(w http.ResponseWriter) error
}
type PreviewDataset200JSONResponse DatasetPreview

func (response PreviewDataset200JSONResponse) VisitPreviewDatasetResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type PutColumnPolicyRequestObject struct {
	DatasetId uuid.UUID
	Column    string
	Body      *ColumnPolicyBody
}
type PutColumnPolicyResponseObject interface {
	VisitPutColumnPolicyResponse(w http.ResponseWriter) error
}
type PutColumnPolicy200JSONResponse ColumnPolicy

func (response PutColumnPolicy200JSONResponse) VisitPutColumnPolicyResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type DeleteColumnPolicyRequestObject struct {
	DatasetId uuid.UUID
	Column    string
}
type DeleteColumnPolicyResponseObject interface {
	VisitDeleteColumnPolicyResponse(w http.ResponseWriter) error
}
type DeleteColumnPolicy204Response struct{}

func (response DeleteColumnPolicy204Response) VisitDeleteColumnPolicyResponse(w http.ResponseWriter) error {
	w.WriteHeader(204)
	return nil
}

type PutRowPolicyRequestObject struct {
	DatasetId uuid.UUID
	RoleKey   string
	Body      *RowPolicyBody
}
type PutRowPolicyResponseObject interface {
	VisitPutRowPolicyResponse(w http.ResponseWriter) error
}
type PutRowPolicy200JSONResponse RowPolicy

func (response PutRowPolicy200JSONResponse) VisitPutRowPolicyResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type DeleteRowPolicyRequestObject struct {
	DatasetId uuid.UUID
	RoleKey   string
}
type DeleteRowPolicyResponseObject interface {
	VisitDeleteRowPolicyResponse(w http.ResponseWriter) error
}
type DeleteRowPolicy204Response struct{}

func (response DeleteRowPolicy204Response) VisitDeleteRowPolicyResponse(w http.ResponseWriter) error {
	w.WriteHeader(204)
	return nil
}

type QueryMetricRequestObject struct {
	MetricId uuid.UUID
	Body     *MetricQueryBody
}
type QueryMetricResponseObject interface {
	VisitQueryMetricResponse(w http.ResponseWriter) error
}
type QueryMetric200JSONResponse MetricValue

func (response QueryMetric200JSONResponse) VisitQueryMetricResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type ExecuteQueryRequestObject struct{ Body *QueryBody }
type ExecuteQueryResponseObject interface {
	VisitExecuteQueryResponse(w http.ResponseWriter) error
}
type ExecuteQuery200JSONResponse QueryResult

func (response ExecuteQuery200JSONResponse) VisitExecuteQueryResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type CancelQueryRequestObject struct{ QueryId uuid.UUID }
type CancelQueryResponseObject interface {
	VisitCancelQueryResponse(w http.ResponseWriter) error
}
type CancelQuery204Response struct{}

func (response CancelQuery204Response) VisitCancelQueryResponse(w http.ResponseWriter) error {
	w.WriteHeader(204)
	return nil
}

type ExportQueryRequestObject struct{ QueryId uuid.UUID }
type ExportQueryResponseObject interface {
	VisitExportQueryResponse(w http.ResponseWriter) error
}
type ExportQuery200TextResponse []byte

func (response ExportQuery200TextResponse) VisitExportQueryResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.WriteHeader(200)
	_, err := w.Write(response)
	return err
}

type PublishDashboardRequestObject struct{ DashboardId uuid.UUID }
type PublishDashboardResponseObject interface {
	VisitPublishDashboardResponse(w http.ResponseWriter) error
}
type PublishDashboard200JSONResponse DashboardVersion

func (response PublishDashboard200JSONResponse) VisitPublishDashboardResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type ShareDashboardRequestObject struct {
	DashboardId uuid.UUID
	Body        *ShareDashboardBody
}
type ShareDashboardResponseObject interface {
	VisitShareDashboardResponse(w http.ResponseWriter) error
}
type ShareDashboard200JSONResponse ShareDashboardResult

func (response ShareDashboard200JSONResponse) VisitShareDashboardResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type GetDashboardWidgetDataRequestObject struct {
	DashboardId uuid.UUID
	WidgetId    string
	Body        *WidgetDataBody
}
type GetDashboardWidgetDataResponseObject interface {
	VisitGetDashboardWidgetDataResponse(w http.ResponseWriter) error
}
type GetDashboardWidgetData200JSONResponse WidgetData

func (response GetDashboardWidgetData200JSONResponse) VisitGetDashboardWidgetDataResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type UpdateScheduledReportRequestObject struct {
	ReportId uuid.UUID
	Body     *ScheduledReportUpdate
}
type UpdateScheduledReportResponseObject interface {
	VisitUpdateScheduledReportResponse(w http.ResponseWriter) error
}
type UpdateScheduledReport200JSONResponse ScheduledReport

func (response UpdateScheduledReport200JSONResponse) VisitUpdateScheduledReportResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}

type DeleteScheduledReportRequestObject struct{ ReportId uuid.UUID }
type DeleteScheduledReportResponseObject interface {
	VisitDeleteScheduledReportResponse(w http.ResponseWriter) error
}
type DeleteScheduledReport204Response struct{}

func (response DeleteScheduledReport204Response) VisitDeleteScheduledReportResponse(w http.ResponseWriter) error {
	w.WriteHeader(204)
	return nil
}

type ListDataSourceSchemasRequestObject struct {
	DataSourceId uuid.UUID
	Params       ListDataSourceSchemasParams
}
type ListDataSourceSchemasParams struct {
	Limit *int `form:"limit,omitempty"`
}
type ListDataSourceSchemasResponseObject interface {
	VisitListDataSourceSchemasResponse(w http.ResponseWriter) error
}
type ListDataSourceSchemas200JSONResponse struct {
	Data []DataSourceSchema `json:"data"`
}

func (response ListDataSourceSchemas200JSONResponse) VisitListDataSourceSchemasResponse(w http.ResponseWriter) error {
	return writeJSON(w, 200, response)
}
