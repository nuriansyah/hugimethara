package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/infrastructure/analytics/clickhouse"
	authzmodel "github.com/nuriansyah/hugimethara/internal/modules/authorization/model"
	model "github.com/nuriansyah/hugimethara/internal/modules/data_platform/model"
	dto "github.com/nuriansyah/hugimethara/internal/modules/data_platform/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/data_platform/repository"
	"github.com/nuriansyah/hugimethara/internal/modules/data_platform/repository/commands"
	"github.com/nuriansyah/hugimethara/internal/modules/data_platform/repository/queries"
	auditmodel "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/model"
	auditsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/services"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
	"github.com/nuriansyah/hugimethara/internal/platform/sqlguard"
)

const previewRows = 200

func (s *DataPlatformService) loadDataset(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (model.Dataset, error) {
	var row repository.DatasetRow
	if err := tx.GetContext(ctx, &row, queries.SQL.Get("GetDataset"), id); err != nil {
		if db.IsNoRows(err) {
			return model.Dataset{}, problem.NotFound("The dataset does not exist.")
		}
		return model.Dataset{}, err
	}
	return row.Model(), nil
}

func (s *DataPlatformService) ListDatasets(ctx context.Context, workspaceID uuid.UUID) ([]model.Dataset, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.require(ctx, p, "read", model.ResourceDataset, &workspaceID, ""); err != nil {
		return nil, err
	}
	var rows []repository.DatasetRow
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, queries.SQL.Get("ListDatasets"), workspaceID)
	}); err != nil {
		return nil, err
	}
	out := make([]model.Dataset, len(rows))
	for i := range rows {
		out[i] = rows[i].Model()
	}
	return out, nil
}

func (s *DataPlatformService) CreateDataset(ctx context.Context, in dto.CreateDatasetInput) (model.Dataset, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Dataset{}, err
	}
	if in.WorkspaceID == uuid.Nil {
		return model.Dataset{}, problem.Validation(problem.FieldError{Field: "workspace_id", Code: "required", Message: "A workspace is required."})
	}
	if err := s.require(ctx, p, "create", model.ResourceDataset, &in.WorkspaceID, ""); err != nil {
		return model.Dataset{}, err
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return model.Dataset{}, problem.Validation(problem.FieldError{Field: "name", Code: "required", Message: "A name is required."})
	}
	if in.Slug == "" {
		in.Slug = slugify(in.Name)
	}
	if err := model.ValidateSlug(in.Slug); err != nil {
		return model.Dataset{}, problem.Validation(problem.FieldError{Field: "slug", Code: "invalid", Message: err.Error()})
	}
	if strings.TrimSpace(in.SQL) == "" {
		in.SQL = "SELECT 1 AS ok"
	}
	if err := s.validateDraftSQL(ctx, p, in.SQL); err != nil {
		return model.Dataset{}, err
	}
	id := uuid.New()
	var out model.Dataset
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertDataset"), p.OrgID, id, in.WorkspaceID, in.Name,
			in.Slug, in.Description, in.Owner, in.SQL, "draft", p.ID); err != nil {
			if isUniqueViolation(err) {
				return problem.NewProblem(http.StatusConflict, "slug_taken", "Conflict", "A dataset with that slug already exists in the workspace.")
			}
			return err
		}
		row, err := s.loadDataset(ctx, tx, id)
		if err != nil {
			return err
		}
		out = row
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "dataset.created",
			ResourceType: model.ResourceDataset, ResourceID: id.String(), WorkspaceID: &in.WorkspaceID,
			Details: map[string]any{"slug": in.Slug}})
	}); err != nil {
		return model.Dataset{}, err
	}
	return out, nil
}

func (s *DataPlatformService) GetDataset(ctx context.Context, id uuid.UUID) (model.Dataset, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Dataset{}, err
	}
	var out model.Dataset
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadDataset(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "read", model.ResourceDataset, &row.WorkspaceID, id.String()); err != nil {
			return err
		}
		out = row
		return nil
	}); err != nil {
		return model.Dataset{}, err
	}
	return out, nil
}

