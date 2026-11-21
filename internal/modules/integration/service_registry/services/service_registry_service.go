package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/infrastructure/security/egress"
	authzmodel "github.com/nuriansyah/hugimethara/internal/modules/authorization/model"
	authzsvc "github.com/nuriansyah/hugimethara/internal/modules/authorization/services"
	auditmodel "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/model"
	auditsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/services"
	serviceregmodel "github.com/nuriansyah/hugimethara/internal/modules/integration/service_registry/model"
	serviceregdto "github.com/nuriansyah/hugimethara/internal/modules/integration/service_registry/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/integration/service_registry/repository"
	"github.com/nuriansyah/hugimethara/internal/modules/integration/service_registry/repository/commands"
	"github.com/nuriansyah/hugimethara/internal/modules/integration/service_registry/repository/queries"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

type ServiceRegistryService struct {
	pool       *sqlx.DB
	az         authzsvc.Authorizer
	policy     egress.Policy
	httpClient *http.Client
}

func NewService(pool *sqlx.DB, az authzsvc.Authorizer, policy egress.Policy) *ServiceRegistryService {
	client := egress.NewClient(egress.Options{
		Policy: policy, Timeout: serviceregmodel.DefaultHTTPTimeout, MaxRedirects: 3,
		MaxResponseBytes: serviceregmodel.MaxSpecBytes,
	})
	if transport, ok := client.Transport.(*http.Transport); ok {
		transport.MaxResponseHeaderBytes = 64 << 10
	}
	return &ServiceRegistryService{pool: pool, az: az, policy: policy, httpClient: client}
}

func (s *ServiceRegistryService) principal(ctx context.Context) (principal.Principal, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return principal.Principal{}, authzmodel.Unauthenticated()
	}
	return p, nil
}

func (s *ServiceRegistryService) tenantTx(ctx context.Context, p principal.Principal, fn func(*sqlx.Tx) error) error {
	return db.InTenantTx(ctx, s.pool, p.OrgID, fn)
}

func (s *ServiceRegistryService) authorize(ctx context.Context, p principal.Principal, action string, workspaceID uuid.UUID, id string) error {
	if workspaceID == uuid.Nil {
		return problem.Validation(problem.FieldError{Field: "workspace_id", Code: "required",
			Message: "A workspace is required."})
	}
	_, err := authzsvc.Require(ctx, s.az, action, authzmodel.Resource{
		Type: serviceregmodel.Resource, ID: id, OrgID: p.OrgID, WorkspaceID: &workspaceID,
	})
	return err
}

func (s *ServiceRegistryService) List(ctx context.Context, workspaceID uuid.UUID) ([]serviceregmodel.RegisteredService, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, p, "read", workspaceID, ""); err != nil {
		return nil, err
	}
	var rows []repository.ServiceRow
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, queries.SQL.Get("ListServices"), []uuid.UUID{workspaceID})
	}); err != nil {
		return nil, err
	}
	out := make([]serviceregmodel.RegisteredService, len(rows))
	for i := range rows {
		out[i] = rows[i].Model()
	}
	return out, nil
}

func (s *ServiceRegistryService) Get(ctx context.Context, id uuid.UUID) (serviceregmodel.RegisteredService, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return serviceregmodel.RegisteredService{}, err
	}
	var out serviceregmodel.RegisteredService
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := loadService(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "read", row.WorkspaceID, id.String()); err != nil {
			return err
		}
		out = row.Model()
		var envs []repository.EnvironmentRow
		if err := tx.SelectContext(ctx, &envs, queries.SQL.Get("ListServiceEnvironments"), id); err != nil {
			return err
		}
		out.Environments = make([]serviceregmodel.Environment, len(envs))
		for i, env := range envs {
			out.Environments[i] = env.Model()
		}
		return nil
	})
	return out, err
}

