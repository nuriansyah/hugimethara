package services

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

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
	auditmodel "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/model"
	auditsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/services"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
	"github.com/nuriansyah/hugimethara/internal/platform/sqlguard"
)

const (
	defaultQueryLimit = 200
	maxQueryLimit     = 1000
	maxExportLimit    = 10_000
)

type preparedQuery struct {
	sql        string
	datasetIDs []uuid.UUID
	masks      map[string]string
}

func (s *AnalyticsService) Execute(ctx context.Context, in dto.QueryInput) (model.QueryResult, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.QueryResult{}, err
	}
	if in.WorkspaceID == uuid.Nil {
		return model.QueryResult{}, problem.Validation(problem.FieldError{Field: "workspace_id", Code: "required", Message: "A workspace is required."})
	}
	if err := s.require(ctx, p, "execute", model.ResourceQuery, &in.WorkspaceID, ""); err != nil {
		return model.QueryResult{}, err
	}
	limit := in.Limit
	if limit <= 0 {
		limit = defaultQueryLimit
	}
	if limit > maxQueryLimit {
		limit = maxQueryLimit
	}
	prepared, err := s.prepare(ctx, p, in.WorkspaceID, in.DatasetID, in.SQL, limit)
	if err != nil {
		return model.QueryResult{}, err
	}
	runID := uuid.New()
	chID := uuid.NewString()
	if err := s.insertRun(ctx, p, runID, in.WorkspaceID, in.DatasetID, nil, in.SQL, chID, in.Export); err != nil {
		return model.QueryResult{}, err
	}
	result, err := s.runPrepared(ctx, p, prepared, chID, limit)
	if err != nil {
		_ = s.finishRun(ctx, p, runID, "failed", 0, 0, 0, nil, nil, err.Error())
		return model.QueryResult{}, err
	}
	if in.Export {
		if err := s.require(ctx, p, "export", model.ResourceQuery, &in.WorkspaceID, ""); err != nil {
			_ = s.finishRun(ctx, p, runID, "failed", 0, 0, 0, nil, nil, "export not permitted")
			return model.QueryResult{}, err
		}
	}
	preview := result.Rows
	if len(preview) > 100 {
		preview = preview[:100]
	}
	previewJSON, _ := json.Marshal(preview)
	if err := s.finishRun(ctx, p, runID, "succeeded", result.RowCount, result.BytesRead, result.DurationMS, previewJSON, result.Columns, ""); err != nil {
		return model.QueryResult{}, err
	}
	action := "query.executed"
	if in.Export {
		action = "query.exported"
	}
	_ = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		if in.Export {
			if _, err := tx.ExecContext(ctx, commands.SQL.Get("MarkQueryExported"), runID); err != nil {
				return err
			}
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: action,
			ResourceType: model.ResourceQuery, ResourceID: runID.String(), WorkspaceID: &in.WorkspaceID,
			Details: map[string]any{"rows": result.RowCount, "bytes": result.BytesRead, "dataset_id": in.DatasetID}})
	})
	result.RunID = runID
	result.Truncated = result.RowCount >= int64(limit)
	return result, nil
}

func (s *AnalyticsService) prepare(ctx context.Context, p principal.Principal, workspaceID uuid.UUID, datasetID *uuid.UUID, sqlText string, limit int) (preparedQuery, error) {
	if err := sqlguard.ValidateReadOnly(sqlText); err != nil {
		return preparedQuery{}, problem.Validation(problem.FieldError{Field: "sql", Code: "invalid", Message: err.Error()})
	}
	allowed, err := s.dp.AllowedTables(ctx)
	if err != nil {
		return preparedQuery{}, err
	}
	if err := sqlguard.ValidateTables(sqlText, allowed); err != nil {
		return preparedQuery{}, problem.Validation(problem.FieldError{Field: "sql", Code: "invalid", Message: err.Error()})
	}
	views, err := s.dp.PublishedViews(ctx)
	if err != nil {
		return preparedQuery{}, err
	}
	var datasetIDs []uuid.UUID
	hasRaw := false
	for _, table := range sqlguard.ReferencedTables(sqlText) {
		bare := table
		if i := strings.LastIndex(bare, "."); i >= 0 {
			bare = bare[i+1:]
		}
		if id, ok := views[bare]; ok {
			datasetIDs = append(datasetIDs, id)
		} else {
			hasRaw = true
		}
	}
	if datasetID != nil {
		datasetIDs = []uuid.UUID{*datasetID}
	}
	hasExport := s.can(ctx, p, "export", datamodel.ResourceDataset, &workspaceID)
	if hasRaw && !hasExport {
		return preparedQuery{}, problem.NewProblem(http.StatusForbidden, "forbidden", "Forbidden",
			"Raw ingestion tables require the dataset:export permission. Query a published dataset instead.")
	}
	if len(datasetIDs) > 1 && !hasExport {
		return preparedQuery{}, problem.Validation(problem.FieldError{Field: "sql", Code: "multi_dataset",
			Message: "Queries spanning multiple datasets are not supported in the workbench; model them as a dataset."})
	}
	out := preparedQuery{datasetIDs: datasetIDs}
	if len(datasetIDs) == 1 {
		filter, masks, err := s.dp.RowFilterFor(ctx, datasetIDs[0])
		if err != nil {
			return preparedQuery{}, err
		}
		out.masks = masks
		body := strings.TrimSuffix(strings.TrimSpace(sqlText), ";")
		out.sql = "SELECT * FROM (" + body + ") AS hugi_q"
		if filter != "" {
			out.sql += " WHERE (" + filter + ")"
		}
		out.sql += fmt.Sprintf(" LIMIT %d", limit)
	} else {
		body := strings.TrimSuffix(strings.TrimSpace(sqlText), ";")
		out.sql = "SELECT * FROM (" + body + ") AS hugi_q LIMIT " + fmt.Sprintf("%d", limit)
	}
	return out, nil
}

