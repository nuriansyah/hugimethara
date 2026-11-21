package connector

import (
	"context"
	"encoding/json"

	"github.com/nuriansyah/hugimethara/internal/modules/integration/connector/model"
	connectordto "github.com/nuriansyah/hugimethara/internal/modules/integration/connector/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/integration/connector/services"
	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/transport/http/dto"
)

type HTTP struct{ svc services.Service }

func NewHTTP(svc services.Service) *HTTP { return &HTTP{svc: svc} }

// @Summary List available connector definitions
// @ID listConnectorDefinitions
// @Tags connectors
// @Success 200 {object} dto.ListConnectorDefinitions200JSONResponse "Built-in and organization connector catalog."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/connector-definitions [get]
func (h *HTTP) ListConnectorDefinitions(ctx context.Context, _ dto.ListConnectorDefinitionsRequestObject) (dto.ListConnectorDefinitionsResponseObject, error) {
	rows, err := h.svc.ListDefinitions(ctx)
	if err != nil {
		return nil, err
	}
	out := dto.ListConnectorDefinitions200JSONResponse{Data: make([]dto.ConnectorDefinition, len(rows))}
	for i, row := range rows {
		out.Data[i] = apiDefinition(row)
	}
	return out, nil
}

// @Summary List connector definition versions
// @ID listConnectorVersions
// @Tags connectors
// @Param definition_id path string true "definition_id"
// @Success 200 {object} dto.ListConnectorVersions200JSONResponse "Version metadata and JSON schemas."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/connector-definitions/{definition_id}/versions [get]
func (h *HTTP) ListConnectorVersions(ctx context.Context, req dto.ListConnectorVersionsRequestObject) (dto.ListConnectorVersionsResponseObject, error) {
	rows, err := h.svc.ListVersions(ctx, req.DefinitionId)
	if err != nil {
		return nil, err
	}
	out := dto.ListConnectorVersions200JSONResponse{Data: make([]dto.ConnectorVersion, len(rows))}
	for i, row := range rows {
		out.Data[i] = apiVersion(row)
	}
	return out, nil
}

// @Summary List configured connector instances
// @ID listConnectors
// @Tags connectors
// @Success 200 {object} dto.ListConnectors200JSONResponse "Connector instances."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/connectors [get]
func (h *HTTP) ListConnectors(ctx context.Context, _ dto.ListConnectorsRequestObject) (dto.ListConnectorsResponseObject, error) {
	rows, err := h.svc.List(ctx)
	if err != nil {
		return nil, err
	}
	out := dto.ListConnectors200JSONResponse{Data: make([]dto.ConnectorInstance, len(rows))}
	for i, row := range rows {
		out.Data[i] = apiInstance(row)
	}
	return out, nil
}

