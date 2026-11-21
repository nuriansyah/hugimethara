package repository

import "github.com/nuriansyah/hugimethara/internal/modules/organization/model"

func (r DepartmentRow) ToModel() model.Department {
	return model.Department{ID: r.ID, Slug: r.Slug, Name: r.Name, Description: r.Description,
		WorkspaceCount: r.WorkspaceCount, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}