func (s *AnalyticsService) runPrepared(ctx context.Context, p principal.Principal, prepared preparedQuery, chID string, limit int) (model.QueryResult, error) {
	if !s.ch.Enabled() {
		return model.QueryResult{}, problem.NewProblem(http.StatusServiceUnavailable, "analytics_unavailable", "Analytics unavailable", "The analytical store is not configured.")
	}
	dbName, err := s.ch.EnsureOrgDatabase(ctx, p.OrgID)
	if err != nil {
		return model.QueryResult{}, analyticsError(err)
	}
	start := time.Now()
	result, err := s.ch.Query(ctx, dbName, chID, prepared.sql, clickhouse.Limits{
		MaxExecutionTime: 20 * time.Second, MaxMemoryBytes: 512 << 20, MaxResultRows: int64(limit),
	})
	if err != nil {
		return model.QueryResult{}, problem.Validation(problem.FieldError{Field: "sql", Code: "invalid", Message: err.Error()})
	}
	duration := int(time.Since(start).Milliseconds())
	rows := result.Rows
	if len(prepared.datasetIDs) == 1 {
		policies, err := s.dp.ColumnPolicies(ctx, prepared.datasetIDs[0])
		if err != nil {
			return model.QueryResult{}, err
		}
		rows = maskRows(result.Columns, rows, policies, prepared.masks)
	}
	return model.QueryResult{Columns: result.Columns, Types: result.Types, Rows: rows, RowCount: result.RowCount,
		BytesRead: result.Bytes, DurationMS: duration}, nil
}

func maskRows(columns []string, rows [][]any, policies []datamodel.ColumnPolicy, roleMasks map[string]string) [][]any {
	maskByColumn := map[string]string{}
	for _, policy := range policies {
		mask := policy.Mask
		if mask == "" || mask == "none" {
			mask = sqlguard.MaskFor(policy.ColumnName, policy.Classification, roleMasks)
		}
		maskByColumn[policy.ColumnName] = mask
	}
	for column, mask := range roleMasks {
		maskByColumn[column] = mask
	}
	out := make([][]any, len(rows))
	for i, row := range rows {
		masked := make([]any, len(row))
		for j, value := range row {
			if j < len(columns) {
				masked[j] = sqlguard.MaskValue(maskByColumn[columns[j]], value)
			} else {
				masked[j] = value
			}
		}
		out[i] = masked
	}
	return out
}

func (s *AnalyticsService) insertRun(ctx context.Context, p principal.Principal, runID, workspaceID uuid.UUID, datasetID, metricID *uuid.UUID, sqlText, chID string, exported bool) error {
	principalType := string(p.Type)
	if principalType != "user" && principalType != "service_account" {
		principalType = "user"
	}
	return s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		_, err := tx.ExecContext(ctx, commands.SQL.Get("InsertQueryRun"), p.OrgID, runID, workspaceID, datasetID,
			metricID, p.ID, principalType, sqlText, chID, exported)
		return err
	})
}

func (s *AnalyticsService) finishRun(ctx context.Context, p principal.Principal, runID uuid.UUID, status string, rows, bytesRead int64, duration int, preview any, columns []string, errText string) error {
	if len(errText) > 1000 {
		errText = errText[:1000]
	}
	columnsJSON, _ := json.Marshal(columns)
	return s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		_, err := tx.ExecContext(ctx, commands.SQL.Get("FinishQueryRun"), runID, status, rows, bytesRead,
			duration, preview, columnsJSON, errText)
		return err
	})
}

