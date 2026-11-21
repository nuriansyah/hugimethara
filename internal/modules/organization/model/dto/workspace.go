package dto

import "github.com/google/uuid"

// WorkspaceInput creates or updates a workspace (Slug is create-only).
type WorkspaceInput struct {
	Slug         string
	Name         string
	Description  string
	DepartmentID *uuid.UUID
}
