package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	model "github.com/nuriansyah/hugimethara/internal/modules/data_platform/model"
	dto "github.com/nuriansyah/hugimethara/internal/modules/data_platform/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/data_platform/repository"
	"github.com/nuriansyah/hugimethara/internal/modules/data_platform/repository/commands"
	"github.com/nuriansyah/hugimethara/internal/modules/data_platform/repository/queries"
	auditmodel "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/model"
	auditsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/services"
	secretsmodel "github.com/nuriansyah/hugimethara/internal/modules/integration/secrets/model"
	platformcron "github.com/nuriansyah/hugimethara/internal/platform/cron"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

func (s *DataPlatformService) loadPipeline(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (model.Pipeline, error) {
	var row repository.PipelineRow
	if err := tx.GetContext(ctx, &row, queries.SQL.Get("GetPipeline"), id); err != nil {
		if db.IsNoRows(err) {
			return model.Pipeline{}, problem.NotFound("The pipeline does not exist.")
		}
		return model.Pipeline{}, err
	}
	return row.Model(), nil
}

func (s *DataPlatformService) ListPipelines(ctx context.Context, workspaceID uuid.UUID) ([]model.Pipeline, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.require(ctx, p, "read", model.ResourceDataSource, &workspaceID, ""); err != nil {
		return nil, err
	}
	var rows []repository.PipelineRow
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, queries.SQL.Get("ListPipelines"), workspaceID)
	}); err != nil {
		return nil, err
	}
	out := make([]model.Pipeline, len(rows))
	for i := range rows {
		out[i] = rows[i].Model()
	}
	return out, nil
}

func (s *DataPlatformService) CreatePipeline(ctx context.Context, in dto.CreatePipelineInput) (model.Pipeline, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Pipeline{}, err
	}
	if in.WorkspaceID == uuid.Nil || in.DataSourceID == uuid.Nil {
		return model.Pipeline{}, problem.Validation(problem.FieldError{Field: "workspace_id", Code: "required", Message: "Workspace and data source are required."})
	}
	if err := s.require(ctx, p, "ingest", model.ResourceDataSource, &in.WorkspaceID, in.DataSourceID.String()); err != nil {
		return model.Pipeline{}, err
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return model.Pipeline{}, problem.Validation(problem.FieldError{Field: "name", Code: "required", Message: "A name is required."})
	}
	if in.Slug == "" {
		in.Slug = slugify(in.Name)
	}
	if err := model.ValidateSlug(in.Slug); err != nil {
		return model.Pipeline{}, problem.Validation(problem.FieldError{Field: "slug", Code: "invalid", Message: err.Error()})
	}
	if in.Mode == "" {
		in.Mode = "full"
	}
	if in.Mode != "full" && in.Mode != "incremental" {
		return model.Pipeline{}, problem.Validation(problem.FieldError{Field: "mode", Code: "invalid", Message: "Mode must be full or incremental."})
	}
	if err := model.ValidateTable(in.TargetTable); err != nil {
		return model.Pipeline{}, problem.Validation(problem.FieldError{Field: "target_table", Code: "invalid", Message: err.Error()})
	}
	if in.SourceObject == "" {
		return model.Pipeline{}, problem.Validation(problem.FieldError{Field: "source_object", Code: "required", Message: "A source table or object is required."})
	}
	if in.Mode == "incremental" {
		if in.WatermarkColumn == "" || in.WatermarkType == "" {
			return model.Pipeline{}, problem.Validation(problem.FieldError{Field: "watermark_column", Code: "required", Message: "Incremental pipelines need a watermark column and type."})
		}
		if err := model.ValidateColumn(in.WatermarkColumn); err != nil {
			return model.Pipeline{}, problem.Validation(problem.FieldError{Field: "watermark_column", Code: "invalid", Message: err.Error()})
		}
		switch in.WatermarkType {
		case "timestamp", "integer":
		default:
			return model.Pipeline{}, problem.Validation(problem.FieldError{Field: "watermark_type", Code: "invalid", Message: "Watermark type must be timestamp or integer."})
		}
	}
	if in.ScheduleCron != "" {
		if _, err := platformcron.Parse(in.ScheduleCron); err != nil {
			return model.Pipeline{}, problem.Validation(problem.FieldError{Field: "schedule_cron", Code: "invalid", Message: err.Error()})
		}
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	id := uuid.New()
	var out model.Pipeline
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		source, err := s.loadDataSource(ctx, tx, in.DataSourceID)
		if err != nil {
			return err
		}
		if source.WorkspaceID != in.WorkspaceID {
			return problem.Validation(problem.FieldError{Field: "data_source_id", Code: "invalid", Message: "The data source is in a different workspace."})
		}
		if !model.Available(source.Kind) {
			return sourceUnavailable(source.Kind)
		}
		config, _ := json.Marshal(in.Config)
		next := nextRun(in.ScheduleCron, enabled)
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertPipeline"), p.OrgID, id, in.WorkspaceID, in.DataSourceID,
			in.Name, in.Slug, in.TargetTable, in.Mode, in.SourceObject, in.WatermarkColumn, in.WatermarkType,
			in.ScheduleCron, config, enabled, next); err != nil {
			if isUniqueViolation(err) {
				return problem.NewProblem(http.StatusConflict, "slug_taken", "Conflict", "A pipeline with that slug already exists.")
			}
			return err
		}
		row, err := s.loadPipeline(ctx, tx, id)
		if err != nil {
			return err
		}
		out = row
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "pipeline.created",
			ResourceType: model.ResourceDataSource, ResourceID: id.String(), WorkspaceID: &in.WorkspaceID,
			Details: map[string]any{"source_id": in.DataSourceID.String(), "target_table": in.TargetTable}})
	}); err != nil {
		return model.Pipeline{}, err
	}
	return out, nil
}

