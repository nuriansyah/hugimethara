package connector

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

	"github.com/nuriansyah/hugimethara/internal/modules/authorization"
	"github.com/nuriansyah/hugimethara/internal/modules/governance/audit"
	"github.com/nuriansyah/hugimethara/internal/platform/database"
	"github.com/nuriansyah/hugimethara/internal/platform/egress"
	"github.com/nuriansyah/hugimethara/internal/platform/httpx"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

var errNotFound = httpx.NotFound("The requested connector resource does not exist.")

type Service struct {
	pool *sqlx.DB
	az   authz.Authorizer
}

func NewService(pool *sqlx.DB, az authz.Authorizer) (*Service, error) {
	if pool == nil {
		return nil, errors.New("connector: database pool is required")
	}
	if az == nil {
		return nil, errors.New("connector: authorizer is required")
	}
	return &Service{pool: pool, az: az}, nil
}

func (s *Service) principal(ctx context.Context) (principal.Principal, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return principal.Principal{}, authz.Unauthenticated()
	}
	return p, nil
}

func (s *Service) require(ctx context.Context, action, id string, workspace *uuid.UUID) (principal.Principal, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return principal.Principal{}, err
	}
	_, err = authz.Require(ctx, s.az, action, authz.Resource{
		Type: Resource, ID: id, OrgID: p.OrgID, WorkspaceID: workspace,
	})
	return p, err
}

func (s *Service) tenantTx(ctx context.Context, p principal.Principal, fn func(*sqlx.Tx) error) error {
	return db.InTenantTx(ctx, s.pool, p.OrgID, fn)
}

func (s *Service) ListDefinitions(ctx context.Context) ([]Definition, error) {
	p, err := s.require(ctx, "read", "", nil)
	if err != nil {
		return nil, err
	}
	out := []Definition{}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &out, q.Get("ListDefinitions"))
	})
	return out, err
}

func (s *Service) ListVersions(ctx context.Context, definitionID uuid.UUID) ([]Version, error) {
	p, err := s.require(ctx, "read", definitionID.String(), nil)
	if err != nil {
		return nil, err
	}
	out := []Version{}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		if err := tx.GetContext(ctx, new(uuid.UUID), `SELECT id FROM connector_definitions WHERE id = $1`, definitionID); err != nil {
			return translateNotFound(err)
		}
		return tx.SelectContext(ctx, &out, q.Get("ListVersions"), definitionID)
	})
	return out, err
}

