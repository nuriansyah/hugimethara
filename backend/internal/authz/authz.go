// Package authz is the single authorization contract used by every domain
// (ADR-0007). S1 ships an interim, deny-by-default implementation; S3 replaces
// it with dynamic RBAC behind the same interface.
package authz

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/nuriansyah/hugimethara/backend/internal/platform/httpx"
	"github.com/nuriansyah/hugimethara/backend/internal/principal"
)

// Resource identifies what is being accessed.
type Resource struct {
	Type        string
	ID          string // empty for collection-level actions
	OrgID       uuid.UUID
	WorkspaceID *uuid.UUID
	Attributes  map[string]any
}

// AuthorizationInput is one access question.
type AuthorizationInput struct {
	Principal principal.Principal
	Action    string
	Resource  Resource
	Context   map[string]any
}

// AuthorizationDecision is the answer plus the reason, for audit and debugging.
type AuthorizationDecision struct {
	Allowed       bool
	Reason        string
	MatchedRules  []string
	PolicyVersion int64
}

// Authorizer evaluates access. Implementations must deny by default.
type Authorizer interface {
	Evaluate(ctx context.Context, in AuthorizationInput) (AuthorizationDecision, error)
}

// Reasons.
const (
	ReasonTenantMismatch = "tenant_mismatch"
	ReasonNoPrincipal    = "no_principal"
)

// Permission formats "<resource_type>:<action>".
func Permission(resourceType, action string) string { return resourceType + ":" + action }

// Require evaluates access for the principal in ctx and returns an HTTP
// problem when denied: 401 without a principal, 404 for other tenants'
// resources (no existence oracle), 403 otherwise.
func Require(ctx context.Context, az Authorizer, action string, res Resource) (principal.Principal, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return p, Unauthenticated()
	}
	d, err := az.Evaluate(ctx, AuthorizationInput{Principal: p, Action: action, Resource: res})
	if err != nil {
		return p, err
	}
	if d.Allowed {
		return p, nil
	}
	if d.Reason == ReasonTenantMismatch {
		return p, httpx.NotFound("The requested resource does not exist.")
	}
	return p, httpx.NewProblem(http.StatusForbidden, "forbidden", "Forbidden",
		"You do not have permission to "+action+" this "+res.Type+".")
}

// Unauthenticated is the 401 problem for requests without a valid session.
func Unauthenticated() *httpx.Problem {
	return httpx.NewProblem(http.StatusUnauthorized, "unauthenticated", "Authentication required",
		"Sign in to continue.")
}

// Scope says which workspaces a principal may act on in bulk (list
// endpoints). Evaluating per row does not scale, so repositories turn a Scope
// into a SQL predicate.
type Scope struct {
	All          bool        // every workspace in the organization
	WorkspaceIDs []uuid.UUID // otherwise only these
}

// None reports whether the scope grants nothing.
func (s Scope) None() bool { return !s.All && len(s.WorkspaceIDs) == 0 }

// ScopeResolver answers list-level access questions.
type ScopeResolver interface {
	WorkspaceScope(ctx context.Context, p principal.Principal, action string) (Scope, error)
}
