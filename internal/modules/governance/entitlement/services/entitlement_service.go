package services

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"

	authzmodel "github.com/nuriansyah/hugimethara/internal/modules/authorization/model"
	authzsvc "github.com/nuriansyah/hugimethara/internal/modules/authorization/services"
	"github.com/nuriansyah/hugimethara/internal/modules/governance/entitlement/model"
	"github.com/nuriansyah/hugimethara/internal/modules/governance/entitlement/repository/queries"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

// UsageFunc counts current usage of a limit feature inside a tenant-bound tx.
type UsageFunc func(ctx context.Context, tx *sqlx.Tx) (int64, error)

// EntitlementService exposes plans and the caller's entitlements with usage.
type EntitlementService struct {
	pool  *sqlx.DB
	az    authzsvc.Authorizer
	usage map[string]UsageFunc
}

// NewService wires usage counters contributed by other domains (feature key → counter).
func NewService(pool *sqlx.DB, az authzsvc.Authorizer, usage map[string]UsageFunc) *EntitlementService {
	return &EntitlementService{pool: pool, az: az, usage: usage}
}

// Current returns the caller's plan, features and usage.
func (s *EntitlementService) Current(ctx context.Context) (model.Set, []model.Usage, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return model.Set{}, nil, authzmodel.Unauthenticated()
	}
	if _, err := authzsvc.Require(ctx, s.az, "read", authzmodel.Resource{Type: model.ResourceType, OrgID: p.OrgID}); err != nil {
		return model.Set{}, nil, err
	}
	var set model.Set
	var out []model.Usage
	err := db.InTenantTx(ctx, s.pool, p.OrgID, func(tx *sqlx.Tx) error {
		var err error
		if set, err = Load(ctx, tx, p.OrgID); err != nil {
			return err
		}
		for _, key := range set.Order {
			u := model.Usage{Feature: set.Features[key]}
			if fn := s.usage[key]; fn != nil {
				n, err := fn(ctx, tx)
				if err != nil {
					return err
				}
				u.Used = &n
			}
			out = append(out, u)
		}
		return nil
	})
	return set, out, err
}

// Plans returns the public plan catalog (any signed-in user may compare plans).
func (s *EntitlementService) Plans(ctx context.Context) ([]model.Plan, error) {
	if _, ok := principal.From(ctx); !ok {
		return nil, authzmodel.Unauthenticated()
	}
	var plans []model.Plan
	if err := s.pool.SelectContext(ctx, &plans, queries.SQL.Get("ListPlans")); err != nil {
		return nil, err
	}
	var ents []struct {
		PlanKey    string        `db:"plan_key"`
		FeatureKey string        `db:"feature_key"`
		Enabled    bool          `db:"enabled"`
		Limit      sql.NullInt64 `db:"limit_value"`
	}
	if err := s.pool.SelectContext(ctx, &ents, queries.SQL.Get("ListPlanEntitlements")); err != nil {
		return nil, err
	}
	idx := map[string]int{}
	for i := range plans {
		plans[i].Entitlements = map[string]model.PlanEntitlement{}
		idx[plans[i].Key] = i
	}
	for _, e := range ents {
		pe := model.PlanEntitlement{Enabled: e.Enabled}
		if e.Limit.Valid {
			v := e.Limit.Int64
			pe.Limit = &v
		}
		plans[idx[e.PlanKey]].Entitlements[e.FeatureKey] = pe
	}
	return plans, nil
}

// Catalog returns every feature definition (name, kind, unit).
func (s *EntitlementService) Catalog(ctx context.Context) ([]model.Feature, error) {
	var out []model.Feature
	err := s.pool.SelectContext(ctx, &out, `SELECT key, name, description, kind, unit,
		true AS enabled, NULL::bigint AS limit_value, false AS overridden FROM features ORDER BY key`)
	return out, err
}
