package servicereg

import (
	"context"

	"github.com/google/uuid"

	"github.com/nuriansyah/hugimethara/backend/internal/api/apigen"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/httpx"
)

type HTTP struct{ svc *Service }

func NewHTTP(svc *Service) *HTTP { return &HTTP{svc: svc} }

func apiService(s RegisteredService) apigen.Service {
	envs := make([]apigen.ServiceEnvironment, len(s.Environments))
	for i, env := range s.Environments {
		var connectorID *uuid.UUID
		if env.ConnectorInstanceID != nil {
			id := *env.ConnectorInstanceID
			connectorID = &id
		}
		envs[i] = apigen.ServiceEnvironment{EnvironmentId: env.EnvironmentID, BaseUrl: env.BaseURL,
			ConnectorInstanceId: connectorID}
	}
	return apigen.Service{Id: s.ID, WorkspaceId: s.WorkspaceID, Name: s.Name, Owner: s.Owner,
		Description: s.Description, Protocol: apigen.ServiceProtocol(s.Protocol), EndpointCount: s.EndpointCount,
		Environments: envs, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt}
}

func apiEnvironment(env Environment) apigen.ServiceEnvironment {
	var connectorID *uuid.UUID
	if env.ConnectorInstanceID != nil {
		id := *env.ConnectorInstanceID
		connectorID = &id
	}
	return apigen.ServiceEnvironment{EnvironmentId: env.EnvironmentID, BaseUrl: env.BaseURL,
		ConnectorInstanceId: connectorID}
}

func apiEndpoint(endpoint Endpoint) apigen.ServiceEndpoint {
	var requestSchema *map[string]interface{}
	if endpoint.RequestSchema != nil {
		schema := make(map[string]interface{}, len(endpoint.RequestSchema))
		for key, value := range endpoint.RequestSchema {
			schema[key] = value
		}
		requestSchema = &schema
	}
	return apigen.ServiceEndpoint{Id: endpoint.ID, ServiceId: endpoint.ServiceID,
		OperationId: endpoint.OperationID, Method: endpoint.Method, Path: endpoint.Path,
		Summary: endpoint.Summary, Parameters: endpoint.Parameters, RequestSchema: requestSchema,
		ResponseSchemas: endpoint.ResponseSchemas, Security: endpoint.Security, Deprecated: endpoint.Deprecated}
}

func (h *HTTP) ListServices(ctx context.Context, req apigen.ListServicesRequestObject) (apigen.ListServicesResponseObject, error) {
	workspaceID := req.Params.WorkspaceId
	if workspaceID == uuid.Nil {
		return nil, httpx.BadRequest("workspace_id is required.")
	}
	services, err := h.svc.List(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := apigen.ListServices200JSONResponse{Data: make([]apigen.Service, len(services))}
	for i := range services {
		out.Data[i] = apiService(services[i])
	}
	return out, nil
}

func (h *HTTP) CreateService(ctx context.Context, req apigen.CreateServiceRequestObject) (apigen.CreateServiceResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.BadRequest("A JSON body is required.")
	}
	in := CreateInput{WorkspaceID: req.Body.WorkspaceId, Name: req.Body.Name}
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
	return apigen.CreateService201JSONResponse(apiService(service)), nil
}

func (h *HTTP) GetService(ctx context.Context, req apigen.GetServiceRequestObject) (apigen.GetServiceResponseObject, error) {
	service, err := h.svc.Get(ctx, req.ServiceId)
	if err != nil {
		return nil, err
	}
	return apigen.GetService200JSONResponse(apiService(service)), nil
}

func (h *HTTP) UpdateService(ctx context.Context, req apigen.UpdateServiceRequestObject) (apigen.UpdateServiceResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.BadRequest("A JSON body is required.")
	}
	in := UpdateInput{Name: req.Body.Name, Owner: req.Body.Owner, Description: req.Body.Description}
	if req.Body.Protocol != nil {
		value := string(*req.Body.Protocol)
		in.Protocol = &value
	}
	service, err := h.svc.Update(ctx, req.ServiceId, in)
	if err != nil {
		return nil, err
	}
	return apigen.UpdateService200JSONResponse(apiService(service)), nil
}

func (h *HTTP) DeleteService(ctx context.Context, req apigen.DeleteServiceRequestObject) (apigen.DeleteServiceResponseObject, error) {
	if err := h.svc.Delete(ctx, req.ServiceId); err != nil {
		return nil, err
	}
	return apigen.DeleteService204Response{}, nil
}

func (h *HTTP) PutServiceEnvironment(ctx context.Context, req apigen.PutServiceEnvironmentRequestObject) (apigen.PutServiceEnvironmentResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.BadRequest("A JSON body is required.")
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
	return apigen.PutServiceEnvironment200JSONResponse(apiEnvironment(env)), nil
}

func (h *HTTP) DeleteServiceEnvironment(ctx context.Context, req apigen.DeleteServiceEnvironmentRequestObject) (apigen.DeleteServiceEnvironmentResponseObject, error) {
	if err := h.svc.DeleteEnvironment(ctx, req.ServiceId, req.EnvironmentId); err != nil {
		return nil, err
	}
	return apigen.DeleteServiceEnvironment204Response{}, nil
}

func (h *HTTP) ImportOpenAPI(ctx context.Context, req apigen.ImportOpenAPIRequestObject) (apigen.ImportOpenAPIResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.BadRequest("A JSON body is required.")
	}
	dryRun := true
	if req.Params.DryRun != nil {
		dryRun = *req.Params.DryRun
	}
	in := ImportInput{Spec: req.Body.Spec, URL: req.Body.Url, DryRun: dryRun}
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
	return apigen.ImportOpenAPI200JSONResponse(apigen.OpenAPIImportResult{DryRun: result.DryRun,
		SpecId: specID, Changes: apiChanges(result.Changes), EndpointCount: result.EndpointCount}), nil
}

func (h *HTTP) ListServiceEndpoints(ctx context.Context, req apigen.ListServiceEndpointsRequestObject) (apigen.ListServiceEndpointsResponseObject, error) {
	endpoints, err := h.svc.ListEndpoints(ctx, req.ServiceId)
	if err != nil {
		return nil, err
	}
	out := apigen.ListServiceEndpoints200JSONResponse{Data: make([]apigen.ServiceEndpoint, len(endpoints))}
	for i := range endpoints {
		out.Data[i] = apiEndpoint(endpoints[i])
	}
	return out, nil
}

func (h *HTTP) TryServiceEndpoint(ctx context.Context, req apigen.TryServiceEndpointRequestObject) (apigen.TryServiceEndpointResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.BadRequest("A JSON body is required.")
	}
	in := ServiceTryInput{EnvironmentID: req.Body.EnvironmentId, Body: req.Body.Body}
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
	return apigen.TryServiceEndpoint200JSONResponse(apigen.ServiceTryResult{
		Status: result.Status, Headers: result.Headers, Body: result.Body, DurationMs: result.DurationMs,
	}), nil
}