func (s *Service) List(ctx context.Context) ([]Instance, error) {
	p, err := s.require(ctx, "read", "", nil)
	if err != nil {
		return nil, err
	}
	rows := []Instance{}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, q.Get("ListInstances"))
	})
	if err != nil {
		return nil, err
	}
	out := make([]Instance, 0, len(rows))
	for _, row := range rows {
		decision, err := s.az.Evaluate(ctx, authz.AuthorizationInput{
			Principal: p, Action: "read",
			Resource: authz.Resource{Type: Resource, ID: row.ID.String(), OrgID: p.OrgID, WorkspaceID: &row.WorkspaceID},
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

func (s *Service) Get(ctx context.Context, id uuid.UUID) (Instance, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return Instance{}, err
	}
	row, err := s.loadInstance(ctx, p, id)
	if err != nil {
		return Instance{}, err
	}
	if _, err := s.require(ctx, "read", id.String(), &row.WorkspaceID); err != nil {
		return Instance{}, err
	}
	row.Config = sanitizeConfig(row.Config)
	return row, nil
}

func (s *Service) Create(ctx context.Context, in CreateInput) (Instance, error) {
	p, err := s.require(ctx, "create", "", &in.WorkspaceID)
	if err != nil {
		return Instance{}, err
	}
	in.RefKey = strings.TrimSpace(in.RefKey)
	if !validRefKey(in.RefKey) {
		return Instance{}, httpx.Validation(httpx.FieldError{Field: "ref_key", Code: "invalid", Message: "Use 2–64 lowercase letters, digits, underscores, or hyphens; begin with a letter."})
	}
	if in.WorkspaceID == uuid.Nil || in.EnvironmentID == uuid.Nil || in.ConnectorVersionID == uuid.Nil {
		return Instance{}, validation("body", "required", "Workspace, environment, and connector version are required.")
	}
	if in.RateLimit == 0 {
		in.RateLimit = 60
	}
	if in.TimeoutMs == 0 {
		in.TimeoutMs = 10000
	}
	if in.RateLimit < 1 || in.RateLimit > 10000 {
		return Instance{}, validation("rate_limit", "out_of_range", "Rate limit must be between 1 and 10000.")
	}
	if in.TimeoutMs < 100 || in.TimeoutMs > 120000 {
		return Instance{}, validation("timeout_ms", "out_of_range", "Timeout must be between 100 and 120000 milliseconds.")
	}
	if in.AuthMethod == "" {
		in.AuthMethod = "none"
	}
	configJSON, err := json.Marshal(in.Config)
	if err != nil {
		return Instance{}, validation("config", "invalid", "Configuration must be valid JSON.")
	}
	var out Instance
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		var detail versionDetail
		if err := tx.GetContext(ctx, &detail, q.Get("GetVersion"), in.ConnectorVersionID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return validation("connector_version_id", "not_found", "The connector version does not exist.")
			}
			return err
		}
		if err := validateConfig(detail.ConfigSchema, configJSON); err != nil {
			return err
		}
		if !containsJSONString(detail.AuthMethods, in.AuthMethod) {
			return validation("auth_method", "unsupported", "The selected authentication method is not supported by this connector version.")
		}
		if in.SecretID != nil {
			if err := validateSecret(ctx, tx, p.OrgID, *in.SecretID, in.EnvironmentID); err != nil {
				return err
			}
			if _, err := authz.Require(ctx, s.az, "use", authz.Resource{Type: "secret", ID: in.SecretID.String(), OrgID: p.OrgID}); err != nil {
				return err
			}
		}
		id := uuid.Must(uuid.NewV7())
		_, err := tx.ExecContext(ctx, q.Get("InsertInstance"), p.OrgID, id, in.WorkspaceID, in.EnvironmentID,
			in.RefKey, in.ConnectorVersionID, configJSON, in.AuthMethod, nullableUUID(in.SecretID), in.RateLimit, in.TimeoutMs)
		if err != nil {
			return translateWriteError(err, in.RefKey)
		}
		var detailRow instanceDetail
		if err := tx.GetContext(ctx, &detailRow, q.Get("GetInstance"), id); err != nil {
			return err
		}
		out = detailRow.Instance
		out.Config = sanitizeConfig(out.Config)
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "connector.created",
			ResourceType: Resource, ResourceID: id.String(), WorkspaceID: &in.WorkspaceID,
			Details: map[string]any{"ref_key": in.RefKey, "definition_key": out.DefinitionKey}})
	})
	return out, err
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (Instance, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return Instance{}, err
	}
	current, err := s.loadInstance(ctx, p, id)
	if err != nil {
		return Instance{}, err
	}
	if _, err := s.require(ctx, "update", id.String(), &current.WorkspaceID); err != nil {
		return Instance{}, err
	}
	if in.Config == nil && in.AuthMethod == nil && in.SecretID == nil && in.RateLimit == nil && in.TimeoutMs == nil && in.Status == nil {
		return Instance{}, validation("body", "empty", "At least one field must be provided.")
	}
	var configJSON []byte
	if in.Config != nil {
		var schema json.RawMessage
		err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
			var detail instanceDetail
			if err := tx.GetContext(ctx, &detail, q.Get("GetInstance"), id); err != nil {
				return translateNotFound(err)
			}
			schema = detail.ConfigSchema
			return nil
		})
		if err != nil {
			return Instance{}, err
		}
		configJSON, err = json.Marshal(*in.Config)
		if err != nil {
			return Instance{}, validation("config", "invalid", "Configuration must be valid JSON.")
		}
		if err := validateConfig(schema, configJSON); err != nil {
			return Instance{}, err
		}
	}
	if in.AuthMethod != nil {
		var methods []byte
		err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
			var detail instanceDetail
			if err := tx.GetContext(ctx, &detail, q.Get("GetInstance"), id); err != nil {
				return translateNotFound(err)
			}
			methods = detail.AuthMethods
			return nil
		})
		if err != nil {
			return Instance{}, err
		}
		if !containsJSONString(methods, *in.AuthMethod) {
			return Instance{}, validation("auth_method", "unsupported", "The selected authentication method is not supported by this connector version.")
		}
	}
	if in.RateLimit != nil && (*in.RateLimit < 1 || *in.RateLimit > 10000) {
		return Instance{}, validation("rate_limit", "out_of_range", "Rate limit must be between 1 and 10000.")
	}
	if in.TimeoutMs != nil && (*in.TimeoutMs < 100 || *in.TimeoutMs > 120000) {
		return Instance{}, validation("timeout_ms", "out_of_range", "Timeout must be between 100 and 120000 milliseconds.")
	}
	if in.Status != nil && *in.Status != "enabled" && *in.Status != "disabled" {
		return Instance{}, validation("status", "invalid", "Status must be enabled or disabled.")
	}
	if in.SecretID != nil {
		if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
			return validateSecret(ctx, tx, p.OrgID, *in.SecretID, current.EnvironmentID)
		}); err != nil {
			return Instance{}, err
		}
		if _, err := authz.Require(ctx, s.az, "use", authz.Resource{Type: "secret", ID: in.SecretID.String(), OrgID: p.OrgID}); err != nil {
			return Instance{}, err
		}
	}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		result, err := tx.ExecContext(ctx, q.Get("UpdateInstance"), id, nullableJSON(configJSON), in.AuthMethod,
			nullableUUID(in.SecretID), in.RateLimit, in.TimeoutMs, in.Status)
		if err != nil {
			return translateWriteError(err, current.RefKey)
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return errNotFound
		}
		var detail instanceDetail
		if err := tx.GetContext(ctx, &detail, q.Get("GetInstance"), id); err != nil {
			return err
		}
		current = detail.Instance
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "connector.updated",
			ResourceType: Resource, ResourceID: id.String(), WorkspaceID: &current.WorkspaceID,
			Details: map[string]any{"ref_key": current.RefKey}})
	})
	current.Config = sanitizeConfig(current.Config)
	return current, err
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
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
		result, err := tx.ExecContext(ctx, q.Get("DeleteInstance"), id)
		if err != nil {
			return err
		}
		if n, err := result.RowsAffected(); err != nil {
			return err
		} else if n == 0 {
			return errNotFound
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "connector.deleted",
			ResourceType: Resource, ResourceID: id.String(), WorkspaceID: &current.WorkspaceID,
			Details: map[string]any{"ref_key": current.RefKey}})
	})
}