func (s *DataPlatformService) GetPipeline(ctx context.Context, id uuid.UUID) (model.Pipeline, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Pipeline{}, err
	}
	var out model.Pipeline
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadPipeline(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "read", model.ResourceDataSource, &row.WorkspaceID, row.DataSourceID.String()); err != nil {
			return err
		}
		out = row
		return nil
	}); err != nil {
		return model.Pipeline{}, err
	}
	return out, nil
}

func (s *DataPlatformService) UpdatePipeline(ctx context.Context, id uuid.UUID, in dto.UpdatePipelineInput) (model.Pipeline, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Pipeline{}, err
	}
	var out model.Pipeline
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadPipeline(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "update", model.ResourceDataSource, &row.WorkspaceID, row.DataSourceID.String()); err != nil {
			return err
		}
		if in.Name != nil {
			name := strings.TrimSpace(*in.Name)
			if name == "" {
				return problem.Validation(problem.FieldError{Field: "name", Code: "required", Message: "A name is required."})
			}
			in.Name = &name
		}
		if in.Mode != nil && *in.Mode != "full" && *in.Mode != "incremental" {
			return problem.Validation(problem.FieldError{Field: "mode", Code: "invalid", Message: "Mode must be full or incremental."})
		}
		if in.TargetTable != nil {
			if err := model.ValidateTable(*in.TargetTable); err != nil {
				return problem.Validation(problem.FieldError{Field: "target_table", Code: "invalid", Message: err.Error()})
			}
		}
		mode := row.Mode
		if in.Mode != nil {
			mode = *in.Mode
		}
		watermarkColumn, watermarkType := row.WatermarkColumn, row.WatermarkType
		if in.WatermarkColumn != nil {
			watermarkColumn = *in.WatermarkColumn
		}
		if in.WatermarkType != nil {
			watermarkType = *in.WatermarkType
		}
		if mode == "incremental" {
			if watermarkColumn == "" || watermarkType == "" {
				return problem.Validation(problem.FieldError{Field: "watermark_column", Code: "required", Message: "Incremental pipelines need a watermark column and type."})
			}
			if err := model.ValidateColumn(watermarkColumn); err != nil {
				return problem.Validation(problem.FieldError{Field: "watermark_column", Code: "invalid", Message: err.Error()})
			}
			if watermarkType != "timestamp" && watermarkType != "integer" {
				return problem.Validation(problem.FieldError{Field: "watermark_type", Code: "invalid", Message: "Watermark type must be timestamp or integer."})
			}
		}
		schedule := row.ScheduleCron
		if in.ScheduleCron != nil {
			schedule = *in.ScheduleCron
			if schedule != "" {
				if _, err := platformcron.Parse(schedule); err != nil {
					return problem.Validation(problem.FieldError{Field: "schedule_cron", Code: "invalid", Message: err.Error()})
				}
			}
		}
		enabled := row.Enabled
		if in.Enabled != nil {
			enabled = *in.Enabled
		}
		var config any
		if in.Config != nil {
			raw, _ := json.Marshal(*in.Config)
			config = json.RawMessage(raw)
		}
		next := nextRun(schedule, enabled)
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("UpdatePipeline"), id, in.Name, in.TargetTable, in.Mode,
			in.SourceObject, in.WatermarkColumn, in.WatermarkType, in.ScheduleCron, config, in.Enabled, true, next); err != nil {
			return err
		}
		row, err = s.loadPipeline(ctx, tx, id)
		if err != nil {
			return err
		}
		out = row
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "pipeline.updated",
			ResourceType: model.ResourceDataSource, ResourceID: id.String(), WorkspaceID: &row.WorkspaceID})
	}); err != nil {
		return model.Pipeline{}, err
	}
	return out, nil
}

func (s *DataPlatformService) DeletePipeline(ctx context.Context, id uuid.UUID) error {
	p, err := s.principal(ctx)
	if err != nil {
		return err
	}
	return s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadPipeline(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "delete", model.ResourceDataSource, &row.WorkspaceID, row.DataSourceID.String()); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("DeletePipeline"), id); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "pipeline.deleted",
			ResourceType: model.ResourceDataSource, ResourceID: id.String(), WorkspaceID: &row.WorkspaceID})
	})
}

