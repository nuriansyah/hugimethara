// Package dto contains transport-agnostic use-case inputs for dashboards.
package dto

import (
	"time"

	"github.com/google/uuid"

	model "github.com/nuriansyah/hugimethara/internal/modules/dashboard/model"
)

type CreateDashboardInput struct {
	WorkspaceID uuid.UUID
	Name        string
	Slug        string
	Description string
	Definition  model.Definition
}

type UpdateDashboardInput struct {
	Name        *string
	Slug        *string
	Description *string
	Definition  *model.Definition
}

type ShareInput struct {
	Enabled   bool
	ExpiresAt *time.Time
}

type CreateReportInput struct {
	WorkspaceID  uuid.UUID
	DashboardID  uuid.UUID
	Name         string
	ScheduleCron string
	Recipients   []string
	Format       string
	Enabled      *bool
}

type UpdateReportInput struct {
	Name         *string
	ScheduleCron *string
	Recipients   *[]string
	Format       *string
	Enabled      *bool
}
