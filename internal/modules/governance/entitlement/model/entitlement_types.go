package model

import "database/sql"

// ResourceType is the authz resource type for the subscription/plan view.
const ResourceType = "subscription"

// Usage pairs an effective feature with its current consumption.
type Usage struct {
	Feature
	Used *int64 // limits with a usage counter only
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
