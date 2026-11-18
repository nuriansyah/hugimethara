package authz

import (
	"context"
)

// InterimAuthorizer is the S1 stand-in for RBAC:
//   - principals never act outside their own organization;
//   - organization owners are allowed everything inside it;
//   - other members get only the explicitly listed member permissions;
//   - everything else is denied.
//
// S3 replaces it with the dynamic RBAC evaluator; callers do not change.
type InterimAuthorizer struct{}

// memberPermissions are granted to every active member.
var memberPermissions = map[string]bool{
	"organization:read": true,
}

func (InterimAuthorizer) Evaluate(_ context.Context, in AuthorizationInput) (AuthorizationDecision, error) {
	p := in.Principal
	if p.OrgID == [16]byte{} {
		return AuthorizationDecision{Reason: ReasonNoPrincipal}, nil
	}
	if p.OrgID != in.Resource.OrgID {
		return AuthorizationDecision{Reason: ReasonTenantMismatch}, nil
	}
	if p.IsOwner {
		return AuthorizationDecision{Allowed: true, Reason: "interim_owner", MatchedRules: []string{"org_owner"}}, nil
	}
	perm := Permission(in.Resource.Type, in.Action)
	if memberPermissions[perm] {
		return AuthorizationDecision{Allowed: true, Reason: "interim_member", MatchedRules: []string{"member:" + perm}}, nil
	}
	return AuthorizationDecision{Reason: "interim_owner_required"}, nil
}
