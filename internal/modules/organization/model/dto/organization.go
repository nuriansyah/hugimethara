package dto

import (
	"github.com/google/uuid"

	"github.com/nuriansyah/hugimethara/internal/modules/organization/model"
)

// NewOrganization is the input to Create.
type NewOrganization struct {
	ID   uuid.UUID
	Slug string
	Name string
}

// Update is a partial update; nil fields are left unchanged.
type Update struct {
	Name     *string
	Settings *model.Settings
}
