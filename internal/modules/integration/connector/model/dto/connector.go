package dto

import "github.com/google/uuid"

type CreateInput struct {
	WorkspaceID        uuid.UUID
	EnvironmentID      uuid.UUID
	RefKey             string
	ConnectorVersionID uuid.UUID
	Config             map[string]any
	AuthMethod         string
	SecretID           *uuid.UUID
	RateLimit          int
	TimeoutMs          int
}

type UpdateInput struct {
	Config     *map[string]any
	AuthMethod *string
	SecretID   *uuid.UUID
	RateLimit  *int
	TimeoutMs  *int
	Status     *string
}
