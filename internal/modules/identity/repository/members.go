package repository

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	identitymodel "github.com/nuriansyah/hugimethara/internal/modules/identity/model"
)

type OrgMemberRow struct {
	ID               uuid.UUID `db:"id"`
	Email            string    `db:"email"`
	DisplayName      string    `db:"display_name"`
	Status           string    `db:"status"`
	MembershipStatus string    `db:"membership_status"`
	JoinedAt         time.Time `db:"joined_at"`
	IsOwner          bool      `db:"is_owner"`
	Roles            []byte    `db:"roles"`
	Groups           []byte    `db:"groups"`
}

func (r OrgMemberRow) ToModel() identitymodel.OrgMember {
	m := identitymodel.OrgMember{User: identitymodel.User{ID: r.ID, Email: r.Email, DisplayName: r.DisplayName, Status: r.Status},
		MembershipStatus: r.MembershipStatus, JoinedAt: r.JoinedAt, IsOwner: r.IsOwner, Roles: []string{}, Groups: []identitymodel.GroupRef{}}
	_ = json.Unmarshal(r.Roles, &m.Roles)
	_ = json.Unmarshal(r.Groups, &m.Groups)
	return m
}
