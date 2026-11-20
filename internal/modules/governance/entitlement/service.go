package entitlement

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/modules/authorization"
	"github.com/nuriansyah/hugimethara/internal/platform/database"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

// ResourceType is the authz resource type for the subscription/plan view.
const ResourceType = "subscription"

// UsageFunc counts current usage of a limit feature inside a tenant-bound tx.
type UsageFunc func(ctx context.Context, tx *sqlx.Tx) (int64, error)

// Service exposes plans and the caller's entitlements with usage.
type Service struct {
	pool  *sqlx.DB
	az    authz.Authorizer
	usage map[string]UsageFunc
}

// NewService wires usage counters contributed by other domains (feature key → counter).
func NewService(pool *sqlx.DB, az authz.Authorizer, usage map[string]UsageFunc) *Service {
	return &Service{pool: pool, az: az, usage: usage}
}

// Usage pairs an effective feature with its current consumption.
type Usage struct {
	Feature
	Used *int64 // limits with a usage counter only
}

// Current returns the caller's plan, features and usage.
func (s *Service) Current(ctx context.Context) (Set, []Usage, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return Set{}, nil, authz.Unauthenticated()
	}
	if _, err := authz.Require(ctx, s.az, "read", authz.Resource{Type: ResourceType, OrgID: p.OrgID}); err != nil {
		return Set{}, nil, err
	}
	var set Set
	var out []Usage
	err := db.InTenantTx(ctx, s.pool, p.OrgID, func(tx *sqlx.Tx) error {
		var err error
		if set, err = Load(ctx, tx, p.OrgID); err != nil {
			return err
		}
		for _, key := range set.Order {
			u := Usage{Feature: set.Features[key]}
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

// Plan is a catalog entry with its entitlements.
type Plan struct {
	Key               string        `db:"key"`
	Name              string        `db:"name"`
	Description       string        `db:"description"`
	PriceMonthlyCents sql.NullInt64 `db:"price_monthly_cents"`
	Currency          string        `db:"currency"`
	Entitlements      map[string]PlanEntitlement
}

// PlanEntitlement is a feature as granted by a plan.
type PlanEntitlement struct {
	Enabled bool
	Limit   *int64
}

// Plans returns the public plan catalog (any signed-in user may compare plans).
func (s *Service) Plans(ctx context.Context) ([]Plan, error) {
	if _, ok := principal.From(ctx); !ok {
		return nil, authz.Unauthenticated()
	}
	var plans []Plan
	if err := s.pool.SelectContext(ctx, &plans, q.Get("ListPlans")); err != nil {
		return nil, err
	}
	var ents []struct {
		PlanKey    string        `db:"plan_key"`
		FeatureKey string        `db:"feature_key"`
		Enabled    bool          `db:"enabled"`
		Limit      sql.NullInt64 `db:"limit_value"`
	}
	if err := s.pool.SelectContext(ctx, &ents, q.Get("ListPlanEntitlements")); err != nil {
		return nil, err
	}
	idx := map[string]int{}
	for i := range plans {
		plans[i].Entitlements = map[string]PlanEntitlement{}
		idx[plans[i].Key] = i
	}
	for _, e := range ents {
		pe := PlanEntitlement{Enabled: e.Enabled}
		if e.Limit.Valid {
			v := e.Limit.Int64
			pe.Limit = &v
		}
		plans[idx[e.PlanKey]].Entitlements[e.FeatureKey] = pe
	}
	return plans, nil
}

// Catalog returns every feature definition (name, kind, unit).
func (s *Service) Catalog(ctx context.Context) ([]Feature, error) {
	var out []Feature
	err := s.pool.SelectContext(ctx, &out, `SELECT key, name, description, kind, unit,
		true AS enabled, NULL::bigint AS limit_value, false AS overridden FROM features ORDER BY key`)
	return out, err
}