func (s *DataPlatformService) UpdateDataset(ctx context.Context, id uuid.UUID, in dto.UpdateDatasetInput) (model.Dataset, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Dataset{}, err
	}
	if in.SQL != nil {
		if err := s.validateDraftSQL(ctx, p, *in.SQL); err != nil {
			return model.Dataset{}, err
		}
	}
	var out model.Dataset
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadDataset(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "update", model.ResourceDataset, &row.WorkspaceID, id.String()); err != nil {
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
			if err := model.ValidateSlug(*in.Slug); err != nil {
				return problem.Validation(problem.FieldError{Field: "slug", Code: "invalid", Message: err.Error()})
			}
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("UpdateDataset"), id, in.Name, in.Slug, in.Description,
			in.Owner, in.SQL, nil); err != nil {
			if isUniqueViolation(err) {
				return problem.NewProblem(http.StatusConflict, "slug_taken", "Conflict", "A dataset with that slug already exists in the workspace.")
			}
			return err
		}
		row, err = s.loadDataset(ctx, tx, id)
		if err != nil {
			return err
		}
		out = row
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "dataset.updated",
			ResourceType: model.ResourceDataset, ResourceID: id.String(), WorkspaceID: &row.WorkspaceID})
	}); err != nil {
		return model.Dataset{}, err
	}
	return out, nil
}

func (s *DataPlatformService) DeleteDataset(ctx context.Context, id uuid.UUID) error {
	p, err := s.principal(ctx)
	if err != nil {
		return err
	}
	var view string
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadDataset(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "delete", model.ResourceDataset, &row.WorkspaceID, id.String()); err != nil {
			return err
		}
		view = model.DatasetView(row.ID, row.Slug)
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("DeleteDataset"), id); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "dataset.deleted",
			ResourceType: model.ResourceDataset, ResourceID: id.String(), WorkspaceID: &row.WorkspaceID})
	}); err != nil {
		return err
	}
	if s.ch.Enabled() {
		dbName, err := s.ch.EnsureOrgDatabase(ctx, p.OrgID)
		if err == nil {
			_ = s.ch.Exec(ctx, "", "DROP VIEW IF EXISTS "+sqlguard.QuoteIdent(dbName)+"."+sqlguard.QuoteIdent(view))
		}
	}
	return nil
}

