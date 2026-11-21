package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/modules/organization/model"
	"github.com/nuriansyah/hugimethara/internal/modules/organization/model/dto"
)

// Code shape: transport adapters and other modules depend on these use-case
// interfaces (ports), never on OrganizationService directly (DIP); each interface
// groups one capability so consumers can depend on the narrowest one (ISP).

// DepartmentUseCases is the department capability of the module.
type DepartmentUseCases interface {
	CreateDepartment(ctx context.Context, in dto.DepartmentInput) (model.Department, error)
	// DeleteDepartment removes a department; its workspaces become unassigned.
	DeleteDepartment(ctx context.Context, id uuid.UUID) error
	ListDepartments(ctx context.Context) ([]model.Department, error)
	UpdateDepartment(ctx context.Context, id uuid.UUID, in dto.DepartmentInput) (model.Department, error)
}

// EnvironmentUseCases is the environment capability of the module.
type EnvironmentUseCases interface {
	CreateEnvironment(ctx context.Context, in dto.EnvironmentInput) (model.Environment, error)
	// DeleteEnvironment removes an environment. The last one cannot be deleted;
	// later slices add foreign keys that block deleting environments in use.
	DeleteEnvironment(ctx context.Context, id uuid.UUID) error
	ListEnvironments(ctx context.Context) ([]model.Environment, error)
	UpdateEnvironment(ctx context.Context, id uuid.UUID, in dto.EnvironmentInput) (model.Environment, error)
}

// WorkspaceUseCases is the workspace capability of the module.
type WorkspaceUseCases interface {
	// AddWorkspaceMember gives an organization member a role in a workspace
	// (idempotent). Requires workspace:manage_members, and the caller cannot grant
	// a role with permissions they do not hold in that workspace.
	AddWorkspaceMember(ctx context.Context, slug string, userID uuid.UUID, roleKey string) error
	// CreateWorkspace creates a workspace and makes the creator its first member.
	CreateWorkspace(ctx context.Context, in dto.WorkspaceInput) (model.Workspace, error)
	// GetWorkspace returns a workspace by slug.
	GetWorkspace(ctx context.Context, slug string) (model.Workspace, error)
	// ListWorkspaceMembers lists members of a workspace the caller can read.
	ListWorkspaceMembers(ctx context.Context, slug string) ([]model.Member, error)
	// ListWorkspaces returns the workspaces the caller may see: all of them for
	// principals with org-wide scope, otherwise only those they belong to.
	ListWorkspaces(ctx context.Context, includeArchived bool) ([]model.Workspace, error)
	// RemoveWorkspaceMember removes all of a user's roles in a workspace.
	RemoveWorkspaceMember(ctx context.Context, slug string, userID uuid.UUID) error
	// SetWorkspaceArchived archives (true) or restores (false) a workspace.
	// Restoring counts against the workspace quota again.
	SetWorkspaceArchived(ctx context.Context, slug string, archived bool) (model.Workspace, error)
	// UpdateWorkspace changes name, description and department.
	UpdateWorkspace(ctx context.Context, slug string, in dto.WorkspaceInput) (model.Workspace, error)
}

// Service is the complete use-case surface of the module.
type Service interface {
	DepartmentUseCases
	EnvironmentUseCases
	WorkspaceUseCases
	// Create inserts an organization and its default environments inside tx
	// (which must be bound to o.ID). Validation is the caller's responsibility
	// (see identity signup).
	Create(ctx context.Context, tx *sqlx.Tx, o dto.NewOrganization) error
	// Current returns the caller's organization.
	Current(ctx context.Context) (model.Organization, error)
	// UpdateCurrent applies u to the caller's organization, recording an audit event
	// and an outbox event in the same transaction.
	UpdateCurrent(ctx context.Context, u dto.Update) (model.Organization, error)
}

var _ Service = (*OrganizationService)(nil)
