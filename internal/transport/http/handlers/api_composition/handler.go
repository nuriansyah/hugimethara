package apicomposition

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/nuriansyah/hugimethara/internal/modules/api_composition/model"
	compositiondto "github.com/nuriansyah/hugimethara/internal/modules/api_composition/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/api_composition/services"
	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	logging "github.com/nuriansyah/hugimethara/internal/platform/logger"
	"github.com/nuriansyah/hugimethara/internal/transport/http/dto"
)

type HTTP struct{ svc services.Service }

func NewHTTP(svc services.Service) *HTTP { return &HTTP{svc: svc} }

func apiComposition(row model.Composition) dto.ApiComposition {
	return dto.ApiComposition{Id: row.ID, WorkspaceId: row.WorkspaceID, Name: row.Name,
		Slug: row.Slug, Draft: row.Draft, Exposed: row.Exposed,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

func apiCompositionVersion(row model.Version) dto.ApiCompositionVersion {
	return dto.ApiCompositionVersion{Id: row.ID, CompositionId: row.CompositionID,
		Version: row.Version, Definition: row.Definition, Checksum: row.Checksum,
		PublishedBy: row.PublishedBy, PublishedAt: row.PublishedAt}
}

func apiCompositionExecution(row model.Execution) (dto.ApiCompositionExecution, error) {
	var summary map[string]any
	if err := json.Unmarshal(row.StepSummary, &summary); err != nil {
		return dto.ApiCompositionExecution{}, fmt.Errorf("composition: decode execution step summary: %w", err)
	}
	if summary == nil {
		return dto.ApiCompositionExecution{}, fmt.Errorf("composition: execution step summary is not a JSON object")
	}
	return dto.ApiCompositionExecution{Id: row.ID, CompositionId: row.CompositionID,
		VersionId: row.VersionID, EnvironmentId: row.EnvironmentID, StartedAt: row.StartedAt,
		DurationMs: row.DurationMs, Status: row.Status, StepSummary: summary, Output: row.Output,
		Error: row.Error, RequestId: row.RequestID}, nil
}

// @Summary List API compositions
// @ID listApiCompositions
// @Tags api-compositions
// @Param workspace_id query string true "Workspace ID"
// @Success 200 {object} dto.ApiCompositions
// @Failure 401 {object} dto.Problem
// @Failure 403 {object} dto.Problem
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/api-compositions [get]
func (h *HTTP) ListApiCompositions(ctx context.Context, req dto.ListApiCompositionsRequestObject) (dto.ListApiCompositionsResponseObject, error) {
	rows, err := h.svc.List(ctx, req.Params.WorkspaceId)
	if err != nil {
		return nil, err
	}
	out := dto.ApiCompositions{Data: make([]dto.ApiComposition, len(rows))}
	for i := range rows {
		out.Data[i] = apiComposition(rows[i])
	}
	return dto.ListApiCompositions200JSONResponse(out), nil
}

// @Summary Create an API composition draft
// @ID createApiComposition
// @Tags api-compositions
// @Param body body dto.ApiCompositionCreate true "Composition draft"
// @Success 201 {object} dto.ApiComposition
// @Failure 401 {object} dto.Problem
// @Failure 403 {object} dto.Problem
// @Failure 422 {object} dto.Problem
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/api-compositions [post]
func (h *HTTP) CreateApiComposition(ctx context.Context, req dto.CreateApiCompositionRequestObject) (dto.CreateApiCompositionResponseObject, error) {
	if req.Body == nil {
		return nil, problem.BadRequest("A JSON body is required.")
	}
	row, err := h.svc.Create(ctx, compositiondto.CreateInput{WorkspaceID: req.Body.WorkspaceId,
		Name: req.Body.Name, Slug: req.Body.Slug, Draft: req.Body.Draft})
	if err != nil {
		return nil, err
	}
	return dto.CreateApiComposition201JSONResponse(apiComposition(row)), nil
}

// @Summary Get an API composition draft
// @ID getApiComposition
// @Tags api-compositions
// @Param composition_id path string true "Composition ID"
// @Success 200 {object} dto.ApiComposition
// @Failure 401 {object} dto.Problem
// @Failure 403 {object} dto.Problem
// @Failure 404 {object} dto.Problem
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/api-compositions/{composition_id} [get]
func (h *HTTP) GetApiComposition(ctx context.Context, req dto.GetApiCompositionRequestObject) (dto.GetApiCompositionResponseObject, error) {
	row, err := h.svc.Get(ctx, uuid.UUID(req.CompositionId))
	if err != nil {
		return nil, err
	}
	return dto.GetApiComposition200JSONResponse(apiComposition(row)), nil
}

// @Summary Update an API composition draft
// @ID updateApiComposition
// @Tags api-compositions
// @Param composition_id path string true "Composition ID"
// @Param body body dto.ApiCompositionUpdate true "Composition changes"
// @Success 200 {object} dto.ApiComposition
// @Failure 401 {object} dto.Problem
// @Failure 403 {object} dto.Problem
// @Failure 404 {object} dto.Problem
// @Failure 422 {object} dto.Problem
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/api-compositions/{composition_id} [patch]
func (h *HTTP) UpdateApiComposition(ctx context.Context, req dto.UpdateApiCompositionRequestObject) (dto.UpdateApiCompositionResponseObject, error) {
	if req.Body == nil {
		return nil, problem.BadRequest("A JSON body is required.")
	}
	var draft *map[string]any
	if req.Body.Draft != nil {
		value := *req.Body.Draft
		draft = &value
	}
	row, err := h.svc.Update(ctx, uuid.UUID(req.CompositionId), compositiondto.UpdateInput{Name: req.Body.Name,
		Slug: req.Body.Slug, Draft: draft, Exposed: req.Body.Exposed})
	if err != nil {
		return nil, err
	}
	return dto.UpdateApiComposition200JSONResponse(apiComposition(row)), nil
}

// @Summary Delete an API composition and its history
// @ID deleteApiComposition
// @Tags api-compositions
// @Param composition_id path string true "Composition ID"
// @Success 204 "Deleted"
// @Failure 401 {object} dto.Problem
// @Failure 403 {object} dto.Problem
// @Failure 404 {object} dto.Problem
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/api-compositions/{composition_id} [delete]
func (h *HTTP) DeleteApiComposition(ctx context.Context, req dto.DeleteApiCompositionRequestObject) (dto.DeleteApiCompositionResponseObject, error) {
	if err := h.svc.Delete(ctx, uuid.UUID(req.CompositionId)); err != nil {
		return nil, err
	}
	return dto.DeleteApiComposition204Response{}, nil
}

// @Summary Validate a composition draft and its references
// @ID validateApiComposition
// @Tags api-compositions
// @Param composition_id path string true "Composition ID"
// @Success 200 {object} dto.ApiCompositionValidation
// @Failure 401 {object} dto.Problem
// @Failure 403 {object} dto.Problem
// @Failure 404 {object} dto.Problem
// @Failure 422 {object} dto.Problem
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/api-compositions/{composition_id}/validate [post]
func (h *HTTP) ValidateApiComposition(ctx context.Context, req dto.ValidateApiCompositionRequestObject) (dto.ValidateApiCompositionResponseObject, error) {
	if err := h.svc.Validate(ctx, uuid.UUID(req.CompositionId)); err != nil {
		return nil, err
	}
	return dto.ValidateApiComposition200JSONResponse{Valid: true}, nil
}

// @Summary Publish an immutable API composition version
// @ID publishApiComposition
// @Tags api-compositions
// @Param composition_id path string true "Composition ID"
// @Success 201 {object} dto.ApiCompositionVersion
// @Failure 401 {object} dto.Problem
// @Failure 403 {object} dto.Problem
// @Failure 404 {object} dto.Problem
// @Failure 422 {object} dto.Problem
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/api-compositions/{composition_id}/publish [post]
func (h *HTTP) PublishApiComposition(ctx context.Context, req dto.PublishApiCompositionRequestObject) (dto.PublishApiCompositionResponseObject, error) {
	row, err := h.svc.Publish(ctx, uuid.UUID(req.CompositionId))
	if err != nil {
		return nil, err
	}
	return dto.PublishApiComposition201JSONResponse(apiCompositionVersion(row)), nil
}

// @Summary List immutable API composition versions
// @ID listApiCompositionVersions
// @Tags api-compositions
// @Param composition_id path string true "Composition ID"
// @Success 200 {object} dto.ApiCompositionVersions
// @Failure 401 {object} dto.Problem
// @Failure 403 {object} dto.Problem
// @Failure 404 {object} dto.Problem
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/api-compositions/{composition_id}/versions [get]
func (h *HTTP) ListApiCompositionVersions(ctx context.Context, req dto.ListApiCompositionVersionsRequestObject) (dto.ListApiCompositionVersionsResponseObject, error) {
	rows, err := h.svc.ListVersions(ctx, uuid.UUID(req.CompositionId))
	if err != nil {
		return nil, err
	}
	out := dto.ApiCompositionVersions{Data: make([]dto.ApiCompositionVersion, len(rows))}
	for i := range rows {
		out.Data[i] = apiCompositionVersion(rows[i])
	}
	return dto.ListApiCompositionVersions200JSONResponse(out), nil
}

// @Summary Execute the latest published API composition version
// @ID executeApiComposition
// @Tags api-compositions
// @Param composition_id path string true "Composition ID"
// @Param body body dto.ApiCompositionExecute true "Execution input"
// @Success 200 {object} dto.ApiCompositionExecution
// @Failure 401 {object} dto.Problem
// @Failure 403 {object} dto.Problem
// @Failure 404 {object} dto.Problem
// @Failure 422 {object} dto.Problem
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/api-compositions/{composition_id}/execute [post]
func (h *HTTP) ExecuteApiComposition(ctx context.Context, req dto.ExecuteApiCompositionRequestObject) (dto.ExecuteApiCompositionResponseObject, error) {
	if req.Body == nil {
		return nil, problem.BadRequest("A JSON body is required.")
	}
	result, err := h.svc.Execute(ctx, uuid.UUID(req.CompositionId), compositiondto.ExecuteInput{
		EnvironmentID: req.Body.EnvironmentId, Request: req.Body.Request,
		RequestID: logging.RequestID(ctx),
	})
	if err != nil {
		return nil, err
	}
	response, err := apiCompositionExecution(result)
	if err != nil {
		return nil, err
	}
	return dto.ExecuteApiComposition200JSONResponse(response), nil
}

// @Summary List recent API composition executions
// @ID listApiCompositionExecutions
// @Tags api-composition-executions
// @Param workspace_id query string true "Workspace ID"
// @Param composition_id query string false "Composition ID"
// @Success 200 {object} dto.ApiCompositionExecutions
// @Failure 401 {object} dto.Problem
// @Failure 403 {object} dto.Problem
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/api-composition-executions [get]
func (h *HTTP) ListApiCompositionExecutions(ctx context.Context, req dto.ListApiCompositionExecutionsRequestObject) (dto.ListApiCompositionExecutionsResponseObject, error) {
	rows, err := h.svc.ListExecutions(ctx, req.Params.WorkspaceId, req.Params.CompositionId)
	if err != nil {
		return nil, err
	}
	out := dto.ApiCompositionExecutions{Data: make([]dto.ApiCompositionExecution, len(rows))}
	for i := range rows {
		out.Data[i], err = apiCompositionExecution(rows[i])
		if err != nil {
			return nil, err
		}
	}
	return dto.ListApiCompositionExecutions200JSONResponse(out), nil
}

// @Summary Get an API composition execution with step results
// @ID getApiCompositionExecution
// @Tags api-composition-executions
// @Param execution_id path string true "Execution ID"
// @Success 200 {object} dto.ApiCompositionExecution
// @Failure 401 {object} dto.Problem
// @Failure 403 {object} dto.Problem
// @Failure 404 {object} dto.Problem
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/api-composition-executions/{execution_id} [get]
func (h *HTTP) GetApiCompositionExecution(ctx context.Context, req dto.GetApiCompositionExecutionRequestObject) (dto.GetApiCompositionExecutionResponseObject, error) {
	row, err := h.svc.GetExecution(ctx, uuid.UUID(req.ExecutionId))
	if err != nil {
		return nil, err
	}
	response, err := apiCompositionExecution(row)
	if err != nil {
		return nil, err
	}
	return dto.GetApiCompositionExecution200JSONResponse(response), nil
}
