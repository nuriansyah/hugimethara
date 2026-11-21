package dto

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/nuriansyah/hugimethara/internal/modules/authorization/model"
	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
)

// RoleInput creates or updates a role (Key is create-only).
type RoleInput struct {
	Key         string
	Name        string
	Description string
	Permissions []string
}

func ValidateRoleText(in *RoleInput) []problem.FieldError {
	in.Name, in.Description = strings.TrimSpace(in.Name), strings.TrimSpace(in.Description)
	var errs []problem.FieldError
	if n := len([]rune(in.Name)); n < 1 || n > 100 {
		errs = append(errs, problem.FieldError{Field: "name", Code: "invalid_length", Message: "Name must be 1–100 characters."})
	}
	if len([]rune(in.Description)) > 500 {
		errs = append(errs, problem.FieldError{Field: "description", Code: "invalid_length", Message: "Description must be at most 500 characters."})
	}
	return errs
}

// BindingFilter narrows ListBindings.
type BindingFilter struct {
	Principal   model.PrincipalRef // zero = everyone; Type alone filters by kind
	ScopeType   string
	WorkspaceID uuid.UUID
}

// BindingInput assigns a role.
type BindingInput struct {
	Principal model.PrincipalRef
	RoleID    uuid.UUID // or RoleKey
	RoleKey   string
	Scope     model.ScopeRef
	ExpiresAt *time.Time
}