func (s *DataPlatformService) RunPipeline(ctx context.Context, id uuid.UUID) (model.IngestionRun, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.IngestionRun{}, err
	}
	var pipeline model.Pipeline
	var source model.DataSource
	var secret []byte
	var before string
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadPipeline(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "ingest", model.ResourceDataSource, &row.WorkspaceID, row.DataSourceID.String()); err != nil {
			return err
		}
		src, err := s.loadDataSource(ctx, tx, row.DataSourceID)
		if err != nil {
			return err
		}
		if !model.Available(src.Kind) {
			return sourceUnavailable(src.Kind)
		}
		if src.SecretID != nil {
			if s.opener == nil {
				return errors.New("data platform: secret resolver is not configured")
			}
			secret, err = s.opener.Open(ctx, tx, p.OrgID, *src.SecretID)
			if err != nil {
				return err
			}
		}
		var runRow repository.RunRow
		if err := tx.GetContext(ctx, &runRow, queries.SQL.Get("LatestSuccessfulRun"), id); err == nil {
			before = runRow.WatermarkAfter
		} else if !db.IsNoRows(err) {
			return err
		}
		pipeline, source = row, src
		return nil
	}); err != nil {
		return model.IngestionRun{}, err
	}
	defer secretsmodel.Zero(secret)
	runID := uuid.New()
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertRun"), p.OrgID, runID, id, pipeline.Mode, before, p.ID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, commands.SQL.Get("StartRun"), runID)
		return err
	}); err != nil {
		return model.IngestionRun{}, err
	}
	return s.executePipeline(ctx, p, pipeline, source, secret, runID, before)
}

func (s *DataPlatformService) ListRuns(ctx context.Context, pipelineID uuid.UUID, limit int) ([]model.IngestionRun, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	var out []model.IngestionRun
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		pipe, err := s.loadPipeline(ctx, tx, pipelineID)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "read", model.ResourceDataSource, &pipe.WorkspaceID, pipe.DataSourceID.String()); err != nil {
			return err
		}
		var rows []repository.RunRow
		if err := tx.SelectContext(ctx, &rows, queries.SQL.Get("ListRuns"), pipelineID, limit); err != nil {
			return err
		}
		out = make([]model.IngestionRun, len(rows))
		for i := range rows {
			out[i] = rows[i].Model()
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *DataPlatformService) executePipeline(ctx context.Context, p principal.Principal, pipeline model.Pipeline, source model.DataSource, secret []byte, runID uuid.UUID, before string) (model.IngestionRun, error) {
	result, err := s.ingest(ctx, p, pipeline, source, secret, before)
	if err != nil {
		finishErr := s.finishRun(ctx, p, pipeline, runID, before, "", 0, 0, nil, map[string]any{"code": "ingestion_failed", "message": err.Error()})
		if finishErr != nil {
			return model.IngestionRun{}, fmt.Errorf("ingestion failed (%v) and recording the failure also failed: %w", err, finishErr)
		}
		return model.IngestionRun{}, err
	}
	if err := s.finishRun(ctx, p, pipeline, runID, before, result.watermark, result.read, result.written, result.drift, nil); err != nil {
		return model.IngestionRun{}, err
	}
	var out model.IngestionRun
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		var row repository.RunRow
		if err := tx.GetContext(ctx, &row, queries.SQL.Get("GetRun"), runID); err != nil {
			return err
		}
		out = row.Model()
		return nil
	}); err != nil {
		return model.IngestionRun{}, err
	}
	return out, nil
}

func (s *DataPlatformService) finishRun(ctx context.Context, p principal.Principal, pipeline model.Pipeline, runID uuid.UUID, before, after string, read, written int64, drift []string, failure map[string]any) error {
	return s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		status := "succeeded"
		if failure != nil {
			status = "failed"
		}
		driftRaw, _ := json.Marshal(drift)
		var errRaw any
		if failure != nil {
			raw, _ := json.Marshal(failure)
			errRaw = json.RawMessage(raw)
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("FinishRun"), runID, status, after, read, written, driftRaw, errRaw); err != nil {
			return err
		}
		next := nextRun(pipeline.ScheduleCron, pipeline.Enabled)
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("TouchPipelineRun"), pipeline.ID, next); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "pipeline." + status,
			ResourceType: model.ResourceDataSource, ResourceID: pipeline.ID.String(), WorkspaceID: &pipeline.WorkspaceID,
			Details: map[string]any{"rows_read": read, "rows_written": written, "drift": len(drift)}})
	})
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

func sourceUnavailable(kind string) error {
	info, _ := model.KindInfo(kind)
	reason := info.Reason
	if reason == "" {
		reason = "The adapter is not available yet."
	}
	return problem.Validation(problem.FieldError{Field: "kind", Code: "source_kind_unavailable", Message: reason})
}
