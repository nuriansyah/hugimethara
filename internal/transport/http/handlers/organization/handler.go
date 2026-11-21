package organization

import (
	"context"

	"github.com/nuriansyah/hugimethara/internal/modules/organization/model"
	orgdto "github.com/nuriansyah/hugimethara/internal/modules/organization/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/organization/services"
	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/transport/http/dto"
)

// HTTP implements the organization slice of dto.StrictServerInterface.
type HTTP struct{ svc services.Service }

func NewHTTP(svc services.Service) *HTTP { return &HTTP{svc: svc} }

// @Summary The caller's organization
// @ID getCurrentOrganization
// @Tags organization
// @Success 200 {object} dto.Organization "The organization."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/organizations/current [get]
func (h *HTTP) GetCurrentOrganization(ctx context.Context, _ dto.GetCurrentOrganizationRequestObject) (dto.GetCurrentOrganizationResponseObject, error) {
	o, err := h.svc.Current(ctx)
	if err != nil {
		return nil, err
	}
	return dto.GetCurrentOrganization200JSONResponse(toAPI(o)), nil
}

// @Summary Update the caller's organization
// @ID updateCurrentOrganization
// @Tags organization
// @Param body body dto.UpdateOrganizationRequest true "Request body"
// @Success 200 {object} dto.Organization "The updated organization."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/organizations/current [patch]
func (h *HTTP) UpdateCurrentOrganization(ctx context.Context, req dto.UpdateCurrentOrganizationRequestObject) (dto.UpdateCurrentOrganizationResponseObject, error) {
	if req.Body == nil || (req.Body.Name == nil && req.Body.Settings == nil) {
		return nil, problem.Validation(problem.FieldError{Field: "", Code: "empty_update", Message: "Provide at least one field to change."})
	}
	var u orgdto.Update
	u.Name = req.Body.Name
	if req.Body.Settings != nil {
		u.Settings = &model.Settings{
			SessionIdleTimeoutMinutes:   req.Body.Settings.SessionIdleTimeoutMinutes,
			SessionAbsoluteTimeoutHours: req.Body.Settings.SessionAbsoluteTimeoutHours,
		}
	}
	o, err := h.svc.UpdateCurrent(ctx, u)
	if err != nil {
		return nil, err
	}
	return dto.UpdateCurrentOrganization200JSONResponse(toAPI(o)), nil
}

func toAPI(o model.Organization) dto.Organization {
	return dto.Organization{
		Id: o.ID, Slug: o.Slug, Name: o.Name, Status: dto.OrganizationStatus(o.Status),
		Settings: dto.OrganizationSettings{
			SessionIdleTimeoutMinutes:   o.Settings.SessionIdleTimeoutMinutes,
			SessionAbsoluteTimeoutHours: o.Settings.SessionAbsoluteTimeoutHours,
		},
		CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt,
	}
}
