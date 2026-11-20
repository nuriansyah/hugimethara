package dto

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type ApiComposition struct {
	Id          uuid.UUID              `json:"id" binding:"required"`
	WorkspaceId uuid.UUID              `json:"workspace_id" binding:"required"`
	Name        string                 `json:"name" binding:"required"`
	Slug        string                 `json:"slug" binding:"required"`
	Draft       map[string]interface{} `json:"draft" binding:"required"`
	Exposed     bool                   `json:"exposed" binding:"required"`
	CreatedAt   time.Time              `json:"created_at" binding:"required"`
	UpdatedAt   time.Time              `json:"updated_at" binding:"required"`
}

type ApiCompositionCreate struct {
	WorkspaceId uuid.UUID              `json:"workspace_id" binding:"required"`
	Name        string                 `json:"name" binding:"required"`
	Slug        string                 `json:"slug" binding:"required"`
	Draft       map[string]interface{} `json:"draft,omitempty"`
}

type ApiCompositionUpdate struct {
	Name    *string                 `json:"name,omitempty"`
	Slug    *string                 `json:"slug,omitempty"`
	Draft   *map[string]interface{} `json:"draft,omitempty"`
	Exposed *bool                   `json:"exposed,omitempty"`
}

type ApiCompositionVersion struct {
	Id            uuid.UUID              `json:"id" binding:"required"`
	CompositionId uuid.UUID              `json:"composition_id" binding:"required"`
	Version       int                    `json:"version" binding:"required"`
	Definition    map[string]interface{} `json:"definition" binding:"required"`
	Checksum      string                 `json:"checksum" binding:"required"`
	PublishedBy   uuid.UUID              `json:"published_by" binding:"required"`
	PublishedAt   time.Time              `json:"published_at" binding:"required"`
}

type ApiCompositionExecution struct {
	Id            uuid.UUID              `json:"id" binding:"required"`
	CompositionId uuid.UUID              `json:"composition_id" binding:"required"`
	VersionId     uuid.UUID              `json:"version_id" binding:"required"`
	EnvironmentId uuid.UUID              `json:"environment_id" binding:"required"`
	StartedAt     time.Time              `json:"started_at" binding:"required"`
	DurationMs    int                    `json:"duration_ms" binding:"required"`
	Status        string                 `json:"status" binding:"required"`
	StepSummary   map[string]interface{} `json:"step_summary" binding:"required"`
	Output        interface{}            `json:"output,omitempty"`
	Error         *string                `json:"error,omitempty"`
	RequestId     string                 `json:"request_id" binding:"required"`
}

type ApiCompositionExecute struct {
	EnvironmentId uuid.UUID              `json:"environment_id" binding:"required"`
	Request       map[string]interface{} `json:"request,omitempty"`
}

type ApiCompositionValidation struct {
	Valid bool `json:"valid" binding:"required"`
}

type ApiCompositions struct {
	Data []ApiComposition `json:"data" binding:"required"`
}

type ApiCompositionVersions struct {
	Data []ApiCompositionVersion `json:"data" binding:"required"`
}

type ApiCompositionExecutions struct {
	Data []ApiCompositionExecution `json:"data" binding:"required"`
}

type ListApiCompositionsParams struct {
	WorkspaceId uuid.UUID `form:"workspace_id" json:"workspace_id"`
}

type ListApiCompositionsRequestObject struct {
	Params ListApiCompositionsParams
}

type CreateApiCompositionRequestObject struct{ Body *ApiCompositionCreate }
type GetApiCompositionRequestObject struct{ CompositionId uuid.UUID }
type UpdateApiCompositionRequestObject struct {
	CompositionId uuid.UUID
	Body          *ApiCompositionUpdate
}
type DeleteApiCompositionRequestObject struct{ CompositionId uuid.UUID }
type ValidateApiCompositionRequestObject struct{ CompositionId uuid.UUID }
type PublishApiCompositionRequestObject struct{ CompositionId uuid.UUID }
type ListApiCompositionVersionsRequestObject struct{ CompositionId uuid.UUID }
type ExecuteApiCompositionRequestObject struct {
	CompositionId uuid.UUID
	Body          *ApiCompositionExecute
}

type ListApiCompositionExecutionsParams struct {
	WorkspaceId   uuid.UUID  `form:"workspace_id" json:"workspace_id"`
	CompositionId *uuid.UUID `form:"composition_id,omitempty" json:"composition_id,omitempty"`
}

type ListApiCompositionExecutionsRequestObject struct {
	Params ListApiCompositionExecutionsParams
}

