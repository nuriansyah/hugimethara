// Package principal carries the authenticated actor through a request context.
// It is a leaf package so that every domain (and authz) can depend on it.
package principal

import (
	"context"

	"github.com/google/uuid"
)

// Type is the kind of actor.
type Type string

const (
	User           Type = "user"
	ServiceAccount Type = "service_account"
)

// Principal is an authenticated actor bound to exactly one organization.
type Principal struct {
	ID        uuid.UUID
	Type      Type
	OrgID     uuid.UUID
	SessionID uuid.UUID // zero for API-key principals
	// IsOwner is interim until S3 RBAC: owners hold every permission in their
	// org. S3 replaces it with role bindings evaluated by authz.
	IsOwner    bool
	Groups     []uuid.UUID
	Attributes map[string]any
}

type ctxKey struct{}

// With returns a context carrying p.
func With(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

// From returns the principal in ctx, if any.
func From(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok
}
