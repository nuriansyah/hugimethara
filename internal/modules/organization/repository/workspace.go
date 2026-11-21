package repository

import (
	"encoding/json"

	"github.com/nuriansyah/hugimethara/internal/modules/organization/model"
)

func (r WorkspaceRow) ToModel() model.Workspace {
	w := model.Workspace{ID: r.ID, Slug: r.Slug, Name: r.Name, Description: r.Description,
		DepartmentName: r.DepartmentName.String, MemberCount: r.MemberCount, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
	if r.DepartmentID.Valid {
		id := r.DepartmentID.UUID
		w.DepartmentID = &id
	}
	if r.ArchivedAt.Valid {
		t := r.ArchivedAt.Time
		w.ArchivedAt = &t
	}
	return w
}

func MembersFrom(rows []MemberRow) []model.Member {
	out := make([]model.Member, len(rows))
	for i, r := range rows {
		m := model.Member{ID: r.ID, Email: r.Email, DisplayName: r.DisplayName, Status: r.Status,
			IsOwner: r.IsOwner, JoinedAt: r.JoinedAt, AddedAt: r.AddedAt, Roles: []string{}}
		_ = json.Unmarshal(r.Roles, &m.Roles)
		out[i] = m
	}
	return out
}
