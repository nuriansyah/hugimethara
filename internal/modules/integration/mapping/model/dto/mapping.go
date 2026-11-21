package dto

import "github.com/google/uuid"

type CreateInput struct {
	WorkspaceID  uuid.UUID
	Name         string
	Draft        map[string]any
	InputSchema  map[string]any
	OutputSchema map[string]any
}

type UpdateInput struct {
	Name         *string
	Draft        *map[string]any
	InputSchema  *map[string]any
	OutputSchema *map[string]any
}

type TestCreateInput struct {
	Name     string
	Input    any
	Expected any
}

type TestUpdateInput struct {
	Name     *string
	Input    any
	Expected any
}
