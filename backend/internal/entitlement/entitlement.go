// Package entitlement decides which paid features and quotas an organization
// has (plans, subscriptions, per-org overrides) and enforces them server-side.
// It depends only on the platform layer so every domain can use it.
package entitlement

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/backend/internal/platform/db"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/httpx"
)

//go:embed queries.sql
var queryFS embed.FS

var q = db.MustLoad(queryFS, "queries.sql")

// Feature keys. Keep in sync with the catalog seeded in migration 00005.
const (
	WorkspacesMax         = "workspaces.max"
	EnvironmentsMax       = "environments.max"
	Departments           = "departments"
	EgressAllowlist       = "environments.egress_allowlist"
	AuditLogRetentionDays = "audit_log.retention_days"
)

// Feature is one effective entitlement.
type Feature struct {
	Key         string        `db:"key"`
	Name        string        `db:"name"`
	Description string        `db:"description"`
	Kind        string        `db:"kind"` // flag | limit
	Unit        string        `db:"unit"`
	Enabled     bool          `db:"enabled"`
	Limit       sql.NullInt64 `db:"limit_value"` // limits only; !Valid = unlimited
	Overridden  bool          `db:"overridden"`
}

// Set is an organization's effective entitlements.
type Set struct {
	OrgID            uuid.UUID
	PlanKey          string
	PlanName         string
	Status           string
	CurrentPeriodEnd *time.Time
	Features         map[string]Feature
	Order            []string
}

// Load reads the effective entitlements for orgID using tx (bound to orgID).
// A canceled subscription falls back to the default plan's entitlements.
func Load(ctx context.Context, tx *sqlx.Tx, orgID uuid.UUID) (Set, error) {
	var sub struct {
		PlanKey          string       `db:"plan_key"`
		PlanName         string       `db:"plan_name"`
		Status           string       `db:"status"`
		CurrentPeriodEnd sql.NullTime `db:"current_period_end"`
	}
	if err := tx.GetContext(ctx, &sub, q.Get("GetSubscription"), orgID); err != nil {
		if db.IsNoRows(err) {
			return Set{}, fmt.Errorf("entitlement: organization %s has no subscription", orgID)
		}
		return Set{}, err
	}
	planForFeatures := sub.PlanKey
	if sub.Status == "canceled" {
		if err := tx.GetContext(ctx, &planForFeatures, `SELECT key FROM plans WHERE is_default`); err != nil {
			return Set{}, err
		}
	}
	var feats []Feature
	if err := tx.SelectContext(ctx, &feats, q.Get("EffectiveFeatures"), orgID, planForFeatures); err != nil {
		return Set{}, err
	}
	s := Set{OrgID: orgID, PlanKey: sub.PlanKey, PlanName: sub.PlanName, Status: sub.Status, Features: map[string]Feature{}}
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
	res, err := tx.ExecContext(ctx, q.Get("ProvisionDefaultSubscription"), orgID)
	if err != nil {
		return fmt.Errorf("entitlement: provision: %w", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return fmt.Errorf("entitlement: no default plan configured")
	}
	return nil
}

// Enabled reports whether a flag (or any limit) feature is available.
func (s Set) Enabled(key string) bool { return s.Features[key].Enabled }

// Limit returns the numeric limit and whether one applies (false = unlimited).
// A disabled limit feature has limit 0.
func (s Set) Limit(key string) (int64, bool) {
	f, ok := s.Features[key]
	if !ok || !f.Enabled {
		return 0, true
	}
	if !f.Limit.Valid {
		return 0, false
	}
	return f.Limit.Int64, true
}

// Require returns a 402 problem unless the flag feature is enabled.
func (s Set) Require(key string) error {
	if s.Enabled(key) {
		return nil
	}
	return &NotEntitled{Feature: key, FeatureName: s.Features[key].Name, Plan: s.PlanKey}
}

// CheckLimit returns a 402 problem when adding `adding` more would exceed the limit.
func (s Set) CheckLimit(key string, current, adding int64) error {
	limit, limited := s.Limit(key)
	if !limited || current+adding <= limit {
		return nil
	}
	return &LimitReached{Feature: key, FeatureName: s.Features[key].Name, Plan: s.PlanKey, Limit: limit, Current: current}
}

// LockFeature serializes concurrent check-then-create operations for one
// org + feature until tx ends, so two parallel requests cannot both pass a
// limit check and exceed the quota.
func LockFeature(ctx context.Context, tx *sqlx.Tx, orgID uuid.UUID, key string) error {
	_, err := tx.ExecContext(ctx, q.Get("LockFeature"), orgID.String(), key)
	return err
}

// NotEntitled: the feature is not part of the organization's plan.
type NotEntitled struct{ Feature, FeatureName, Plan string }

func (e *NotEntitled) Error() string { return "feature not in plan: " + e.Feature }

func (e *NotEntitled) Problem() *httpx.Problem {
	p := httpx.NewProblem(http.StatusPaymentRequired, "feature_not_in_plan", "Upgrade required",
		fmt.Sprintf("%s is not included in your current plan.", nameOr(e.FeatureName, e.Feature)))
	p.Extensions = map[string]any{"feature": e.Feature, "plan": e.Plan}
	return p
}

// LimitReached: creating more would exceed the plan quota.
type LimitReached struct {
	Feature, FeatureName, Plan string
	Limit, Current             int64
}

func (e *LimitReached) Error() string { return "plan limit reached: " + e.Feature }

func (e *LimitReached) Problem() *httpx.Problem {
	p := httpx.NewProblem(http.StatusPaymentRequired, "plan_limit_reached", "Plan limit reached",
		fmt.Sprintf("Your plan allows %d %s. Archive or remove one, or upgrade your plan.",
			e.Limit, nameOr(lower(e.FeatureName), e.Feature)))
	p.Extensions = map[string]any{"feature": e.Feature, "plan": e.Plan, "limit": e.Limit, "current": e.Current}
	return p
}

func nameOr(name, key string) string {
	if name != "" {
		return name
	}
	return key
}

func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
