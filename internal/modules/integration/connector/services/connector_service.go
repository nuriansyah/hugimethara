package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/infrastructure/security/egress"
	authzmodel "github.com/nuriansyah/hugimethara/internal/modules/authorization/model"
	authzdto "github.com/nuriansyah/hugimethara/internal/modules/authorization/model/dto"
	authzsvc "github.com/nuriansyah/hugimethara/internal/modules/authorization/services"
	auditmodel "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/model"
	auditsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/services"
	connectormodel "github.com/nuriansyah/hugimethara/internal/modules/integration/connector/model"
	connectordto "github.com/nuriansyah/hugimethara/internal/modules/integration/connector/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/integration/connector/repository/commands"
	"github.com/nuriansyah/hugimethara/internal/modules/integration/connector/repository/queries"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	apperrors "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

type ConnectorService struct {
	pool *sqlx.DB
	az   authzsvc.Authorizer
}

func NewService(pool *sqlx.DB, az authzsvc.Authorizer) (*ConnectorService, error) {
	if pool == nil {
		return nil, errors.New("connector: database pool is required")
	}
	if az == nil {
		return nil, errors.New("connector: authorizer is required")
	}
	return &ConnectorService{pool: pool, az: az}, nil
}

func (s *ConnectorService) principal(ctx context.Context) (principal.Principal, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return principal.Principal{}, authzmodel.Unauthenticated()
	}
	return p, nil
}

func (s *ConnectorService) require(ctx context.Context, action, id string, workspace *uuid.UUID) (principal.Principal, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return principal.Principal{}, err
	}
	_, err = authzsvc.Require(ctx, s.az, action, authzmodel.Resource{
		Type: connectormodel.Resource, ID: id, OrgID: p.OrgID, WorkspaceID: workspace,
	})
	return p, err
}

func (s *ConnectorService) tenantTx(ctx context.Context, p principal.Principal, fn func(*sqlx.Tx) error) error {
	return db.InTenantTx(ctx, s.pool, p.OrgID, fn)
}

func (s *ConnectorService) ListDefinitions(ctx context.Context) ([]connectormodel.Definition, error) {
	p, err := s.require(ctx, "read", "", nil)
	if err != nil {
		return nil, err
	}
	out := []connectormodel.Definition{}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &out, queries.SQL.Get("ListDefinitions"))
	})
	return out, err
}

func (s *ConnectorService) ListVersions(ctx context.Context, definitionID uuid.UUID) ([]connectormodel.Version, error) {
	p, err := s.require(ctx, "read", definitionID.String(), nil)
	if err != nil {
		return nil, err
	}
	out := []connectormodel.Version{}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		if err := tx.GetContext(ctx, new(uuid.UUID), `SELECT id FROM connector_definitions WHERE id = $1`, definitionID); err != nil {
			return connectormodel.TranslateNotFound(err)
		}
		return tx.SelectContext(ctx, &out, queries.SQL.Get("ListVersions"), definitionID)
	})
	return out, err
}

func (s *ConnectorService) List(ctx context.Context) ([]connectormodel.Instance, error) {
	p, err := s.require(ctx, "read", "", nil)
	if err != nil {
		return nil, err
	}
	rows := []connectormodel.Instance{}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, queries.SQL.Get("ListInstances"))
	})
	if err != nil {
		return nil, err
	}
	out := make([]connectormodel.Instance, 0, len(rows))
	for _, row := range rows {
		decision, err := s.az.Evaluate(ctx, authzdto.AuthorizationInput{
			Principal: p, Action: "read",
			Resource: authzmodel.Resource{Type: connectormodel.Resource, ID: row.ID.String(), OrgID: p.OrgID, WorkspaceID: &row.WorkspaceID},
		})
		if err != nil {
			return nil, err
		}
		if decision.Allowed {
			row.Config = sanitizeConfig(row.Config)
			out = append(out, row)
		}
	}
	return out, nil
}

func (s *ConnectorService) Get(ctx context.Context, id uuid.UUID) (connectormodel.Instance, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return connectormodel.Instance{}, err
	}
	row, err := s.loadInstance(ctx, p, id)
	if err != nil {
		return connectormodel.Instance{}, err
	}
	if _, err := s.require(ctx, "read", id.String(), &row.WorkspaceID); err != nil {
		return connectormodel.Instance{}, err
	}
	row.Config = sanitizeConfig(row.Config)
	return row, nil
}

