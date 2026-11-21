package serviceregistry

import (
	"context"

	"github.com/google/uuid"

	"github.com/nuriansyah/hugimethara/internal/modules/integration/service_registry/model"
	serviceregdto "github.com/nuriansyah/hugimethara/internal/modules/integration/service_registry/model/dto"
	serviceregsvc "github.com/nuriansyah/hugimethara/internal/modules/integration/service_registry/services"
	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/transport/http/dto"
)

type HTTP struct{ svc serviceregsvc.Service }

func NewHTTP(svc serviceregsvc.Service) *HTTP { return &HTTP{svc: svc} }

func apiService(s model.RegisteredService) dto.Service {
	envs := make([]dto.ServiceEnvironment, len(s.Environments))
	for i, env := range s.Environments {
		var connectorID *uuid.UUID
		if env.ConnectorInstanceID != nil {
			id := *env.ConnectorInstanceID
			connectorID = &id
		}
		envs[i] = dto.ServiceEnvironment{EnvironmentId: env.EnvironmentID, BaseUrl: env.BaseURL,
			ConnectorInstanceId: connectorID}
	}
	return dto.Service{Id: s.ID, WorkspaceId: s.WorkspaceID, Name: s.Name, Owner: s.Owner,
		Description: s.Description, Protocol: dto.ServiceProtocol(s.Protocol), EndpointCount: s.EndpointCount,
		Environments: envs, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt}
}

func apiEnvironment(env model.Environment) dto.ServiceEnvironment {
	var connectorID *uuid.UUID
	if env.ConnectorInstanceID != nil {
		id := *env.ConnectorInstanceID
		connectorID = &id
	}
	return dto.ServiceEnvironment{EnvironmentId: env.EnvironmentID, BaseUrl: env.BaseURL,
		ConnectorInstanceId: connectorID}
}

func apiEndpoint(endpoint model.Endpoint) dto.ServiceEndpoint {
	var requestSchema *map[string]interface{}
	if endpoint.RequestSchema != nil {
		schema := make(map[string]interface{}, len(endpoint.RequestSchema))
		for key, value := range endpoint.RequestSchema {
			schema[key] = value
		}
		requestSchema = &schema
	}
	return dto.ServiceEndpoint{Id: endpoint.ID, ServiceId: endpoint.ServiceID,
		OperationId: endpoint.OperationID, Method: endpoint.Method, Path: endpoint.Path,
		Summary: endpoint.Summary, Parameters: endpoint.Parameters, RequestSchema: requestSchema,
		ResponseSchemas: endpoint.ResponseSchemas, Security: endpoint.Security, Deprecated: endpoint.Deprecated}
}

