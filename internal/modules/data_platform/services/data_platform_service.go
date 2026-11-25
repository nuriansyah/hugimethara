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

	"github.com/nuriansyah/hugimethara/internal/infrastructure/analytics/clickhouse"
	authzmodel "github.com/nuriansyah/hugimethara/internal/modules/authorization/model"
	authzsvc "github.com/nuriansyah/hugimethara/internal/modules/authorization/services"
	model "github.com/nuriansyah/hugimethara/internal/modules/data_platform/model"
	dto "github.com/nuriansyah/hugimethara/internal/modules/data_platform/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/data_platform/repository"
	"github.com/nuriansyah/hugimethara/internal/modules/data_platform/repository/commands"
	"github.com/nuriansyah/hugimethara/internal/modules/data_platform/repository/queries"
	auditmodel "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/model"
	auditsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/services"
	secretsmodel "github.com/nuriansyah/hugimethara/internal/modules/integration/secrets/model"
	secretsvc "github.com/nuriansyah/hugimethara/internal/modules/integration/secrets/services"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
	"github.com/nuriansyah/hugimethara/internal/platform/sqlguard"
)

var errSecretResolver = errors.New("data platform: secret resolver is not configured")

// DataPlatformService implements Service.
type DataPlatformService struct {
	pool   *sqlx.DB
	az     authzsvc.Authorizer
	opener *secretsvc.Opener
	ch     *clickhouse.Client
}

func NewService(pool *sqlx.DB, az authzsvc.Authorizer, opener *secretsvc.Opener, ch *clickhouse.Client) (*DataPlatformService, error) {
	if pool == nil {
		return nil, errors.New("data platform: database pool is required")
	}
	if az == nil {
		return nil, errors.New("data platform: authorizer is required")
	}
	return &DataPlatformService{pool: pool, az: az, opener: opener, ch: ch}, nil
}

func (s *DataPlatformService) Kinds() []model.Kind { return model.Kinds }

func (s *DataPlatformService) principal(ctx context.Context) (principal.Principal, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return principal.Principal{}, authzmodel.Unauthenticated()
	}
	return p, nil
}

func (s *DataPlatformService) tenantTx(ctx context.Context, p principal.Principal, fn func(*sqlx.Tx) error) error {
	return db.InTenantTx(ctx, s.pool, p.OrgID, fn)
}

func (s *DataPlatformService) require(ctx context.Context, p principal.Principal, action, resourceType string, workspaceID *uuid.UUID, id string) error {
	_, err := authzsvc.Require(ctx, s.az, action, authzmodel.Resource{
		Type: resourceType, ID: id, OrgID: p.OrgID, WorkspaceID: workspaceID,
	})
	return err
}

func (s *DataPlatformService) loadDataSource(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (model.DataSource, error) {
	var row repository.DataSourceRow
	if err := tx.GetContext(ctx, &row, queries.SQL.Get("GetDataSource"), id); err != nil {
		if db.IsNoRows(err) {
			return model.DataSource{}, problem.NotFound("The data source does not exist.")
		}
		return model.DataSource{}, err
	}
	return row.Model(), nil
}

func (s *DataPlatformService) dataSourceWithSecret(ctx context.Context, id uuid.UUID, action string) (model.DataSource, []byte, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.DataSource{}, nil, err
	}
	var ds model.DataSource
	var secret []byte
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadDataSource(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, action, model.ResourceDataSource, &row.WorkspaceID, id.String()); err != nil {
			return err
		}
		ds = row
		if row.SecretID != nil {
			if s.opener == nil {
				return errSecretResolver
			}
			secret, err = s.opener.Open(ctx, tx, p.OrgID, *row.SecretID)
			if err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return model.DataSource{}, nil, err
	}
	return ds, secret, nil
}

func (s *DataPlatformService) ListDataSources(ctx context.Context, workspaceID uuid.UUID) ([]model.DataSource, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.require(ctx, p, "read", model.ResourceDataSource, &workspaceID, ""); err != nil {
		return nil, err
	}
	var rows []repository.DataSourceRow
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, queries.SQL.Get("ListDataSources"), workspaceID)
	}); err != nil {
		return nil, err
	}
	out := make([]model.DataSource, len(rows))
	for i := range rows {
		out[i] = rows[i].Model()
		out[i].Config = sanitizeConfig(out[i].Config)
	}
	return out, nil
}

