package model

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/google/uuid"

	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
)

// Invitation limits.
const (
	DefaultInvitationDays = 7
	MaxInvitationDays     = 30
	MaxInvitationGrants   = 50
)

// RoleRef names a role.
type RoleRef struct {
	ID   uuid.UUID
	Key  string
	Name string
}

// InvitationWorkspace is a workspace an invitee joins, with their role there.
type InvitationWorkspace struct {
	WorkspaceID   uuid.UUID `json:"workspace_id"`
	WorkspaceName string    `json:"workspace_name"`
	WorkspaceSlug string    `json:"workspace_slug"`
	RoleID        uuid.UUID `json:"role_id"`
	RoleName      string    `json:"role_name"`
}

// Invitation is an invitation to join the organization.
type Invitation struct {
	ID            uuid.UUID
	Email         string
	Status        string // pending | accepted | revoked | expired
	OrgRole       *RoleRef
	Workspaces    []InvitationWorkspace
	InvitedByName string
	CreatedAt     time.Time
	ExpiresAt     time.Time
	AcceptedAt    *time.Time
}

// WorkspaceGrant is a workspace role an invitation hands out.
type WorkspaceGrant struct {
	WorkspaceID uuid.UUID `db:"workspace_id"`
	RoleID      uuid.UUID `db:"role_id"`
}

// AcceptPath is the web route of the invitation page.
func AcceptPath(token string) string { return "/invite/" + token }

// InvitationPreview is what an invitation link shows before accepting.
type InvitationPreview struct {
	Organization  OrgSummary
	Email         string
	InvitedByName string
	ExpiresAt     time.Time
	AccountExists bool
}

type InvitationLookup struct {
	ID          uuid.UUID      `db:"invitation_id"`
	OrgID       uuid.UUID      `db:"org_id"`
	OrgSlug     string         `db:"org_slug"`
	OrgName     string         `db:"org_name"`
	OrgSettings []byte         `db:"org_settings"`
	Email       string         `db:"email"`
	ExpiresAt   time.Time      `db:"expires_at"`
	AcceptedAt  sql.NullTime   `db:"accepted_at"`
	RevokedAt   sql.NullTime   `db:"revoked_at"`
	InviterName sql.NullString `db:"inviter_name"`
	UserExists  bool           `db:"user_exists"`
}

var ErrInvitationNotFound = problem.NotFound("This invitation link is not valid. Ask for a new invitation.")

func invitationGone(code, detail string) *problem.Problem {
	return problem.NewProblem(http.StatusGone, code, "Invitation no longer valid", detail)
}

func (l InvitationLookup) Usable(now time.Time) error {
	switch {
	case l.AcceptedAt.Valid:
		return invitationGone("invitation_accepted", "This invitation has already been used. Sign in instead.")
	case l.RevokedAt.Valid:
		return invitationGone("invitation_revoked", "This invitation was withdrawn. Ask for a new one.")
	case !now.Before(l.ExpiresAt):
		return invitationGone("invitation_expired", "This invitation has expired. Ask for a new one.")
	}
	return nil
}