func (s *ServiceRegistryService) Create(ctx context.Context, in serviceregdto.CreateInput) (serviceregmodel.RegisteredService, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return serviceregmodel.RegisteredService{}, err
	}
	if err := s.validateCreate(&in); err != nil {
		return serviceregmodel.RegisteredService{}, err
	}
	if err := s.authorize(ctx, p, "create", in.WorkspaceID, ""); err != nil {
		return serviceregmodel.RegisteredService{}, err
	}
	id := uuid.Must(uuid.NewV7())
	var out serviceregmodel.RegisteredService
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertService"), p.OrgID, id,
			in.WorkspaceID, in.Name, in.Owner, in.Description, in.Protocol); err != nil {
			if db.IsUniqueViolation(err) {
				return problem.NewProblem(http.StatusConflict, "service_name_taken", "Conflict",
					"A service with this name already exists in the workspace.")
			}
			if db.IsForeignKeyViolation(err, "services_org_id_workspace_id_fkey") {
				return problem.Validation(problem.FieldError{Field: "workspace_id", Code: "not_found",
					Message: "The workspace does not exist in this organization."})
			}
			return err
		}
		row, err := loadService(ctx, tx, id)
		if err != nil {
			return err
		}
		out = row.Model()
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "service.created",
			ResourceType: serviceregmodel.Resource, ResourceID: id.String(), WorkspaceID: &in.WorkspaceID,
			Details: map[string]any{"name": in.Name, "protocol": in.Protocol}})
	})
	return out, err
}

func (s *ServiceRegistryService) Update(ctx context.Context, id uuid.UUID, in serviceregdto.UpdateInput) (serviceregmodel.RegisteredService, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return serviceregmodel.RegisteredService{}, err
	}
	if in.Name == nil && in.Owner == nil && in.Description == nil && in.Protocol == nil {
		return serviceregmodel.RegisteredService{}, problem.Validation(problem.FieldError{Field: "body", Code: "empty",
			Message: "At least one field must be provided."})
	}
	if err := serviceregdto.ValidateUpdate(&in); err != nil {
		return serviceregmodel.RegisteredService{}, err
	}
	var out serviceregmodel.RegisteredService
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := loadService(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "update", row.WorkspaceID, id.String()); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("UpdateService"), id,
			in.Name, in.Owner, in.Description, in.Protocol); err != nil {
			if db.IsUniqueViolation(err) {
				return problem.NewProblem(http.StatusConflict, "service_name_taken", "Conflict",
					"A service with this name already exists in the workspace.")
			}
			return err
		}
		updated, err := loadService(ctx, tx, id)
		if err != nil {
			return err
		}
		out = updated.Model()
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
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "service.updated",
			ResourceType: serviceregmodel.Resource, ResourceID: id.String(), WorkspaceID: &row.WorkspaceID, Details: details})
	})
	return out, err
}

func (s *ServiceRegistryService) Delete(ctx context.Context, id uuid.UUID) error {
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
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("DeleteService"), id); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "service.deleted",
			ResourceType: serviceregmodel.Resource, ResourceID: id.String(), WorkspaceID: &row.WorkspaceID,
			Details: map[string]any{"name": row.Name}})
	})
}

func (s *ServiceRegistryService) PutEnvironment(ctx context.Context, serviceID, environmentID uuid.UUID, baseURL string, connectorID *uuid.UUID) (serviceregmodel.Environment, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return serviceregmodel.Environment{}, err
	}
	baseURL, err = serviceregmodel.ValidateBaseURL(baseURL)
	if err != nil {
		return serviceregmodel.Environment{}, err
	}
	var out serviceregmodel.Environment
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := loadService(ctx, tx, serviceID)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "update", row.WorkspaceID, serviceID.String()); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("UpsertServiceEnvironment"), p.OrgID,
			serviceID, environmentID, baseURL, serviceregmodel.NullableUUID(connectorID)); err != nil {
			if db.IsForeignKeyViolation(err, "service_environments_org_id_environment_id_fkey") {
				return problem.Validation(problem.FieldError{Field: "environment_id", Code: "not_found",
					Message: "The environment does not exist in this organization."})
			}
			if db.IsForeignKeyViolation(err, "service_environments_org_id_connector_instance_id_fkey") {
				return problem.Validation(problem.FieldError{Field: "connector_instance_id", Code: "not_found",
					Message: "The connector instance does not exist in this organization."})
			}
			return err
		}
		var env repository.EnvironmentRow
		if err := tx.GetContext(ctx, &env, queries.SQL.Get("GetServiceEnvironment"), serviceID, environmentID); err != nil {
			return err
		}
		out = env.Model()
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "service.environment_updated",
			ResourceType: serviceregmodel.Resource, ResourceID: serviceID.String(), WorkspaceID: &row.WorkspaceID,
			Details: map[string]any{"environment_id": environmentID.String()}})
	})
	return out, err
}

