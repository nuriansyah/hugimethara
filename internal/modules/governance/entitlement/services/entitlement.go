package services

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/modules/governance/entitlement/model"
	"github.com/nuriansyah/hugimethara/internal/modules/governance/entitlement/repository/commands"
	"github.com/nuriansyah/hugimethara/internal/modules/governance/entitlement/repository/queries"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
)

// Load reads the effective entitlements for orgID using tx (bound to orgID).
// A canceled subscription falls back to the default plan's entitlements.
func Load(ctx context.Context, tx *sqlx.Tx, orgID uuid.UUID) (model.Set, error) {
	var sub struct {
		PlanKey          string       `db:"plan_key"`
		PlanName         string       `db:"plan_name"`
		Status           string       `db:"status"`
		CurrentPeriodEnd sql.NullTime `db:"current_period_end"`
	}
	if err := tx.GetContext(ctx, &sub, queries.SQL.Get("GetSubscription"), orgID); err != nil {
		if db.IsNoRows(err) {
			return model.Set{}, fmt.Errorf("entitlement: organization %s has no subscription", orgID)
		}
		return model.Set{}, err
	}
	planForFeatures := sub.PlanKey
	if sub.Status == "canceled" {
		if err := tx.GetContext(ctx, &planForFeatures, `SELECT key FROM plans WHERE is_default`); err != nil {
			return model.Set{}, err
		}
	}
	var feats []model.Feature
	if err := tx.SelectContext(ctx, &feats, queries.SQL.Get("EffectiveFeatures"), orgID, planForFeatures); err != nil {
		return model.Set{}, err
	}
	s := model.Set{OrgID: orgID, PlanKey: sub.PlanKey, PlanName: sub.PlanName, Status: sub.Status, Features: map[string]model.Feature{}}
	if sub.CurrentPeriodEnd.Valid {
		t := sub.CurrentPeriodEnd.Time
		s.CurrentPeriodEnd = &t
	}
	for _, f := range feats {
		s.Features[f.Key] = f
		s.Order = append(s.Order, f.Key)
	}
	return s, nil
}

// Provision gives a new organization the default plan (inside the signup tx).
func Provision(ctx context.Context, tx *sqlx.Tx, orgID uuid.UUID) error {
	res, err := tx.ExecContext(ctx, commands.SQL.Get("ProvisionDefaultSubscription"), orgID)
	if err != nil {
		return fmt.Errorf("entitlement: provision: %w", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return fmt.Errorf("entitlement: no default plan configured")
	}
	return nil
}

// LockFeature serializes concurrent check-then-create operations for one
// org + feature until tx ends, so two parallel requests cannot both pass a
// limit check and exceed the quota.
func LockFeature(ctx context.Context, tx *sqlx.Tx, orgID uuid.UUID, key string) error {
	_, err := tx.ExecContext(ctx, queries.SQL.Get("LockFeature"), orgID.String(), key)
	return err
}