func (s *DataPlatformService) PublishDataset(ctx context.Context, id uuid.UUID) (model.DatasetVersion, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.DatasetVersion{}, err
	}
	if !s.ch.Enabled() {
		return model.DatasetVersion{}, problem.NewProblem(http.StatusServiceUnavailable, "analytics_unavailable", "Analytics unavailable", "The analytical store is not configured.")
	}
	var dataset model.Dataset
	var allowed []string
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadDataset(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "publish", model.ResourceDataset, &row.WorkspaceID, id.String()); err != nil {
			return err
		}
		dataset = row
		return nil
	}); err != nil {
		return model.DatasetVersion{}, err
	}
	allowed, err = s.AllowedTables(ctx)
	if err != nil {
		return model.DatasetVersion{}, err
	}
	if err := sqlguard.ValidateDatasetSQL(dataset.DraftSQL, allowed); err != nil {
		return model.DatasetVersion{}, problem.Validation(problem.FieldError{Field: "sql", Code: "invalid", Message: err.Error()})
	}
	dbName, err := s.ch.EnsureOrgDatabase(ctx, p.OrgID)
	if err != nil {
		return model.DatasetVersion{}, analyticsError(err)
	}
	probe := "SELECT * FROM (" + strings.TrimSuffix(strings.TrimSpace(dataset.DraftSQL), ";") + ") LIMIT 0"
	result, err := s.ch.Query(ctx, dbName, "", probe, clickhouse.Limits{MaxResultRows: 1})
	if err != nil {
		return model.DatasetVersion{}, problem.Validation(problem.FieldError{Field: "sql", Code: "invalid", Message: "The SQL could not be compiled: " + err.Error()})
	}
	columns := make([]model.SchemaColumn, len(result.Columns))
	for i, name := range result.Columns {
		columns[i] = model.SchemaColumn{Name: name, Nullable: true}
	}
	if len(result.Types) == len(columns) {
		for i, typ := range result.Types {
			columns[i].Type = typ
			columns[i].Nullable = strings.HasPrefix(typ, "Nullable(")
		}
	}
	view := model.DatasetView(dataset.ID, dataset.Slug)
	if err := s.ch.Exec(ctx, "", "CREATE OR REPLACE VIEW "+sqlguard.QuoteIdent(dbName)+"."+sqlguard.QuoteIdent(view)+" AS "+dataset.DraftSQL); err != nil {
		return model.DatasetVersion{}, problem.Validation(problem.FieldError{Field: "sql", Code: "invalid", Message: "Could not create the dataset view: " + err.Error()})
	}
	var version model.DatasetVersion
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		number := 1
		var latest repository.DatasetVersionRow
		if err := tx.GetContext(ctx, &latest, queries.SQL.Get("LatestDatasetVersion"), dataset.ID); err == nil {
			number = latest.Version + 1
		} else if !db.IsNoRows(err) {
			return err
		}
		rawColumns, _ := json.Marshal(columns)
		checksum := sqlguard.Checksum(map[string]any{"sql": dataset.DraftSQL, "columns": columns})
		versionID := uuid.New()
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertDatasetVersion"), p.OrgID, versionID, dataset.ID,
			number, dataset.DraftSQL, rawColumns, checksum, p.ID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("UpdateDataset"), dataset.ID, nil, nil, nil, nil, nil, "published"); err != nil {
			return err
		}
		if err := s.replaceLineage(ctx, tx, p, dataset); err != nil {
			return err
		}
		var row repository.DatasetVersionRow
		if err := tx.GetContext(ctx, &row, queries.SQL.Get("GetDatasetVersion"), versionID); err != nil {
			return err
		}
		version = row.Model()
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "dataset.published",
			ResourceType: model.ResourceDataset, ResourceID: dataset.ID.String(), WorkspaceID: &dataset.WorkspaceID,
			Details: map[string]any{"version": number, "view": view}})
	}); err != nil {
		return model.DatasetVersion{}, err
	}
	return version, nil
}

func (s *DataPlatformService) replaceLineage(ctx context.Context, tx *sqlx.Tx, p principal.Principal, dataset model.Dataset) error {
	if _, err := tx.ExecContext(ctx, commands.SQL.Get("DeleteLineage"), dataset.ID); err != nil {
		return err
	}
	type published struct {
		ID   uuid.UUID `db:"id"`
		Slug string    `db:"slug"`
	}
	var datasets []published
	if err := tx.SelectContext(ctx, &datasets, queries.SQL.Get("PublishedDatasets"), p.OrgID); err != nil {
		return err
	}
	byView := map[string]uuid.UUID{}
	for _, d := range datasets {
		byView[model.DatasetView(d.ID, d.Slug)] = d.ID
	}
	var rawTables []string
	if err := tx.SelectContext(ctx, &rawTables, queries.SQL.Get("RawTables"), p.OrgID); err != nil {
		return err
	}
	raw := map[string]bool{}
	for _, name := range rawTables {
		raw[name] = true
	}
	for _, table := range sqlguard.ReferencedTables(dataset.DraftSQL) {
		bare := table
		if i := strings.LastIndex(bare, "."); i >= 0 {
			bare = bare[i+1:]
		}
		var kind string
		var upstream uuid.UUID
		if id, ok := byView[bare]; ok && id != dataset.ID {
			kind, upstream = "dataset", id
		} else if raw[bare] {
			var pipe struct {
				ID           uuid.UUID `db:"id"`
				DataSourceID uuid.UUID `db:"data_source_id"`
			}
			if err := tx.GetContext(ctx, &pipe, queries.SQL.Get("PipelineByTarget"), p.OrgID, bare); err != nil {
				continue
			}
			kind, upstream = "datasource", pipe.DataSourceID
		} else {
			continue
		}
		columns, _ := json.Marshal([]string{})
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertLineage"), p.OrgID, uuid.New(), dataset.ID, kind, upstream, columns); err != nil {
			return err
		}
	}
	return nil
}

