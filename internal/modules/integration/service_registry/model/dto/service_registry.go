package dto

import (
	"strings"

	"github.com/google/uuid"

	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
)

type CreateInput struct {
	WorkspaceID uuid.UUID
	Name        string
	Owner       string
	Description string
	Protocol    string
}

type UpdateInput struct {
	Name        *string
	Owner       *string
	Description *string
	Protocol    *string
}

type ImportInput struct {
	Spec         *string
	URL          *string
	VersionLabel string
	DryRun       bool
}

type ServiceTryInput struct {
	EnvironmentID uuid.UUID
	Body          any
	Headers       map[string]string
	PathValues    map[string]string
	Query         map[string]string
}

func ValidateUpdate(in *UpdateInput) error {
	fields := []problem.FieldError{}
	if in.Name != nil {
		*in.Name = strings.TrimSpace(*in.Name)
		if len(*in.Name) < 1 || len(*in.Name) > 100 {
			fields = append(fields, problem.FieldError{Field: "name", Code: "invalid", Message: "Name must be between 1 and 100 characters."})
		}
	}
	if in.Owner != nil {
		*in.Owner = strings.TrimSpace(*in.Owner)
		if len(*in.Owner) > 100 {
			fields = append(fields, problem.FieldError{Field: "owner", Code: "too_long", Message: "Owner must be no more than 100 characters."})
		}
	}
	if in.Description != nil {
		*in.Description = strings.TrimSpace(*in.Description)
		if len(*in.Description) > 1000 {
			fields = append(fields, problem.FieldError{Field: "description", Code: "too_long", Message: "Description must be no more than 1000 characters."})
		}
	}
	if in.Protocol != nil && *in.Protocol != "rest" && *in.Protocol != "graphql" && *in.Protocol != "grpc" {
		fields = append(fields, problem.FieldError{Field: "protocol", Code: "invalid", Message: "Protocol must be rest, graphql, or grpc."})
	}
	if len(fields) != 0 {
		return problem.Validation(fields...)
	}
	return nil
}