func (s *ServiceRegistryService) DeleteEnvironment(ctx context.Context, serviceID, environmentID uuid.UUID) error {
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
		result, err := tx.ExecContext(ctx, commands.SQL.Get("DeleteServiceEnvironment"), serviceID, environmentID)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return serviceregmodel.ErrServiceNotFound
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "service.environment_deleted",
			ResourceType: serviceregmodel.Resource, ResourceID: serviceID.String(), WorkspaceID: &row.WorkspaceID,
			Details: map[string]any{"environment_id": environmentID.String()}})
	})
}

func (s *ServiceRegistryService) ListEndpoints(ctx context.Context, serviceID uuid.UUID) ([]serviceregmodel.Endpoint, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	var rows []repository.EndpointRow
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		row, err := loadService(ctx, tx, serviceID)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, p, "read", row.WorkspaceID, serviceID.String()); err != nil {
			return err
		}
		return tx.SelectContext(ctx, &rows, queries.SQL.Get("ListServiceEndpoints"), serviceID)
	})
	if err != nil {
		return nil, err
	}
	out := make([]serviceregmodel.Endpoint, len(rows))
	for i, row := range rows {
		out[i], err = row.Model()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *ServiceRegistryService) ImportOpenAPI(ctx context.Context, serviceID uuid.UUID, in serviceregdto.ImportInput) (serviceregmodel.ImportResult, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return serviceregmodel.ImportResult{}, err
	}
	var service repository.ServiceRow
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		var err error
		service, err = loadService(ctx, tx, serviceID)
		if err != nil {
			return err
		}
		return s.authorize(ctx, p, "update", service.WorkspaceID, serviceID.String())
	}); err != nil {
		return serviceregmodel.ImportResult{}, err
	}
	if (in.Spec == nil) == (in.URL == nil) {
		return serviceregmodel.ImportResult{}, problem.Validation(problem.FieldError{Field: "spec", Code: "one_of",
			Message: "Supply exactly one of spec or url."})
	}
	var raw []byte
	format := "json"
	if in.Spec != nil {
		if len(*in.Spec) > serviceregmodel.MaxSpecBytes {
			return serviceregmodel.ImportResult{}, serviceregmodel.InvalidSpec("The OpenAPI document exceeds the size limit.")
		}
		raw = []byte(*in.Spec)
		if !json.Valid(raw) {
			format = "yaml"
		}
	} else {
		if len(*in.URL) > 2000 {
			return serviceregmodel.ImportResult{}, problem.Validation(problem.FieldError{Field: "url", Code: "too_long",
				Message: "The OpenAPI URL must be no more than 2000 characters."})
		}
		fetchURL, err := serviceregmodel.ValidateSourceURL(*in.URL)
		if err != nil {
			return serviceregmodel.ImportResult{}, err
		}
		var req *http.Request
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, fetchURL.String(), nil)
		if err != nil {
			return serviceregmodel.ImportResult{}, serviceregmodel.InvalidSpec("The OpenAPI URL is invalid.")
		}
		resp, err := s.httpClient.Do(req)
		if err != nil {
			return serviceregmodel.ImportResult{}, problem.NewProblem(http.StatusUnprocessableEntity, "openapi_fetch_failed",
				"OpenAPI import failed", "The OpenAPI document could not be fetched.")
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			if err := resp.Body.Close(); err != nil {
				return serviceregmodel.ImportResult{}, problem.NewProblem(http.StatusUnprocessableEntity, "openapi_fetch_failed",
					"OpenAPI import failed", "The OpenAPI URL returned an unsuccessful status.")
			}
			return serviceregmodel.ImportResult{}, problem.NewProblem(http.StatusUnprocessableEntity, "openapi_fetch_failed",
				"OpenAPI import failed", "The OpenAPI URL returned an unsuccessful status.")
		}
		raw, err = io.ReadAll(egress.LimitBody(resp.Body, serviceregmodel.MaxSpecBytes))
		closeErr := resp.Body.Close()
		if err != nil {
			return serviceregmodel.ImportResult{}, serviceregmodel.InvalidSpec("The OpenAPI document could not be read within the size limit.")
		}
		if closeErr != nil {
			return serviceregmodel.ImportResult{}, problem.NewProblem(http.StatusUnprocessableEntity, "openapi_fetch_failed",
				"OpenAPI import failed", "The OpenAPI response could not be completed.")
		}
		format = serviceregmodel.FormatFromMediaType(resp.Header.Get("Content-Type"), raw)
	}

	doc, endpoints, canonical, err := parseOpenAPI(raw)
	if err != nil {
		return serviceregmodel.ImportResult{}, err
	}
	if in.VersionLabel == "" && doc.Info != nil {
		in.VersionLabel = doc.Info.Version
	}
	if len(in.VersionLabel) > 100 {
		return serviceregmodel.ImportResult{}, problem.Validation(problem.FieldError{Field: "version_label", Code: "too_long",
			Message: "The version label must be no more than 100 characters."})
	}

	var result serviceregmodel.ImportResult
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		current, err := loadService(ctx, tx, serviceID)
		if err != nil {
			return err
		}
		if current.WorkspaceID != service.WorkspaceID {
			return serviceregmodel.ErrServiceNotFound
		}
		if err := s.authorize(ctx, p, "update", current.WorkspaceID, serviceID.String()); err != nil {
			return err
		}
		var previous []repository.DiffEndpointRow
		if err := tx.SelectContext(ctx, &previous, queries.SQL.Get("ListServiceEndpointsForDiff"), serviceID); err != nil {
			return err
		}
		result = repository.Diff(endpoints, previous)
		result.DryRun = in.DryRun
		result.EndpointCount = len(endpoints)
		if in.DryRun {
			return nil
		}
		specID := uuid.Must(uuid.NewV7())
		hash := sha256.Sum256(raw)
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertServiceSpec"), p.OrgID, specID, serviceID,
			format, in.VersionLabel, hex.EncodeToString(hash[:]), canonical); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("DeleteServiceEndpoints"), serviceID); err != nil {
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
			if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertServiceEndpoint"), p.OrgID,
				uuid.Must(uuid.NewV7()), serviceID, specID, endpoint.OperationID, endpoint.Method,
				endpoint.Path, endpoint.Summary, params, requestSchema, responseSchemas, security,
				endpoint.Deprecated); err != nil {
				return err
			}
		}
		result.SpecID = &specID
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "service.openapi_imported",
			ResourceType: serviceregmodel.Resource, ResourceID: serviceID.String(), WorkspaceID: &current.WorkspaceID,
			Details: map[string]any{"endpoint_count": len(endpoints), "version_label": in.VersionLabel}})
	})
	return result, err
}

