package model

import (
	"time"

	"github.com/google/uuid"
)

// ResourceType is a registered resource kind and its actions.
type ResourceType struct {
	Type             string
	Description      string
	Actions          []string
	SensitiveActions []string
}

// Role is an organization-defined set of permissions.
type Role struct {
	ID           uuid.UUID
	Key          string
	Name         string
	Description  string
	IsSystem     bool
	Immutable    bool
	Permissions  []string
	BindingCount int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Binding grants a role to a principal (user, group or service account) at a scope.
type Binding struct {
	ID             uuid.UUID
	Principal      PrincipalRef
	PrincipalName  string
	PrincipalEmail string // users only
	RoleID         uuid.UUID
	RoleKey        string
	RoleName       string
	Scope          ScopeRef
	WorkspaceSlug  string
	WorkspaceName  string
	CreatedAt      time.Time
	ExpiresAt      *time.Time
}
