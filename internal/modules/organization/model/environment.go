package model

import (
	"time"

	"github.com/google/uuid"

	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
)

// Environment is a deployment stage (development, staging, production, …).
type Environment struct {
	ID              uuid.UUID
	Slug            string
	Name            string
	IsProduction    bool
	EgressAllowlist []string // CIDRs allowed through the SSRF guard
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

var ErrEnvironmentNotFound = problem.NotFound("Environment not found.")
