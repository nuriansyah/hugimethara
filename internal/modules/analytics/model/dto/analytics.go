// Package dto contains transport-agnostic use-case inputs for analytics.
package dto

import (
	"github.com/google/uuid"

	model "github.com/nuriansyah/hugimethara/internal/modules/analytics/model"
)

type CreateSemanticModelInput struct {
	WorkspaceID uuid.UUID
	DatasetID   uuid.UUID
	Name        string
	Slug        string
	Description string
	Dimensions  []model.Dimension
}

type UpdateSemanticModelInput struct {
	Name        *string
	Slug        *string
	Description *string
	Dimensions  *[]model.Dimension
}

type CreateMetricInput struct {
	WorkspaceID     uuid.UUID
	SemanticModelID uuid.UUID
	Name            string
	Slug            string
	Description     string
	Expression      string
	Filters         []string
	Format          string
}

type UpdateMetricInput struct {
	Name        *string
	Slug        *string
	Description *string
	Expression  *string
	Filters     *[]string
	Format      *string
}

type QueryInput struct {
	WorkspaceID uuid.UUID
	DatasetID   *uuid.UUID
	SQL         string
	Limit       int
	Export      bool
}

type MetricQueryInput struct {
	Filters []string
	Limit   int
}
