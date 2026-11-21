package model

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
)

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

// NotEntitled: the feature is not part of the organization's plan.
type NotEntitled struct{ Feature, FeatureName, Plan string }

func (e *NotEntitled) Error() string { return "feature not in plan: " + e.Feature }

func (e *NotEntitled) Problem() *problem.Problem {
	p := problem.NewProblem(http.StatusPaymentRequired, "feature_not_in_plan", "Upgrade required",
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

func (e *LimitReached) Problem() *problem.Problem {
	p := problem.NewProblem(http.StatusPaymentRequired, "plan_limit_reached", "Plan limit reached",
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