func (s *DataPlatformService) ListDatasetVersions(ctx context.Context, id uuid.UUID) ([]model.DatasetVersion, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	var out []model.DatasetVersion
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadDataset(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "read", model.ResourceDataset, &row.WorkspaceID, id.String()); err != nil {
			return err
		}
		var rows []repository.DatasetVersionRow
		if err := tx.SelectContext(ctx, &rows, queries.SQL.Get("ListDatasetVersions"), id); err != nil {
			return err
		}
		out = make([]model.DatasetVersion, len(rows))
		for i := range rows {
			out[i] = rows[i].Model()
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *DataPlatformService) PreviewDataset(ctx context.Context, id uuid.UUID, sqlText string) (Preview, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return Preview{}, err
	}
	if !s.ch.Enabled() {
		return Preview{}, problem.NewProblem(http.StatusServiceUnavailable, "analytics_unavailable", "Analytics unavailable", "The analytical store is not configured.")
	}
	var dataset model.Dataset
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadDataset(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "query", model.ResourceDataset, &row.WorkspaceID, id.String()); err != nil {
			return err
		}
		dataset = row
		return nil
	}); err != nil {
		return Preview{}, err
	}
	if strings.TrimSpace(sqlText) == "" {
		sqlText = dataset.DraftSQL
	}
	if err := s.validateDraftSQL(ctx, p, sqlText); err != nil {
		return Preview{}, err
	}
	filter, masks, err := s.RowFilterFor(ctx, id)
	if err != nil {
		return Preview{}, err
	}
	dbName, err := s.ch.EnsureOrgDatabase(ctx, p.OrgID)
	if err != nil {
		return Preview{}, analyticsError(err)
	}
	body := strings.TrimSuffix(strings.TrimSpace(sqlText), ";")
	query := "SELECT * FROM (" + body + ") AS hugi_q"
	if filter != "" {
		query += " WHERE (" + filter + ")"
	}
	query += fmt.Sprintf(" LIMIT %d", previewRows)
	result, err := s.ch.Query(ctx, dbName, "", query, clickhouse.Limits{MaxResultRows: previewRows, MaxExecutionTime: 15e9})
	if err != nil {
		return Preview{}, problem.Validation(problem.FieldError{Field: "sql", Code: "invalid", Message: "The SQL could not be executed: " + err.Error()})
	}
	columnPolicies, err := s.ColumnPolicies(ctx, id)
	if err != nil {
		return Preview{}, err
	}
	out := Preview{Columns: result.Columns, Rows: make([][]any, len(result.Rows)), RowCount: result.RowCount}
	for i, row := range result.Rows {
		masked := make([]any, len(row))
		for j, value := range row {
			if j >= len(result.Columns) {
				masked[j] = value
				continue
			}
			mask := ""
			for _, cp := range columnPolicies {
				if cp.ColumnName == result.Columns[j] {
					mask = cp.Mask
					if mask == "" || mask == "none" {
						mask = sqlguard.MaskFor(cp.ColumnName, cp.Classification, masks)
					}
					break
				}
			}
			if roleMask, ok := masks[result.Columns[j]]; ok && roleMask != "" && roleMask != "none" {
				mask = roleMask
			}
			masked[j] = sqlguard.MaskValue(mask, value)
		}
		out.Rows[i] = masked
	}
	return out, nil
}

