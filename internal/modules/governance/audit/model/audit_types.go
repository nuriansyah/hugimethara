package model

import (
	"time"

	"github.com/google/uuid"
)

// ResourceType is the authz resource type for audit events.
const ResourceType = "audit_event"

// Entry is one listed event.
type Entry struct {
	ID           uuid.UUID
	OccurredAt   time.Time
	ActorType    string
	ActorID      *uuid.UUID
	ActorEmail   string
	Action       string
	ResourceType string
	ResourceID   string
	WorkspaceID  *uuid.UUID
	Outcome      string
	RequestID    string
	IP           string
	Details      map[string]any
}

type Cursor struct {
	T  time.Time `json:"t"`
	ID uuid.UUID `json:"id"`
}
