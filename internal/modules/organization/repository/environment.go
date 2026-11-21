package repository

import (
	"encoding/json"

	"github.com/nuriansyah/hugimethara/internal/modules/organization/model"
)

func (r EnvironmentRow) ToModel() model.Environment {
	e := model.Environment{ID: r.ID, Slug: r.Slug, Name: r.Name, IsProduction: r.IsProduction,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, EgressAllowlist: []string{}}
	_ = json.Unmarshal(r.EgressAllowlist, &e.EgressAllowlist)
	return e
}
