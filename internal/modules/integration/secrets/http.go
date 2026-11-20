package secret

import (
	"context"

	"github.com/google/uuid"

	"github.com/nuriansyah/hugimethara/internal/platform/httpx"
	"github.com/nuriansyah/hugimethara/internal/transport/http/dto"
)

type HTTP struct{ svc *Service }

func NewHTTP(svc *Service) *HTTP { return &HTTP{svc: svc} }

func apiCredential(c Credential) dto.Credential {
	out := dto.Credential{Id: c.ID, Name: c.Name, Kind: c.Kind, Configured: true,
		Hint: c.Hint, Version: c.Version, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}
	if c.EnvironmentID != nil {
		id := *c.EnvironmentID
		out.EnvironmentId = &id
	}
	return out
}

// @Summary List credential metadata
// @ID listCredentials
// @Tags secrets
// @Success 200 {object} dto.ListCredentials200JSONResponse "Credential metadata."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/credentials [get]
func (h *HTTP) ListCredentials(ctx context.Context, _ dto.ListCredentialsRequestObject) (dto.ListCredentialsResponseObject, error) {
	credentials, err := h.svc.List(ctx)
	if err != nil {
		return nil, err
	}
	out := dto.ListCredentials200JSONResponse{Data: make([]dto.Credential, len(credentials))}
	for i, c := range credentials {
		out.Data[i] = apiCredential(c)
	}
	return out, nil
}

// @Summary Store an encrypted credential
// @ID createCredential
// @Tags secrets
// @Param body body dto.CredentialCreate true "Request body"
// @Success 201 {object} dto.Credential "Stored credential metadata."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/credentials [post]
func (h *HTTP) CreateCredential(ctx context.Context, req dto.CreateCredentialRequestObject) (dto.CreateCredentialResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.BadRequest("A JSON body is required.")
	}
	var environmentID *uuid.UUID
	if req.Body.EnvironmentId != nil {
		id := *req.Body.EnvironmentId
		environmentID = &id
	}
	c, err := h.svc.Create(ctx, CreateInput{Name: req.Body.Name, Kind: req.Body.Kind,
		Value: req.Body.Value, EnvironmentID: environmentID})
	if err != nil {
		return nil, err
	}
	return dto.CreateCredential201JSONResponse(apiCredential(c)), nil
}

// @Summary Get credential metadata
// @ID getCredential
// @Tags secrets
// @Param credential_id path string true "credential_id"
// @Success 200 {object} dto.Credential "Credential metadata."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/credentials/{credential_id} [get]
func (h *HTTP) GetCredential(ctx context.Context, req dto.GetCredentialRequestObject) (dto.GetCredentialResponseObject, error) {
	c, err := h.svc.Get(ctx, req.CredentialId)
	if err != nil {
		return nil, err
	}
	return dto.GetCredential200JSONResponse(apiCredential(c)), nil
}

// @Summary Update credential metadata or rotate its value
// @ID updateCredential
// @Tags secrets
// @Param credential_id path string true "credential_id"
// @Param body body dto.CredentialUpdate true "Request body"
// @Success 200 {object} dto.Credential "Updated credential metadata."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/credentials/{credential_id} [patch]
func (h *HTTP) UpdateCredential(ctx context.Context, req dto.UpdateCredentialRequestObject) (dto.UpdateCredentialResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.BadRequest("A JSON body is required.")
	}
	c, err := h.svc.Update(ctx, req.CredentialId, UpdateInput{Name: req.Body.Name, Value: req.Body.Value})
	if err != nil {
		return nil, err
	}
	return dto.UpdateCredential200JSONResponse(apiCredential(c)), nil
}

// @Summary Delete a credential
// @ID deleteCredential
// @Tags secrets
// @Param credential_id path string true "credential_id"
// @Success 204 "Deleted."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/credentials/{credential_id} [delete]
func (h *HTTP) DeleteCredential(ctx context.Context, req dto.DeleteCredentialRequestObject) (dto.DeleteCredentialResponseObject, error) {
	if err := h.svc.Delete(ctx, req.CredentialId); err != nil {
		return nil, err
	}
	return dto.DeleteCredential204Response{}, nil
}
