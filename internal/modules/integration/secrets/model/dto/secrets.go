package dto

import "github.com/google/uuid"

type CreateInput struct {
	Name          string
	Kind          string
	Value         string
	EnvironmentID *uuid.UUID
}

type UpdateInput struct {
	Name  *string
	Value *string
}
