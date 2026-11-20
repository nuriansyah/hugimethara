package secret

import (
	"context"

	"github.com/google/uuid"

	"github.com/nuriansyah/hugimethara/backend/internal/api/apigen"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/httpx"
)

type HTTP struct{ svc *Service }

func NewHTTP(svc *Service) *HTTP { return &HTTP{svc: svc} }

func apiCredential(c Credential) apigen.Credential {
	out := apigen.Credential{Id: c.ID, Name: c.Name, Kind: c.Kind, Configured: true,
		Hint: c.Hint, Version: c.Version, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}
	if c.EnvironmentID != nil {
		id := *c.EnvironmentID
		out.EnvironmentId = &id
	}
	return out
}

func (h *HTTP) ListCredentials(ctx context.Context, _ apigen.ListCredentialsRequestObject) (apigen.ListCredentialsResponseObject, error) {
	credentials, err := h.svc.List(ctx)
	if err != nil {
		return nil, err
	}
	out := apigen.ListCredentials200JSONResponse{Data: make([]apigen.Credential, len(credentials))}
	for i, c := range credentials {
		out.Data[i] = apiCredential(c)
	}
	return out, nil
}

func (h *HTTP) CreateCredential(ctx context.Context, req apigen.CreateCredentialRequestObject) (apigen.CreateCredentialResponseObject, error) {
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
	return apigen.CreateCredential201JSONResponse(apiCredential(c)), nil
}

func (h *HTTP) GetCredential(ctx context.Context, req apigen.GetCredentialRequestObject) (apigen.GetCredentialResponseObject, error) {
	c, err := h.svc.Get(ctx, req.CredentialId)
	if err != nil {
		return nil, err
	}
	return apigen.GetCredential200JSONResponse(apiCredential(c)), nil
}

func (h *HTTP) UpdateCredential(ctx context.Context, req apigen.UpdateCredentialRequestObject) (apigen.UpdateCredentialResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.BadRequest("A JSON body is required.")
	}
	c, err := h.svc.Update(ctx, req.CredentialId, UpdateInput{Name: req.Body.Name, Value: req.Body.Value})
	if err != nil {
		return nil, err
	}
	return apigen.UpdateCredential200JSONResponse(apiCredential(c)), nil
}

func (h *HTTP) DeleteCredential(ctx context.Context, req apigen.DeleteCredentialRequestObject) (apigen.DeleteCredentialResponseObject, error) {
	if err := h.svc.Delete(ctx, req.CredentialId); err != nil {
		return nil, err
	}
	return apigen.DeleteCredential204Response{}, nil
}
