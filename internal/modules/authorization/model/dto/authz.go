package dto

import (
	"github.com/nuriansyah/hugimethara/internal/modules/authorization/model"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

// AuthorizationInput is one access question.
type AuthorizationInput struct {
	Principal principal.Principal
	Action    string
	Resource  model.Resource
	Context   map[string]any
}