func (s *DataPlatformService) Lineage(ctx context.Context, id uuid.UUID) ([]model.Lineage, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	var out []model.Lineage
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadDataset(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "read", model.ResourceDataset, &row.WorkspaceID, id.String()); err != nil {
			return err
		}
		var rows []repository.LineageRow
		if err := tx.SelectContext(ctx, &rows, queries.SQL.Get("ListLineage"), id); err != nil {
			return err
		}
		out = make([]model.Lineage, len(rows))
		for i := range rows {
			out[i] = rows[i].Model()
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *DataPlatformService) ColumnPolicies(ctx context.Context, datasetID uuid.UUID) ([]model.ColumnPolicy, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	var out []model.ColumnPolicy
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadDataset(ctx, tx, datasetID)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "read", model.ResourceDataset, &row.WorkspaceID, datasetID.String()); err != nil {
			return err
		}
		var rows []repository.ColumnPolicyRow
		if err := tx.SelectContext(ctx, &rows, queries.SQL.Get("ListColumnPolicies"), datasetID); err != nil {
			return err
		}
		out = make([]model.ColumnPolicy, len(rows))
		for i := range rows {
			out[i] = rows[i].Model()
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *DataPlatformService) PutColumnPolicy(ctx context.Context, datasetID uuid.UUID, in dto.ColumnPolicyInput) (model.ColumnPolicy, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.ColumnPolicy{}, err
	}
	if err := model.ValidateColumn(in.ColumnName); err != nil {
		return model.ColumnPolicy{}, problem.Validation(problem.FieldError{Field: "column_name", Code: "invalid", Message: err.Error()})
	}
	switch in.Classification {
	case "public", "internal", "confidential", "pii", "restricted":
	default:
		return model.ColumnPolicy{}, problem.Validation(problem.FieldError{Field: "classification", Code: "invalid", Message: "Unknown classification."})
	}
	switch in.Mask {
	case "":
		in.Mask = "none"
	case "none", "hash", "redact", "null":
	default:
		return model.ColumnPolicy{}, problem.Validation(problem.FieldError{Field: "mask", Code: "invalid", Message: "Mask must be none, hash, redact or null."})
	}
	var out model.ColumnPolicy
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadDataset(ctx, tx, datasetID)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "update", model.ResourceDataset, &row.WorkspaceID, datasetID.String()); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("UpsertColumnPolicy"), p.OrgID, uuid.New(), datasetID,
			in.ColumnName, in.Classification, in.Mask); err != nil {
			return err
		}
		out = model.ColumnPolicy{ColumnName: in.ColumnName, Classification: in.Classification, Mask: in.Mask}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "dataset.column_policy.updated",
			ResourceType: model.ResourceDataset, ResourceID: datasetID.String(), WorkspaceID: &row.WorkspaceID,
			Details: map[string]any{"column": in.ColumnName, "classification": in.Classification, "mask": in.Mask}})
	}); err != nil {
		return model.ColumnPolicy{}, err
	}
	return out, nil
}

func (s *DataPlatformService) DeleteColumnPolicy(ctx context.Context, datasetID uuid.UUID, column string) error {
	p, err := s.principal(ctx)
	if err != nil {
		return err
	}
	return s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadDataset(ctx, tx, datasetID)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "update", model.ResourceDataset, &row.WorkspaceID, datasetID.String()); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("DeleteColumnPolicy"), datasetID, column); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "dataset.column_policy.deleted",
			ResourceType: model.ResourceDataset, ResourceID: datasetID.String(), WorkspaceID: &row.WorkspaceID,
			Details: map[string]any{"column": column}})
	})
}

