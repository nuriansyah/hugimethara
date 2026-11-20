package entitlement

import (
	"context"

	"github.com/nuriansyah/hugimethara/internal/transport/http/apigen"
)

// HTTP implements the billing slice of apigen.StrictServerInterface.
type HTTP struct{ svc *Service }

func NewHTTP(svc *Service) *HTTP { return &HTTP{svc: svc} }

func (h *HTTP) GetEntitlements(ctx context.Context, _ apigen.GetEntitlementsRequestObject) (apigen.GetEntitlementsResponseObject, error) {
	set, usage, err := h.svc.Current(ctx)
	if err != nil {
		return nil, err
	}
	out := apigen.GetEntitlements200JSONResponse{Status: apigen.EntitlementsStatus(set.Status), CurrentPeriodEnd: set.CurrentPeriodEnd}
	out.Plan.Key, out.Plan.Name = set.PlanKey, set.PlanName
	out.Features = make([]apigen.EntitlementFeature, len(usage))
	for i, u := range usage {
		f := apigen.EntitlementFeature{Key: u.Key, Name: u.Name, Description: u.Description,
			Kind: apigen.EntitlementFeatureKind(u.Kind), Unit: u.Unit, Enabled: u.Enabled, Overridden: u.Overridden, Used: u.Used}
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

func (h *HTTP) ListPlans(ctx context.Context, _ apigen.ListPlansRequestObject) (apigen.ListPlansResponseObject, error) {
	plans, err := h.svc.Plans(ctx)
	if err != nil {
		return nil, err
	}
	catalog, err := h.svc.Catalog(ctx)
	if err != nil {
		return nil, err
	}
	out := apigen.ListPlans200JSONResponse{Data: make([]apigen.Plan, len(plans)), Features: make([]apigen.FeatureDefinition, len(catalog))}
	for i, p := range plans {
		ap := apigen.Plan{Key: p.Key, Name: p.Name, Description: p.Description, Currency: p.Currency,
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
		out.Features[i] = apigen.FeatureDefinition{Key: f.Key, Name: f.Name, Description: f.Description,
			Kind: apigen.FeatureDefinitionKind(f.Kind), Unit: f.Unit}
	}
	return out, nil
}
