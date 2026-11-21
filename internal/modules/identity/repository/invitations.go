package repository

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	identitymodel "github.com/nuriansyah/hugimethara/internal/modules/identity/model"
)

type InvitationRow struct {
	ID            uuid.UUID      `db:"id"`
	Email         string         `db:"email"`
	CreatedAt     time.Time      `db:"created_at"`
	ExpiresAt     time.Time      `db:"expires_at"`
	AcceptedAt    sql.NullTime   `db:"accepted_at"`
	Status        string         `db:"status"`
	OrgRoleID     uuid.NullUUID  `db:"org_role_id"`
	OrgRoleKey    sql.NullString `db:"org_role_key"`
	OrgRoleName   sql.NullString `db:"org_role_name"`
	InvitedByName sql.NullString `db:"invited_by_name"`
	Workspaces    []byte         `db:"workspaces"`
}

func (r InvitationRow) ToModel() identitymodel.Invitation {
	inv := identitymodel.Invitation{ID: r.ID, Email: r.Email, Status: r.Status, InvitedByName: r.InvitedByName.String,
		CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt, Workspaces: []identitymodel.InvitationWorkspace{}}
	if r.OrgRoleID.Valid {
		inv.OrgRole = &identitymodel.RoleRef{ID: r.OrgRoleID.UUID, Key: r.OrgRoleKey.String, Name: r.OrgRoleName.String}
	}
	if r.AcceptedAt.Valid {
		t := r.AcceptedAt.Time
		inv.AcceptedAt = &t
	}
	_ = json.Unmarshal(r.Workspaces, &inv.Workspaces)
	return inv
}
