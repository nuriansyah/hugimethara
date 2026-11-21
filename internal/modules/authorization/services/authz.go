package services

import (
	"context"
	"net/http"

	authzmodel "github.com/nuriansyah/hugimethara/internal/modules/authorization/model"
	"github.com/nuriansyah/hugimethara/internal/modules/authorization/model/dto"
	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

// Authorizer evaluates access. Implementations must deny by default.
type Authorizer interface {
	Evaluate(ctx context.Context, in dto.AuthorizationInput) (authzmodel.AuthorizationDecision, error)
}

// Require evaluates access for the principal in ctx and returns an HTTP
// problem when denied: 401 without a principal, 404 for other tenants'
// resources (no existence oracle), 403 otherwise.
func Require(ctx context.Context, az Authorizer, action string, res authzmodel.Resource) (principal.Principal, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return p, authzmodel.Unauthenticated()
	}
	d, err := az.Evaluate(ctx, dto.AuthorizationInput{Principal: p, Action: action, Resource: res})
	if err != nil {
		return p, err
	}
	if d.Allowed {
		return p, nil
	}
	if d.Reason == authzmodel.ReasonTenantMismatch {
		return p, problem.NotFound("The requested resource does not exist.")
	}
	return p, problem.NewProblem(http.StatusForbidden, "forbidden", "Forbidden",
		"You do not have permission to "+action+" this "+res.Type+".")
}

// ScopeResolver answers list-level access questions.
type ScopeResolver interface {
	WorkspaceScope(ctx context.Context, p principal.Principal, action string) (authzmodel.Scope, error)
}
