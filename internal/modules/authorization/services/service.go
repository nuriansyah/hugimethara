package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	authzmodel "github.com/nuriansyah/hugimethara/internal/modules/authorization/model"
	"github.com/nuriansyah/hugimethara/internal/modules/authorization/model/dto"
	"github.com/nuriansyah/hugimethara/internal/platform/pagination"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

// Code shape: transport adapters and other modules depend on these use-case
// interfaces (ports), never on AuthorizationService directly (DIP); each interface
// groups one capability so consumers can depend on the narrowest one (ISP).

// Service is the complete use-case surface of the module.
type Service interface {
	// BindInTx creates a binding inside the caller's transaction after the
	// no-escalation check, bumping the authz version and auditing. Other domains
	// (workspace membership) call it after their own permission check; they must
	// call Invalidate after commit. Returns uuid.Nil if already bound.
	BindInTx(ctx context.Context, tx *sqlx.Tx, caller principal.Principal, in dto.BindingInput) (uuid.UUID, error)
	// CheckCanHandOut enforces the no-escalation rule for indirect grants: before
	// a user joins a group, the caller must be able to grant every role the group
	// holds, at the scope it holds it.
	CheckCanHandOut(ctx context.Context, tx *sqlx.Tx, caller principal.Principal, target authzmodel.PrincipalRef) error
	// CheckGrantRole loads role id and verifies the caller could grant it at
	// scope (no escalation). Used to validate grants that are applied later, such
	// as the roles an invitation hands out when it is accepted.
	CheckGrantRole(ctx context.Context, tx *sqlx.Tx, caller principal.Principal, roleID uuid.UUID, scope authzmodel.ScopeRef) (authzmodel.Role, error)
	// CreateBinding assigns a role (permission role:bind at the target scope, and
	// no escalation beyond the caller's own permissions there).
	CreateBinding(ctx context.Context, in dto.BindingInput) (authzmodel.Binding, error)
	CreateRole(ctx context.Context, in dto.RoleInput) (authzmodel.Role, error)
	// DeleteBinding removes an assignment (role:bind at its scope). The last
	// organization owner cannot be removed.
	DeleteBinding(ctx context.Context, id uuid.UUID) error
	DeleteRole(ctx context.Context, id uuid.UUID) error
	GetRole(ctx context.Context, id uuid.UUID) (authzmodel.Role, error)
	// Invalidate publishes an authorization change for org (call after commit).
	Invalidate(ctx context.Context, org uuid.UUID)
	ListBindings(ctx context.Context, f dto.BindingFilter) ([]authzmodel.Binding, error)
	// ListDecisions pages through the decision log (authorization:read_decisions).
	ListDecisions(ctx context.Context, allowed *bool, page pagination.Page) ([]authzmodel.Decision, string, error)
	ListRoles(ctx context.Context) ([]authzmodel.Role, error)
	// MyPermissions returns the caller's effective permissions (for UI gating;
	// the API still authorizes every request).
	MyPermissions(ctx context.Context) (authzmodel.Effective, error)
	// RemoveWorkspaceAccessInTx removes all of a user's workspace-scope bindings
	// in a workspace (caller already authorized). Returns how many were removed.
	RemoveWorkspaceAccessInTx(ctx context.Context, tx *sqlx.Tx, org uuid.UUID, workspace uuid.UUID, user uuid.UUID) (int64, error)
	// ResourceTypes lists the permission catalog.
	ResourceTypes(ctx context.Context) ([]authzmodel.ResourceType, error)
	// Simulate answers "could user U do action A on resource R?" without logging
	// the answer as a real decision (permission authorization:simulate).
	Simulate(ctx context.Context, who authzmodel.PrincipalRef, action string, res authzmodel.Resource) (authzmodel.AuthorizationDecision, error)
	UpdateRole(ctx context.Context, id uuid.UUID, in dto.RoleInput) (authzmodel.Role, error)
}

var _ Service = (*AuthorizationService)(nil)
