package connector

import (
	"context"
	"encoding/json"

	"github.com/nuriansyah/hugimethara/backend/internal/api/apigen"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/httpx"
)

type HTTP struct{ svc *Service }

func NewHTTP(svc *Service) *HTTP { return &HTTP{svc: svc} }

func (h *HTTP) ListConnectorDefinitions(ctx context.Context, _ apigen.ListConnectorDefinitionsRequestObject) (apigen.ListConnectorDefinitionsResponseObject, error) {
	rows, err := h.svc.ListDefinitions(ctx)
	if err != nil {
		return nil, err
	}
	out := apigen.ListConnectorDefinitions200JSONResponse{Data: make([]apigen.ConnectorDefinition, len(rows))}
	for i, row := range rows {
		out.Data[i] = apiDefinition(row)
	}
	return out, nil
}

func (h *HTTP) ListConnectorVersions(ctx context.Context, req apigen.ListConnectorVersionsRequestObject) (apigen.ListConnectorVersionsResponseObject, error) {
	rows, err := h.svc.ListVersions(ctx, req.DefinitionId)
	if err != nil {
		return nil, err
	}
	out := apigen.ListConnectorVersions200JSONResponse{Data: make([]apigen.ConnectorVersion, len(rows))}
	for i, row := range rows {
		out.Data[i] = apiVersion(row)
	}
	return out, nil
}

func (h *HTTP) ListConnectors(ctx context.Context, _ apigen.ListConnectorsRequestObject) (apigen.ListConnectorsResponseObject, error) {
	rows, err := h.svc.List(ctx)
	if err != nil {
		return nil, err
	}
	out := apigen.ListConnectors200JSONResponse{Data: make([]apigen.ConnectorInstance, len(rows))}
	for i, row := range rows {
		out.Data[i] = apiInstance(row)
	}
	return out, nil
}

func (h *HTTP) CreateConnector(ctx context.Context, req apigen.CreateConnectorRequestObject) (apigen.CreateConnectorResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.BadRequest("A JSON body is required.")
	}
	body := req.Body
	authMethod := ""
	if body.AuthMethod != nil {
		authMethod = *body.AuthMethod
	}
	rateLimit, timeoutMs := 0, 0
	if body.RateLimit != nil {
		rateLimit = *body.RateLimit
	}
	if body.TimeoutMs != nil {
		timeoutMs = *body.TimeoutMs
	}
	row, err := h.svc.Create(ctx, CreateInput{
		WorkspaceID: body.WorkspaceId, EnvironmentID: body.EnvironmentId, RefKey: body.RefKey,
		ConnectorVersionID: body.ConnectorVersionId, Config: body.Config, AuthMethod: authMethod,
		SecretID: body.SecretId, RateLimit: rateLimit, TimeoutMs: timeoutMs,
	})
	if err != nil {
		return nil, err
	}
	return apigen.CreateConnector201JSONResponse(apiInstance(row)), nil
}

func (h *HTTP) GetConnector(ctx context.Context, req apigen.GetConnectorRequestObject) (apigen.GetConnectorResponseObject, error) {
	row, err := h.svc.Get(ctx, req.ConnectorId)
	if err != nil {
		return nil, err
	}
	return apigen.GetConnector200JSONResponse(apiInstance(row)), nil
}

func (h *HTTP) UpdateConnector(ctx context.Context, req apigen.UpdateConnectorRequestObject) (apigen.UpdateConnectorResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.BadRequest("A JSON body is required.")
	}
	body := req.Body
	var status *string
	if body.Status != nil {
		value := string(*body.Status)
		status = &value
	}
	row, err := h.svc.Update(ctx, req.ConnectorId, UpdateInput{
		Config: body.Config, AuthMethod: body.AuthMethod, SecretID: body.SecretId,
		RateLimit: body.RateLimit, TimeoutMs: body.TimeoutMs, Status: status,
	})
	if err != nil {
		return nil, err
	}
	return apigen.UpdateConnector200JSONResponse(apiInstance(row)), nil
}

func (h *HTTP) DeleteConnector(ctx context.Context, req apigen.DeleteConnectorRequestObject) (apigen.DeleteConnectorResponseObject, error) {
	if err := h.svc.Delete(ctx, req.ConnectorId); err != nil {
		return nil, err
	}
	return apigen.DeleteConnector204Response{}, nil
}

func (h *HTTP) TestConnector(ctx context.Context, req apigen.TestConnectorRequestObject) (apigen.TestConnectorResponseObject, error) {
	result, err := h.svc.Test(ctx, req.ConnectorId)
	if err != nil {
		return nil, err
	}
	return apigen.TestConnector200JSONResponse(apiTestResult(result)), nil
}

func (h *HTTP) GetConnectorHealth(ctx context.Context, req apigen.GetConnectorHealthRequestObject) (apigen.GetConnectorHealthResponseObject, error) {
	rows, err := h.svc.Health(ctx, req.ConnectorId)
	if err != nil {
		return nil, err
	}
	out := apigen.GetConnectorHealth200JSONResponse{Data: make([]apigen.ConnectorTestResult, len(rows))}
	for i, row := range rows {
		out.Data[i] = apigen.ConnectorTestResult{CheckedAt: row.CheckedAt, Ok: row.OK,
			LatencyMs: row.LatencyMs, Message: row.Message, ErrorCode: optionalString(row.ErrorCode)}
	}
	return out, nil
}

func apiDefinition(row Definition) apigen.ConnectorDefinition {
	return apigen.ConnectorDefinition{Id: row.ID, Key: row.Key, Name: row.Name,
		Category: row.Category, Tier: apigen.ConnectorDefinitionTier(row.Tier), Icon: row.Icon}
}

func apiVersion(row Version) apigen.ConnectorVersion {
	var schema map[string]interface{}
	_ = json.Unmarshal(row.ConfigSchema, &schema)
	var methods []string
	_ = json.Unmarshal(row.AuthMethods, &methods)
	var operations []map[string]interface{}
	_ = json.Unmarshal(row.Operations, &operations)
	return apigen.ConnectorVersion{Id: row.ID, DefinitionId: row.DefinitionID, Version: row.Version,
		ConfigSchema: schema, AuthMethods: methods, Operations: operations, DeprecatedAt: row.DeprecatedAt}
}

func apiInstance(row Instance) apigen.ConnectorInstance {
	var config map[string]interface{}
	_ = json.Unmarshal(row.Config, &config)
	if config == nil {
		config = map[string]interface{}{}
	}
	result := apigen.ConnectorInstance{
		Id: row.ID, WorkspaceId: row.WorkspaceID, EnvironmentId: row.EnvironmentID,
		RefKey: row.RefKey, Name: row.RefKey, DefinitionId: row.DefinitionID, DefinitionKey: row.DefinitionKey,
		Version: row.Version, Config: config, AuthMethod: row.AuthMethod, RateLimit: row.RateLimit,
		TimeoutMs: row.TimeoutMs, Status: apigen.ConnectorInstanceStatus(row.Status),
		CredentialConfigured: row.CredentialConfigured, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	if row.SecretID != nil {
		id := *row.SecretID
		result.SecretId = &id
	}
	return result
}

func apiTestResult(result TestResult) apigen.ConnectorTestResult {
	return apigen.ConnectorTestResult{CheckedAt: result.CheckedAt, Ok: result.OK, LatencyMs: result.LatencyMs,
		ErrorCode: optionalString(result.ErrorCode), Message: result.Message}
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