func (s *DataPlatformService) CreateDataSource(ctx context.Context, in dto.CreateDataSourceInput) (model.DataSource, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.DataSource{}, err
	}
	if err := s.require(ctx, p, "create", model.ResourceDataSource, &in.WorkspaceID, ""); err != nil {
		return model.DataSource{}, err
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return model.DataSource{}, problem.Validation(problem.FieldError{Field: "name", Code: "required", Message: "A name is required."})
	}
	if in.Slug == "" {
		in.Slug = slugify(in.Name)
	}
	if err := model.ValidateSlug(in.Slug); err != nil {
		return model.DataSource{}, problem.Validation(problem.FieldError{Field: "slug", Code: "invalid", Message: err.Error()})
	}
	if _, ok := model.KindInfo(in.Kind); !ok {
		return model.DataSource{}, problem.Validation(problem.FieldError{Field: "kind", Code: "invalid", Message: "Unknown source kind."})
	}
	if in.WorkspaceID == uuid.Nil || in.EnvironmentID == uuid.Nil {
		return model.DataSource{}, problem.Validation(problem.FieldError{Field: "workspace_id", Code: "required", Message: "Workspace and environment are required."})
	}
	if in.Config == nil {
		in.Config = map[string]any{}
	}
	if model.Available(in.Kind) {
		if err := model.ValidateSourceConfig(in.Kind, in.Config); err != nil {
			return model.DataSource{}, problem.Validation(problem.FieldError{Field: "config", Code: "invalid", Message: err.Error()})
		}
	}
	if hasInlineSecret(in.Config) {
		return model.DataSource{}, problem.Validation(problem.FieldError{Field: "config", Code: "secret_inline", Message: "Secrets must be stored as credentials, not in the source config."})
	}
	var envWorkspace uuid.UUID
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.GetContext(ctx, &envWorkspace, queries.SQL.Get("EnvironmentWorkspace"), in.EnvironmentID)
	}); err != nil {
		if db.IsNoRows(err) {
			return model.DataSource{}, problem.Validation(problem.FieldError{Field: "environment_id", Code: "invalid", Message: "The environment does not exist."})
		}
		return model.DataSource{}, err
	}
	if envWorkspace != in.WorkspaceID {
		return model.DataSource{}, problem.Validation(problem.FieldError{Field: "environment_id", Code: "invalid", Message: "The environment is in a different workspace."})
	}
	id := uuid.New()
	var out model.DataSource
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		config, _ := json.Marshal(in.Config)
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertDataSource"), p.OrgID, id, in.WorkspaceID,
			in.EnvironmentID, in.Name, in.Slug, in.Kind, config, in.SecretID, "enabled", p.ID); err != nil {
			if isUniqueViolation(err) {
				return problem.NewProblem(http.StatusConflict, "slug_taken", "Conflict", "A data source with that slug already exists in the workspace.")
			}
			return err
		}
		row, err := s.loadDataSource(ctx, tx, id)
		if err != nil {
			return err
		}
		out = row
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "datasource.created",
			ResourceType: model.ResourceDataSource, ResourceID: id.String(), WorkspaceID: &in.WorkspaceID,
			Details: map[string]any{"kind": in.Kind, "slug": in.Slug}})
	}); err != nil {
		return model.DataSource{}, err
	}
	return out, nil
}

func (s *DataPlatformService) GetDataSource(ctx context.Context, id uuid.UUID) (model.DataSource, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.DataSource{}, err
	}
	var out model.DataSource
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadDataSource(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "read", model.ResourceDataSource, &row.WorkspaceID, id.String()); err != nil {
			return err
		}
		out = row
		return nil
	}); err != nil {
		return model.DataSource{}, err
	}
	out.Config = sanitizeConfig(out.Config)
	return out, nil
}

func (s *DataPlatformService) UpdateDataSource(ctx context.Context, id uuid.UUID, in dto.UpdateDataSourceInput) (model.DataSource, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.DataSource{}, err
	}
	var out model.DataSource
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadDataSource(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "update", model.ResourceDataSource, &row.WorkspaceID, id.String()); err != nil {
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
		if in.Status != nil && *in.Status != "enabled" && *in.Status != "disabled" {
			return problem.Validation(problem.FieldError{Field: "status", Code: "invalid", Message: "Status must be enabled or disabled."})
		}
		if in.Config != nil {
			if hasInlineSecret(*in.Config) {
				return problem.Validation(problem.FieldError{Field: "config", Code: "secret_inline", Message: "Secrets must be stored as credentials, not in the source config."})
			}
			if model.Available(row.Kind) {
				if err := model.ValidateSourceConfig(row.Kind, *in.Config); err != nil {
					return problem.Validation(problem.FieldError{Field: "config", Code: "invalid", Message: err.Error()})
				}
			}
		}
		var config any
		if in.Config != nil {
			raw, _ := json.Marshal(*in.Config)
			config = json.RawMessage(raw)
		}
		hasSecret := in.SecretID != nil
		var secret any
		if hasSecret && *in.SecretID != uuid.Nil {
			secret = *in.SecretID
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("UpdateDataSource"), id, in.Name, in.Slug, config,
			hasSecret, secret, in.Status); err != nil {
			if isUniqueViolation(err) {
				return problem.NewProblem(http.StatusConflict, "slug_taken", "Conflict", "A data source with that slug already exists in the workspace.")
			}
			return err
		}
		row, err = s.loadDataSource(ctx, tx, id)
		if err != nil {
			return err
		}
		out = row
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "datasource.updated",
			ResourceType: model.ResourceDataSource, ResourceID: id.String(), WorkspaceID: &row.WorkspaceID})
	}); err != nil {
		return model.DataSource{}, err
	}
	return out, nil
}