func (s *ConnectorService) Create(ctx context.Context, in connectordto.CreateInput) (connectormodel.Instance, error) {
	p, err := s.require(ctx, "create", "", &in.WorkspaceID)
	if err != nil {
		return connectormodel.Instance{}, err
	}
	in.RefKey = strings.TrimSpace(in.RefKey)
	if !connectormodel.ValidRefKey(in.RefKey) {
		return connectormodel.Instance{}, apperrors.Validation(apperrors.FieldError{Field: "ref_key", Code: "invalid", Message: "Use 2–64 lowercase letters, digits, underscores, or hyphens; begin with a letter."})
	}
	if in.WorkspaceID == uuid.Nil || in.EnvironmentID == uuid.Nil || in.ConnectorVersionID == uuid.Nil {
		return connectormodel.Instance{}, connectormodel.Validation("body", "required", "Workspace, environment, and connector version are required.")
	}
	if in.RateLimit == 0 {
		in.RateLimit = 60
	}
	if in.TimeoutMs == 0 {
		in.TimeoutMs = 10000
	}
	if in.RateLimit < 1 || in.RateLimit > 10000 {
		return connectormodel.Instance{}, connectormodel.Validation("rate_limit", "out_of_range", "Rate limit must be between 1 and 10000.")
	}
	if in.TimeoutMs < 100 || in.TimeoutMs > 120000 {
		return connectormodel.Instance{}, connectormodel.Validation("timeout_ms", "out_of_range", "Timeout must be between 100 and 120000 milliseconds.")
	}
	if in.AuthMethod == "" {
		in.AuthMethod = "none"
	}
	configJSON, err := json.Marshal(in.Config)
	if err != nil {
		return connectormodel.Instance{}, connectormodel.Validation("config", "invalid", "Configuration must be valid JSON.")
	}
	var out connectormodel.Instance
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		var detail connectormodel.VersionDetail
		if err := tx.GetContext(ctx, &detail, queries.SQL.Get("GetVersion"), in.ConnectorVersionID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return connectormodel.Validation("connector_version_id", "not_found", "The connector version does not exist.")
			}
			return err
		}
		if err := validateConfig(detail.ConfigSchema, configJSON); err != nil {
			return err
		}
		if !connectormodel.ContainsJSONString(detail.AuthMethods, in.AuthMethod) {
			return connectormodel.Validation("auth_method", "unsupported", "The selected authentication method is not supported by this connector version.")
		}
		if in.SecretID != nil {
			if err := validateSecret(ctx, tx, p.OrgID, *in.SecretID, in.EnvironmentID); err != nil {
				return err
			}
			if _, err := authzsvc.Require(ctx, s.az, "use", authzmodel.Resource{Type: "secret", ID: in.SecretID.String(), OrgID: p.OrgID}); err != nil {
				return err
			}
		}
		id := uuid.Must(uuid.NewV7())
		_, err := tx.ExecContext(ctx, commands.SQL.Get("InsertInstance"), p.OrgID, id, in.WorkspaceID, in.EnvironmentID,
			in.RefKey, in.ConnectorVersionID, configJSON, in.AuthMethod, connectormodel.NullableUUID(in.SecretID), in.RateLimit, in.TimeoutMs)
		if err != nil {
			return translateWriteError(err, in.RefKey)
		}
		var detailRow connectormodel.InstanceDetail
		if err := tx.GetContext(ctx, &detailRow, queries.SQL.Get("GetInstance"), id); err != nil {
			return err
		}
		out = detailRow.Instance
		out.Config = sanitizeConfig(out.Config)
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "connector.created",
			ResourceType: connectormodel.Resource, ResourceID: id.String(), WorkspaceID: &in.WorkspaceID,
			Details: map[string]any{"ref_key": in.RefKey, "definition_key": out.DefinitionKey}})
	})
	return out, err
}

