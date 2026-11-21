package services

import (
	"context"

	"github.com/nuriansyah/hugimethara/internal/modules/governance/entitlement/model"
)

// Code shape: transport adapters and other modules depend on these use-case
// interfaces (ports), never on EntitlementService directly (DIP); each interface
// groups one capability so consumers can depend on the narrowest one (ISP).

// Service is the complete use-case surface of the module.
type Service interface {
	// Catalog returns every feature definition (name, kind, unit).
	Catalog(ctx context.Context) ([]model.Feature, error)
	// Current returns the caller's plan, features and usage.
	Current(ctx context.Context) (model.Set, []model.Usage, error)
	// Plans returns the public plan catalog (any signed-in user may compare plans).
	Plans(ctx context.Context) ([]model.Plan, error)
}

var _ Service = (*EntitlementService)(nil)