func (s *DataPlatformService) DeleteDataSource(ctx context.Context, id uuid.UUID) error {
	p, err := s.principal(ctx)
	if err != nil {
		return err
	}
	return s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadDataSource(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "delete", model.ResourceDataSource, &row.WorkspaceID, id.String()); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("DeleteDataSource"), id); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "datasource.deleted",
			ResourceType: model.ResourceDataSource, ResourceID: id.String(), WorkspaceID: &row.WorkspaceID})
	})
}

func (s *DataPlatformService) TestDataSource(ctx context.Context, id uuid.UUID) (TestResult, error) {
	ds, secret, err := s.dataSourceWithSecret(ctx, id, "test")
	if err != nil {
		return TestResult{}, err
	}
	defer secretsmodel.Zero(secret)
	start := time.Now()
	result := s.testSource(ctx, ds, secret)
	result.LatencyMS = int(time.Since(start).Milliseconds())
	p, _ := s.principal(ctx)
	_ = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "datasource.tested",
			ResourceType: model.ResourceDataSource, ResourceID: id.String(), WorkspaceID: &ds.WorkspaceID,
			Details: map[string]any{"ok": result.OK, "error_code": result.ErrorCode}})
	})
	return result, nil
}

func (s *DataPlatformService) DiscoverDataSource(ctx context.Context, id uuid.UUID) (Discovery, error) {
	ds, secret, err := s.dataSourceWithSecret(ctx, id, "update")
	if err != nil {
		return Discovery{}, err
	}
	defer secretsmodel.Zero(secret)
	schema, err := s.discoverSource(ctx, ds, secret)
	if err != nil {
		return Discovery{}, err
	}
	checksum := sqlguard.Checksum(schema)
	p, _ := s.principal(ctx)
	var previous model.Schema
	var drift []string
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		var row repository.SchemaRow
		if err := tx.GetContext(ctx, &row, queries.SQL.Get("LatestSchema"), id); err == nil {
			previous = row.Model().Schema
		} else if !db.IsNoRows(err) {
			return err
		}
		drift = model.DiffSchema(previous, schema)
		raw, _ := json.Marshal(schema)
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertSchema"), p.OrgID, uuid.New(), id, raw, checksum); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "datasource.discovered",
			ResourceType: model.ResourceDataSource, ResourceID: id.String(), WorkspaceID: &ds.WorkspaceID,
			Details: map[string]any{"objects": len(schema.Objects), "drift": len(drift)}})
	}); err != nil {
		return Discovery{}, err
	}
	return Discovery{Schema: schema, Checksum: checksum, Drift: drift, CapturedAt: time.Now().UTC().Format(time.RFC3339)}, nil
}

func (s *DataPlatformService) ListSchemas(ctx context.Context, id uuid.UUID, limit int) ([]model.DataSourceSchema, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	var out []model.DataSourceSchema
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := s.loadDataSource(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.require(ctx, p, "read", model.ResourceDataSource, &row.WorkspaceID, id.String()); err != nil {
			return err
		}
		var rows []repository.SchemaRow
		if err := tx.SelectContext(ctx, &rows, queries.SQL.Get("ListSchemas"), id, limit); err != nil {
			return err
		}
		out = make([]model.DataSourceSchema, len(rows))
		for i := range rows {
			out[i] = rows[i].Model()
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

func sanitizeConfig(config map[string]any) map[string]any {
	if config == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(config))
	for k, v := range config {
		lower := strings.ToLower(k)
		if strings.Contains(lower, "password") || strings.Contains(lower, "secret") ||
			strings.Contains(lower, "token") || strings.Contains(lower, "api_key") {
			continue
		}
		out[k] = v
	}
	return out
}

func hasInlineSecret(config map[string]any) bool {
	for k := range config {
		lower := strings.ToLower(k)
		if strings.Contains(lower, "password") || strings.Contains(lower, "secret") ||
			strings.Contains(lower, "token") || strings.Contains(lower, "api_key") {
			return true
		}
	}
	return false
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

func isForeignKeyViolation(err error) bool {
	return strings.Contains(strings.ToLower(fmt.Sprint(err)), "foreign key")
}