func (s *AnalyticsService) Cancel(ctx context.Context, runID uuid.UUID) error {
	p, err := s.principal(ctx)
	if err != nil {
		return err
	}
	var run model.QueryRun
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadRun(ctx, tx, runID)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "execute", model.ResourceQuery, &row.WorkspaceID, runID.String()); err != nil {
			return err
		}
		run = row
		return nil
	}); err != nil {
		return err
	}
	if run.Status != "running" {
		return problem.NewProblem(http.StatusConflict, "not_running", "Conflict", "The query is no longer running.")
	}
	if s.ch.Enabled() && run.ClickHouseID != "" {
		if err := s.ch.Cancel(ctx, run.ClickHouseID); err != nil {
			return analyticsError(err)
		}
	}
	if err := s.finishRun(ctx, p, runID, "cancelled", 0, 0, 0, nil, nil, "cancelled by user"); err != nil {
		return err
	}
	return auditsvc.RecordStandalone(ctx, s.pool, auditmodel.Event{OrgID: p.OrgID, Action: "query.cancelled",
		ResourceType: model.ResourceQuery, ResourceID: runID.String(), WorkspaceID: &run.WorkspaceID})
}

func (s *AnalyticsService) ExportCSV(ctx context.Context, runID uuid.UUID) ([]byte, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	var run model.QueryRun
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadRun(ctx, tx, runID)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "export", model.ResourceQuery, &row.WorkspaceID, runID.String()); err != nil {
			return err
		}
		run = row
		return nil
	}); err != nil {
		return nil, err
	}
	if run.Status != "succeeded" {
		return nil, problem.NewProblem(http.StatusConflict, "not_succeeded", "Conflict", "Only successful queries can be exported.")
	}
	prepared, err := s.prepare(ctx, p, run.WorkspaceID, run.DatasetID, run.SQL, maxExportLimit)
	if err != nil {
		return nil, err
	}
	result, err := s.runPrepared(ctx, p, prepared, uuid.NewString(), maxExportLimit)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(result.Columns); err != nil {
		return nil, err
	}
	for _, row := range result.Rows {
		record := make([]string, len(row))
		for i, value := range row {
			if value != nil {
				record[i] = fmt.Sprint(value)
			}
		}
		if err := w.Write(record); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("MarkQueryExported"), runID); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "query.exported",
			ResourceType: model.ResourceQuery, ResourceID: runID.String(), WorkspaceID: &run.WorkspaceID,
			Details: map[string]any{"rows": result.RowCount}})
	}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *AnalyticsService) ListQueryRuns(ctx context.Context, workspaceID uuid.UUID, limit int) ([]model.QueryRun, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.require(ctx, p, "read", model.ResourceQuery, &workspaceID, ""); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	var rows []repository.QueryRunRow
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, queries.SQL.Get("ListQueryRuns"), workspaceID, limit)
	}); err != nil {
		return nil, err
	}
	out := make([]model.QueryRun, len(rows))
	for i := range rows {
		out[i] = rows[i].Model()
	}
	return out, nil
}

func (s *AnalyticsService) GetQueryRun(ctx context.Context, runID uuid.UUID) (model.QueryRun, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.QueryRun{}, err
	}
	var out model.QueryRun
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadRun(ctx, tx, runID)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "read", model.ResourceQuery, &row.WorkspaceID, runID.String()); err != nil {
			return err
		}
		out = row
		return nil
	}); err != nil {
		return model.QueryRun{}, err
	}
	return out, nil
}

func (s *AnalyticsService) loadRun(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (model.QueryRun, error) {
	var row repository.QueryRunRow
	if err := tx.GetContext(ctx, &row, queries.SQL.Get("GetQueryRun"), id); err != nil {
		if db.IsNoRows(err) {
			return model.QueryRun{}, problem.NotFound("The query run does not exist.")
		}
		return model.QueryRun{}, err
	}
	return row.Model(), nil
}

func (s *AnalyticsService) can(ctx context.Context, p principal.Principal, action, resourceType string, workspaceID *uuid.UUID) bool {
	_, err := authzsvc.Require(ctx, s.az, action, authzmodel.Resource{Type: resourceType, OrgID: p.OrgID, WorkspaceID: workspaceID})
	return err == nil
}

func analyticsError(err error) error {
	if errors.Is(err, clickhouse.ErrUnavailable) {
		return problem.NewProblem(http.StatusServiceUnavailable, "analytics_unavailable", "Analytics unavailable", "The analytical store is not configured.")
	}
	return err
}