func (s *ServiceRegistryService) TryEndpoint(ctx context.Context, endpointID uuid.UUID, in serviceregdto.ServiceTryInput) (serviceregmodel.ServiceTryResult, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return serviceregmodel.ServiceTryResult{}, err
	}
	var endpoint repository.TryEndpointRow
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		if err := tx.GetContext(ctx, &endpoint, queries.SQL.Get("GetEndpointForTry"), endpointID, in.EnvironmentID); err != nil {
			if db.IsNoRows(err) {
				return serviceregmodel.ErrServiceNotFound
			}
			return err
		}
		return s.authorize(ctx, p, "invoke", endpoint.WorkspaceID, endpoint.ServiceID.String())
	})
	if err != nil {
		return serviceregmodel.ServiceTryResult{}, err
	}
	environmentPolicy, err := s.environmentPolicy(endpoint.EgressAllowlist)
	if err != nil {
		return serviceregmodel.ServiceTryResult{}, err
	}
	allowed := append([]netip.Prefix(nil), s.policy.Allow...)
	allowed = append(allowed, environmentPolicy.Allow...)
	client := egress.NewClient(egress.Options{
		Policy: egress.Policy{Allow: allowed}, Timeout: serviceregmodel.DefaultHTTPTimeout,
		MaxRedirects: 3, MaxResponseBytes: serviceregmodel.MaxExplorerBytes,
	})
	if transport, ok := client.Transport.(*http.Transport); ok {
		transport.MaxResponseHeaderBytes = 64 << 10
	}
	target, err := serviceregmodel.BuildEndpointURL(endpoint.BaseURL, endpoint.Path, in.PathValues, in.Query)
	if err != nil {
		return serviceregmodel.ServiceTryResult{}, err
	}
	var body io.Reader
	if in.Body != nil {
		data, err := json.Marshal(in.Body)
		if err != nil {
			return serviceregmodel.ServiceTryResult{}, problem.Validation(problem.FieldError{Field: "body", Code: "invalid",
				Message: "The request body must be valid JSON."})
		}
		if len(data) > serviceregmodel.MaxExplorerBytes {
			return serviceregmodel.ServiceTryResult{}, problem.Validation(problem.FieldError{Field: "body", Code: "too_large",
				Message: "The request body exceeds the size limit."})
		}
		body = strings.NewReader(string(data))
	}
	if len(in.Headers) > 64 {
		return serviceregmodel.ServiceTryResult{}, problem.Validation(problem.FieldError{Field: "headers", Code: "too_many",
			Message: "No more than 64 request headers are allowed."})
	}
	req, err := http.NewRequestWithContext(ctx, endpoint.Method, target, body)
	if err != nil {
		return serviceregmodel.ServiceTryResult{}, problem.Validation(problem.FieldError{Field: "path", Code: "invalid",
			Message: "The endpoint request could not be constructed."})
	}
	headerBytes := 0
	for name, value := range in.Headers {
		if !serviceregmodel.ValidRequestHeader(name, value) {
			return serviceregmodel.ServiceTryResult{}, problem.Validation(problem.FieldError{Field: "headers", Code: "invalid",
				Message: "One or more request headers are not allowed."})
		}
		headerBytes += len(name) + len(value)
		if headerBytes > 16<<10 {
			return serviceregmodel.ServiceTryResult{}, problem.Validation(problem.FieldError{Field: "headers", Code: "too_large",
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
		return serviceregmodel.ServiceTryResult{}, problem.NewProblem(http.StatusUnprocessableEntity, "service_request_failed",
			"Request failed", "The service endpoint request could not be completed.")
	}
	data, err := io.ReadAll(egress.LimitBody(resp.Body, serviceregmodel.MaxExplorerBytes))
	closeErr := resp.Body.Close()
	if err != nil {
		return serviceregmodel.ServiceTryResult{}, problem.NewProblem(http.StatusUnprocessableEntity, "response_too_large",
			"Response too large", "The service response exceeded the size limit.")
	}
	if closeErr != nil {
		return serviceregmodel.ServiceTryResult{}, problem.NewProblem(http.StatusUnprocessableEntity, "service_request_failed",
			"Request failed", "The service response could not be completed.")
	}
	var responseBody any
	if len(data) > 0 {
		if json.Unmarshal(data, &responseBody) != nil {
			responseBody = string(data)
		}
	}
	return serviceregmodel.ServiceTryResult{Status: resp.StatusCode, Headers: serviceregmodel.FilteredResponseHeaders(resp.Header),
		Body: responseBody, DurationMs: int(time.Since(started).Milliseconds())}, nil
}

func (s *ServiceRegistryService) environmentPolicy(encoded []byte) (egress.Policy, error) {
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

func loadService(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (repository.ServiceRow, error) {
	var row repository.ServiceRow
	if err := tx.GetContext(ctx, &row, queries.SQL.Get("GetService"), id); err != nil {
		if db.IsNoRows(err) {
			return row, serviceregmodel.ErrServiceNotFound
		}
		return row, err
	}
	return row, nil
}

func (s *ServiceRegistryService) validateCreate(in *serviceregdto.CreateInput) error {
	in.Name = strings.TrimSpace(in.Name)
	in.Owner = strings.TrimSpace(in.Owner)
	in.Description = strings.TrimSpace(in.Description)
	if in.Protocol == "" {
		in.Protocol = "rest"
	}
	fields := []problem.FieldError{}
	if in.WorkspaceID == uuid.Nil {
		fields = append(fields, problem.FieldError{Field: "workspace_id", Code: "required", Message: "A workspace is required."})
	}
	if len(in.Name) < 1 || len(in.Name) > 100 {
		fields = append(fields, problem.FieldError{Field: "name", Code: "invalid", Message: "Name must be between 1 and 100 characters."})
	}
	if len(in.Owner) > 100 {
		fields = append(fields, problem.FieldError{Field: "owner", Code: "too_long", Message: "Owner must be no more than 100 characters."})
	}
	if len(in.Description) > 1000 {
		fields = append(fields, problem.FieldError{Field: "description", Code: "too_long", Message: "Description must be no more than 1000 characters."})
	}
	if in.Protocol != "rest" && in.Protocol != "graphql" && in.Protocol != "grpc" {
		fields = append(fields, problem.FieldError{Field: "protocol", Code: "invalid", Message: "Protocol must be rest, graphql, or grpc."})
	}
	if len(fields) != 0 {
		return problem.Validation(fields...)
	}
	return nil
}
