package authz

import (
	"database/sql"
	"embed"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/nuriansyah/hugimethara/internal/platform/database"
)

//go:embed queries.sql
var queryFS embed.FS

var q = db.MustLoad(queryFS, "queries.sql")

type grantRow struct {
	Permission    string         `db:"permission"`
	ScopeType     string         `db:"scope_type"`
	WorkspaceID   uuid.NullUUID  `db:"workspace_id"`
	ResourceType  sql.NullString `db:"resource_type"`
	ResourceID    sql.NullString `db:"resource_id"`
	RoleKey       string         `db:"role_key"`
	BindingID     uuid.UUID      `db:"binding_id"`
	PrincipalType string         `db:"principal_type"`
}

func (r grantRow) toGrant() Grant {
	return Grant{Permission: r.Permission, RoleKey: r.RoleKey, BindingID: r.BindingID, Via: r.PrincipalType, Scope: ScopeRef{
		Type: r.ScopeType, WorkspaceID: r.WorkspaceID.UUID, ResourceType: r.ResourceType.String, ResourceID: r.ResourceID.String,
	}}
}

type resourceTypeRow struct {
	Type             string `db:"type"`
	Description      string `db:"description"`
	Actions          []byte `db:"actions"`           // JSON array
	SensitiveActions []byte `db:"sensitive_actions"` // JSON array
}

// ResourceType is a registered resource kind and its actions.
type ResourceType struct {
	Type             string
	Description      string
	Actions          []string
	SensitiveActions []string
}

func (r resourceTypeRow) toModel() ResourceType {
	m := ResourceType{Type: r.Type, Description: r.Description}
	_ = json.Unmarshal(r.Actions, &m.Actions)
	_ = json.Unmarshal(r.SensitiveActions, &m.SensitiveActions)
	return m
}

type roleRow struct {
	ID           uuid.UUID `db:"id"`
	Key          string    `db:"key"`
	Name         string    `db:"name"`
	Description  string    `db:"description"`
	IsSystem     bool      `db:"is_system"`
	Immutable    bool      `db:"immutable"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
	Permissions  []byte    `db:"permissions"` // JSON array
	BindingCount int64     `db:"binding_count"`
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

func (r roleRow) toModel() Role {
	m := Role{ID: r.ID, Key: r.Key, Name: r.Name, Description: r.Description, IsSystem: r.IsSystem,
		Immutable: r.Immutable, BindingCount: r.BindingCount, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Permissions: []string{}}
	_ = json.Unmarshal(r.Permissions, &m.Permissions)
	return m
}

type bindingRow struct {
	ID             uuid.UUID      `db:"id"`
	PrincipalType  string         `db:"principal_type"`
	PrincipalID    uuid.UUID      `db:"principal_id"`
	PrincipalName  string         `db:"principal_name"`
	PrincipalEmail sql.NullString `db:"principal_email"`
	RoleID         uuid.UUID      `db:"role_id"`
	RoleKey        string         `db:"role_key"`
	RoleName       string         `db:"role_name"`
	ScopeType      string         `db:"scope_type"`
	WorkspaceID    uuid.NullUUID  `db:"workspace_id"`
	WorkspaceSlug  sql.NullString `db:"workspace_slug"`
	WorkspaceName  sql.NullString `db:"workspace_name"`
	ResourceType   sql.NullString `db:"resource_type"`
	ResourceID     sql.NullString `db:"resource_id"`
	CreatedAt      time.Time      `db:"created_at"`
	ExpiresAt      sql.NullTime   `db:"expires_at"`
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

func (r bindingRow) toModel() Binding {
	b := Binding{ID: r.ID, Principal: PrincipalRef{Type: r.PrincipalType, ID: r.PrincipalID}, PrincipalName: r.PrincipalName,
		PrincipalEmail: r.PrincipalEmail.String, RoleID: r.RoleID, RoleKey: r.RoleKey, RoleName: r.RoleName,
		WorkspaceSlug: r.WorkspaceSlug.String, WorkspaceName: r.WorkspaceName.String,
		CreatedAt: r.CreatedAt, Scope: ScopeRef{Type: r.ScopeType, WorkspaceID: r.WorkspaceID.UUID,
			ResourceType: r.ResourceType.String, ResourceID: r.ResourceID.String}}
	if r.ExpiresAt.Valid {
		t := r.ExpiresAt.Time
		b.ExpiresAt = &t
	}
	return b
}