func (s *DataPlatformService) RowPolicies(ctx context.Context, datasetID uuid.UUID) ([]model.RowPolicy, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	var out []model.RowPolicy
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadDataset(ctx, tx, datasetID)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "read", model.ResourceDataset, &row.WorkspaceID, datasetID.String()); err != nil {
			return err
		}
		var rows []repository.RowPolicyRow
		if err := tx.SelectContext(ctx, &rows, queries.SQL.Get("ListRowPolicies"), datasetID); err != nil {
			return err
		}
		out = make([]model.RowPolicy, len(rows))
		for i := range rows {
			out[i] = rows[i].Model()
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *DataPlatformService) PutRowPolicy(ctx context.Context, datasetID uuid.UUID, in dto.RowPolicyInput) (model.RowPolicy, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.RowPolicy{}, err
	}
	if in.RoleKey == "" {
		return model.RowPolicy{}, problem.Validation(problem.FieldError{Field: "role_key", Code: "required", Message: "A role is required."})
	}
	if in.RowFilter != "" {
		if err := sqlguard.ValidateReadOnly("SELECT 1 WHERE " + in.RowFilter); err != nil {
			return model.RowPolicy{}, problem.Validation(problem.FieldError{Field: "row_filter", Code: "invalid", Message: err.Error()})
		}
	}
	for column, mask := range in.ColumnMasks {
		if err := model.ValidateColumn(column); err != nil {
			return model.RowPolicy{}, problem.Validation(problem.FieldError{Field: "column_masks", Code: "invalid", Message: err.Error()})
		}
		switch mask {
		case "none", "hash", "redact", "null":
		default:
			return model.RowPolicy{}, problem.Validation(problem.FieldError{Field: "column_masks", Code: "invalid", Message: "Unknown mask for " + column + "."})
		}
	}
	var out model.RowPolicy
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadDataset(ctx, tx, datasetID)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "update", model.ResourceDataset, &row.WorkspaceID, datasetID.String()); err != nil {
			return err
		}
		masks, _ := json.Marshal(in.ColumnMasks)
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("UpsertRowPolicy"), p.OrgID, uuid.New(), datasetID,
			in.RoleKey, in.RowFilter, masks); err != nil {
			if isForeignKeyViolation(err) || isUniqueViolation(err) {
				return problem.NewProblem(http.StatusConflict, "role_not_found", "Conflict", "The role does not exist in this organization.")
			}
			return err
		}
		out = model.RowPolicy{RoleKey: in.RoleKey, RowFilter: in.RowFilter, ColumnMasks: in.ColumnMasks}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "dataset.row_policy.updated",
			ResourceType: model.ResourceDataset, ResourceID: datasetID.String(), WorkspaceID: &row.WorkspaceID,
			Details: map[string]any{"role": in.RoleKey, "has_filter": in.RowFilter != ""}})
	}); err != nil {
		return model.RowPolicy{}, err
	}
	return out, nil
}

func (s *DataPlatformService) DeleteRowPolicy(ctx context.Context, datasetID uuid.UUID, roleKey string) error {
	p, err := s.principal(ctx)
	if err != nil {
		return err
	}
	return s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadDataset(ctx, tx, datasetID)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "update", model.ResourceDataset, &row.WorkspaceID, datasetID.String()); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("DeleteRowPolicy"), datasetID, roleKey); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "dataset.row_policy.deleted",
			ResourceType: model.ResourceDataset, ResourceID: datasetID.String(), WorkspaceID: &row.WorkspaceID,
			Details: map[string]any{"role": roleKey}})
	})
}

