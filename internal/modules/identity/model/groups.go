package model

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
)

// Group is a set of members that roles can be assigned to.
type Group struct {
	ID          uuid.UUID `db:"id"`
	Name        string    `db:"name"`
	Description string    `db:"description"`
	Source      string    `db:"source"` // internal | idp (managed by an identity provider, S5)
	MemberCount int64     `db:"member_count"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// GroupMember is one member of a group.
type GroupMember struct {
	ID          uuid.UUID `db:"id"`
	Email       string    `db:"email"`
	DisplayName string    `db:"display_name"`
	AddedAt     time.Time `db:"added_at"`
}

var (
	ErrGroupNotFound = problem.NotFound("Group not found.")
	ErrGroupNameTake = problem.NewProblem(http.StatusConflict, "name_taken", "Conflict", "A group with this name already exists.")
	ErrGroupIdP      = problem.NewProblem(http.StatusConflict, "group_managed_by_idp", "Conflict",
		"This group's members come from your identity provider. Change them there.")
)

func ValidateNameDesc(name, desc *string, label string) error {
	*name, *desc = strings.TrimSpace(*name), strings.TrimSpace(*desc)
	var errs []problem.FieldError
	if n := len([]rune(*name)); n < 1 || n > 100 {
		errs = append(errs, problem.FieldError{Field: "name", Code: "invalid_length", Message: label + " name must be 1–100 characters."})
	}
	if len([]rune(*desc)) > 500 {
		errs = append(errs, problem.FieldError{Field: "description", Code: "invalid_length", Message: "Description must be at most 500 characters."})
	}
	if len(errs) > 0 {
		return problem.Validation(errs...)
	}
	return nil
}
