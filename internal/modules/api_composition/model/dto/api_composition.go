package dto

import "github.com/google/uuid"

type CreateInput struct {
	WorkspaceID uuid.UUID
	Name        string
	Slug        string
	Draft       map[string]any
}

type UpdateInput struct {
	Name    *string
	Slug    *string
	Draft   *map[string]any
	Exposed *bool
}

type ExecuteInput struct {
	EnvironmentID uuid.UUID      `json:"environment_id"`
	Request       map[string]any `json:"request"`
	RequestID     string
}