func (s *ConnectorService) Update(ctx context.Context, id uuid.UUID, in connectordto.UpdateInput) (connectormodel.Instance, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return connectormodel.Instance{}, err
	}
	current, err := s.loadInstance(ctx, p, id)
	if err != nil {
		return connectormodel.Instance{}, err
	}
	if _, err := s.require(ctx, "update", id.String(), &current.WorkspaceID); err != nil {
		return connectormodel.Instance{}, err
	}
	if in.Config == nil && in.AuthMethod == nil && in.SecretID == nil && in.RateLimit == nil && in.TimeoutMs == nil && in.Status == nil {
		return connectormodel.Instance{}, connectormodel.Validation("body", "empty", "At least one field must be provided.")
	}
	var configJSON []byte
	if in.Config != nil {
		var schema json.RawMessage
		err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
			var detail connectormodel.InstanceDetail
			if err := tx.GetContext(ctx, &detail, queries.SQL.Get("GetInstance"), id); err != nil {
				return connectormodel.TranslateNotFound(err)
			}
			schema = detail.ConfigSchema
			return nil
		})
		if err != nil {
			return connectormodel.Instance{}, err
		}
		configJSON, err = json.Marshal(*in.Config)
		if err != nil {
			return connectormodel.Instance{}, connectormodel.Validation("config", "invalid", "Configuration must be valid JSON.")
		}
		if err := validateConfig(schema, configJSON); err != nil {
			return connectormodel.Instance{}, err
		}
	}
	if in.AuthMethod != nil {
		var methods []byte
		err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
			var detail connectormodel.InstanceDetail
			if err := tx.GetContext(ctx, &detail, queries.SQL.Get("GetInstance"), id); err != nil {
				return connectormodel.TranslateNotFound(err)
			}
			methods = detail.AuthMethods
			return nil
		})
		if err != nil {
			return connectormodel.Instance{}, err
		}
		if !connectormodel.ContainsJSONString(methods, *in.AuthMethod) {
			return connectormodel.Instance{}, connectormodel.Validation("auth_method", "unsupported", "The selected authentication method is not supported by this connector version.")
		}
	}
	if in.RateLimit != nil && (*in.RateLimit < 1 || *in.RateLimit > 10000) {
		return connectormodel.Instance{}, connectormodel.Validation("rate_limit", "out_of_range", "Rate limit must be between 1 and 10000.")
	}
	if in.TimeoutMs != nil && (*in.TimeoutMs < 100 || *in.TimeoutMs > 120000) {
		return connectormodel.Instance{}, connectormodel.Validation("timeout_ms", "out_of_range", "Timeout must be between 100 and 120000 milliseconds.")
	}
	if in.Status != nil && *in.Status != "enabled" && *in.Status != "disabled" {
		return connectormodel.Instance{}, connectormodel.Validation("status", "invalid", "Status must be enabled or disabled.")
	}
	if in.SecretID != nil {
		if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
			return validateSecret(ctx, tx, p.OrgID, *in.SecretID, current.EnvironmentID)
		}); err != nil {
			return connectormodel.Instance{}, err
		}
		if _, err := authzsvc.Require(ctx, s.az, "use", authzmodel.Resource{Type: "secret", ID: in.SecretID.String(), OrgID: p.OrgID}); err != nil {
			return connectormodel.Instance{}, err
		}
	}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		result, err := tx.ExecContext(ctx, commands.SQL.Get("UpdateInstance"), id, connectormodel.NullableJSON(configJSON), in.AuthMethod,
			connectormodel.NullableUUID(in.SecretID), in.RateLimit, in.TimeoutMs, in.Status)
		if err != nil {
			return translateWriteError(err, current.RefKey)
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return connectormodel.ErrNotFound
		}
		var detail connectormodel.InstanceDetail
		if err := tx.GetContext(ctx, &detail, queries.SQL.Get("GetInstance"), id); err != nil {
			return err
		}
		current = detail.Instance
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "connector.updated",
			ResourceType: connectormodel.Resource, ResourceID: id.String(), WorkspaceID: &current.WorkspaceID,
			Details: map[string]any{"ref_key": current.RefKey}})
	})
	current.Config = sanitizeConfig(current.Config)
	return current, err
}