// @Summary List registered services
// @ID listServices
// @Tags services
// @Param workspace_id query string true "workspace_id"
// @Success 200 {object} dto.ListServices200JSONResponse "Services in the current workspace."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/services [get]
func (h *HTTP) ListServices(ctx context.Context, req dto.ListServicesRequestObject) (dto.ListServicesResponseObject, error) {
	workspaceID := req.Params.WorkspaceId
	if workspaceID == uuid.Nil {
		return nil, problem.BadRequest("workspace_id is required.")
	}
	services, err := h.svc.List(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := dto.ListServices200JSONResponse{Data: make([]dto.Service, len(services))}
	for i := range services {
		out.Data[i] = apiService(services[i])
	}
	return out, nil
}

// @Summary Register an API service
// @ID createService
// @Tags services
// @Param body body dto.ServiceCreate true "Request body"
// @Success 201 {object} dto.Service "Registered service."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/services [post]
func (h *HTTP) CreateService(ctx context.Context, req dto.CreateServiceRequestObject) (dto.CreateServiceResponseObject, error) {
	if req.Body == nil {
		return nil, problem.BadRequest("A JSON body is required.")
	}
	in := serviceregdto.CreateInput{WorkspaceID: req.Body.WorkspaceId, Name: req.Body.Name}
	if req.Body.Owner != nil {
		in.Owner = *req.Body.Owner
	}
	if req.Body.Description != nil {
		in.Description = *req.Body.Description
	}
	if req.Body.Protocol != nil {
		in.Protocol = string(*req.Body.Protocol)
	}
	service, err := h.svc.Create(ctx, in)
	if err != nil {
		return nil, err
	}
	return dto.CreateService201JSONResponse(apiService(service)), nil
}

// @Summary Get a registered service
// @ID getService
// @Tags services
// @Param service_id path string true "service_id"
// @Success 200 {object} dto.Service "Service and environment bindings."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/services/{service_id} [get]
func (h *HTTP) GetService(ctx context.Context, req dto.GetServiceRequestObject) (dto.GetServiceResponseObject, error) {
	service, err := h.svc.Get(ctx, req.ServiceId)
	if err != nil {
		return nil, err
	}
	return dto.GetService200JSONResponse(apiService(service)), nil
}

// @Summary Update a registered service
// @ID updateService
// @Tags services
// @Param service_id path string true "service_id"
// @Param body body dto.ServiceUpdate true "Request body"
// @Success 200 {object} dto.Service "Updated service."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/services/{service_id} [patch]
func (h *HTTP) UpdateService(ctx context.Context, req dto.UpdateServiceRequestObject) (dto.UpdateServiceResponseObject, error) {
	if req.Body == nil {
		return nil, problem.BadRequest("A JSON body is required.")
	}
	in := serviceregdto.UpdateInput{Name: req.Body.Name, Owner: req.Body.Owner, Description: req.Body.Description}
	if req.Body.Protocol != nil {
		value := string(*req.Body.Protocol)
		in.Protocol = &value
	}
	service, err := h.svc.Update(ctx, req.ServiceId, in)
	if err != nil {
		return nil, err
	}
	return dto.UpdateService200JSONResponse(apiService(service)), nil
}

// @Summary Delete a service and its imported API specs
// @ID deleteService
// @Tags services
// @Param service_id path string true "service_id"
// @Success 204 "Deleted."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/services/{service_id} [delete]
func (h *HTTP) DeleteService(ctx context.Context, req dto.DeleteServiceRequestObject) (dto.DeleteServiceResponseObject, error) {
	if err := h.svc.Delete(ctx, req.ServiceId); err != nil {
		return nil, err
	}
	return dto.DeleteService204Response{}, nil
}

// @Summary Set a service base URL for an environment
// @ID putServiceEnvironment
// @Tags services
// @Param service_id path string true "service_id"
// @Param environment_id path string true "environment_id"
// @Param body body dto.ServiceEnvironmentInput true "Request body"
// @Success 200 {object} dto.ServiceEnvironment "Updated environment binding."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/services/{service_id}/environments/{environment_id} [put]
func (h *HTTP) PutServiceEnvironment(ctx context.Context, req dto.PutServiceEnvironmentRequestObject) (dto.PutServiceEnvironmentResponseObject, error) {
	if req.Body == nil {
		return nil, problem.BadRequest("A JSON body is required.")
	}
	var connectorID *uuid.UUID
	if req.Body.ConnectorInstanceId != nil {
		id := *req.Body.ConnectorInstanceId
		connectorID = &id
	}
	env, err := h.svc.PutEnvironment(ctx, req.ServiceId, req.EnvironmentId, req.Body.BaseUrl, connectorID)
	if err != nil {
		return nil, err
	}
	return dto.PutServiceEnvironment200JSONResponse(apiEnvironment(env)), nil
}

// @Summary Remove a service environment binding
// @ID deleteServiceEnvironment
// @Tags services
// @Param service_id path string true "service_id"
// @Param environment_id path string true "environment_id"
// @Success 204 "Deleted."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/services/{service_id}/environments/{environment_id} [delete]
func (h *HTTP) DeleteServiceEnvironment(ctx context.Context, req dto.DeleteServiceEnvironmentRequestObject) (dto.DeleteServiceEnvironmentResponseObject, error) {
	if err := h.svc.DeleteEnvironment(ctx, req.ServiceId, req.EnvironmentId); err != nil {
		return nil, err
	}
	return dto.DeleteServiceEnvironment204Response{}, nil
}

// @Summary Import an OpenAPI document and preview changes
// @ID importOpenAPI
// @Tags services
// @Param service_id path string true "service_id"
// @Param dry_run query bool false "dry_run"
// @Param body body dto.OpenAPIImportRequest true "Request body"
// @Success 200 {object} dto.OpenAPIImportResult "Import diff (and applied version when dry_run is false)."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/services/{service_id}/import-openapi [post]
func (h *HTTP) ImportOpenAPI(ctx context.Context, req dto.ImportOpenAPIRequestObject) (dto.ImportOpenAPIResponseObject, error) {
	if req.Body == nil {
		return nil, problem.BadRequest("A JSON body is required.")
	}
	dryRun := true
	if req.Params.DryRun != nil {
		dryRun = *req.Params.DryRun
	}
	in := serviceregdto.ImportInput{Spec: req.Body.Spec, URL: req.Body.Url, DryRun: dryRun}
	if req.Body.VersionLabel != nil {
		in.VersionLabel = *req.Body.VersionLabel
	}
	result, err := h.svc.ImportOpenAPI(ctx, req.ServiceId, in)
	if err != nil {
		return nil, err
	}
	var specID *uuid.UUID
	if result.SpecID != nil {
		id := *result.SpecID
		specID = &id
	}
	return dto.ImportOpenAPI200JSONResponse(dto.OpenAPIImportResult{DryRun: result.DryRun,
		SpecId: specID, Changes: apiChanges(result.Changes), EndpointCount: result.EndpointCount}), nil
}

// @Summary List imported API endpoints
// @ID listServiceEndpoints
// @Tags services
// @Param service_id path string true "service_id"
// @Success 200 {object} dto.ListServiceEndpoints200JSONResponse "Imported operation catalog."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/services/{service_id}/endpoints [get]
func (h *HTTP) ListServiceEndpoints(ctx context.Context, req dto.ListServiceEndpointsRequestObject) (dto.ListServiceEndpointsResponseObject, error) {
	endpoints, err := h.svc.ListEndpoints(ctx, req.ServiceId)
	if err != nil {
		return nil, err
	}
	out := dto.ListServiceEndpoints200JSONResponse{Data: make([]dto.ServiceEndpoint, len(endpoints))}
	for i := range endpoints {
		out.Data[i] = apiEndpoint(endpoints[i])
	}
	return out, nil
}

// @Summary Send a test request to an imported endpoint
// @ID tryServiceEndpoint
// @Tags services
// @Param endpoint_id path string true "endpoint_id"
// @Param body body dto.ServiceTryRequest true "Request body"
// @Success 200 {object} dto.ServiceTryResult "Upstream response (bounded to 1 MiB)."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/service-endpoints/{endpoint_id}/try [post]
func (h *HTTP) TryServiceEndpoint(ctx context.Context, req dto.TryServiceEndpointRequestObject) (dto.TryServiceEndpointResponseObject, error) {
	if req.Body == nil {
		return nil, problem.BadRequest("A JSON body is required.")
	}
	in := serviceregdto.ServiceTryInput{EnvironmentID: req.Body.EnvironmentId, Body: req.Body.Body}
	if req.Body.Headers != nil {
		in.Headers = *req.Body.Headers
	}
	if req.Body.PathValues != nil {
		in.PathValues = *req.Body.PathValues
	}
	if req.Body.Query != nil {
		in.Query = *req.Body.Query
	}
	result, err := h.svc.TryEndpoint(ctx, req.EndpointId, in)
	if err != nil {
		return nil, err
	}
	return dto.TryServiceEndpoint200JSONResponse(dto.ServiceTryResult{
		Status: result.Status, Headers: result.Headers, Body: result.Body, DurationMs: result.DurationMs,
	}), nil
}

// apiChanges maps an OpenAPI diff to its wire representation.
func apiChanges(changes []model.OpenAPIChange) []dto.OpenAPIChange {
	out := make([]dto.OpenAPIChange, len(changes))
	for i, change := range changes {
		kind := dto.OpenAPIChangeKind(change.Kind)
		breaking := change.Breaking
		out[i] = dto.OpenAPIChange{OperationId: change.OperationID, Method: change.Method,
			Path: change.Path, Kind: kind, Breaking: &breaking}
	}
	return out
}