// @Summary Configure a connector instance
// @ID createConnector
// @Tags connectors
// @Param body body dto.ConnectorInstanceCreate true "Request body"
// @Success 201 {object} dto.ConnectorInstance "Configured connector."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/connectors [post]
func (h *HTTP) CreateConnector(ctx context.Context, req dto.CreateConnectorRequestObject) (dto.CreateConnectorResponseObject, error) {
	if req.Body == nil {
		return nil, problem.BadRequest("A JSON body is required.")
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
	row, err := h.svc.Create(ctx, connectordto.CreateInput{
		WorkspaceID: body.WorkspaceId, EnvironmentID: body.EnvironmentId, RefKey: body.RefKey,
		ConnectorVersionID: body.ConnectorVersionId, Config: body.Config, AuthMethod: authMethod,
		SecretID: body.SecretId, RateLimit: rateLimit, TimeoutMs: timeoutMs,
	})
	if err != nil {
		return nil, err
	}
	return dto.CreateConnector201JSONResponse(apiInstance(row)), nil
}

// @Summary Get a connector instance
// @ID getConnector
// @Tags connectors
// @Param connector_id path string true "connector_id"
// @Success 200 {object} dto.ConnectorInstance "Connector instance."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/connectors/{connector_id} [get]
func (h *HTTP) GetConnector(ctx context.Context, req dto.GetConnectorRequestObject) (dto.GetConnectorResponseObject, error) {
	row, err := h.svc.Get(ctx, req.ConnectorId)
	if err != nil {
		return nil, err
	}
	return dto.GetConnector200JSONResponse(apiInstance(row)), nil
}

// @Summary Update a connector instance
// @ID updateConnector
// @Tags connectors
// @Param connector_id path string true "connector_id"
// @Param body body dto.ConnectorInstanceUpdate true "Request body"
// @Success 200 {object} dto.ConnectorInstance "Updated connector."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/connectors/{connector_id} [patch]
func (h *HTTP) UpdateConnector(ctx context.Context, req dto.UpdateConnectorRequestObject) (dto.UpdateConnectorResponseObject, error) {
	if req.Body == nil {
		return nil, problem.BadRequest("A JSON body is required.")
	}
	body := req.Body
	var status *string
	if body.Status != nil {
		value := string(*body.Status)
		status = &value
	}
	row, err := h.svc.Update(ctx, req.ConnectorId, connectordto.UpdateInput{
		Config: body.Config, AuthMethod: body.AuthMethod, SecretID: body.SecretId,
		RateLimit: body.RateLimit, TimeoutMs: body.TimeoutMs, Status: status,
	})
	if err != nil {
		return nil, err
	}
	return dto.UpdateConnector200JSONResponse(apiInstance(row)), nil
}

// @Summary Delete a connector instance
// @ID deleteConnector
// @Tags connectors
// @Param connector_id path string true "connector_id"
// @Success 204 "Deleted."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/connectors/{connector_id} [delete]
func (h *HTTP) DeleteConnector(ctx context.Context, req dto.DeleteConnectorRequestObject) (dto.DeleteConnectorResponseObject, error) {
	if err := h.svc.Delete(ctx, req.ConnectorId); err != nil {
		return nil, err
	}
	return dto.DeleteConnector204Response{}, nil
}

// @Summary Test connector reachability
// @ID testConnector
// @Tags connectors
// @Param connector_id path string true "connector_id"
// @Success 200 {object} dto.ConnectorTestResult "Test result."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/connectors/{connector_id}/test [post]
func (h *HTTP) TestConnector(ctx context.Context, req dto.TestConnectorRequestObject) (dto.TestConnectorResponseObject, error) {
	result, err := h.svc.Test(ctx, req.ConnectorId)
	if err != nil {
		return nil, err
	}
	return dto.TestConnector200JSONResponse(apiTestResult(result)), nil
}

// @Summary Get connector health history
// @ID getConnectorHealth
// @Tags connectors
// @Param connector_id path string true "connector_id"
// @Success 200 {object} dto.GetConnectorHealth200JSONResponse "Recent test results."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/connectors/{connector_id}/health [get]
func (h *HTTP) GetConnectorHealth(ctx context.Context, req dto.GetConnectorHealthRequestObject) (dto.GetConnectorHealthResponseObject, error) {
	rows, err := h.svc.Health(ctx, req.ConnectorId)
	if err != nil {
		return nil, err
	}
	out := dto.GetConnectorHealth200JSONResponse{Data: make([]dto.ConnectorTestResult, len(rows))}
	for i, row := range rows {
		out.Data[i] = dto.ConnectorTestResult{CheckedAt: row.CheckedAt, Ok: row.OK,
			LatencyMs: row.LatencyMs, Message: row.Message, ErrorCode: optionalString(row.ErrorCode)}
	}
	return out, nil
}

func apiDefinition(row model.Definition) dto.ConnectorDefinition {
	return dto.ConnectorDefinition{Id: row.ID, Key: row.Key, Name: row.Name,
		Category: row.Category, Tier: dto.ConnectorDefinitionTier(row.Tier), Icon: row.Icon}
}

func apiVersion(row model.Version) dto.ConnectorVersion {
	var schema map[string]interface{}
	_ = json.Unmarshal(row.ConfigSchema, &schema)
	var methods []string
	_ = json.Unmarshal(row.AuthMethods, &methods)
	var operations []map[string]interface{}
	_ = json.Unmarshal(row.Operations, &operations)
	return dto.ConnectorVersion{Id: row.ID, DefinitionId: row.DefinitionID, Version: row.Version,
		ConfigSchema: schema, AuthMethods: methods, Operations: operations, DeprecatedAt: row.DeprecatedAt}
}

func apiInstance(row model.Instance) dto.ConnectorInstance {
	var config map[string]interface{}
	_ = json.Unmarshal(row.Config, &config)
	if config == nil {
		config = map[string]interface{}{}
	}
	result := dto.ConnectorInstance{
		Id: row.ID, WorkspaceId: row.WorkspaceID, EnvironmentId: row.EnvironmentID,
		RefKey: row.RefKey, Name: row.RefKey, DefinitionId: row.DefinitionID, DefinitionKey: row.DefinitionKey,
		Version: row.Version, Config: config, AuthMethod: row.AuthMethod, RateLimit: row.RateLimit,
		TimeoutMs: row.TimeoutMs, Status: dto.ConnectorInstanceStatus(row.Status),
		CredentialConfigured: row.CredentialConfigured, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	if row.SecretID != nil {
		id := *row.SecretID
		result.SecretId = &id
	}
	return result
}

func apiTestResult(result model.TestResult) dto.ConnectorTestResult {
	return dto.ConnectorTestResult{CheckedAt: result.CheckedAt, Ok: result.OK, LatencyMs: result.LatencyMs,
		ErrorCode: optionalString(result.ErrorCode), Message: result.Message}
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
