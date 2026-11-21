package services

import (
	"context"

	"github.com/jmoiron/sqlx"

	authzmodel "github.com/nuriansyah/hugimethara/internal/modules/authorization/model"
	authzsvc "github.com/nuriansyah/hugimethara/internal/modules/authorization/services"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

// authorize checks action on a resource in the caller's organization.
func (s *OrganizationService) authorize(ctx context.Context, action, resourceType, id string, attrs map[string]any) (principal.Principal, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return p, authzmodel.Unauthenticated()
	}
	return authzsvc.Require(ctx, s.az, action, authzmodel.Resource{Type: resourceType, ID: id, OrgID: p.OrgID, Attributes: attrs})
}

// tx runs fn in a transaction bound to the caller's organization.
func (s *OrganizationService) tx(ctx context.Context, p principal.Principal, fn func(*sqlx.Tx) error) error {
	return db.InTenantTx(ctx, s.pool, p.OrgID, fn)
}