func (s *ConnectorService) Delete(ctx context.Context, id uuid.UUID) error {
	p, err := s.principal(ctx)
	if err != nil {
		return err
	}
	current, err := s.loadInstance(ctx, p, id)
	if err != nil {
		return err
	}
	if _, err := s.require(ctx, "delete", id.String(), &current.WorkspaceID); err != nil {
		return err
	}
	return s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		result, err := tx.ExecContext(ctx, commands.SQL.Get("DeleteInstance"), id)
		if err != nil {
			return err
		}
		if n, err := result.RowsAffected(); err != nil {
			return err
		} else if n == 0 {
			return connectormodel.ErrNotFound
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "connector.deleted",
			ResourceType: connectormodel.Resource, ResourceID: id.String(), WorkspaceID: &current.WorkspaceID,
			Details: map[string]any{"ref_key": current.RefKey}})
	})
}

func (s *ConnectorService) Test(ctx context.Context, id uuid.UUID) (connectormodel.TestResult, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return connectormodel.TestResult{}, err
	}
	current, err := s.loadInstance(ctx, p, id)
	if err != nil {
		return connectormodel.TestResult{}, err
	}
	if _, err := s.require(ctx, "test", id.String(), &current.WorkspaceID); err != nil {
		return connectormodel.TestResult{}, err
	}
	policy, err := s.environmentPolicy(ctx, p, current.EnvironmentID)
	if err != nil {
		return connectormodel.TestResult{}, err
	}
	target, err := connectormodel.EndpointFromConfig(current.Config)
	var result connectormodel.TestResult
	if err == nil {
		result = checkEndpoint(ctx, target, policy, current.TimeoutMs)
	} else {
		address, tcpErr := connectormodel.TcpEndpointFromConfig(current.Config)
		if tcpErr != nil {
			return connectormodel.TestResult{}, err
		}
		result = checkTCP(ctx, address, policy, current.TimeoutMs)
	}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertHealth"), p.OrgID, id, result.OK, result.LatencyMs, result.ErrorCode, result.Message); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "connector.tested",
			ResourceType: connectormodel.Resource, ResourceID: id.String(), WorkspaceID: &current.WorkspaceID,
			Details: map[string]any{"ok": result.OK, "error_code": result.ErrorCode}})
	})
	return result, err
}

func (s *ConnectorService) Health(ctx context.Context, id uuid.UUID) ([]connectormodel.Health, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	current, err := s.loadInstance(ctx, p, id)
	if err != nil {
		return nil, err
	}
	if _, err := s.require(ctx, "read", id.String(), &current.WorkspaceID); err != nil {
		return nil, err
	}
	out := []connectormodel.Health{}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &out, queries.SQL.Get("ListHealth"), id)
	})
	return out, err
}

func (s *ConnectorService) loadInstance(ctx context.Context, p principal.Principal, id uuid.UUID) (connectormodel.Instance, error) {
	var out connectormodel.InstanceDetail
	err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		if err := tx.GetContext(ctx, &out, queries.SQL.Get("GetInstance"), id); err != nil {
			return connectormodel.TranslateNotFound(err)
		}
		return nil
	})
	return out.Instance, err
}

func (s *ConnectorService) environmentPolicy(ctx context.Context, p principal.Principal, environmentID uuid.UUID) (egress.Policy, error) {
	var encoded []byte
	err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		if err := tx.GetContext(ctx, &encoded, queries.SQL.Get("GetEnvironmentEgressAllowlist"), environmentID, p.OrgID); err != nil {
			return connectormodel.TranslateNotFound(err)
		}
		return nil
	})
	if err != nil {
		return egress.Policy{}, err
	}
	return parseEnvironmentAllowlist(encoded)
}

func validateSecret(ctx context.Context, tx *sqlx.Tx, orgID, secretID, environmentID uuid.UUID) error {
	var found uuid.UUID
	err := tx.GetContext(ctx, &found, queries.SQL.Get("GetSecretForInstance"), secretID, orgID, environmentID)
	if errors.Is(err, sql.ErrNoRows) {
		return connectormodel.Validation("secret_id", "not_found", "The credential does not exist in this organization and environment.")
	}
	return err
}

func translateWriteError(err error, refKey string) error {
	if db.IsUniqueViolation(err) {
		return apperrors.NewProblem(http.StatusConflict, "connector_ref_key_taken", "Conflict",
			fmt.Sprintf("A connector with reference key %q already exists in this workspace and environment.", refKey))
	}
	if db.IsForeignKeyViolation(err) {
		return connectormodel.Validation("resource_id", "not_found", "A workspace, environment, version, or credential does not exist in this organization.")
	}
	return err
}
