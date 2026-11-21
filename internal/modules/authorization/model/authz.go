package model

import (
	"net/http"

	"github.com/google/uuid"

	apperrors "github.com/nuriansyah/hugimethara/internal/platform/errors"
)

// Resource identifies what is being accessed.
type Resource struct {
	Type        string
	ID          string // empty for collection-level actions
	OrgID       uuid.UUID
	WorkspaceID *uuid.UUID
	Attributes  map[string]any
}

// AuthorizationDecision is the answer plus the reason, for audit and debugging.
type AuthorizationDecision struct {
	Allowed       bool
	Reason        string
	MatchedRules  []string
	PolicyVersion int64
}

// Reasons.
const (
	ReasonTenantMismatch = "tenant_mismatch"
	ReasonNoPrincipal    = "no_principal"
)

// Permission formats "<resource_type>:<action>".
func Permission(resourceType, action string) string { return resourceType + ":" + action }

// Unauthenticated is the 401 problem for requests without a valid session.
func Unauthenticated() *apperrors.Problem {
	return apperrors.NewProblem(http.StatusUnauthorized, "unauthenticated", "Authentication required",
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
