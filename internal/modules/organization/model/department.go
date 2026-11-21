package model

import (
	"time"

	"github.com/google/uuid"

	apperrors "github.com/nuriansyah/hugimethara/internal/platform/errors"
)

// Department groups workspaces (e.g. Marketing, Finance).
type Department struct {
	ID             uuid.UUID
	Slug           string
	Name           string
	Description    string
	WorkspaceCount int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

var ErrDepartmentNotFound = apperrors.NotFound("Department not found.")