type GetApiCompositionExecutionRequestObject struct{ ExecutionId uuid.UUID }

type ListApiCompositionsResponseObject interface {
	VisitListApiCompositionsResponse(http.ResponseWriter) error
}
type CreateApiCompositionResponseObject interface {
	VisitCreateApiCompositionResponse(http.ResponseWriter) error
}
type GetApiCompositionResponseObject interface {
	VisitGetApiCompositionResponse(http.ResponseWriter) error
}
type UpdateApiCompositionResponseObject interface {
	VisitUpdateApiCompositionResponse(http.ResponseWriter) error
}
type DeleteApiCompositionResponseObject interface {
	VisitDeleteApiCompositionResponse(http.ResponseWriter) error
}
type ValidateApiCompositionResponseObject interface {
	VisitValidateApiCompositionResponse(http.ResponseWriter) error
}
type PublishApiCompositionResponseObject interface {
	VisitPublishApiCompositionResponse(http.ResponseWriter) error
}
type ListApiCompositionVersionsResponseObject interface {
	VisitListApiCompositionVersionsResponse(http.ResponseWriter) error
}
type ExecuteApiCompositionResponseObject interface {
	VisitExecuteApiCompositionResponse(http.ResponseWriter) error
}
type ListApiCompositionExecutionsResponseObject interface {
	VisitListApiCompositionExecutionsResponse(http.ResponseWriter) error
}
type GetApiCompositionExecutionResponseObject interface {
	VisitGetApiCompositionExecutionResponse(http.ResponseWriter) error
}

type ListApiCompositions200JSONResponse ApiCompositions
type CreateApiComposition201JSONResponse ApiComposition
type GetApiComposition200JSONResponse ApiComposition
type UpdateApiComposition200JSONResponse ApiComposition
type DeleteApiComposition204Response struct{}
type ValidateApiComposition200JSONResponse ApiCompositionValidation
type PublishApiComposition201JSONResponse ApiCompositionVersion
type ListApiCompositionVersions200JSONResponse ApiCompositionVersions
type ExecuteApiComposition200JSONResponse ApiCompositionExecution
type ListApiCompositionExecutions200JSONResponse ApiCompositionExecutions
type GetApiCompositionExecution200JSONResponse ApiCompositionExecution

func writeCompositionJSON(w http.ResponseWriter, status int, value any) error {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(value); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err := body.WriteTo(w)
	return err
}

func (r ListApiCompositions200JSONResponse) VisitListApiCompositionsResponse(w http.ResponseWriter) error {
	return writeCompositionJSON(w, http.StatusOK, r)
}
func (r CreateApiComposition201JSONResponse) VisitCreateApiCompositionResponse(w http.ResponseWriter) error {
	return writeCompositionJSON(w, http.StatusCreated, r)
}
func (r GetApiComposition200JSONResponse) VisitGetApiCompositionResponse(w http.ResponseWriter) error {
	return writeCompositionJSON(w, http.StatusOK, r)
}
func (r UpdateApiComposition200JSONResponse) VisitUpdateApiCompositionResponse(w http.ResponseWriter) error {
	return writeCompositionJSON(w, http.StatusOK, r)
}
func (DeleteApiComposition204Response) VisitDeleteApiCompositionResponse(w http.ResponseWriter) error {
	w.WriteHeader(http.StatusNoContent)
	return nil
}
func (r ValidateApiComposition200JSONResponse) VisitValidateApiCompositionResponse(w http.ResponseWriter) error {
	return writeCompositionJSON(w, http.StatusOK, r)
}
func (r PublishApiComposition201JSONResponse) VisitPublishApiCompositionResponse(w http.ResponseWriter) error {
	return writeCompositionJSON(w, http.StatusCreated, r)
}
func (r ListApiCompositionVersions200JSONResponse) VisitListApiCompositionVersionsResponse(w http.ResponseWriter) error {
	return writeCompositionJSON(w, http.StatusOK, r)
}
func (r ExecuteApiComposition200JSONResponse) VisitExecuteApiCompositionResponse(w http.ResponseWriter) error {
	return writeCompositionJSON(w, http.StatusOK, r)
}
func (r ListApiCompositionExecutions200JSONResponse) VisitListApiCompositionExecutionsResponse(w http.ResponseWriter) error {
	return writeCompositionJSON(w, http.StatusOK, r)
}
func (r GetApiCompositionExecution200JSONResponse) VisitGetApiCompositionExecutionResponse(w http.ResponseWriter) error {
	return writeCompositionJSON(w, http.StatusOK, r)
}
