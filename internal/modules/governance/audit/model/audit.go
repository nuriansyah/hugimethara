package model

import (
	"context"

	"github.com/google/uuid"

	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

// Outcomes.
const (
	Success = "success"
	Failure = "failure"
	Denied  = "denied"
)

// Actor overrides the actor derived from the request principal (e.g. during
// signup, before a principal exists).
type Actor struct {
	Type string // user | service_account | system | anonymous
	ID   uuid.UUID
}

// Event is one audit record.
type Event struct {
	OrgID        uuid.UUID
	Action       string // e.g. "organization.updated"
	ResourceType string
	ResourceID   string
	WorkspaceID  *uuid.UUID
	Outcome      string         // default Success
	Details      map[string]any // never secrets; sensitive-looking keys are redacted
	Actor        *Actor
}

func ResolveActor(ctx context.Context, override *Actor) Actor {
	if override != nil {
		return *override
	}
	if p, ok := principal.From(ctx); ok {
		return Actor{Type: string(p.Type), ID: p.ID}
	}
	return Actor{Type: "anonymous"}
}
