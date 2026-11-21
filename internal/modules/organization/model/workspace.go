package model

import (
	"time"

	"github.com/google/uuid"

	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
)

// Workspace is a team space inside an organization.
type Workspace struct {
	ID             uuid.UUID
	Slug           string
	Name           string
	Description    string
	DepartmentID   *uuid.UUID
	DepartmentName string
	ArchivedAt     *time.Time
	MemberCount    int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Member is an organization member as seen from a workspace or the org.
type Member struct {
	ID          uuid.UUID
	Email       string
	DisplayName string
	Status      string
	IsOwner     bool
	Roles       []string  // org-level roles (org list) or workspace roles (workspace list)
	JoinedAt    time.Time // org memberships
	AddedAt     time.Time // workspace memberships
}

var ErrWorkspaceNotFound = problem.NotFound("Workspace not found.")

func Nullable(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return *id
}

// DefaultWorkspaceRole is assigned when adding a member without a role.
const DefaultWorkspaceRole = "editor"
