package organization

import (
	"context"

	"github.com/nuriansyah/hugimethara/backend/internal/api/apigen"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/httpx"
)

// HTTP implements the organization slice of apigen.StrictServerInterface.
type HTTP struct{ svc *Service }

func NewHTTP(svc *Service) *HTTP { return &HTTP{svc: svc} }

func (h *HTTP) GetCurrentOrganization(ctx context.Context, _ apigen.GetCurrentOrganizationRequestObject) (apigen.GetCurrentOrganizationResponseObject, error) {
	o, err := h.svc.Current(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.GetCurrentOrganization200JSONResponse(toAPI(o)), nil
}

func (h *HTTP) UpdateCurrentOrganization(ctx context.Context, req apigen.UpdateCurrentOrganizationRequestObject) (apigen.UpdateCurrentOrganizationResponseObject, error) {
	if req.Body == nil || (req.Body.Name == nil && req.Body.Settings == nil) {
		return nil, httpx.Validation(httpx.FieldError{Field: "", Code: "empty_update", Message: "Provide at least one field to change."})
	}
	var u Update
	u.Name = req.Body.Name
	if req.Body.Settings != nil {
		u.Settings = &Settings{
			SessionIdleTimeoutMinutes:   req.Body.Settings.SessionIdleTimeoutMinutes,
			SessionAbsoluteTimeoutHours: req.Body.Settings.SessionAbsoluteTimeoutHours,
		}
	}
	o, err := h.svc.UpdateCurrent(ctx, u)
	if err != nil {
		return nil, err
	}
	return apigen.UpdateCurrentOrganization200JSONResponse(toAPI(o)), nil
}

func toAPI(o Organization) apigen.Organization {
	return apigen.Organization{
		Id: o.ID, Slug: o.Slug, Name: o.Name, Status: apigen.OrganizationStatus(o.Status),
		Settings: apigen.OrganizationSettings{
			SessionIdleTimeoutMinutes:   o.Settings.SessionIdleTimeoutMinutes,
			SessionAbsoluteTimeoutHours: o.Settings.SessionAbsoluteTimeoutHours,
		},
		CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt,
	}
}