func (s *DataPlatformService) AllowedTables(ctx context.Context) ([]string, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var raw []string
	var datasets []struct {
		ID   uuid.UUID `db:"id"`
		Slug string    `db:"slug"`
	}
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		if err := tx.SelectContext(ctx, &raw, queries.SQL.Get("RawTables"), p.OrgID); err != nil {
			return err
		}
		return tx.SelectContext(ctx, &datasets, queries.SQL.Get("PublishedDatasets"), p.OrgID)
	}); err != nil {
		return nil, err
	}
	for _, name := range raw {
		seen[strings.ToLower(name)] = true
	}
	for _, d := range datasets {
		seen[strings.ToLower(model.DatasetView(d.ID, d.Slug))] = true
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

func (s *DataPlatformService) PublishedViews(ctx context.Context) (map[string]uuid.UUID, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	var datasets []struct {
		ID   uuid.UUID `db:"id"`
		Slug string    `db:"slug"`
	}
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &datasets, queries.SQL.Get("PublishedDatasets"), p.OrgID)
	}); err != nil {
		return nil, err
	}
	out := make(map[string]uuid.UUID, len(datasets))
	for _, d := range datasets {
		out[strings.ToLower(model.DatasetView(d.ID, d.Slug))] = d.ID
	}
	return out, nil
}

func (s *DataPlatformService) DatasetForQuery(ctx context.Context, id uuid.UUID) (model.Dataset, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Dataset{}, err
	}
	var out model.Dataset
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadDataset(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "query", model.ResourceDataset, &row.WorkspaceID, id.String()); err != nil {
			return err
		}
		out = row
		return nil
	}); err != nil {
		return model.Dataset{}, err
	}
	return out, nil
}

func (s *DataPlatformService) RowFilterFor(ctx context.Context, datasetID uuid.UUID) (string, map[string]string, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return "", nil, err
	}
	var policies []model.RowPolicy
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		var rows []repository.RowPolicyRow
		if err := tx.SelectContext(ctx, &rows, queries.SQL.Get("ListRowPolicies"), datasetID); err != nil {
			return err
		}
		policies = make([]model.RowPolicy, len(rows))
		for i := range rows {
			policies[i] = rows[i].Model()
		}
		return nil
	}); err != nil {
		return "", nil, err
	}
	if len(policies) == 0 {
		return "", nil, nil
	}
	roleKeys := map[string]bool{}
	if g, ok := s.az.(interface {
		Grants(ctx context.Context, p principal.Principal) ([]authzmodel.Grant, int64, error)
	}); ok {
		grants, _, err := g.Grants(ctx, p)
		if err != nil {
			return "", nil, err
		}
		for _, grant := range grants {
			roleKeys[grant.RoleKey] = true
		}
	}
	var filters []string
	masks := map[string]string{}
	for _, policy := range policies {
		if !roleKeys[policy.RoleKey] {
			continue
		}
		for column, mask := range policy.ColumnMasks {
			masks[column] = mask
		}
		if strings.TrimSpace(policy.RowFilter) == "" {
			return "", masks, nil
		}
		if err := sqlguard.ValidateReadOnly("SELECT 1 WHERE " + policy.RowFilter); err != nil {
			return "", nil, fmt.Errorf("dataset row policy for %s is invalid: %w", policy.RoleKey, err)
		}
		filters = append(filters, "("+policy.RowFilter+")")
	}
	return strings.Join(filters, " OR "), masks, nil
}

func (s *DataPlatformService) validateDraftSQL(ctx context.Context, p principal.Principal, sqlText string) error {
	if err := sqlguard.ValidateReadOnly(sqlText); err != nil {
		return problem.Validation(problem.FieldError{Field: "sql", Code: "invalid", Message: err.Error()})
	}
	if len(sqlguard.ReferencedTables(sqlText)) == 0 {
		return nil
	}
	allowed, err := s.AllowedTables(ctx)
	if err != nil {
		return err
	}
	if err := sqlguard.ValidateTables(sqlText, allowed); err != nil {
		return problem.Validation(problem.FieldError{Field: "sql", Code: "invalid", Message: err.Error()})
	}
	return nil
}

func analyticsError(err error) error {
	if errors.Is(err, clickhouse.ErrUnavailable) {
		return problem.NewProblem(http.StatusServiceUnavailable, "analytics_unavailable", "Analytics unavailable", "The analytical store is not configured.")
	}
	return err
}
