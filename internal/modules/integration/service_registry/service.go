// Package servicereg implements the workspace-scoped service catalog and
// OpenAPI explorer.
package servicereg

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/modules/authorization"
	"github.com/nuriansyah/hugimethara/internal/modules/governance/audit"
	"github.com/nuriansyah/hugimethara/internal/platform/database"
	"github.com/nuriansyah/hugimethara/internal/platform/egress"
	"github.com/nuriansyah/hugimethara/internal/platform/httpx"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

const (
	Resource           = "service"
	maxSpecBytes       = 2 << 20
	maxExplorerBytes   = 1 << 20
	defaultHTTPTimeout = 20 * time.Second
)

var errServiceNotFound = httpx.NotFound("Service not found.")

type RegisteredService struct {
	ID            uuid.UUID
	WorkspaceID   uuid.UUID
	Name          string
	Owner         string
	Description   string
	Protocol      string
	EndpointCount int64
	Environments  []Environment
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Environment struct {
	EnvironmentID       uuid.UUID
	BaseURL             string
	ConnectorInstanceID *uuid.UUID
}

type Endpoint struct {
	ID              uuid.UUID
	ServiceID       uuid.UUID
	OperationID     string
	Method          string
	Path            string
	Summary         string
	Parameters      []map[string]any
	RequestSchema   map[string]any
	ResponseSchemas map[string]any
	Security        []map[string]any
	Deprecated      bool
}

type CreateInput struct {
	WorkspaceID uuid.UUID
	Name        string
	Owner       string
	Description string
	Protocol    string
}

type UpdateInput struct {
	Name        *string
	Owner       *string
	Description *string
	Protocol    *string
}

type ImportInput struct {
	Spec         *string
	URL          *string
	VersionLabel string
	DryRun       bool
}

type ServiceTryInput struct {
	EnvironmentID uuid.UUID
	Body          any
	Headers       map[string]string
	PathValues    map[string]string
	Query         map[string]string
}

type serviceRow struct {
	ID            uuid.UUID `db:"id"`
	WorkspaceID   uuid.UUID `db:"workspace_id"`
	Name          string    `db:"name"`
	Owner         string    `db:"owner"`
	Description   string    `db:"description"`
	Protocol      string    `db:"protocol"`
	EndpointCount int64     `db:"endpoint_count"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}

type environmentRow struct {
	EnvironmentID       uuid.UUID     `db:"environment_id"`
	BaseURL             string        `db:"base_url"`
	ConnectorInstanceID uuid.NullUUID `db:"connector_instance_id"`
}

type endpointRow struct {
	ID              uuid.UUID       `db:"id"`
	ServiceID       uuid.UUID       `db:"service_id"`
	OperationID     string          `db:"operation_id"`
	Method          string          `db:"method"`
	Path            string          `db:"path"`
	Summary         string          `db:"summary"`
	Parameters      json.RawMessage `db:"parameters"`
	RequestSchema   json.RawMessage `db:"request_schema"`
	ResponseSchemas json.RawMessage `db:"response_schemas"`
	Security        json.RawMessage `db:"security"`
	Deprecated      bool            `db:"deprecated"`
}

type diffEndpointRow struct {
	OperationID     string          `db:"operation_id"`
	Method          string          `db:"method"`
	Path            string          `db:"path"`
	Summary         string          `db:"summary"`
	Parameters      json.RawMessage `db:"parameters"`
	RequestSchema   json.RawMessage `db:"request_schema"`
	ResponseSchemas json.RawMessage `db:"response_schemas"`
	Security        json.RawMessage `db:"security"`
	Deprecated      bool            `db:"deprecated"`
}

type tryEndpointRow struct {
	ID              uuid.UUID       `db:"id"`
	ServiceID       uuid.UUID       `db:"service_id"`
	Method          string          `db:"method"`
	Path            string          `db:"path"`
	WorkspaceID     uuid.UUID       `db:"workspace_id"`
	BaseURL         string          `db:"base_url"`
	EgressAllowlist json.RawMessage `db:"egress_allowlist"`
}

type ServiceTryResult struct {
	Status     int
	Headers    map[string]string
	Body       any
	DurationMs int
}

type Service struct {
	pool       *sqlx.DB
	az         authz.Authorizer
	policy     egress.Policy
	httpClient *http.Client
}

func NewService(pool *sqlx.DB, az authz.Authorizer, policy egress.Policy) *Service {
	client := egress.NewClient(egress.Options{
		Policy: policy, Timeout: defaultHTTPTimeout, MaxRedirects: 3,
		MaxResponseBytes: maxSpecBytes,
	})
	if transport, ok := client.Transport.(*http.Transport); ok {
		transport.MaxResponseHeaderBytes = 64 << 10
	}
	return &Service{pool: pool, az: az, policy: policy, httpClient: client}
}

func (s *Service) principal(ctx context.Context) (principal.Principal, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return principal.Principal{}, authz.Unauthenticated()
	}
	return p, nil
}

func (s *Service) tenantTx(ctx context.Context, p principal.Principal, fn func(*sqlx.Tx) error) error {
	return db.InTenantTx(ctx, s.pool, p.OrgID, fn)
}

func (s *Service) authorize(ctx context.Context, p principal.Principal, action string, workspaceID uuid.UUID, id string) error {
	if workspaceID == uuid.Nil {
		return httpx.Validation(httpx.FieldError{Field: "workspace_id", Code: "required",
			Message: "A workspace is required."})
	}
	_, err := authz.Require(ctx, s.az, action, authz.Resource{
		Type: Resource, ID: id, OrgID: p.OrgID, WorkspaceID: &workspaceID,
	})
	return err
}

func (s *Service) List(ctx context.Context, workspaceID uuid.UUID) ([]RegisteredService, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, p, "read", workspaceID, ""); err != nil {
		return nil, err
	}
	var rows []serviceRow
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, q.Get("ListServices"), []uuid.UUID{workspaceID})
	}); err != nil {
		return nil, err
	}
	out := make([]RegisteredService, len(rows))
	for i := range rows {
		out[i] = rows[i].model()
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (RegisteredService, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return RegisteredService{}, err
	}
	var out RegisteredService
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := loadService(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "read", row.WorkspaceID, id.String()); err != nil {
			return err
		}
		out = row.model()
		var envs []environmentRow
		if err := tx.SelectContext(ctx, &envs, q.Get("ListServiceEnvironments"), id); err != nil {
			return err
		}
		out.Environments = make([]Environment, len(envs))
		for i, env := range envs {
			out.Environments[i] = env.model()
		}
		return nil
	})
	return out, err
}

func (s *Service) Create(ctx context.Context, in CreateInput) (RegisteredService, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return RegisteredService{}, err
	}
	if err := s.validateCreate(&in); err != nil {
		return RegisteredService{}, err
	}
	if err := s.authorize(ctx, p, "create", in.WorkspaceID, ""); err != nil {
		return RegisteredService{}, err
	}
	id := uuid.Must(uuid.NewV7())
	var out RegisteredService
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, q.Get("InsertService"), p.OrgID, id,
			in.WorkspaceID, in.Name, in.Owner, in.Description, in.Protocol); err != nil {
			if db.IsUniqueViolation(err) {
				return httpx.NewProblem(http.StatusConflict, "service_name_taken", "Conflict",
					"A service with this name already exists in the workspace.")
			}
			if db.IsForeignKeyViolation(err, "services_org_id_workspace_id_fkey") {
				return httpx.Validation(httpx.FieldError{Field: "workspace_id", Code: "not_found",
					Message: "The workspace does not exist in this organization."})
			}
			return err
		}
		row, err := loadService(ctx, tx, id)
		if err != nil {
			return err
		}
		out = row.model()
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "service.created",
			ResourceType: Resource, ResourceID: id.String(), WorkspaceID: &in.WorkspaceID,
			Details: map[string]any{"name": in.Name, "protocol": in.Protocol}})
	})
	return out, err
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (RegisteredService, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return RegisteredService{}, err
	}
	if in.Name == nil && in.Owner == nil && in.Description == nil && in.Protocol == nil {
		return RegisteredService{}, httpx.Validation(httpx.FieldError{Field: "body", Code: "empty",
			Message: "At least one field must be provided."})
	}
	if err := validateUpdate(&in); err != nil {
		return RegisteredService{}, err
	}
	var out RegisteredService
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := loadService(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "update", row.WorkspaceID, id.String()); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, q.Get("UpdateService"), id,
			in.Name, in.Owner, in.Description, in.Protocol); err != nil {
			if db.IsUniqueViolation(err) {
				return httpx.NewProblem(http.StatusConflict, "service_name_taken", "Conflict",
					"A service with this name already exists in the workspace.")
			}
			return err
		}
		updated, err := loadService(ctx, tx, id)
		if err != nil {
			return err
		}
		out = updated.model()
		details := map[string]any{}
		if in.Name != nil {
			details["name"] = *in.Name
		}
		if in.Owner != nil {
			details["owner"] = *in.Owner
		}
		if in.Description != nil {
			details["description"] = *in.Description
		}
		if in.Protocol != nil {
			details["protocol"] = *in.Protocol
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "service.updated",
			ResourceType: Resource, ResourceID: id.String(), WorkspaceID: &row.WorkspaceID, Details: details})
	})
	return out, err
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	p, err := s.principal(ctx)
	if err != nil {
		return err
	}
	return s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := loadService(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "delete", row.WorkspaceID, id.String()); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, q.Get("DeleteService"), id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "service.deleted",
			ResourceType: Resource, ResourceID: id.String(), WorkspaceID: &row.WorkspaceID,
			Details: map[string]any{"name": row.Name}})
	})
}

func (s *Service) PutEnvironment(ctx context.Context, serviceID, environmentID uuid.UUID, baseURL string, connectorID *uuid.UUID) (Environment, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return Environment{}, err
	}
	baseURL, err = validateBaseURL(baseURL)
	if err != nil {
		return Environment{}, err
	}
	var out Environment
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := loadService(ctx, tx, serviceID)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "update", row.WorkspaceID, serviceID.String()); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, q.Get("UpsertServiceEnvironment"), p.OrgID,
			serviceID, environmentID, baseURL, nullableUUID(connectorID)); err != nil {
			if db.IsForeignKeyViolation(err, "service_environments_org_id_environment_id_fkey") {
				return httpx.Validation(httpx.FieldError{Field: "environment_id", Code: "not_found",
					Message: "The environment does not exist in this organization."})
			}
			if db.IsForeignKeyViolation(err, "service_environments_org_id_connector_instance_id_fkey") {
				return httpx.Validation(httpx.FieldError{Field: "connector_instance_id", Code: "not_found",
					Message: "The connector instance does not exist in this organization."})
			}
			return err
		}
		var env environmentRow
		if err := tx.GetContext(ctx, &env, q.Get("GetServiceEnvironment"), serviceID, environmentID); err != nil {
			return err
		}
		out = env.model()
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "service.environment_updated",
			ResourceType: Resource, ResourceID: serviceID.String(), WorkspaceID: &row.WorkspaceID,
			Details: map[string]any{"environment_id": environmentID.String()}})
	})
	return out, err
}

func (s *Service) DeleteEnvironment(ctx context.Context, serviceID, environmentID uuid.UUID) error {
	p, err := s.principal(ctx)
	if err != nil {
		return err
	}
	return s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := loadService(ctx, tx, serviceID)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "update", row.WorkspaceID, serviceID.String()); err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, q.Get("DeleteServiceEnvironment"), serviceID, environmentID)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return errServiceNotFound
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "service.environment_deleted",
			ResourceType: Resource, ResourceID: serviceID.String(), WorkspaceID: &row.WorkspaceID,
			Details: map[string]any{"environment_id": environmentID.String()}})
	})
}

func (s *Service) ListEndpoints(ctx context.Context, serviceID uuid.UUID) ([]Endpoint, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	var rows []endpointRow
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := loadService(ctx, tx, serviceID)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "read", row.WorkspaceID, serviceID.String()); err != nil {
			return err
		}
		return tx.SelectContext(ctx, &rows, q.Get("ListServiceEndpoints"), serviceID)
	})
	if err != nil {
		return nil, err
	}
	out := make([]Endpoint, len(rows))
	for i, row := range rows {
		out[i], err = row.model()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Service) ImportOpenAPI(ctx context.Context, serviceID uuid.UUID, in ImportInput) (ImportResult, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return ImportResult{}, err
	}
	var service serviceRow
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		var err error
		service, err = loadService(ctx, tx, serviceID)
		if err != nil {
			return err
		}
		return s.authorize(ctx, p, "update", service.WorkspaceID, serviceID.String())
	}); err != nil {
		return ImportResult{}, err
	}
	if (in.Spec == nil) == (in.URL == nil) {
		return ImportResult{}, httpx.Validation(httpx.FieldError{Field: "spec", Code: "one_of",
			Message: "Supply exactly one of spec or url."})
	}
	var raw []byte
	format := "json"
	if in.Spec != nil {
		if len(*in.Spec) > maxSpecBytes {
			return ImportResult{}, invalidSpec("The OpenAPI document exceeds the size limit.")
		}
		raw = []byte(*in.Spec)
		if !json.Valid(raw) {
			format = "yaml"
		}
	} else {
		if len(*in.URL) > 2000 {
			return ImportResult{}, httpx.Validation(httpx.FieldError{Field: "url", Code: "too_long",
				Message: "The OpenAPI URL must be no more than 2000 characters."})
		}
		fetchURL, err := validateSourceURL(*in.URL)
		if err != nil {
			return ImportResult{}, err
		}
		var req *http.Request
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, fetchURL.String(), nil)
		if err != nil {
			return ImportResult{}, invalidSpec("The OpenAPI URL is invalid.")
		}
		resp, err := s.httpClient.Do(req)
		if err != nil {
			return ImportResult{}, httpx.NewProblem(http.StatusUnprocessableEntity, "openapi_fetch_failed",
				"OpenAPI import failed", "The OpenAPI document could not be fetched.")
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			if err := resp.Body.Close(); err != nil {
				return ImportResult{}, httpx.NewProblem(http.StatusUnprocessableEntity, "openapi_fetch_failed",
					"OpenAPI import failed", "The OpenAPI URL returned an unsuccessful status.")
			}
			return ImportResult{}, httpx.NewProblem(http.StatusUnprocessableEntity, "openapi_fetch_failed",
				"OpenAPI import failed", "The OpenAPI URL returned an unsuccessful status.")
		}
		raw, err = io.ReadAll(egress.LimitBody(resp.Body, maxSpecBytes))
		closeErr := resp.Body.Close()
		if err != nil {
			return ImportResult{}, invalidSpec("The OpenAPI document could not be read within the size limit.")
		}
		if closeErr != nil {
			return ImportResult{}, httpx.NewProblem(http.StatusUnprocessableEntity, "openapi_fetch_failed",
				"OpenAPI import failed", "The OpenAPI response could not be completed.")
		}
		format = formatFromMediaType(resp.Header.Get("Content-Type"), raw)
	}

	doc, endpoints, canonical, err := parseOpenAPI(raw)
	if err != nil {
		return ImportResult{}, err
	}
	if in.VersionLabel == "" && doc.Info != nil {
		in.VersionLabel = doc.Info.Version
	}
	if len(in.VersionLabel) > 100 {
		return ImportResult{}, httpx.Validation(httpx.FieldError{Field: "version_label", Code: "too_long",
			Message: "The version label must be no more than 100 characters."})
	}

	var result ImportResult
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		current, err := loadService(ctx, tx, serviceID)
		if err != nil {
			return err
		}
		if current.WorkspaceID != service.WorkspaceID {
			return errServiceNotFound
		}
		if err := s.authorize(ctx, p, "update", current.WorkspaceID, serviceID.String()); err != nil {
			return err
		}
		var previous []diffEndpointRow
		if err := tx.SelectContext(ctx, &previous, q.Get("ListServiceEndpointsForDiff"), serviceID); err != nil {
			return err
		}
		result = Diff(endpoints, previous)
		result.DryRun = in.DryRun
		result.EndpointCount = len(endpoints)
		if in.DryRun {
			return nil
		}
		specID := uuid.Must(uuid.NewV7())
		hash := sha256.Sum256(raw)
		if _, err := tx.ExecContext(ctx, q.Get("InsertServiceSpec"), p.OrgID, specID, serviceID,
			format, in.VersionLabel, hex.EncodeToString(hash[:]), canonical); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, q.Get("DeleteServiceEndpoints"), serviceID); err != nil {
			return err
		}
		for _, endpoint := range endpoints {
			params, err := json.Marshal(endpoint.Parameters)
			if err != nil {
				return err
			}
			responseSchemas, err := json.Marshal(endpoint.ResponseSchemas)
			if err != nil {
				return err
			}
			security, err := json.Marshal(endpoint.Security)
			if err != nil {
				return err
			}
			var requestSchema any
			if endpoint.RequestSchema != nil {
				requestSchema, err = json.Marshal(endpoint.RequestSchema)
				if err != nil {
					return err
				}
			}
			if _, err := tx.ExecContext(ctx, q.Get("InsertServiceEndpoint"), p.OrgID,
				uuid.Must(uuid.NewV7()), serviceID, specID, endpoint.OperationID, endpoint.Method,
				endpoint.Path, endpoint.Summary, params, requestSchema, responseSchemas, security,
				endpoint.Deprecated); err != nil {
				return err
			}
		}
		result.SpecID = &specID
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "service.openapi_imported",
			ResourceType: Resource, ResourceID: serviceID.String(), WorkspaceID: &current.WorkspaceID,
			Details: map[string]any{"endpoint_count": len(endpoints), "version_label": in.VersionLabel}})
	})
	return result, err
}

func (s *Service) TryEndpoint(ctx context.Context, endpointID uuid.UUID, in ServiceTryInput) (ServiceTryResult, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return ServiceTryResult{}, err
	}
	var endpoint tryEndpointRow
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		if err := tx.GetContext(ctx, &endpoint, q.Get("GetEndpointForTry"), endpointID, in.EnvironmentID); err != nil {
			if db.IsNoRows(err) {
				return errServiceNotFound
			}
			return err
		}
		return s.authorize(ctx, p, "invoke", endpoint.WorkspaceID, endpoint.ServiceID.String())
	})
	if err != nil {
		return ServiceTryResult{}, err
	}
	environmentPolicy, err := s.environmentPolicy(endpoint.EgressAllowlist)
	if err != nil {
		return ServiceTryResult{}, err
	}
	allowed := append([]netip.Prefix(nil), s.policy.Allow...)
	allowed = append(allowed, environmentPolicy.Allow...)
	client := egress.NewClient(egress.Options{
		Policy: egress.Policy{Allow: allowed}, Timeout: defaultHTTPTimeout,
		MaxRedirects: 3, MaxResponseBytes: maxExplorerBytes,
	})
	if transport, ok := client.Transport.(*http.Transport); ok {
		transport.MaxResponseHeaderBytes = 64 << 10
	}
	target, err := buildEndpointURL(endpoint.BaseURL, endpoint.Path, in.PathValues, in.Query)
	if err != nil {
		return ServiceTryResult{}, err
	}
	var body io.Reader
	if in.Body != nil {
		data, err := json.Marshal(in.Body)
		if err != nil {
			return ServiceTryResult{}, httpx.Validation(httpx.FieldError{Field: "body", Code: "invalid",
				Message: "The request body must be valid JSON."})
		}
		if len(data) > maxExplorerBytes {
			return ServiceTryResult{}, httpx.Validation(httpx.FieldError{Field: "body", Code: "too_large",
				Message: "The request body exceeds the size limit."})
		}
		body = strings.NewReader(string(data))
	}
	if len(in.Headers) > 64 {
		return ServiceTryResult{}, httpx.Validation(httpx.FieldError{Field: "headers", Code: "too_many",
			Message: "No more than 64 request headers are allowed."})
	}
	req, err := http.NewRequestWithContext(ctx, endpoint.Method, target, body)
	if err != nil {
		return ServiceTryResult{}, httpx.Validation(httpx.FieldError{Field: "path", Code: "invalid",
			Message: "The endpoint request could not be constructed."})
	}
	headerBytes := 0
	for name, value := range in.Headers {
		if !validRequestHeader(name, value) {
			return ServiceTryResult{}, httpx.Validation(httpx.FieldError{Field: "headers", Code: "invalid",
				Message: "One or more request headers are not allowed."})
		}
		headerBytes += len(name) + len(value)
		if headerBytes > 16<<10 {
			return ServiceTryResult{}, httpx.Validation(httpx.FieldError{Field: "headers", Code: "too_large",
				Message: "The request headers exceed the size limit."})
		}
		req.Header.Set(name, value)
	}
	if in.Body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	started := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return ServiceTryResult{}, httpx.NewProblem(http.StatusUnprocessableEntity, "service_request_failed",
			"Request failed", "The service endpoint request could not be completed.")
	}
	data, err := io.ReadAll(egress.LimitBody(resp.Body, maxExplorerBytes))
	closeErr := resp.Body.Close()
	if err != nil {
		return ServiceTryResult{}, httpx.NewProblem(http.StatusUnprocessableEntity, "response_too_large",
			"Response too large", "The service response exceeded the size limit.")
	}
	if closeErr != nil {
		return ServiceTryResult{}, httpx.NewProblem(http.StatusUnprocessableEntity, "service_request_failed",
			"Request failed", "The service response could not be completed.")
	}
	var responseBody any
	if len(data) > 0 {
		if json.Unmarshal(data, &responseBody) != nil {
			responseBody = string(data)
		}
	}
	return ServiceTryResult{Status: resp.StatusCode, Headers: filteredResponseHeaders(resp.Header),
		Body: responseBody, DurationMs: int(time.Since(started).Milliseconds())}, nil
}

func (s *Service) environmentPolicy(encoded []byte) (egress.Policy, error) {
	var allowlist []string
	if err := json.Unmarshal(encoded, &allowlist); err != nil || allowlist == nil {
		return egress.Policy{}, errors.New("servicereg: invalid environment egress allowlist")
	}
	policy, err := egress.ParsePolicy(allowlist)
	if err != nil {
		return egress.Policy{}, errors.New("servicereg: invalid environment egress allowlist")
	}
	return policy, nil
}

func loadService(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (serviceRow, error) {
	var row serviceRow
	if err := tx.GetContext(ctx, &row, q.Get("GetService"), id); err != nil {
		if db.IsNoRows(err) {
			return row, errServiceNotFound
		}
		return row, err
	}
	return row, nil
}

func (r serviceRow) model() RegisteredService {
	return RegisteredService{ID: r.ID, WorkspaceID: r.WorkspaceID, Name: r.Name, Owner: r.Owner,
		Description: r.Description, Protocol: r.Protocol, EndpointCount: r.EndpointCount,
		Environments: []Environment{}, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

func (r environmentRow) model() Environment {
	out := Environment{EnvironmentID: r.EnvironmentID, BaseURL: r.BaseURL}
	if r.ConnectorInstanceID.Valid {
		id := r.ConnectorInstanceID.UUID
		out.ConnectorInstanceID = &id
	}
	return out
}

func (r endpointRow) model() (Endpoint, error) {
	out := Endpoint{ID: r.ID, ServiceID: r.ServiceID, OperationID: r.OperationID, Method: r.Method,
		Path: r.Path, Summary: r.Summary, Deprecated: r.Deprecated}
	if err := unmarshalJSON(r.Parameters, &out.Parameters); err != nil {
		return Endpoint{}, err
	}
	if len(r.RequestSchema) > 0 && string(r.RequestSchema) != "null" {
		out.RequestSchema = map[string]any{}
		if err := unmarshalJSON(r.RequestSchema, &out.RequestSchema); err != nil {
			return Endpoint{}, err
		}
	}
	if err := unmarshalJSON(r.ResponseSchemas, &out.ResponseSchemas); err != nil {
		return Endpoint{}, err
	}
	if err := unmarshalJSON(r.Security, &out.Security); err != nil {
		return Endpoint{}, err
	}
	return out, nil
}

func unmarshalJSON(data []byte, target any) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	return json.Unmarshal(data, target)
}

func (s *Service) validateCreate(in *CreateInput) error {
	in.Name = strings.TrimSpace(in.Name)
	in.Owner = strings.TrimSpace(in.Owner)
	in.Description = strings.TrimSpace(in.Description)
	if in.Protocol == "" {
		in.Protocol = "rest"
	}
	fields := []httpx.FieldError{}
	if in.WorkspaceID == uuid.Nil {
		fields = append(fields, httpx.FieldError{Field: "workspace_id", Code: "required", Message: "A workspace is required."})
	}
	if len(in.Name) < 1 || len(in.Name) > 100 {
		fields = append(fields, httpx.FieldError{Field: "name", Code: "invalid", Message: "Name must be between 1 and 100 characters."})
	}
	if len(in.Owner) > 100 {
		fields = append(fields, httpx.FieldError{Field: "owner", Code: "too_long", Message: "Owner must be no more than 100 characters."})
	}
	if len(in.Description) > 1000 {
		fields = append(fields, httpx.FieldError{Field: "description", Code: "too_long", Message: "Description must be no more than 1000 characters."})
	}
	if in.Protocol != "rest" && in.Protocol != "graphql" && in.Protocol != "grpc" {
		fields = append(fields, httpx.FieldError{Field: "protocol", Code: "invalid", Message: "Protocol must be rest, graphql, or grpc."})
	}
	if len(fields) != 0 {
		return httpx.Validation(fields...)
	}
	return nil
}

func validateUpdate(in *UpdateInput) error {
	fields := []httpx.FieldError{}
	if in.Name != nil {
		*in.Name = strings.TrimSpace(*in.Name)
		if len(*in.Name) < 1 || len(*in.Name) > 100 {
			fields = append(fields, httpx.FieldError{Field: "name", Code: "invalid", Message: "Name must be between 1 and 100 characters."})
		}
	}
	if in.Owner != nil {
		*in.Owner = strings.TrimSpace(*in.Owner)
		if len(*in.Owner) > 100 {
			fields = append(fields, httpx.FieldError{Field: "owner", Code: "too_long", Message: "Owner must be no more than 100 characters."})
		}
	}
	if in.Description != nil {
		*in.Description = strings.TrimSpace(*in.Description)
		if len(*in.Description) > 1000 {
			fields = append(fields, httpx.FieldError{Field: "description", Code: "too_long", Message: "Description must be no more than 1000 characters."})
		}
	}
	if in.Protocol != nil && *in.Protocol != "rest" && *in.Protocol != "graphql" && *in.Protocol != "grpc" {
		fields = append(fields, httpx.FieldError{Field: "protocol", Code: "invalid", Message: "Protocol must be rest, graphql, or grpc."})
	}
	if len(fields) != 0 {
		return httpx.Validation(fields...)
	}
	return nil
}

func nullableUUID(v *uuid.UUID) any {
	if v == nil || *v == uuid.Nil {
		return nil
	}
	return *v
}

func validateBaseURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" ||
		u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" ||
		strings.ContainsAny(u.Host+u.Path, "\r\n\\") {
		return "", httpx.Validation(httpx.FieldError{Field: "base_url", Code: "invalid",
			Message: "Base URL must be an HTTP(S) URL without credentials, query, or fragment."})
	}
	for _, segment := range strings.Split(u.Path, "/") {
		if segment == "." || segment == ".." {
			return "", httpx.Validation(httpx.FieldError{Field: "base_url", Code: "invalid",
				Message: "Base URL path must not contain traversal segments."})
		}
	}
	u.Path = strings.TrimRight(u.Path, "/")
	return u.String(), nil
}

func validateSourceURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" ||
		u.User != nil || u.Fragment != "" || strings.ContainsAny(u.Host, "\r\n") {
		return nil, httpx.Validation(httpx.FieldError{Field: "url", Code: "invalid",
			Message: "The OpenAPI source must be an HTTP(S) URL without credentials or a fragment."})
	}
	return u, nil
}

func formatFromMediaType(contentType string, data []byte) string {
	if strings.Contains(strings.ToLower(contentType), "yaml") {
		return "yaml"
	}
	if !json.Valid(data) {
		return "yaml"
	}
	return "json"
}

func invalidSpec(message string) error {
	return httpx.NewProblem(http.StatusUnprocessableEntity, "invalid_openapi", "Invalid OpenAPI document", message)
}

func translateOpenAPIError(err error) error {
	if err == nil {
		return nil
	}
	return invalidSpec("The document is not a valid OpenAPI 3.0 or 3.1 document.")
}
