package entitlement

import (
	"context"

	"github.com/nuriansyah/hugimethara/internal/transport/http/dto"
)

// HTTP implements the billing slice of dto.StrictServerInterface.
type HTTP struct{ svc *Service }

func NewHTTP(svc *Service) *HTTP { return &HTTP{svc: svc} }

// @Summary The organization's plan, paid features, quotas and usage
// @ID getEntitlements
// @Tags billing
// @Success 200 {object} dto.Entitlements "Effective entitlements."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/entitlements [get]
func (h *HTTP) GetEntitlements(ctx context.Context, _ dto.GetEntitlementsRequestObject) (dto.GetEntitlementsResponseObject, error) {
	set, usage, err := h.svc.Current(ctx)
	if err != nil {
		return nil, err
	}
	out := dto.GetEntitlements200JSONResponse{Status: dto.EntitlementsStatus(set.Status), CurrentPeriodEnd: set.CurrentPeriodEnd}
	out.Plan.Key, out.Plan.Name = set.PlanKey, set.PlanName
	out.Features = make([]dto.EntitlementFeature, len(usage))
	for i, u := range usage {
		f := dto.EntitlementFeature{Key: u.Key, Name: u.Name, Description: u.Description,
			Kind: dto.EntitlementFeatureKind(u.Kind), Unit: u.Unit, Enabled: u.Enabled, Overridden: u.Overridden, Used: u.Used}
		if u.Kind == "limit" && u.Limit.Valid {
			v := u.Limit.Int64
			f.Limit = &v
		}
		if u.Kind == "limit" && !u.Enabled {
			zero := int64(0)
			f.Limit = &zero
		}
		out.Features[i] = f
	}
	return out, nil
}

// @Summary Plan catalog for comparison
// @ID listPlans
// @Tags billing
// @Success 200 {object} dto.ListPlans200JSONResponse "Plans with their entitlements."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/plans [get]
func (h *HTTP) ListPlans(ctx context.Context, _ dto.ListPlansRequestObject) (dto.ListPlansResponseObject, error) {
	plans, err := h.svc.Plans(ctx)
	if err != nil {
		return nil, err
	}
	catalog, err := h.svc.Catalog(ctx)
	if err != nil {
		return nil, err
	}
	out := dto.ListPlans200JSONResponse{Data: make([]dto.Plan, len(plans)), Features: make([]dto.FeatureDefinition, len(catalog))}
	for i, p := range plans {
		ap := dto.Plan{Key: p.Key, Name: p.Name, Description: p.Description, Currency: p.Currency,
			Entitlements: map[string]struct {
				Enabled bool   `json:"enabled"`
				Limit   *int64 `json:"limit,omitempty"`
			}{}}
		if p.PriceMonthlyCents.Valid {
			v := p.PriceMonthlyCents.Int64
			ap.PriceMonthlyCents = &v
		}
		for k, e := range p.Entitlements {
			ap.Entitlements[k] = struct {
				Enabled bool   `json:"enabled"`
				Limit   *int64 `json:"limit,omitempty"`
			}{Enabled: e.Enabled, Limit: e.Limit}
		}
		out.Data[i] = ap
	}
	for i, f := range catalog {
		out.Features[i] = dto.FeatureDefinition{Key: f.Key, Name: f.Name, Description: f.Description,
			Kind: dto.FeatureDefinitionKind(f.Kind), Unit: f.Unit}
	}
	return out, nil
}