func (s *Service) Test(ctx context.Context, id uuid.UUID) (TestResult, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return TestResult{}, err
	}
	current, err := s.loadInstance(ctx, p, id)
	if err != nil {
		return TestResult{}, err
	}
	if _, err := s.require(ctx, "test", id.String(), &current.WorkspaceID); err != nil {
		return TestResult{}, err
	}
	policy, err := s.environmentPolicy(ctx, p, current.EnvironmentID)
	if err != nil {
		return TestResult{}, err
	}
	target, err := endpointFromConfig(current.Config)
	var result TestResult
	if err == nil {
		result = checkEndpoint(ctx, target, policy, current.TimeoutMs)
	} else {
		address, tcpErr := tcpEndpointFromConfig(current.Config)
		if tcpErr != nil {
			return TestResult{}, err
		}
		result = checkTCP(ctx, address, policy, current.TimeoutMs)
	}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, q.Get("InsertHealth"), p.OrgID, id, result.OK, result.LatencyMs, result.ErrorCode, result.Message); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "connector.tested",
			ResourceType: Resource, ResourceID: id.String(), WorkspaceID: &current.WorkspaceID,
			Details: map[string]any{"ok": result.OK, "error_code": result.ErrorCode}})
	})
	return result, err
}

func (s *Service) Health(ctx context.Context, id uuid.UUID) ([]Health, error) {
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
	out := []Health{}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &out, q.Get("ListHealth"), id)
	})
	return out, err
}

func (s *Service) loadInstance(ctx context.Context, p principal.Principal, id uuid.UUID) (Instance, error) {
	var out instanceDetail
	err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		if err := tx.GetContext(ctx, &out, q.Get("GetInstance"), id); err != nil {
			return translateNotFound(err)
		}
		return nil
	})
	return out.Instance, err
}

func (s *Service) environmentPolicy(ctx context.Context, p principal.Principal, environmentID uuid.UUID) (egress.Policy, error) {
	var encoded []byte
	err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		if err := tx.GetContext(ctx, &encoded, q.Get("GetEnvironmentEgressAllowlist"), environmentID, p.OrgID); err != nil {
			return translateNotFound(err)
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
	err := tx.GetContext(ctx, &found, q.Get("GetSecretForInstance"), secretID, orgID, environmentID)
	if errors.Is(err, sql.ErrNoRows) {
		return validation("secret_id", "not_found", "The credential does not exist in this organization and environment.")
	}
	return err
}

func translateWriteError(err error, refKey string) error {
	if db.IsUniqueViolation(err) {
		return httpx.NewProblem(http.StatusConflict, "connector_ref_key_taken", "Conflict",
			fmt.Sprintf("A connector with reference key %q already exists in this workspace and environment.", refKey))
	}
	if db.IsForeignKeyViolation(err) {
		return validation("resource_id", "not_found", "A workspace, environment, version, or credential does not exist in this organization.")
	}
	return err
}

func translateNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return errNotFound
	}
	return err
}

func validation(field, code, message string) error {
	return httpx.Validation(httpx.FieldError{Field: field, Code: code, Message: message})
}

func nullableUUID(v *uuid.UUID) any {
	if v == nil || *v == uuid.Nil {
		return nil
	}
	return *v
}

func nullableJSON(v []byte) any {
	if v == nil {
		return nil
	}
	return v
}

func validRefKey(s string) bool {
	if len(s) < 2 || len(s) > 64 || s[0] < 'a' || s[0] > 'z' {
		return false
	}
	for _, r := range s[1:] {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' && r != '-' {
			return false
		}
	}
	return true
}
