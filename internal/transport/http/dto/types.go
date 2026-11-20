package dto

import (
	"encoding/json"

	"fmt"

	"bytes"
	uuid "github.com/google/uuid"
	"net/http"
	"time"
)

const (
	AuditEventActorTypeAnonymous      AuditEventActorType = "anonymous"
	AuditEventActorTypeServiceAccount AuditEventActorType = "service_account"
	AuditEventActorTypeSystem         AuditEventActorType = "system"
	AuditEventActorTypeUser           AuditEventActorType = "user"
)

func (e AuditEventActorType) Valid() bool {
	switch e {
	case AuditEventActorTypeAnonymous:
		return true
	case AuditEventActorTypeServiceAccount:
		return true
	case AuditEventActorTypeSystem:
		return true
	case AuditEventActorTypeUser:
		return true
	default:
		return false
	}
}

const (
	Denied  AuditEventOutcome = "denied"
	Failure AuditEventOutcome = "failure"
	Success AuditEventOutcome = "success"
)

func (e AuditEventOutcome) Valid() bool {
	switch e {
	case Denied:
		return true
	case Failure:
		return true
	case Success:
		return true
	default:
		return false
	}
}

const (
	BindingScopeTypeOrg       BindingScopeType = "org"
	BindingScopeTypeResource  BindingScopeType = "resource"
	BindingScopeTypeWorkspace BindingScopeType = "workspace"
)

func (e BindingScopeType) Valid() bool {
	switch e {
	case BindingScopeTypeOrg:
		return true
	case BindingScopeTypeResource:
		return true
	case BindingScopeTypeWorkspace:
		return true
	default:
		return false
	}
}

const (
	Builtin     ConnectorDefinitionTier = "builtin"
	Declarative ConnectorDefinitionTier = "declarative"
)

func (e ConnectorDefinitionTier) Valid() bool {
	switch e {
	case Builtin:
		return true
	case Declarative:
		return true
	default:
		return false
	}
}

const (
	ConnectorInstanceStatusDisabled ConnectorInstanceStatus = "disabled"
	ConnectorInstanceStatusEnabled  ConnectorInstanceStatus = "enabled"
)

func (e ConnectorInstanceStatus) Valid() bool {
	switch e {
	case ConnectorInstanceStatusDisabled:
		return true
	case ConnectorInstanceStatusEnabled:
		return true
	default:
		return false
	}
}

const (
	ConnectorInstanceUpdateStatusDisabled ConnectorInstanceUpdateStatus = "disabled"
	ConnectorInstanceUpdateStatusEnabled  ConnectorInstanceUpdateStatus = "enabled"
)

func (e ConnectorInstanceUpdateStatus) Valid() bool {
	switch e {
	case ConnectorInstanceUpdateStatusDisabled:
		return true
	case ConnectorInstanceUpdateStatusEnabled:
		return true
	default:
		return false
	}
}

const (
	EntitlementFeatureKindFlag  EntitlementFeatureKind = "flag"
	EntitlementFeatureKindLimit EntitlementFeatureKind = "limit"
)

func (e EntitlementFeatureKind) Valid() bool {
	switch e {
	case EntitlementFeatureKindFlag:
		return true
	case EntitlementFeatureKindLimit:
		return true
	default:
		return false
	}
}

const (
	EntitlementsStatusActive   EntitlementsStatus = "active"
	EntitlementsStatusCanceled EntitlementsStatus = "canceled"
	EntitlementsStatusPastDue  EntitlementsStatus = "past_due"
	EntitlementsStatusTrialing EntitlementsStatus = "trialing"
)

func (e EntitlementsStatus) Valid() bool {
	switch e {
	case EntitlementsStatusActive:
		return true
	case EntitlementsStatusCanceled:
		return true
	case EntitlementsStatusPastDue:
		return true
	case EntitlementsStatusTrialing:
		return true
	default:
		return false
	}
}

const (
	FeatureDefinitionKindFlag  FeatureDefinitionKind = "flag"
	FeatureDefinitionKindLimit FeatureDefinitionKind = "limit"
)

func (e FeatureDefinitionKind) Valid() bool {
	switch e {
	case FeatureDefinitionKindFlag:
		return true
	case FeatureDefinitionKindLimit:
		return true
	default:
		return false
	}
}

const (
	GroupSourceIdp      GroupSource = "idp"
	GroupSourceInternal GroupSource = "internal"
)

func (e GroupSource) Valid() bool {
	switch e {
	case GroupSourceIdp:
		return true
	case GroupSourceInternal:
		return true
	default:
		return false
	}
}

const (
	GroupMappingGroupSourceIdp      GroupMappingGroupSource = "idp"
	GroupMappingGroupSourceInternal GroupMappingGroupSource = "internal"
)

func (e GroupMappingGroupSource) Valid() bool {
	switch e {
	case GroupMappingGroupSourceIdp:
		return true
	case GroupMappingGroupSourceInternal:
		return true
	default:
		return false
	}
}

const (
	HealthReportChecksStatusFail HealthReportChecksStatus = "fail"
	HealthReportChecksStatusOk   HealthReportChecksStatus = "ok"
)

func (e HealthReportChecksStatus) Valid() bool {
	switch e {
	case HealthReportChecksStatusFail:
		return true
	case HealthReportChecksStatusOk:
		return true
	default:
		return false
	}
}

const (
	HealthReportStatusFail HealthReportStatus = "fail"
	HealthReportStatusOk   HealthReportStatus = "ok"
)

func (e HealthReportStatus) Valid() bool {
	switch e {
	case HealthReportStatusFail:
		return true
	case HealthReportStatusOk:
		return true
	default:
		return false
	}
}

const (
	Oidc IdentityProviderProtocol = "oidc"
)

func (e IdentityProviderProtocol) Valid() bool {
	switch e {
	case Oidc:
		return true
	default:
		return false
	}
}

const (
	Entra    IdentityProviderPreset = "entra"
	Generic  IdentityProviderPreset = "generic"
	Google   IdentityProviderPreset = "google"
	Keycloak IdentityProviderPreset = "keycloak"
)

func (e IdentityProviderPreset) Valid() bool {
	switch e {
	case Entra:
		return true
	case Generic:
		return true
	case Google:
		return true
	case Keycloak:
		return true
	default:
		return false
	}
}

const (
	InvitationStatusAccepted InvitationStatus = "accepted"
	InvitationStatusExpired  InvitationStatus = "expired"
	InvitationStatusPending  InvitationStatus = "pending"
	InvitationStatusRevoked  InvitationStatus = "revoked"
)

func (e InvitationStatus) Valid() bool {
	switch e {
	case InvitationStatusAccepted:
		return true
	case InvitationStatusExpired:
		return true
	case InvitationStatusPending:
		return true
	case InvitationStatusRevoked:
		return true
	default:
		return false
	}
}

const (
	MemberMembershipStatusActive    MemberMembershipStatus = "active"
	MemberMembershipStatusSuspended MemberMembershipStatus = "suspended"
)

func (e MemberMembershipStatus) Valid() bool {
	switch e {
	case MemberMembershipStatusActive:
		return true
	case MemberMembershipStatusSuspended:
		return true
	default:
		return false
	}
}

const (
	Added   OpenAPIChangeKind = "added"
	Changed OpenAPIChangeKind = "changed"
	Removed OpenAPIChangeKind = "removed"
)

func (e OpenAPIChangeKind) Valid() bool {
	switch e {
	case Added:
		return true
	case Changed:
		return true
	case Removed:
		return true
	default:
		return false
	}
}

const (
	OrganizationStatusActive    OrganizationStatus = "active"
	OrganizationStatusSuspended OrganizationStatus = "suspended"
)

func (e OrganizationStatus) Valid() bool {
	switch e {
	case OrganizationStatusActive:
		return true
	case OrganizationStatusSuspended:
		return true
	default:
		return false
	}
}

const (
	PrincipalTypeGroup          PrincipalType = "group"
	PrincipalTypeServiceAccount PrincipalType = "service_account"
	PrincipalTypeUser           PrincipalType = "user"
)

func (e PrincipalType) Valid() bool {
	switch e {
	case PrincipalTypeGroup:
		return true
	case PrincipalTypeServiceAccount:
		return true
	case PrincipalTypeUser:
		return true
	default:
		return false
	}
}

const (
	ServiceProtocolGraphql ServiceProtocol = "graphql"
	ServiceProtocolGrpc    ServiceProtocol = "grpc"
	ServiceProtocolRest    ServiceProtocol = "rest"
)

func (e ServiceProtocol) Valid() bool {
	switch e {
	case ServiceProtocolGraphql:
		return true
	case ServiceProtocolGrpc:
		return true
	case ServiceProtocolRest:
		return true
	default:
		return false
	}
}

const (
	ServiceCreateProtocolGraphql ServiceCreateProtocol = "graphql"
	ServiceCreateProtocolGrpc    ServiceCreateProtocol = "grpc"
	ServiceCreateProtocolRest    ServiceCreateProtocol = "rest"
)

func (e ServiceCreateProtocol) Valid() bool {
	switch e {
	case ServiceCreateProtocolGraphql:
		return true
	case ServiceCreateProtocolGrpc:
		return true
	case ServiceCreateProtocolRest:
		return true
	default:
		return false
	}
}

const (
	ServiceUpdateProtocolGraphql ServiceUpdateProtocol = "graphql"
	ServiceUpdateProtocolGrpc    ServiceUpdateProtocol = "grpc"
	ServiceUpdateProtocolRest    ServiceUpdateProtocol = "rest"
)

func (e ServiceUpdateProtocol) Valid() bool {
	switch e {
	case ServiceUpdateProtocolGraphql:
		return true
	case ServiceUpdateProtocolGrpc:
		return true
	case ServiceUpdateProtocolRest:
		return true
	default:
		return false
	}
}

const (
	Development SystemInfoEnvironment = "development"
	Production  SystemInfoEnvironment = "production"
	Staging     SystemInfoEnvironment = "staging"
	Test        SystemInfoEnvironment = "test"
)

func (e SystemInfoEnvironment) Valid() bool {
	switch e {
	case Development:
		return true
	case Production:
		return true
	case Staging:
		return true
	case Test:
		return true
	default:
		return false
	}
}

const (
	UserStatusActive   UserStatus = "active"
	UserStatusDisabled UserStatus = "disabled"
)

func (e UserStatus) Valid() bool {
	switch e {
	case UserStatusActive:
		return true
	case UserStatusDisabled:
		return true
	default:
		return false
	}
}

const (
	UserUpdateStatusActive    UserUpdateStatus = "active"
	UserUpdateStatusSuspended UserUpdateStatus = "suspended"
)

func (e UserUpdateStatus) Valid() bool {
	switch e {
	case UserUpdateStatusActive:
		return true
	case UserUpdateStatusSuspended:
		return true
	default:
		return false
	}
}

const (
	ListInvitationsParamsStatusAll     ListInvitationsParamsStatus = "all"
	ListInvitationsParamsStatusPending ListInvitationsParamsStatus = "pending"
)

func (e ListInvitationsParamsStatus) Valid() bool {
	switch e {
	case ListInvitationsParamsStatusAll:
		return true
	case ListInvitationsParamsStatusPending:
		return true
	default:
		return false
	}
}

const (
	ListRoleBindingsParamsPrincipalTypeGroup          ListRoleBindingsParamsPrincipalType = "group"
	ListRoleBindingsParamsPrincipalTypeServiceAccount ListRoleBindingsParamsPrincipalType = "service_account"
	ListRoleBindingsParamsPrincipalTypeUser           ListRoleBindingsParamsPrincipalType = "user"
)

func (e ListRoleBindingsParamsPrincipalType) Valid() bool {
	switch e {
	case ListRoleBindingsParamsPrincipalTypeGroup:
		return true
	case ListRoleBindingsParamsPrincipalTypeServiceAccount:
		return true
	case ListRoleBindingsParamsPrincipalTypeUser:
		return true
	default:
		return false
	}
}

const (
	ListRoleBindingsParamsScopeTypeOrg       ListRoleBindingsParamsScopeType = "org"
	ListRoleBindingsParamsScopeTypeResource  ListRoleBindingsParamsScopeType = "resource"
	ListRoleBindingsParamsScopeTypeWorkspace ListRoleBindingsParamsScopeType = "workspace"
)

func (e ListRoleBindingsParamsScopeType) Valid() bool {
	switch e {
	case ListRoleBindingsParamsScopeTypeOrg:
		return true
	case ListRoleBindingsParamsScopeTypeResource:
		return true
	case ListRoleBindingsParamsScopeTypeWorkspace:
		return true
	default:
		return false
	}
}

type AccessDecision struct {
	Allowed       bool     `json:"allowed" binding:"required"`
	MatchedRules  []string `json:"matched_rules" binding:"required"`
	PolicyVersion int64    `json:"policy_version" binding:"required"`
	Reason        string   `json:"reason" binding:"required"`
}

type ApiKey struct {
	CreatedAt  time.Time  `json:"created_at" binding:"required"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	Id         uuid.UUID  `json:"id" binding:"required"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	Name       string     `json:"name" binding:"required"`

	Prefix    string     `json:"prefix" binding:"required"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

type ApiKeyCreate struct {
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Name      string     `json:"name" binding:"required"`
}

type ApiKeyCreated struct {
	Key ApiKey `json:"key" binding:"required"`

	Secret string `json:"secret" binding:"required"`
}

// AuditEvent is a record in the organization audit trail.
type AuditEvent struct {
	// Action is the exact event action, for example organization.updated.
	Action       string                 `json:"action" binding:"required"`
	ActorEmail   *string                `json:"actor_email,omitempty"`
	ActorId      *uuid.UUID             `json:"actor_id,omitempty"`
	ActorType    AuditEventActorType    `json:"actor_type" binding:"required"`
	Details      map[string]interface{} `json:"details" binding:"required"`
	Id           uuid.UUID              `json:"id" binding:"required"`
	Ip           *string                `json:"ip,omitempty"`
	OccurredAt   time.Time              `json:"occurred_at" binding:"required"`
	Outcome      AuditEventOutcome      `json:"outcome" binding:"required"`
	RequestId    *string                `json:"request_id,omitempty"`
	ResourceId   *string                `json:"resource_id,omitempty"`
	ResourceType string                 `json:"resource_type" binding:"required"`
	WorkspaceId  *uuid.UUID             `json:"workspace_id,omitempty"`
}

// AuditEventActorType identifies who or what caused an audit event.
type AuditEventActorType string

// AuditEventOutcome describes the result of an audit event.
type AuditEventOutcome string

// AuditEventPage is one page of audit events.
type AuditEventPage struct {
	Data       []AuditEvent `json:"data" binding:"required"`
	NextCursor *string      `json:"next_cursor,omitempty"`
}

type AuthResponse struct {
	Organization OrganizationSummary `json:"organization" binding:"required"`
	Session      SessionSummary      `json:"session" binding:"required"`
	User         User                `json:"user" binding:"required"`
}

type BindingScope struct {
	ResourceId   *string          `json:"resource_id,omitempty"`
	ResourceType *string          `json:"resource_type,omitempty"`
	Type         BindingScopeType `json:"type" binding:"required"`
	WorkspaceId  *uuid.UUID       `json:"workspace_id,omitempty"`
}

type BindingScopeType string

type ClaimMapping struct {
	Email string `json:"email" binding:"required"`

	Groups *string `json:"groups,omitempty"`

	Name string `json:"name" binding:"required"`
}

type ConnectorDefinition struct {
	Category string                  `json:"category" binding:"required"`
	Icon     string                  `json:"icon" binding:"required"`
	Id       uuid.UUID               `json:"id" binding:"required"`
	Key      string                  `json:"key" binding:"required"`
	Name     string                  `json:"name" binding:"required"`
	Tier     ConnectorDefinitionTier `json:"tier" binding:"required"`
}

type ConnectorDefinitionTier string

type ConnectorInstance struct {
	AuthMethod           string                  `json:"auth_method" binding:"required"`
	Config               map[string]interface{}  `json:"config" binding:"required"`
	CreatedAt            time.Time               `json:"created_at" binding:"required"`
	CredentialConfigured bool                    `json:"credential_configured" binding:"required"`
	DefinitionId         uuid.UUID               `json:"definition_id" binding:"required"`
	DefinitionKey        string                  `json:"definition_key" binding:"required"`
	EnvironmentId        uuid.UUID               `json:"environment_id" binding:"required"`
	Id                   uuid.UUID               `json:"id" binding:"required"`
	Name                 string                  `json:"name" binding:"required"`
	RateLimit            int                     `json:"rate_limit" binding:"required"`
	RefKey               string                  `json:"ref_key" binding:"required"`
	SecretId             *uuid.UUID              `json:"secret_id,omitempty"`
	Status               ConnectorInstanceStatus `json:"status" binding:"required"`
	TimeoutMs            int                     `json:"timeout_ms" binding:"required"`
	UpdatedAt            time.Time               `json:"updated_at" binding:"required"`
	Version              string                  `json:"version" binding:"required"`
	WorkspaceId          uuid.UUID               `json:"workspace_id" binding:"required"`
}

type ConnectorInstanceStatus string

type ConnectorInstanceCreate struct {
	AuthMethod         *string                `json:"auth_method,omitempty"`
	Config             map[string]interface{} `json:"config" binding:"required"`
	ConnectorVersionId uuid.UUID              `json:"connector_version_id" binding:"required"`
	EnvironmentId      uuid.UUID              `json:"environment_id" binding:"required"`
	RateLimit          *int                   `json:"rate_limit,omitempty"`
	RefKey             string                 `json:"ref_key" binding:"required"`
	SecretId           *uuid.UUID             `json:"secret_id,omitempty"`
	TimeoutMs          *int                   `json:"timeout_ms,omitempty"`
	WorkspaceId        uuid.UUID              `json:"workspace_id" binding:"required"`
}

type ConnectorInstanceUpdate struct {
	AuthMethod *string                        `json:"auth_method,omitempty"`
	Config     *map[string]interface{}        `json:"config,omitempty"`
	RateLimit  *int                           `json:"rate_limit,omitempty"`
	SecretId   *uuid.UUID                     `json:"secret_id,omitempty"`
	Status     *ConnectorInstanceUpdateStatus `json:"status,omitempty"`
	TimeoutMs  *int                           `json:"timeout_ms,omitempty"`
}

type ConnectorInstanceUpdateStatus string

type ConnectorTestResult struct {
	CheckedAt time.Time `json:"checked_at" binding:"required"`
	ErrorCode *string   `json:"error_code,omitempty"`
	LatencyMs int       `json:"latency_ms" binding:"required"`
	Message   string    `json:"message" binding:"required"`
	Ok        bool      `json:"ok" binding:"required"`
}

type ConnectorVersion struct {
	AuthMethods  []string                 `json:"auth_methods" binding:"required"`
	ConfigSchema map[string]interface{}   `json:"config_schema" binding:"required"`
	DefinitionId uuid.UUID                `json:"definition_id" binding:"required"`
	DeprecatedAt *time.Time               `json:"deprecated_at,omitempty"`
	Id           uuid.UUID                `json:"id" binding:"required"`
	Operations   []map[string]interface{} `json:"operations" binding:"required"`
	Version      string                   `json:"version" binding:"required"`
}

type Credential struct {
	Configured    bool       `json:"configured" binding:"required"`
	CreatedAt     time.Time  `json:"created_at" binding:"required"`
	EnvironmentId *uuid.UUID `json:"environment_id,omitempty"`

	Hint      string    `json:"hint" binding:"required"`
	Id        uuid.UUID `json:"id" binding:"required"`
	Kind      string    `json:"kind" binding:"required"`
	Name      string    `json:"name" binding:"required"`
	UpdatedAt time.Time `json:"updated_at" binding:"required"`
	Version   int64     `json:"version" binding:"required"`
}

type CredentialCreate struct {
	EnvironmentId *uuid.UUID `json:"environment_id,omitempty"`
	Kind          string     `json:"kind" binding:"required"`
	Name          string     `json:"name" binding:"required"`
	Value         string     `json:"value" binding:"required"`
}

type CredentialUpdate struct {
	Name  *string `json:"name,omitempty"`
	Value *string `json:"value,omitempty"`
}

type Department struct {
	CreatedAt      time.Time `json:"created_at" binding:"required"`
	Description    string    `json:"description" binding:"required"`
	Id             uuid.UUID `json:"id" binding:"required"`
	Name           string    `json:"name" binding:"required"`
	Slug           string    `json:"slug" binding:"required"`
	UpdatedAt      time.Time `json:"updated_at" binding:"required"`
	WorkspaceCount int64     `json:"workspace_count" binding:"required"`
}

type DepartmentCreate struct {
	Description *string `json:"description,omitempty"`
	Name        string  `json:"name" binding:"required"`
	Slug        string  `json:"slug" binding:"required"`
}

type DepartmentUpdate struct {
	Description *string `json:"description,omitempty"`
	Name        string  `json:"name" binding:"required"`
}

type EffectivePermissions struct {
	Organization []string `json:"organization" binding:"required"`

	Workspaces map[string][]string `json:"workspaces" binding:"required"`
}

type EntitlementFeature struct {
	Description string                 `json:"description" binding:"required"`
	Enabled     bool                   `json:"enabled" binding:"required"`
	Key         string                 `json:"key" binding:"required"`
	Kind        EntitlementFeatureKind `json:"kind" binding:"required"`

	Limit *int64 `json:"limit,omitempty"`
	Name  string `json:"name" binding:"required"`

	Overridden bool   `json:"overridden" binding:"required"`
	Unit       string `json:"unit" binding:"required"`
	Used       *int64 `json:"used,omitempty"`
}

type EntitlementFeatureKind string

type Entitlements struct {
	CurrentPeriodEnd *time.Time           `json:"current_period_end,omitempty"`
	Features         []EntitlementFeature `json:"features" binding:"required"`
	Plan             struct {
		Key  string `json:"key"`
		Name string `json:"name"`
	} `json:"plan" binding:"required"`
	Status EntitlementsStatus `json:"status" binding:"required"`
}

type EntitlementsStatus string

type Environment struct {
	CreatedAt time.Time `json:"created_at" binding:"required"`

	EgressAllowlist []string  `json:"egress_allowlist" binding:"required"`
	Id              uuid.UUID `json:"id" binding:"required"`
	IsProduction    bool      `json:"is_production" binding:"required"`
	Name            string    `json:"name" binding:"required"`
	Slug            string    `json:"slug" binding:"required"`
	UpdatedAt       time.Time `json:"updated_at" binding:"required"`
}

type EnvironmentCreate struct {
	EgressAllowlist *[]string `json:"egress_allowlist,omitempty"`
	IsProduction    *bool     `json:"is_production,omitempty"`
	Name            string    `json:"name" binding:"required"`
	Slug            string    `json:"slug" binding:"required"`
}

type EnvironmentUpdate struct {
	EgressAllowlist *[]string `json:"egress_allowlist,omitempty"`
	IsProduction    bool      `json:"is_production" binding:"required"`
	Name            string    `json:"name" binding:"required"`
}

type EvaluateRequest struct {
	Action           string     `json:"action" binding:"required"`
	ResourceId       *string    `json:"resource_id,omitempty"`
	ResourceType     string     `json:"resource_type" binding:"required"`
	ServiceAccountId *uuid.UUID `json:"service_account_id,omitempty"`
	UserId           *uuid.UUID `json:"user_id,omitempty"`
	WorkspaceId      *uuid.UUID `json:"workspace_id,omitempty"`
}

type FeatureDefinition struct {
	Description string                `json:"description" binding:"required"`
	Key         string                `json:"key" binding:"required"`
	Kind        FeatureDefinitionKind `json:"kind" binding:"required"`
	Name        string                `json:"name" binding:"required"`
	Unit        string                `json:"unit" binding:"required"`
}

type FeatureDefinitionKind string

type FieldError struct {
	Code    string `json:"code" binding:"required"`
	Field   string `json:"field" binding:"required"`
	Message string `json:"message" binding:"required"`
}

type Group struct {
	CreatedAt   time.Time   `json:"created_at" binding:"required"`
	Description string      `json:"description" binding:"required"`
	Id          uuid.UUID   `json:"id" binding:"required"`
	MemberCount int64       `json:"member_count" binding:"required"`
	Name        string      `json:"name" binding:"required"`
	Source      GroupSource `json:"source" binding:"required"`
	UpdatedAt   time.Time   `json:"updated_at" binding:"required"`
}

type GroupSource string

type GroupInput struct {
	Description *string `json:"description,omitempty"`
	Name        string  `json:"name" binding:"required"`
}

type GroupMapping struct {
	CreatedAt     time.Time               `json:"created_at" binding:"required"`
	ExternalGroup string                  `json:"external_group" binding:"required"`
	GroupId       uuid.UUID               `json:"group_id" binding:"required"`
	GroupName     string                  `json:"group_name" binding:"required"`
	GroupSource   GroupMappingGroupSource `json:"group_source" binding:"required"`
	Id            uuid.UUID               `json:"id" binding:"required"`
}

type GroupMappingGroupSource string

type GroupMappingCreate struct {
	ExternalGroup string `json:"external_group" binding:"required"`

	GroupId *uuid.UUID `json:"group_id,omitempty"`
}

type GroupMember struct {
	AddedAt     time.Time `json:"added_at" binding:"required"`
	DisplayName string    `json:"display_name" binding:"required"`
	Email       string    `json:"email" binding:"required"`
	Id          uuid.UUID `json:"id" binding:"required"`
}

type GroupRef struct {
	Id   uuid.UUID `json:"id" binding:"required"`
	Name string    `json:"name" binding:"required"`
}

type HealthReport struct {
	Checks *map[string]struct {
		Error  *string                  `json:"error,omitempty"`
		Status HealthReportChecksStatus `json:"status"`
	} `json:"checks,omitempty"`
	Status HealthReportStatus `json:"status" binding:"required"`
}

type HealthReportChecksStatus string

type HealthReportStatus string

type IdentityProvider struct {
	AllowedDomains    []string     `json:"allowed_domains" binding:"required"`
	ClaimMapping      ClaimMapping `json:"claim_mapping" binding:"required"`
	ClientId          string       `json:"client_id" binding:"required"`
	CreatedAt         time.Time    `json:"created_at" binding:"required"`
	Enabled           bool         `json:"enabled" binding:"required"`
	GroupMappingCount int64        `json:"group_mapping_count" binding:"required"`

	HasClientSecret bool      `json:"has_client_secret" binding:"required"`
	Id              uuid.UUID `json:"id" binding:"required"`
	Issuer          string    `json:"issuer" binding:"required"`

	JitProvisioning bool                     `json:"jit_provisioning" binding:"required"`
	Name            string                   `json:"name" binding:"required"`
	Preset          IdentityProviderPreset   `json:"preset" binding:"required"`
	Protocol        IdentityProviderProtocol `json:"protocol" binding:"required"`

	RedirectUri string   `json:"redirect_uri" binding:"required"`
	Scopes      []string `json:"scopes" binding:"required"`

	SsoUrl    string    `json:"sso_url" binding:"required"`
	UpdatedAt time.Time `json:"updated_at" binding:"required"`
}

type IdentityProviderProtocol string

type IdentityProviderCreate struct {
	AllowedDomains *[]string     `json:"allowed_domains,omitempty"`
	ClaimMapping   *ClaimMapping `json:"claim_mapping,omitempty"`
	ClientId       string        `json:"client_id" binding:"required"`

	ClientSecret *string `json:"client_secret,omitempty"`
	Enabled      *bool   `json:"enabled,omitempty"`

	Issuer          string                 `json:"issuer" binding:"required"`
	JitProvisioning *bool                  `json:"jit_provisioning,omitempty"`
	Name            string                 `json:"name" binding:"required"`
	Preset          IdentityProviderPreset `json:"preset" binding:"required"`
	Scopes          *[]string              `json:"scopes,omitempty"`
}

type IdentityProviderPreset string

type IdentityProviderTest struct {
	AuthorizationEndpoint *string   `json:"authorization_endpoint,omitempty"`
	CheckedAt             time.Time `json:"checked_at" binding:"required"`

	Error         *string `json:"error,omitempty"`
	Issuer        *string `json:"issuer,omitempty"`
	JwksUri       *string `json:"jwks_uri,omitempty"`
	LatencyMs     int64   `json:"latency_ms" binding:"required"`
	Ok            bool    `json:"ok" binding:"required"`
	TokenEndpoint *string `json:"token_endpoint,omitempty"`
}

type IdentityProviderUpdate struct {
	AllowedDomains *[]string     `json:"allowed_domains,omitempty"`
	ClaimMapping   *ClaimMapping `json:"claim_mapping,omitempty"`
	ClientId       *string       `json:"client_id,omitempty"`

	ClientSecret    *string                 `json:"client_secret,omitempty"`
	Enabled         *bool                   `json:"enabled,omitempty"`
	Issuer          *string                 `json:"issuer,omitempty"`
	JitProvisioning *bool                   `json:"jit_provisioning,omitempty"`
	Name            *string                 `json:"name,omitempty"`
	Preset          *IdentityProviderPreset `json:"preset,omitempty"`
	Scopes          *[]string               `json:"scopes,omitempty"`
}

type Invitation struct {
	AcceptedAt    *time.Time `json:"accepted_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at" binding:"required"`
	Email         string     `json:"email" binding:"required"`
	ExpiresAt     time.Time  `json:"expires_at" binding:"required"`
	Id            uuid.UUID  `json:"id" binding:"required"`
	InvitedByName *string    `json:"invited_by_name,omitempty"`
	OrgRole       *struct {
		Id   uuid.UUID `json:"id"`
		Key  string    `json:"key"`
		Name string    `json:"name"`
	} `json:"org_role,omitempty"`
	Status     InvitationStatus      `json:"status" binding:"required"`
	Workspaces []InvitationWorkspace `json:"workspaces" binding:"required"`
}

type InvitationStatus string

type InvitationAccept struct {
	DisplayName *string `json:"display_name,omitempty"`
	Password    *string `json:"password,omitempty"`
}

type InvitationCreate struct {
	Email         string `json:"email" binding:"required"`
	ExpiresInDays *int   `json:"expires_in_days,omitempty"`

	OrgRoleId  *uuid.UUID `json:"org_role_id,omitempty"`
	Workspaces *[]struct {
		RoleId      uuid.UUID `json:"role_id"`
		WorkspaceId uuid.UUID `json:"workspace_id"`
	} `json:"workspaces,omitempty"`
}

type InvitationCreated struct {
	AcceptPath string     `json:"accept_path" binding:"required"`
	Invitation Invitation `json:"invitation" binding:"required"`

	Token string `json:"token" binding:"required"`
}

type InvitationPreview struct {
	AccountExists bool                `json:"account_exists" binding:"required"`
	Email         string              `json:"email" binding:"required"`
	ExpiresAt     time.Time           `json:"expires_at" binding:"required"`
	InvitedByName *string             `json:"invited_by_name,omitempty"`
	Organization  OrganizationSummary `json:"organization" binding:"required"`
}

type InvitationWorkspace struct {
	RoleId        uuid.UUID `json:"role_id" binding:"required"`
	RoleName      string    `json:"role_name" binding:"required"`
	WorkspaceId   uuid.UUID `json:"workspace_id" binding:"required"`
	WorkspaceName string    `json:"workspace_name" binding:"required"`
	WorkspaceSlug *string   `json:"workspace_slug,omitempty"`
}

type LoggedDecision struct {
	Action       string    `json:"action" binding:"required"`
	Allowed      bool      `json:"allowed" binding:"required"`
	Id           uuid.UUID `json:"id" binding:"required"`
	MatchedRules []string  `json:"matched_rules" binding:"required"`
	OccurredAt   time.Time `json:"occurred_at" binding:"required"`

	PrincipalEmail *string        `json:"principal_email,omitempty"`
	PrincipalId    uuid.UUID      `json:"principal_id" binding:"required"`
	PrincipalType  *PrincipalType `json:"principal_type,omitempty"`
	Reason         string         `json:"reason" binding:"required"`
	RequestId      *string        `json:"request_id,omitempty"`
	ResourceId     *string        `json:"resource_id,omitempty"`
	ResourceType   string         `json:"resource_type" binding:"required"`
	WorkspaceId    *uuid.UUID     `json:"workspace_id,omitempty"`
}

type LoginRequest struct {
	Email            string  `json:"email" binding:"required"`
	OrganizationSlug *string `json:"organization_slug,omitempty"`
	Password         string  `json:"password" binding:"required"`
}

type Mapping struct {
	CreatedAt    time.Time               `json:"created_at" binding:"required"`
	Draft        map[string]interface{}  `json:"draft" binding:"required"`
	Id           uuid.UUID               `json:"id" binding:"required"`
	InputSchema  *map[string]interface{} `json:"input_schema,omitempty"`
	Name         string                  `json:"name" binding:"required"`
	OutputSchema *map[string]interface{} `json:"output_schema,omitempty"`
	UpdatedAt    time.Time               `json:"updated_at" binding:"required"`
	WorkspaceId  uuid.UUID               `json:"workspace_id" binding:"required"`
}

type MappingCreate struct {
	Draft        *map[string]interface{} `json:"draft,omitempty"`
	InputSchema  *map[string]interface{} `json:"input_schema,omitempty"`
	Name         string                  `json:"name" binding:"required"`
	OutputSchema *map[string]interface{} `json:"output_schema,omitempty"`
	WorkspaceId  uuid.UUID               `json:"workspace_id" binding:"required"`
}

type MappingTest struct {
	CreatedAt time.Time   `json:"created_at" binding:"required"`
	Expected  interface{} `json:"expected" binding:"required"`
	Id        uuid.UUID   `json:"id" binding:"required"`
	Input     interface{} `json:"input" binding:"required"`
	MappingId uuid.UUID   `json:"mapping_id" binding:"required"`
	Name      string      `json:"name" binding:"required"`
	UpdatedAt time.Time   `json:"updated_at" binding:"required"`
}

type MappingTestCreate struct {
	Expected interface{} `json:"expected" binding:"required"`
	Input    interface{} `json:"input" binding:"required"`
	Name     string      `json:"name" binding:"required"`
}

type MappingTestRequest struct {
	Input interface{} `json:"input" binding:"required"`
}

type MappingTestResult struct {
	Output            interface{} `json:"output" binding:"required"`
	StoredTestsFailed []struct {
		Actual   interface{} `json:"actual"`
		Expected interface{} `json:"expected"`
		Name     string      `json:"name"`
	} `json:"stored_tests_failed" binding:"required"`
	StoredTestsPassed int `json:"stored_tests_passed" binding:"required"`
}

type MappingTestUpdate struct {
	Expected interface{} `json:"expected,omitempty"`
	Input    interface{} `json:"input,omitempty"`
	Name     *string     `json:"name,omitempty"`
}

type MappingUpdate struct {
	Draft        *map[string]interface{} `json:"draft,omitempty"`
	InputSchema  *map[string]interface{} `json:"input_schema,omitempty"`
	Name         *string                 `json:"name,omitempty"`
	OutputSchema *map[string]interface{} `json:"output_schema,omitempty"`
}

type MappingVersion struct {
	Checksum    string                 `json:"checksum" binding:"required"`
	Definition  map[string]interface{} `json:"definition" binding:"required"`
	Id          uuid.UUID              `json:"id" binding:"required"`
	MappingId   uuid.UUID              `json:"mapping_id" binding:"required"`
	PublishedAt time.Time              `json:"published_at" binding:"required"`
	PublishedBy uuid.UUID              `json:"published_by" binding:"required"`
	Version     int                    `json:"version" binding:"required"`
}

type Me struct {
	Membership struct {
		IsOwner  bool      `json:"is_owner"`
		JoinedAt time.Time `json:"joined_at"`
	} `json:"membership" binding:"required"`
	Organization OrganizationSummary `json:"organization" binding:"required"`
	Session      SessionSummary      `json:"session" binding:"required"`
	User         User                `json:"user" binding:"required"`
}

type Member struct {
	AddedAt     *time.Time `json:"added_at,omitempty"`
	DisplayName string     `json:"display_name" binding:"required"`
	Email       string     `json:"email" binding:"required"`

	Groups   *[]GroupRef `json:"groups,omitempty"`
	Id       uuid.UUID   `json:"id" binding:"required"`
	IsOwner  bool        `json:"is_owner" binding:"required"`
	JoinedAt *time.Time  `json:"joined_at,omitempty"`

	MembershipStatus *MemberMembershipStatus `json:"membership_status,omitempty"`

	Roles  *[]string `json:"roles,omitempty"`
	Status string    `json:"status" binding:"required"`
}

type MemberMembershipStatus string

type OidcProviderOption struct {
	Id     uuid.UUID              `json:"id" binding:"required"`
	Name   string                 `json:"name" binding:"required"`
	Preset IdentityProviderPreset `json:"preset" binding:"required"`
}

type OpenAPIChange struct {
	Breaking    *bool             `json:"breaking,omitempty"`
	Kind        OpenAPIChangeKind `json:"kind" binding:"required"`
	Method      string            `json:"method" binding:"required"`
	OperationId string            `json:"operation_id" binding:"required"`
	Path        string            `json:"path" binding:"required"`
}

type OpenAPIChangeKind string

type OpenAPIImportRequest struct {
	Spec         *string `json:"spec,omitempty"`
	Url          *string `json:"url,omitempty"`
	VersionLabel *string `json:"version_label,omitempty"`
}

type OpenAPIImportResult struct {
	Changes       []OpenAPIChange `json:"changes" binding:"required"`
	DryRun        bool            `json:"dry_run" binding:"required"`
	EndpointCount int             `json:"endpoint_count" binding:"required"`
	SpecId        *uuid.UUID      `json:"spec_id,omitempty"`
}

type Organization struct {
	CreatedAt time.Time            `json:"created_at" binding:"required"`
	Id        uuid.UUID            `json:"id" binding:"required"`
	Name      string               `json:"name" binding:"required"`
	Settings  OrganizationSettings `json:"settings" binding:"required"`
	Slug      string               `json:"slug" binding:"required"`
	Status    OrganizationStatus   `json:"status" binding:"required"`
	UpdatedAt time.Time            `json:"updated_at" binding:"required"`
}

type OrganizationStatus string

type OrganizationChoice struct {
	Name string `json:"name" binding:"required"`
	Slug string `json:"slug" binding:"required"`
}

type OrganizationChoiceProblem struct {
	Code          string               `json:"code" binding:"required"`
	Detail        *string              `json:"detail,omitempty"`
	Errors        *[]FieldError        `json:"errors,omitempty"`
	Organizations []OrganizationChoice `json:"organizations" binding:"required"`
	RequestId     *string              `json:"request_id,omitempty"`
	Status        int                  `json:"status" binding:"required"`
	Title         string               `json:"title" binding:"required"`
	Type          string               `json:"type" binding:"required"`
}

type OrganizationSettings struct {
	SessionAbsoluteTimeoutHours int `json:"session_absolute_timeout_hours" binding:"required"`
	SessionIdleTimeoutMinutes   int `json:"session_idle_timeout_minutes" binding:"required"`
}

type OrganizationSummary struct {
	Id   uuid.UUID `json:"id" binding:"required"`
	Name string    `json:"name" binding:"required"`
	Slug string    `json:"slug" binding:"required"`
}

type Plan struct {
	Currency     string `json:"currency" binding:"required"`
	Description  string `json:"description" binding:"required"`
	Entitlements map[string]struct {
		Enabled bool   `json:"enabled"`
		Limit   *int64 `json:"limit,omitempty"`
	} `json:"entitlements" binding:"required"`
	Key  string `json:"key" binding:"required"`
	Name string `json:"name" binding:"required"`

	PriceMonthlyCents *int64 `json:"price_monthly_cents,omitempty"`
}

type PlanProblem struct {
	Code      string        `json:"code" binding:"required"`
	Current   *int64        `json:"current,omitempty"`
	Detail    *string       `json:"detail,omitempty"`
	Errors    *[]FieldError `json:"errors,omitempty"`
	Feature   string        `json:"feature" binding:"required"`
	Limit     *int64        `json:"limit,omitempty"`
	Plan      string        `json:"plan" binding:"required"`
	RequestId *string       `json:"request_id,omitempty"`
	Status    int           `json:"status" binding:"required"`
	Title     string        `json:"title" binding:"required"`
	Type      string        `json:"type" binding:"required"`
}

type PrincipalType string

type Problem struct {
	Code      string        `json:"code" binding:"required"`
	Detail    *string       `json:"detail,omitempty"`
	Errors    *[]FieldError `json:"errors,omitempty"`
	RequestId *string       `json:"request_id,omitempty"`
	Status    int           `json:"status" binding:"required"`
	Title     string        `json:"title" binding:"required"`
	Type      string        `json:"type" binding:"required"`
}

type ResourceType struct {
	Actions          []string `json:"actions" binding:"required"`
	Description      string   `json:"description" binding:"required"`
	SensitiveActions []string `json:"sensitive_actions" binding:"required"`
	Type             string   `json:"type" binding:"required"`
}

type Role struct {
	BindingCount int64     `json:"binding_count" binding:"required"`
	CreatedAt    time.Time `json:"created_at" binding:"required"`
	Description  string    `json:"description" binding:"required"`
	Id           uuid.UUID `json:"id" binding:"required"`
	Immutable    bool      `json:"immutable" binding:"required"`
	IsSystem     bool      `json:"is_system" binding:"required"`
	Key          string    `json:"key" binding:"required"`
	Name         string    `json:"name" binding:"required"`
	Permissions  []string  `json:"permissions" binding:"required"`
	UpdatedAt    time.Time `json:"updated_at" binding:"required"`
}

type RoleBinding struct {
	CreatedAt time.Time  `json:"created_at" binding:"required"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Id        uuid.UUID  `json:"id" binding:"required"`
	Principal struct {
		Email *string   `json:"email,omitempty"`
		Id    uuid.UUID `json:"id"`

		Name string        `json:"name"`
		Type PrincipalType `json:"type"`
	} `json:"principal" binding:"required"`
	Role struct {
		Id   uuid.UUID `json:"id"`
		Key  string    `json:"key"`
		Name string    `json:"name"`
	} `json:"role" binding:"required"`
	Scope BindingScope `json:"scope" binding:"required"`

	User *struct {
		DisplayName string    `json:"display_name"`
		Email       string    `json:"email"`
		Id          uuid.UUID `json:"id"`
	} `json:"user,omitempty"`
	WorkspaceName *string `json:"workspace_name,omitempty"`
	WorkspaceSlug *string `json:"workspace_slug,omitempty"`
}

type RoleBindingCreate struct {
	ExpiresAt        *time.Time   `json:"expires_at,omitempty"`
	GroupId          *uuid.UUID   `json:"group_id,omitempty"`
	RoleId           uuid.UUID    `json:"role_id" binding:"required"`
	Scope            BindingScope `json:"scope" binding:"required"`
	ServiceAccountId *uuid.UUID   `json:"service_account_id,omitempty"`
	UserId           *uuid.UUID   `json:"user_id,omitempty"`
}

type RoleCreate struct {
	Description *string  `json:"description,omitempty"`
	Key         string   `json:"key" binding:"required"`
	Name        string   `json:"name" binding:"required"`
	Permissions []string `json:"permissions" binding:"required"`
}

type RoleUpdate struct {
	Description *string  `json:"description,omitempty"`
	Name        string   `json:"name" binding:"required"`
	Permissions []string `json:"permissions" binding:"required"`
}

type Service struct {
	CreatedAt     time.Time            `json:"created_at" binding:"required"`
	Description   string               `json:"description" binding:"required"`
	EndpointCount int64                `json:"endpoint_count" binding:"required"`
	Environments  []ServiceEnvironment `json:"environments" binding:"required"`
	Id            uuid.UUID            `json:"id" binding:"required"`
	Name          string               `json:"name" binding:"required"`
	Owner         string               `json:"owner" binding:"required"`
	Protocol      ServiceProtocol      `json:"protocol" binding:"required"`
	UpdatedAt     time.Time            `json:"updated_at" binding:"required"`
	WorkspaceId   uuid.UUID            `json:"workspace_id" binding:"required"`
}

type ServiceProtocol string

type ServiceAccount struct {
	ActiveKeyCount int64     `json:"active_key_count" binding:"required"`
	CreatedAt      time.Time `json:"created_at" binding:"required"`
	Description    string    `json:"description" binding:"required"`
	Disabled       bool      `json:"disabled" binding:"required"`
	Id             uuid.UUID `json:"id" binding:"required"`
	Name           string    `json:"name" binding:"required"`
	UpdatedAt      time.Time `json:"updated_at" binding:"required"`
}

type ServiceAccountCreate struct {
	Description *string `json:"description,omitempty"`
	Name        string  `json:"name" binding:"required"`
}

type ServiceAccountUpdate struct {
	Description *string `json:"description,omitempty"`
	Disabled    bool    `json:"disabled" binding:"required"`
	Name        string  `json:"name" binding:"required"`
}

type ServiceCreate struct {
	Description *string                `json:"description,omitempty"`
	Name        string                 `json:"name" binding:"required"`
	Owner       *string                `json:"owner,omitempty"`
	Protocol    *ServiceCreateProtocol `json:"protocol,omitempty"`
	WorkspaceId uuid.UUID              `json:"workspace_id" binding:"required"`
}

type ServiceCreateProtocol string

type ServiceEndpoint struct {
	Deprecated      bool                     `json:"deprecated" binding:"required"`
	Id              uuid.UUID                `json:"id" binding:"required"`
	Method          string                   `json:"method" binding:"required"`
	OperationId     string                   `json:"operation_id" binding:"required"`
	Parameters      []map[string]interface{} `json:"parameters" binding:"required"`
	Path            string                   `json:"path" binding:"required"`
	RequestSchema   *map[string]interface{}  `json:"request_schema,omitempty"`
	ResponseSchemas map[string]interface{}   `json:"response_schemas" binding:"required"`
	Security        []map[string]interface{} `json:"security" binding:"required"`
	ServiceId       uuid.UUID                `json:"service_id" binding:"required"`
	Summary         string                   `json:"summary" binding:"required"`
}

type ServiceEnvironment struct {
	BaseUrl             string     `json:"base_url" binding:"required"`
	ConnectorInstanceId *uuid.UUID `json:"connector_instance_id" binding:"required"`
	EnvironmentId       uuid.UUID  `json:"environment_id" binding:"required"`
}

type ServiceEnvironmentInput struct {
	BaseUrl             string     `json:"base_url" binding:"required"`
	ConnectorInstanceId *uuid.UUID `json:"connector_instance_id,omitempty"`
}

type ServiceTryRequest struct {
	Body          interface{}        `json:"body,omitempty"`
	EnvironmentId uuid.UUID          `json:"environment_id" binding:"required"`
	Headers       *map[string]string `json:"headers,omitempty"`
	PathValues    *map[string]string `json:"path_values,omitempty"`
	Query         *map[string]string `json:"query,omitempty"`
}

type ServiceTryResult struct {
	Body       interface{}       `json:"body" binding:"required"`
	DurationMs int               `json:"duration_ms" binding:"required"`
	Headers    map[string]string `json:"headers" binding:"required"`
	Status     int               `json:"status" binding:"required"`
}

type ServiceUpdate struct {
	Description *string                `json:"description,omitempty"`
	Name        *string                `json:"name,omitempty"`
	Owner       *string                `json:"owner,omitempty"`
	Protocol    *ServiceUpdateProtocol `json:"protocol,omitempty"`
}

type ServiceUpdateProtocol string

type SessionInfo struct {
	CreatedAt     time.Time `json:"created_at" binding:"required"`
	Current       bool      `json:"current" binding:"required"`
	ExpiresAt     time.Time `json:"expires_at" binding:"required"`
	Id            uuid.UUID `json:"id" binding:"required"`
	IdleExpiresAt time.Time `json:"idle_expires_at" binding:"required"`
	Ip            *string   `json:"ip,omitempty"`
	LastSeenAt    time.Time `json:"last_seen_at" binding:"required"`
	UserAgent     *string   `json:"user_agent,omitempty"`
}

type SessionSummary struct {
	ExpiresAt     time.Time `json:"expires_at" binding:"required"`
	Id            uuid.UUID `json:"id" binding:"required"`
	IdleExpiresAt time.Time `json:"idle_expires_at" binding:"required"`
}

type SignupRequest struct {
	DisplayName string `json:"display_name" binding:"required"`

	Email string `json:"email" binding:"required"`

	OrganizationName string `json:"organization_name" binding:"required"`

	OrganizationSlug string `json:"organization_slug" binding:"required"`
	Password         string `json:"password" binding:"required"`
}

type SystemInfo struct {
	ApiVersion  string                `json:"api_version" binding:"required"`
	Commit      string                `json:"commit" binding:"required"`
	Environment SystemInfoEnvironment `json:"environment" binding:"required"`
	GoVersion   string                `json:"go_version" binding:"required"`

	Name string `json:"name" binding:"required"`

	Version string `json:"version" binding:"required"`
}

type SystemInfoEnvironment string

type UpdateOrganizationRequest struct {
	Name     *string               `json:"name,omitempty"`
	Settings *OrganizationSettings `json:"settings,omitempty"`
}

type User struct {
	DisplayName string     `json:"display_name" binding:"required"`
	Email       string     `json:"email" binding:"required"`
	Id          uuid.UUID  `json:"id" binding:"required"`
	Status      UserStatus `json:"status" binding:"required"`
}

type UserStatus string

type UserUpdate struct {
	Status UserUpdateStatus `json:"status" binding:"required"`
}

type UserUpdateStatus string

type Workspace struct {
	ArchivedAt     *time.Time `json:"archived_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at" binding:"required"`
	DepartmentId   *uuid.UUID `json:"department_id,omitempty"`
	DepartmentName *string    `json:"department_name,omitempty"`
	Description    string     `json:"description" binding:"required"`
	Id             uuid.UUID  `json:"id" binding:"required"`
	MemberCount    int64      `json:"member_count" binding:"required"`
	Name           string     `json:"name" binding:"required"`
	Slug           string     `json:"slug" binding:"required"`
	UpdatedAt      time.Time  `json:"updated_at" binding:"required"`
}

type WorkspaceCreate struct {
	DepartmentId *uuid.UUID `json:"department_id,omitempty"`
	Description  *string    `json:"description,omitempty"`
	Name         string     `json:"name" binding:"required"`
	Slug         string     `json:"slug" binding:"required"`
}

type WorkspaceUpdate struct {
	DepartmentId *uuid.UUID `json:"department_id,omitempty"`
	Description  *string    `json:"description,omitempty"`
	Name         string     `json:"name" binding:"required"`
}

type Cursor = string

type Limit = int

type WorkspaceSlug = string

type ListAuditEventsParams struct {
	Action       *string    `form:"action,omitempty" json:"action,omitempty"`
	ActorId      *uuid.UUID `form:"actor_id,omitempty" json:"actor_id,omitempty"`
	ResourceType *string    `form:"resource_type,omitempty" json:"resource_type,omitempty"`
	Since        *time.Time `form:"since,omitempty" json:"since,omitempty"`
	Until        *time.Time `form:"until,omitempty" json:"until,omitempty"`
	Limit        *Limit     `form:"limit,omitempty" json:"limit,omitempty"`
	Cursor       *Cursor    `form:"cursor,omitempty" json:"cursor,omitempty"`
}

type CompleteOidcLoginParams struct {
	Code             *string `form:"code,omitempty" json:"code,omitempty"`
	State            *string `form:"state,omitempty" json:"state,omitempty"`
	Error            *string `form:"error,omitempty" json:"error,omitempty"`
	ErrorDescription *string `form:"error_description,omitempty" json:"error_description,omitempty"`
}

type ListOidcProvidersParams struct {
	Organization string `form:"organization" json:"organization" binding:"required"`
}

type StartOidcLoginParams struct {
	Redirect *string `form:"redirect,omitempty" json:"redirect,omitempty"`
}

type ListAccessDecisionsParams struct {
	Allowed *bool   `form:"allowed,omitempty" json:"allowed,omitempty"`
	Limit   *Limit  `form:"limit,omitempty" json:"limit,omitempty"`
	Cursor  *Cursor `form:"cursor,omitempty" json:"cursor,omitempty"`
}

type AddGroupMemberJSONBody struct {
	UserId uuid.UUID `json:"user_id" binding:"required"`
}

type ListInvitationsParams struct {
	Status *ListInvitationsParamsStatus `form:"status,omitempty" json:"status,omitempty"`
}

type ListInvitationsParamsStatus string

type ListMappingsParams struct {
	WorkspaceId uuid.UUID `form:"workspace_id" json:"workspace_id" binding:"required"`
}

type ListRoleBindingsParams struct {
	UserId           *uuid.UUID                           `form:"user_id,omitempty" json:"user_id,omitempty"`
	GroupId          *uuid.UUID                           `form:"group_id,omitempty" json:"group_id,omitempty"`
	ServiceAccountId *uuid.UUID                           `form:"service_account_id,omitempty" json:"service_account_id,omitempty"`
	PrincipalType    *ListRoleBindingsParamsPrincipalType `form:"principal_type,omitempty" json:"principal_type,omitempty"`
	ScopeType        *ListRoleBindingsParamsScopeType     `form:"scope_type,omitempty" json:"scope_type,omitempty"`
	WorkspaceId      *uuid.UUID                           `form:"workspace_id,omitempty" json:"workspace_id,omitempty"`
}

type ListRoleBindingsParamsPrincipalType string

type ListRoleBindingsParamsScopeType string

type ListServicesParams struct {
	WorkspaceId uuid.UUID `form:"workspace_id" json:"workspace_id" binding:"required"`
}

type ImportOpenAPIParams struct {
	DryRun *bool `form:"dry_run,omitempty" json:"dry_run,omitempty"`
}

type ListWorkspacesParams struct {
	IncludeArchived *bool `form:"include_archived,omitempty" json:"include_archived,omitempty"`
}

type AddWorkspaceMemberJSONBody struct {
	RoleKey *string   `json:"role_key,omitempty"`
	UserId  uuid.UUID `json:"user_id" binding:"required"`
}

type LoginJSONRequestBody = LoginRequest

type EvaluateAccessJSONRequestBody = EvaluateRequest

type CreateConnectorJSONRequestBody = ConnectorInstanceCreate

type UpdateConnectorJSONRequestBody = ConnectorInstanceUpdate

type CreateCredentialJSONRequestBody = CredentialCreate

type UpdateCredentialJSONRequestBody = CredentialUpdate

type CreateDepartmentJSONRequestBody = DepartmentCreate

type UpdateDepartmentJSONRequestBody = DepartmentUpdate

type CreateEnvironmentJSONRequestBody = EnvironmentCreate

type UpdateEnvironmentJSONRequestBody = EnvironmentUpdate

type CreateGroupJSONRequestBody = GroupInput

type UpdateGroupJSONRequestBody = GroupInput

type AddGroupMemberJSONRequestBody AddGroupMemberJSONBody

type CreateIdentityProviderJSONRequestBody = IdentityProviderCreate

type UpdateIdentityProviderJSONRequestBody = IdentityProviderUpdate

type CreateGroupMappingJSONRequestBody = GroupMappingCreate

type AcceptInvitationJSONRequestBody = InvitationAccept

type CreateInvitationJSONRequestBody = InvitationCreate

type CreateMappingJSONRequestBody = MappingCreate

type UpdateMappingJSONRequestBody = MappingUpdate

type TestMappingJSONRequestBody = MappingTestRequest

type CreateMappingTestJSONRequestBody = MappingTestCreate

type UpdateMappingTestJSONRequestBody = MappingTestUpdate

type CreateOrganizationJSONRequestBody = SignupRequest

type UpdateCurrentOrganizationJSONRequestBody = UpdateOrganizationRequest

type CreateRoleBindingJSONRequestBody = RoleBindingCreate

type CreateRoleJSONRequestBody = RoleCreate

type UpdateRoleJSONRequestBody = RoleUpdate

type CreateServiceAccountJSONRequestBody = ServiceAccountCreate

type UpdateServiceAccountJSONRequestBody = ServiceAccountUpdate

type CreateApiKeyJSONRequestBody = ApiKeyCreate

type TryServiceEndpointJSONRequestBody = ServiceTryRequest

type CreateServiceJSONRequestBody = ServiceCreate

type UpdateServiceJSONRequestBody = ServiceUpdate

type PutServiceEnvironmentJSONRequestBody = ServiceEnvironmentInput

type ImportOpenAPIJSONRequestBody = OpenAPIImportRequest

type UpdateUserJSONRequestBody = UserUpdate

type CreateWorkspaceJSONRequestBody = WorkspaceCreate

type UpdateWorkspaceJSONRequestBody = WorkspaceUpdate

type AddWorkspaceMemberJSONRequestBody AddWorkspaceMemberJSONBody

type MiddlewareFunc func(http.Handler) http.Handler

type UnescapedCookieParamError struct {
	ParamName string
	Err       error
}

type UnmarshalingParamError struct {
	ParamName string
	Err       error
}

type RequiredParamError struct {
	ParamName string
}

type RequiredHeaderError struct {
	ParamName string
	Err       error
}

type InvalidParamFormatError struct {
	ParamName string
	Err       error
}

type TooManyValuesForParamError struct {
	ParamName string
	Count     int
}

type PlanProblemApplicationProblemPlusJSONResponse PlanProblem

type ProblemApplicationProblemPlusJSONResponse Problem

type GetLivenessRequestObject struct {
}

type GetLivenessResponseObject interface {
	VisitGetLivenessResponse(w http.ResponseWriter) error
}

type GetLiveness200JSONResponse HealthReport

func (response GetLiveness200JSONResponse) VisitGetLivenessResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type GetReadinessRequestObject struct {
}

type GetReadinessResponseObject interface {
	VisitGetReadinessResponse(w http.ResponseWriter) error
}

type GetReadiness200JSONResponse HealthReport

func (response GetReadiness200JSONResponse) VisitGetReadinessResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type GetReadiness503JSONResponse HealthReport

func (response GetReadiness503JSONResponse) VisitGetReadinessResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(503)
	_, err := buf.WriteTo(w)
	return err
}

type ListAuditEventsRequestObject struct {
	Params ListAuditEventsParams
}

type ListAuditEventsResponseObject interface {
	VisitListAuditEventsResponse(w http.ResponseWriter) error
}

type ListAuditEvents200JSONResponse AuditEventPage

func (response ListAuditEvents200JSONResponse) VisitListAuditEventsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type ListAuditEvents401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response ListAuditEvents401ApplicationProblemPlusJSONResponse) VisitListAuditEventsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type ListAuditEvents403ApplicationProblemPlusJSONResponse Problem

func (response ListAuditEvents403ApplicationProblemPlusJSONResponse) VisitListAuditEventsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type ListAuditEvents422ApplicationProblemPlusJSONResponse Problem

func (response ListAuditEvents422ApplicationProblemPlusJSONResponse) VisitListAuditEventsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type ListAuditEventsdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response ListAuditEventsdefaultApplicationProblemPlusJSONResponse) VisitListAuditEventsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type LoginRequestObject struct {
	Body *LoginJSONRequestBody
}

type LoginResponseObject interface {
	VisitLoginResponse(w http.ResponseWriter) error
}

type Login200JSONResponse AuthResponse

func (response Login200JSONResponse) VisitLoginResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type Login401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response Login401ApplicationProblemPlusJSONResponse) VisitLoginResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type Login409ApplicationProblemPlusJSONResponse OrganizationChoiceProblem

func (response Login409ApplicationProblemPlusJSONResponse) VisitLoginResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(409)
	_, err := buf.WriteTo(w)
	return err
}

type Login429ApplicationProblemPlusJSONResponse Problem

func (response Login429ApplicationProblemPlusJSONResponse) VisitLoginResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(429)
	_, err := buf.WriteTo(w)
	return err
}

type LogindefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response LogindefaultApplicationProblemPlusJSONResponse) VisitLoginResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type LogoutRequestObject struct {
}

type LogoutResponseObject interface {
	VisitLogoutResponse(w http.ResponseWriter) error
}

type Logout204Response struct {
}

func (response Logout204Response) VisitLogoutResponse(w http.ResponseWriter) error {
	w.WriteHeader(204)
	return nil
}

type LogoutdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response LogoutdefaultApplicationProblemPlusJSONResponse) VisitLogoutResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type CompleteOidcLoginRequestObject struct {
	Params CompleteOidcLoginParams
}

type CompleteOidcLoginResponseObject interface {
	VisitCompleteOidcLoginResponse(w http.ResponseWriter) error
}

type CompleteOidcLogin302ResponseHeaders struct {
	Location *string
}

type CompleteOidcLogin302Response struct {
	Headers CompleteOidcLogin302ResponseHeaders
}

func (response CompleteOidcLogin302Response) VisitCompleteOidcLoginResponse(w http.ResponseWriter) error {
	if response.Headers.Location != nil {
		w.Header().Set("Location", fmt.Sprint(*response.Headers.Location))
	}
	w.WriteHeader(302)
	return nil
}

type CompleteOidcLogin429ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response CompleteOidcLogin429ApplicationProblemPlusJSONResponse) VisitCompleteOidcLoginResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(429)
	_, err := buf.WriteTo(w)
	return err
}

type CompleteOidcLogindefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response CompleteOidcLogindefaultApplicationProblemPlusJSONResponse) VisitCompleteOidcLoginResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type ListOidcProvidersRequestObject struct {
	Params ListOidcProvidersParams
}

type ListOidcProvidersResponseObject interface {
	VisitListOidcProvidersResponse(w http.ResponseWriter) error
}

type ListOidcProviders200JSONResponse struct {
	Data []OidcProviderOption `json:"data" binding:"required"`
}

func (response ListOidcProviders200JSONResponse) VisitListOidcProvidersResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type ListOidcProviders429ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response ListOidcProviders429ApplicationProblemPlusJSONResponse) VisitListOidcProvidersResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(429)
	_, err := buf.WriteTo(w)
	return err
}

type ListOidcProvidersdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response ListOidcProvidersdefaultApplicationProblemPlusJSONResponse) VisitListOidcProvidersResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type StartOidcLoginRequestObject struct {
	IdpId  uuid.UUID `json:"idp_id" binding:"required"`
	Params StartOidcLoginParams
}

type StartOidcLoginResponseObject interface {
	VisitStartOidcLoginResponse(w http.ResponseWriter) error
}

type StartOidcLogin302ResponseHeaders struct {
	Location *string
}

type StartOidcLogin302Response struct {
	Headers StartOidcLogin302ResponseHeaders
}

func (response StartOidcLogin302Response) VisitStartOidcLoginResponse(w http.ResponseWriter) error {
	if response.Headers.Location != nil {
		w.Header().Set("Location", fmt.Sprint(*response.Headers.Location))
	}
	w.WriteHeader(302)
	return nil
}

type StartOidcLogin429ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response StartOidcLogin429ApplicationProblemPlusJSONResponse) VisitStartOidcLoginResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(429)
	_, err := buf.WriteTo(w)
	return err
}

type StartOidcLogindefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response StartOidcLogindefaultApplicationProblemPlusJSONResponse) VisitStartOidcLoginResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type ListAccessDecisionsRequestObject struct {
	Params ListAccessDecisionsParams
}

type ListAccessDecisionsResponseObject interface {
	VisitListAccessDecisionsResponse(w http.ResponseWriter) error
}

type ListAccessDecisions200JSONResponse struct {
	Data       []LoggedDecision `json:"data" binding:"required"`
	NextCursor *string          `json:"next_cursor,omitempty"`
}

func (response ListAccessDecisions200JSONResponse) VisitListAccessDecisionsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type ListAccessDecisions403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response ListAccessDecisions403ApplicationProblemPlusJSONResponse) VisitListAccessDecisionsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type ListAccessDecisionsdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response ListAccessDecisionsdefaultApplicationProblemPlusJSONResponse) VisitListAccessDecisionsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type EvaluateAccessRequestObject struct {
	Body *EvaluateAccessJSONRequestBody
}

type EvaluateAccessResponseObject interface {
	VisitEvaluateAccessResponse(w http.ResponseWriter) error
}

type EvaluateAccess200JSONResponse AccessDecision

func (response EvaluateAccess200JSONResponse) VisitEvaluateAccessResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type EvaluateAccess403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response EvaluateAccess403ApplicationProblemPlusJSONResponse) VisitEvaluateAccessResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type EvaluateAccess422ApplicationProblemPlusJSONResponse Problem

func (response EvaluateAccess422ApplicationProblemPlusJSONResponse) VisitEvaluateAccessResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type EvaluateAccessdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response EvaluateAccessdefaultApplicationProblemPlusJSONResponse) VisitEvaluateAccessResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type ListConnectorDefinitionsRequestObject struct {
}

type ListConnectorDefinitionsResponseObject interface {
	VisitListConnectorDefinitionsResponse(w http.ResponseWriter) error
}

type ListConnectorDefinitions200JSONResponse struct {
	Data []ConnectorDefinition `json:"data" binding:"required"`
}

func (response ListConnectorDefinitions200JSONResponse) VisitListConnectorDefinitionsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type ListConnectorDefinitions401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response ListConnectorDefinitions401ApplicationProblemPlusJSONResponse) VisitListConnectorDefinitionsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type ListConnectorDefinitions403ApplicationProblemPlusJSONResponse Problem

func (response ListConnectorDefinitions403ApplicationProblemPlusJSONResponse) VisitListConnectorDefinitionsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type ListConnectorDefinitionsdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response ListConnectorDefinitionsdefaultApplicationProblemPlusJSONResponse) VisitListConnectorDefinitionsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type ListConnectorVersionsRequestObject struct {
	DefinitionId uuid.UUID `json:"definition_id" binding:"required"`
}

type ListConnectorVersionsResponseObject interface {
	VisitListConnectorVersionsResponse(w http.ResponseWriter) error
}

type ListConnectorVersions200JSONResponse struct {
	Data []ConnectorVersion `json:"data" binding:"required"`
}

func (response ListConnectorVersions200JSONResponse) VisitListConnectorVersionsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type ListConnectorVersions401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response ListConnectorVersions401ApplicationProblemPlusJSONResponse) VisitListConnectorVersionsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type ListConnectorVersions403ApplicationProblemPlusJSONResponse Problem

func (response ListConnectorVersions403ApplicationProblemPlusJSONResponse) VisitListConnectorVersionsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type ListConnectorVersions404ApplicationProblemPlusJSONResponse Problem

func (response ListConnectorVersions404ApplicationProblemPlusJSONResponse) VisitListConnectorVersionsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type ListConnectorVersionsdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response ListConnectorVersionsdefaultApplicationProblemPlusJSONResponse) VisitListConnectorVersionsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type ListConnectorsRequestObject struct {
}

type ListConnectorsResponseObject interface {
	VisitListConnectorsResponse(w http.ResponseWriter) error
}

type ListConnectors200JSONResponse struct {
	Data []ConnectorInstance `json:"data" binding:"required"`
}

func (response ListConnectors200JSONResponse) VisitListConnectorsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type ListConnectors401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response ListConnectors401ApplicationProblemPlusJSONResponse) VisitListConnectorsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type ListConnectors403ApplicationProblemPlusJSONResponse Problem

func (response ListConnectors403ApplicationProblemPlusJSONResponse) VisitListConnectorsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type ListConnectorsdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response ListConnectorsdefaultApplicationProblemPlusJSONResponse) VisitListConnectorsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type CreateConnectorRequestObject struct {
	Body *CreateConnectorJSONRequestBody
}

type CreateConnectorResponseObject interface {
	VisitCreateConnectorResponse(w http.ResponseWriter) error
}

type CreateConnector201JSONResponse ConnectorInstance

func (response CreateConnector201JSONResponse) VisitCreateConnectorResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	_, err := buf.WriteTo(w)
	return err
}

type CreateConnector401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response CreateConnector401ApplicationProblemPlusJSONResponse) VisitCreateConnectorResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type CreateConnector403ApplicationProblemPlusJSONResponse Problem

func (response CreateConnector403ApplicationProblemPlusJSONResponse) VisitCreateConnectorResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type CreateConnector422ApplicationProblemPlusJSONResponse Problem

func (response CreateConnector422ApplicationProblemPlusJSONResponse) VisitCreateConnectorResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type CreateConnectordefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response CreateConnectordefaultApplicationProblemPlusJSONResponse) VisitCreateConnectorResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteConnectorRequestObject struct {
	ConnectorId uuid.UUID `json:"connector_id" binding:"required"`
}

type DeleteConnectorResponseObject interface {
	VisitDeleteConnectorResponse(w http.ResponseWriter) error
}

type DeleteConnector204Response struct {
}

func (response DeleteConnector204Response) VisitDeleteConnectorResponse(w http.ResponseWriter) error {
	w.WriteHeader(204)
	return nil
}

type DeleteConnector401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response DeleteConnector401ApplicationProblemPlusJSONResponse) VisitDeleteConnectorResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteConnector403ApplicationProblemPlusJSONResponse Problem

func (response DeleteConnector403ApplicationProblemPlusJSONResponse) VisitDeleteConnectorResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteConnector404ApplicationProblemPlusJSONResponse Problem

func (response DeleteConnector404ApplicationProblemPlusJSONResponse) VisitDeleteConnectorResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteConnectordefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response DeleteConnectordefaultApplicationProblemPlusJSONResponse) VisitDeleteConnectorResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type GetConnectorRequestObject struct {
	ConnectorId uuid.UUID `json:"connector_id" binding:"required"`
}

type GetConnectorResponseObject interface {
	VisitGetConnectorResponse(w http.ResponseWriter) error
}

type GetConnector200JSONResponse ConnectorInstance

func (response GetConnector200JSONResponse) VisitGetConnectorResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type GetConnector401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response GetConnector401ApplicationProblemPlusJSONResponse) VisitGetConnectorResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type GetConnector403ApplicationProblemPlusJSONResponse Problem

func (response GetConnector403ApplicationProblemPlusJSONResponse) VisitGetConnectorResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type GetConnector404ApplicationProblemPlusJSONResponse Problem

func (response GetConnector404ApplicationProblemPlusJSONResponse) VisitGetConnectorResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type GetConnectordefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response GetConnectordefaultApplicationProblemPlusJSONResponse) VisitGetConnectorResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateConnectorRequestObject struct {
	ConnectorId uuid.UUID `json:"connector_id" binding:"required"`
	Body        *UpdateConnectorJSONRequestBody
}

type UpdateConnectorResponseObject interface {
	VisitUpdateConnectorResponse(w http.ResponseWriter) error
}

type UpdateConnector200JSONResponse ConnectorInstance

func (response UpdateConnector200JSONResponse) VisitUpdateConnectorResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateConnector401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response UpdateConnector401ApplicationProblemPlusJSONResponse) VisitUpdateConnectorResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateConnector403ApplicationProblemPlusJSONResponse Problem

func (response UpdateConnector403ApplicationProblemPlusJSONResponse) VisitUpdateConnectorResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateConnector404ApplicationProblemPlusJSONResponse Problem

func (response UpdateConnector404ApplicationProblemPlusJSONResponse) VisitUpdateConnectorResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateConnector422ApplicationProblemPlusJSONResponse Problem

func (response UpdateConnector422ApplicationProblemPlusJSONResponse) VisitUpdateConnectorResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateConnectordefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response UpdateConnectordefaultApplicationProblemPlusJSONResponse) VisitUpdateConnectorResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type GetConnectorHealthRequestObject struct {
	ConnectorId uuid.UUID `json:"connector_id" binding:"required"`
}

type GetConnectorHealthResponseObject interface {
	VisitGetConnectorHealthResponse(w http.ResponseWriter) error
}

type GetConnectorHealth200JSONResponse struct {
	Data []ConnectorTestResult `json:"data" binding:"required"`
}

func (response GetConnectorHealth200JSONResponse) VisitGetConnectorHealthResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type GetConnectorHealth401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response GetConnectorHealth401ApplicationProblemPlusJSONResponse) VisitGetConnectorHealthResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type GetConnectorHealth403ApplicationProblemPlusJSONResponse Problem

func (response GetConnectorHealth403ApplicationProblemPlusJSONResponse) VisitGetConnectorHealthResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type GetConnectorHealth404ApplicationProblemPlusJSONResponse Problem

func (response GetConnectorHealth404ApplicationProblemPlusJSONResponse) VisitGetConnectorHealthResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type GetConnectorHealthdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response GetConnectorHealthdefaultApplicationProblemPlusJSONResponse) VisitGetConnectorHealthResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type TestConnectorRequestObject struct {
	ConnectorId uuid.UUID `json:"connector_id" binding:"required"`
}

type TestConnectorResponseObject interface {
	VisitTestConnectorResponse(w http.ResponseWriter) error
}

type TestConnector200JSONResponse ConnectorTestResult

func (response TestConnector200JSONResponse) VisitTestConnectorResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type TestConnector401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response TestConnector401ApplicationProblemPlusJSONResponse) VisitTestConnectorResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type TestConnector403ApplicationProblemPlusJSONResponse Problem

func (response TestConnector403ApplicationProblemPlusJSONResponse) VisitTestConnectorResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type TestConnector404ApplicationProblemPlusJSONResponse Problem

func (response TestConnector404ApplicationProblemPlusJSONResponse) VisitTestConnectorResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type TestConnectordefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response TestConnectordefaultApplicationProblemPlusJSONResponse) VisitTestConnectorResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type ListCredentialsRequestObject struct {
}

type ListCredentialsResponseObject interface {
	VisitListCredentialsResponse(w http.ResponseWriter) error
}

type ListCredentials200JSONResponse struct {
	Data []Credential `json:"data" binding:"required"`
}

func (response ListCredentials200JSONResponse) VisitListCredentialsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type ListCredentials401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response ListCredentials401ApplicationProblemPlusJSONResponse) VisitListCredentialsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type ListCredentials403ApplicationProblemPlusJSONResponse Problem

func (response ListCredentials403ApplicationProblemPlusJSONResponse) VisitListCredentialsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type ListCredentialsdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response ListCredentialsdefaultApplicationProblemPlusJSONResponse) VisitListCredentialsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type CreateCredentialRequestObject struct {
	Body *CreateCredentialJSONRequestBody
}

type CreateCredentialResponseObject interface {
	VisitCreateCredentialResponse(w http.ResponseWriter) error
}

type CreateCredential201JSONResponse Credential

func (response CreateCredential201JSONResponse) VisitCreateCredentialResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	_, err := buf.WriteTo(w)
	return err
}

type CreateCredential401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response CreateCredential401ApplicationProblemPlusJSONResponse) VisitCreateCredentialResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type CreateCredential403ApplicationProblemPlusJSONResponse Problem

func (response CreateCredential403ApplicationProblemPlusJSONResponse) VisitCreateCredentialResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type CreateCredential422ApplicationProblemPlusJSONResponse Problem

func (response CreateCredential422ApplicationProblemPlusJSONResponse) VisitCreateCredentialResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type CreateCredentialdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response CreateCredentialdefaultApplicationProblemPlusJSONResponse) VisitCreateCredentialResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteCredentialRequestObject struct {
	CredentialId uuid.UUID `json:"credential_id" binding:"required"`
}

type DeleteCredentialResponseObject interface {
	VisitDeleteCredentialResponse(w http.ResponseWriter) error
}

type DeleteCredential204Response struct {
}

func (response DeleteCredential204Response) VisitDeleteCredentialResponse(w http.ResponseWriter) error {
	w.WriteHeader(204)
	return nil
}

type DeleteCredential401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response DeleteCredential401ApplicationProblemPlusJSONResponse) VisitDeleteCredentialResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteCredential403ApplicationProblemPlusJSONResponse Problem

func (response DeleteCredential403ApplicationProblemPlusJSONResponse) VisitDeleteCredentialResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteCredential404ApplicationProblemPlusJSONResponse Problem

func (response DeleteCredential404ApplicationProblemPlusJSONResponse) VisitDeleteCredentialResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteCredentialdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response DeleteCredentialdefaultApplicationProblemPlusJSONResponse) VisitDeleteCredentialResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type GetCredentialRequestObject struct {
	CredentialId uuid.UUID `json:"credential_id" binding:"required"`
}

type GetCredentialResponseObject interface {
	VisitGetCredentialResponse(w http.ResponseWriter) error
}

type GetCredential200JSONResponse Credential

func (response GetCredential200JSONResponse) VisitGetCredentialResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type GetCredential401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response GetCredential401ApplicationProblemPlusJSONResponse) VisitGetCredentialResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type GetCredential403ApplicationProblemPlusJSONResponse Problem

func (response GetCredential403ApplicationProblemPlusJSONResponse) VisitGetCredentialResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type GetCredential404ApplicationProblemPlusJSONResponse Problem

func (response GetCredential404ApplicationProblemPlusJSONResponse) VisitGetCredentialResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type GetCredentialdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response GetCredentialdefaultApplicationProblemPlusJSONResponse) VisitGetCredentialResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateCredentialRequestObject struct {
	CredentialId uuid.UUID `json:"credential_id" binding:"required"`
	Body         *UpdateCredentialJSONRequestBody
}

type UpdateCredentialResponseObject interface {
	VisitUpdateCredentialResponse(w http.ResponseWriter) error
}

type UpdateCredential200JSONResponse Credential

func (response UpdateCredential200JSONResponse) VisitUpdateCredentialResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateCredential401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response UpdateCredential401ApplicationProblemPlusJSONResponse) VisitUpdateCredentialResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateCredential403ApplicationProblemPlusJSONResponse Problem

func (response UpdateCredential403ApplicationProblemPlusJSONResponse) VisitUpdateCredentialResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateCredential404ApplicationProblemPlusJSONResponse Problem

func (response UpdateCredential404ApplicationProblemPlusJSONResponse) VisitUpdateCredentialResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateCredential422ApplicationProblemPlusJSONResponse Problem

func (response UpdateCredential422ApplicationProblemPlusJSONResponse) VisitUpdateCredentialResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateCredentialdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response UpdateCredentialdefaultApplicationProblemPlusJSONResponse) VisitUpdateCredentialResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type ListDepartmentsRequestObject struct {
}

type ListDepartmentsResponseObject interface {
	VisitListDepartmentsResponse(w http.ResponseWriter) error
}

type ListDepartments200JSONResponse struct {
	Data []Department `json:"data" binding:"required"`
}

func (response ListDepartments200JSONResponse) VisitListDepartmentsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type ListDepartments401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response ListDepartments401ApplicationProblemPlusJSONResponse) VisitListDepartmentsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type ListDepartments403ApplicationProblemPlusJSONResponse Problem

func (response ListDepartments403ApplicationProblemPlusJSONResponse) VisitListDepartmentsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type ListDepartmentsdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response ListDepartmentsdefaultApplicationProblemPlusJSONResponse) VisitListDepartmentsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type CreateDepartmentRequestObject struct {
	Body *CreateDepartmentJSONRequestBody
}

type CreateDepartmentResponseObject interface {
	VisitCreateDepartmentResponse(w http.ResponseWriter) error
}

type CreateDepartment201JSONResponse Department

func (response CreateDepartment201JSONResponse) VisitCreateDepartmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	_, err := buf.WriteTo(w)
	return err
}

type CreateDepartment402ApplicationProblemPlusJSONResponse struct {
	PlanProblemApplicationProblemPlusJSONResponse
}

func (response CreateDepartment402ApplicationProblemPlusJSONResponse) VisitCreateDepartmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(402)
	_, err := buf.WriteTo(w)
	return err
}

type CreateDepartment403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response CreateDepartment403ApplicationProblemPlusJSONResponse) VisitCreateDepartmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type CreateDepartment409ApplicationProblemPlusJSONResponse Problem

func (response CreateDepartment409ApplicationProblemPlusJSONResponse) VisitCreateDepartmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(409)
	_, err := buf.WriteTo(w)
	return err
}

type CreateDepartment422ApplicationProblemPlusJSONResponse Problem

func (response CreateDepartment422ApplicationProblemPlusJSONResponse) VisitCreateDepartmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type CreateDepartmentdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response CreateDepartmentdefaultApplicationProblemPlusJSONResponse) VisitCreateDepartmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteDepartmentRequestObject struct {
	DepartmentId uuid.UUID `json:"department_id" binding:"required"`
}

type DeleteDepartmentResponseObject interface {
	VisitDeleteDepartmentResponse(w http.ResponseWriter) error
}

type DeleteDepartment204Response struct {
}

func (response DeleteDepartment204Response) VisitDeleteDepartmentResponse(w http.ResponseWriter) error {
	w.WriteHeader(204)
	return nil
}

type DeleteDepartment403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response DeleteDepartment403ApplicationProblemPlusJSONResponse) VisitDeleteDepartmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteDepartment404ApplicationProblemPlusJSONResponse Problem

func (response DeleteDepartment404ApplicationProblemPlusJSONResponse) VisitDeleteDepartmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteDepartment409ApplicationProblemPlusJSONResponse Problem

func (response DeleteDepartment409ApplicationProblemPlusJSONResponse) VisitDeleteDepartmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(409)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteDepartmentdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response DeleteDepartmentdefaultApplicationProblemPlusJSONResponse) VisitDeleteDepartmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateDepartmentRequestObject struct {
	DepartmentId uuid.UUID `json:"department_id" binding:"required"`
	Body         *UpdateDepartmentJSONRequestBody
}

type UpdateDepartmentResponseObject interface {
	VisitUpdateDepartmentResponse(w http.ResponseWriter) error
}

type UpdateDepartment200JSONResponse Department

func (response UpdateDepartment200JSONResponse) VisitUpdateDepartmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateDepartment402ApplicationProblemPlusJSONResponse struct {
	PlanProblemApplicationProblemPlusJSONResponse
}

func (response UpdateDepartment402ApplicationProblemPlusJSONResponse) VisitUpdateDepartmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(402)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateDepartment403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response UpdateDepartment403ApplicationProblemPlusJSONResponse) VisitUpdateDepartmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateDepartment404ApplicationProblemPlusJSONResponse Problem

func (response UpdateDepartment404ApplicationProblemPlusJSONResponse) VisitUpdateDepartmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateDepartment422ApplicationProblemPlusJSONResponse Problem

func (response UpdateDepartment422ApplicationProblemPlusJSONResponse) VisitUpdateDepartmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateDepartmentdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response UpdateDepartmentdefaultApplicationProblemPlusJSONResponse) VisitUpdateDepartmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type GetEntitlementsRequestObject struct {
}

type GetEntitlementsResponseObject interface {
	VisitGetEntitlementsResponse(w http.ResponseWriter) error
}

type GetEntitlements200JSONResponse Entitlements

func (response GetEntitlements200JSONResponse) VisitGetEntitlementsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type GetEntitlements403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response GetEntitlements403ApplicationProblemPlusJSONResponse) VisitGetEntitlementsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type GetEntitlementsdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response GetEntitlementsdefaultApplicationProblemPlusJSONResponse) VisitGetEntitlementsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type ListEnvironmentsRequestObject struct {
}

type ListEnvironmentsResponseObject interface {
	VisitListEnvironmentsResponse(w http.ResponseWriter) error
}

type ListEnvironments200JSONResponse struct {
	Data []Environment `json:"data" binding:"required"`
}

func (response ListEnvironments200JSONResponse) VisitListEnvironmentsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type ListEnvironments401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response ListEnvironments401ApplicationProblemPlusJSONResponse) VisitListEnvironmentsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type ListEnvironments403ApplicationProblemPlusJSONResponse Problem

func (response ListEnvironments403ApplicationProblemPlusJSONResponse) VisitListEnvironmentsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type ListEnvironmentsdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response ListEnvironmentsdefaultApplicationProblemPlusJSONResponse) VisitListEnvironmentsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type CreateEnvironmentRequestObject struct {
	Body *CreateEnvironmentJSONRequestBody
}

type CreateEnvironmentResponseObject interface {
	VisitCreateEnvironmentResponse(w http.ResponseWriter) error
}

type CreateEnvironment201JSONResponse Environment

func (response CreateEnvironment201JSONResponse) VisitCreateEnvironmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	_, err := buf.WriteTo(w)
	return err
}

type CreateEnvironment402ApplicationProblemPlusJSONResponse struct {
	PlanProblemApplicationProblemPlusJSONResponse
}

func (response CreateEnvironment402ApplicationProblemPlusJSONResponse) VisitCreateEnvironmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(402)
	_, err := buf.WriteTo(w)
	return err
}

type CreateEnvironment403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response CreateEnvironment403ApplicationProblemPlusJSONResponse) VisitCreateEnvironmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type CreateEnvironment409ApplicationProblemPlusJSONResponse Problem

func (response CreateEnvironment409ApplicationProblemPlusJSONResponse) VisitCreateEnvironmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(409)
	_, err := buf.WriteTo(w)
	return err
}

type CreateEnvironment422ApplicationProblemPlusJSONResponse Problem

func (response CreateEnvironment422ApplicationProblemPlusJSONResponse) VisitCreateEnvironmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type CreateEnvironmentdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response CreateEnvironmentdefaultApplicationProblemPlusJSONResponse) VisitCreateEnvironmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteEnvironmentRequestObject struct {
	EnvironmentId uuid.UUID `json:"environment_id" binding:"required"`
}

type DeleteEnvironmentResponseObject interface {
	VisitDeleteEnvironmentResponse(w http.ResponseWriter) error
}

type DeleteEnvironment204Response struct {
}

func (response DeleteEnvironment204Response) VisitDeleteEnvironmentResponse(w http.ResponseWriter) error {
	w.WriteHeader(204)
	return nil
}

type DeleteEnvironment403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response DeleteEnvironment403ApplicationProblemPlusJSONResponse) VisitDeleteEnvironmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteEnvironment404ApplicationProblemPlusJSONResponse Problem

func (response DeleteEnvironment404ApplicationProblemPlusJSONResponse) VisitDeleteEnvironmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteEnvironment409ApplicationProblemPlusJSONResponse Problem

func (response DeleteEnvironment409ApplicationProblemPlusJSONResponse) VisitDeleteEnvironmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(409)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteEnvironmentdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response DeleteEnvironmentdefaultApplicationProblemPlusJSONResponse) VisitDeleteEnvironmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateEnvironmentRequestObject struct {
	EnvironmentId uuid.UUID `json:"environment_id" binding:"required"`
	Body          *UpdateEnvironmentJSONRequestBody
}

type UpdateEnvironmentResponseObject interface {
	VisitUpdateEnvironmentResponse(w http.ResponseWriter) error
}

type UpdateEnvironment200JSONResponse Environment

func (response UpdateEnvironment200JSONResponse) VisitUpdateEnvironmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateEnvironment402ApplicationProblemPlusJSONResponse struct {
	PlanProblemApplicationProblemPlusJSONResponse
}

func (response UpdateEnvironment402ApplicationProblemPlusJSONResponse) VisitUpdateEnvironmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(402)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateEnvironment403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response UpdateEnvironment403ApplicationProblemPlusJSONResponse) VisitUpdateEnvironmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateEnvironment404ApplicationProblemPlusJSONResponse Problem

func (response UpdateEnvironment404ApplicationProblemPlusJSONResponse) VisitUpdateEnvironmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateEnvironment422ApplicationProblemPlusJSONResponse Problem

func (response UpdateEnvironment422ApplicationProblemPlusJSONResponse) VisitUpdateEnvironmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateEnvironmentdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response UpdateEnvironmentdefaultApplicationProblemPlusJSONResponse) VisitUpdateEnvironmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type ListGroupsRequestObject struct {
}

type ListGroupsResponseObject interface {
	VisitListGroupsResponse(w http.ResponseWriter) error
}

type ListGroups200JSONResponse struct {
	Data []Group `json:"data" binding:"required"`
}

func (response ListGroups200JSONResponse) VisitListGroupsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type ListGroups403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response ListGroups403ApplicationProblemPlusJSONResponse) VisitListGroupsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type ListGroupsdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response ListGroupsdefaultApplicationProblemPlusJSONResponse) VisitListGroupsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type CreateGroupRequestObject struct {
	Body *CreateGroupJSONRequestBody
}

type CreateGroupResponseObject interface {
	VisitCreateGroupResponse(w http.ResponseWriter) error
}

type CreateGroup201JSONResponse Group

func (response CreateGroup201JSONResponse) VisitCreateGroupResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	_, err := buf.WriteTo(w)
	return err
}

type CreateGroup403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response CreateGroup403ApplicationProblemPlusJSONResponse) VisitCreateGroupResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type CreateGroup409ApplicationProblemPlusJSONResponse Problem

func (response CreateGroup409ApplicationProblemPlusJSONResponse) VisitCreateGroupResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(409)
	_, err := buf.WriteTo(w)
	return err
}

type CreateGroup422ApplicationProblemPlusJSONResponse Problem

func (response CreateGroup422ApplicationProblemPlusJSONResponse) VisitCreateGroupResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type CreateGroupdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response CreateGroupdefaultApplicationProblemPlusJSONResponse) VisitCreateGroupResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteGroupRequestObject struct {
	GroupId uuid.UUID `json:"group_id" binding:"required"`
}

type DeleteGroupResponseObject interface {
	VisitDeleteGroupResponse(w http.ResponseWriter) error
}

type DeleteGroup204Response struct {
}

func (response DeleteGroup204Response) VisitDeleteGroupResponse(w http.ResponseWriter) error {
	w.WriteHeader(204)
	return nil
}

type DeleteGroup403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response DeleteGroup403ApplicationProblemPlusJSONResponse) VisitDeleteGroupResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteGroup404ApplicationProblemPlusJSONResponse Problem

func (response DeleteGroup404ApplicationProblemPlusJSONResponse) VisitDeleteGroupResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteGroupdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response DeleteGroupdefaultApplicationProblemPlusJSONResponse) VisitDeleteGroupResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type GetGroupRequestObject struct {
	GroupId uuid.UUID `json:"group_id" binding:"required"`
}

type GetGroupResponseObject interface {
	VisitGetGroupResponse(w http.ResponseWriter) error
}

type GetGroup200JSONResponse Group

func (response GetGroup200JSONResponse) VisitGetGroupResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type GetGroup403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response GetGroup403ApplicationProblemPlusJSONResponse) VisitGetGroupResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type GetGroup404ApplicationProblemPlusJSONResponse Problem

func (response GetGroup404ApplicationProblemPlusJSONResponse) VisitGetGroupResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type GetGroupdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response GetGroupdefaultApplicationProblemPlusJSONResponse) VisitGetGroupResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateGroupRequestObject struct {
	GroupId uuid.UUID `json:"group_id" binding:"required"`
	Body    *UpdateGroupJSONRequestBody
}

type UpdateGroupResponseObject interface {
	VisitUpdateGroupResponse(w http.ResponseWriter) error
}

type UpdateGroup200JSONResponse Group

func (response UpdateGroup200JSONResponse) VisitUpdateGroupResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateGroup403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response UpdateGroup403ApplicationProblemPlusJSONResponse) VisitUpdateGroupResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateGroup404ApplicationProblemPlusJSONResponse Problem

func (response UpdateGroup404ApplicationProblemPlusJSONResponse) VisitUpdateGroupResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateGroup409ApplicationProblemPlusJSONResponse Problem

func (response UpdateGroup409ApplicationProblemPlusJSONResponse) VisitUpdateGroupResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(409)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateGroup422ApplicationProblemPlusJSONResponse Problem

func (response UpdateGroup422ApplicationProblemPlusJSONResponse) VisitUpdateGroupResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateGroupdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response UpdateGroupdefaultApplicationProblemPlusJSONResponse) VisitUpdateGroupResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type ListGroupMembersRequestObject struct {
	GroupId uuid.UUID `json:"group_id" binding:"required"`
}

type ListGroupMembersResponseObject interface {
	VisitListGroupMembersResponse(w http.ResponseWriter) error
}

type ListGroupMembers200JSONResponse struct {
	Data []GroupMember `json:"data" binding:"required"`
}

func (response ListGroupMembers200JSONResponse) VisitListGroupMembersResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type ListGroupMembers403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response ListGroupMembers403ApplicationProblemPlusJSONResponse) VisitListGroupMembersResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type ListGroupMembers404ApplicationProblemPlusJSONResponse Problem

func (response ListGroupMembers404ApplicationProblemPlusJSONResponse) VisitListGroupMembersResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type ListGroupMembersdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response ListGroupMembersdefaultApplicationProblemPlusJSONResponse) VisitListGroupMembersResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type AddGroupMemberRequestObject struct {
	GroupId uuid.UUID `json:"group_id" binding:"required"`
	Body    *AddGroupMemberJSONRequestBody
}

type AddGroupMemberResponseObject interface {
	VisitAddGroupMemberResponse(w http.ResponseWriter) error
}

type AddGroupMember204Response struct {
}

func (response AddGroupMember204Response) VisitAddGroupMemberResponse(w http.ResponseWriter) error {
	w.WriteHeader(204)
	return nil
}

type AddGroupMember403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response AddGroupMember403ApplicationProblemPlusJSONResponse) VisitAddGroupMemberResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type AddGroupMember404ApplicationProblemPlusJSONResponse Problem

func (response AddGroupMember404ApplicationProblemPlusJSONResponse) VisitAddGroupMemberResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type AddGroupMember422ApplicationProblemPlusJSONResponse Problem

func (response AddGroupMember422ApplicationProblemPlusJSONResponse) VisitAddGroupMemberResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type AddGroupMemberdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response AddGroupMemberdefaultApplicationProblemPlusJSONResponse) VisitAddGroupMemberResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type RemoveGroupMemberRequestObject struct {
	GroupId uuid.UUID `json:"group_id" binding:"required"`
	UserId  uuid.UUID `json:"user_id" binding:"required"`
}

type RemoveGroupMemberResponseObject interface {
	VisitRemoveGroupMemberResponse(w http.ResponseWriter) error
}

type RemoveGroupMember204Response struct {
}

func (response RemoveGroupMember204Response) VisitRemoveGroupMemberResponse(w http.ResponseWriter) error {
	w.WriteHeader(204)
	return nil
}

type RemoveGroupMember403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response RemoveGroupMember403ApplicationProblemPlusJSONResponse) VisitRemoveGroupMemberResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type RemoveGroupMember404ApplicationProblemPlusJSONResponse Problem

func (response RemoveGroupMember404ApplicationProblemPlusJSONResponse) VisitRemoveGroupMemberResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type RemoveGroupMemberdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response RemoveGroupMemberdefaultApplicationProblemPlusJSONResponse) VisitRemoveGroupMemberResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type ListIdentityProvidersRequestObject struct {
}

type ListIdentityProvidersResponseObject interface {
	VisitListIdentityProvidersResponse(w http.ResponseWriter) error
}

type ListIdentityProviders200JSONResponse struct {
	Data []IdentityProvider `json:"data" binding:"required"`
}

func (response ListIdentityProviders200JSONResponse) VisitListIdentityProvidersResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type ListIdentityProviders403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response ListIdentityProviders403ApplicationProblemPlusJSONResponse) VisitListIdentityProvidersResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type ListIdentityProvidersdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response ListIdentityProvidersdefaultApplicationProblemPlusJSONResponse) VisitListIdentityProvidersResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type CreateIdentityProviderRequestObject struct {
	Body *CreateIdentityProviderJSONRequestBody
}

type CreateIdentityProviderResponseObject interface {
	VisitCreateIdentityProviderResponse(w http.ResponseWriter) error
}

type CreateIdentityProvider201JSONResponse IdentityProvider

func (response CreateIdentityProvider201JSONResponse) VisitCreateIdentityProviderResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	_, err := buf.WriteTo(w)
	return err
}

type CreateIdentityProvider403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response CreateIdentityProvider403ApplicationProblemPlusJSONResponse) VisitCreateIdentityProviderResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type CreateIdentityProvider409ApplicationProblemPlusJSONResponse Problem

func (response CreateIdentityProvider409ApplicationProblemPlusJSONResponse) VisitCreateIdentityProviderResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(409)
	_, err := buf.WriteTo(w)
	return err
}

type CreateIdentityProvider422ApplicationProblemPlusJSONResponse Problem

func (response CreateIdentityProvider422ApplicationProblemPlusJSONResponse) VisitCreateIdentityProviderResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type CreateIdentityProviderdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response CreateIdentityProviderdefaultApplicationProblemPlusJSONResponse) VisitCreateIdentityProviderResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteIdentityProviderRequestObject struct {
	ProviderId uuid.UUID `json:"provider_id" binding:"required"`
}

type DeleteIdentityProviderResponseObject interface {
	VisitDeleteIdentityProviderResponse(w http.ResponseWriter) error
}

type DeleteIdentityProvider204Response struct {
}

func (response DeleteIdentityProvider204Response) VisitDeleteIdentityProviderResponse(w http.ResponseWriter) error {
	w.WriteHeader(204)
	return nil
}

type DeleteIdentityProvider403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response DeleteIdentityProvider403ApplicationProblemPlusJSONResponse) VisitDeleteIdentityProviderResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteIdentityProvider404ApplicationProblemPlusJSONResponse Problem

func (response DeleteIdentityProvider404ApplicationProblemPlusJSONResponse) VisitDeleteIdentityProviderResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteIdentityProviderdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response DeleteIdentityProviderdefaultApplicationProblemPlusJSONResponse) VisitDeleteIdentityProviderResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type GetIdentityProviderRequestObject struct {
	ProviderId uuid.UUID `json:"provider_id" binding:"required"`
}

type GetIdentityProviderResponseObject interface {
	VisitGetIdentityProviderResponse(w http.ResponseWriter) error
}

type GetIdentityProvider200JSONResponse IdentityProvider

func (response GetIdentityProvider200JSONResponse) VisitGetIdentityProviderResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type GetIdentityProvider403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response GetIdentityProvider403ApplicationProblemPlusJSONResponse) VisitGetIdentityProviderResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type GetIdentityProvider404ApplicationProblemPlusJSONResponse Problem

func (response GetIdentityProvider404ApplicationProblemPlusJSONResponse) VisitGetIdentityProviderResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type GetIdentityProviderdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response GetIdentityProviderdefaultApplicationProblemPlusJSONResponse) VisitGetIdentityProviderResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateIdentityProviderRequestObject struct {
	ProviderId uuid.UUID `json:"provider_id" binding:"required"`
	Body       *UpdateIdentityProviderJSONRequestBody
}

type UpdateIdentityProviderResponseObject interface {
	VisitUpdateIdentityProviderResponse(w http.ResponseWriter) error
}

type UpdateIdentityProvider200JSONResponse IdentityProvider

func (response UpdateIdentityProvider200JSONResponse) VisitUpdateIdentityProviderResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateIdentityProvider403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response UpdateIdentityProvider403ApplicationProblemPlusJSONResponse) VisitUpdateIdentityProviderResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateIdentityProvider404ApplicationProblemPlusJSONResponse Problem

func (response UpdateIdentityProvider404ApplicationProblemPlusJSONResponse) VisitUpdateIdentityProviderResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateIdentityProvider409ApplicationProblemPlusJSONResponse Problem

func (response UpdateIdentityProvider409ApplicationProblemPlusJSONResponse) VisitUpdateIdentityProviderResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(409)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateIdentityProvider422ApplicationProblemPlusJSONResponse Problem

func (response UpdateIdentityProvider422ApplicationProblemPlusJSONResponse) VisitUpdateIdentityProviderResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateIdentityProviderdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response UpdateIdentityProviderdefaultApplicationProblemPlusJSONResponse) VisitUpdateIdentityProviderResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type ListGroupMappingsRequestObject struct {
	ProviderId uuid.UUID `json:"provider_id" binding:"required"`
}

type ListGroupMappingsResponseObject interface {
	VisitListGroupMappingsResponse(w http.ResponseWriter) error
}

type ListGroupMappings200JSONResponse struct {
	Data []GroupMapping `json:"data" binding:"required"`
}

func (response ListGroupMappings200JSONResponse) VisitListGroupMappingsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type ListGroupMappings403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response ListGroupMappings403ApplicationProblemPlusJSONResponse) VisitListGroupMappingsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type ListGroupMappings404ApplicationProblemPlusJSONResponse Problem

func (response ListGroupMappings404ApplicationProblemPlusJSONResponse) VisitListGroupMappingsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type ListGroupMappingsdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response ListGroupMappingsdefaultApplicationProblemPlusJSONResponse) VisitListGroupMappingsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type CreateGroupMappingRequestObject struct {
	ProviderId uuid.UUID `json:"provider_id" binding:"required"`
	Body       *CreateGroupMappingJSONRequestBody
}

type CreateGroupMappingResponseObject interface {
	VisitCreateGroupMappingResponse(w http.ResponseWriter) error
}

type CreateGroupMapping201JSONResponse GroupMapping

func (response CreateGroupMapping201JSONResponse) VisitCreateGroupMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	_, err := buf.WriteTo(w)
	return err
}

type CreateGroupMapping403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response CreateGroupMapping403ApplicationProblemPlusJSONResponse) VisitCreateGroupMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type CreateGroupMapping404ApplicationProblemPlusJSONResponse Problem

func (response CreateGroupMapping404ApplicationProblemPlusJSONResponse) VisitCreateGroupMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type CreateGroupMapping409ApplicationProblemPlusJSONResponse Problem

func (response CreateGroupMapping409ApplicationProblemPlusJSONResponse) VisitCreateGroupMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(409)
	_, err := buf.WriteTo(w)
	return err
}

type CreateGroupMapping422ApplicationProblemPlusJSONResponse Problem

func (response CreateGroupMapping422ApplicationProblemPlusJSONResponse) VisitCreateGroupMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type CreateGroupMappingdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response CreateGroupMappingdefaultApplicationProblemPlusJSONResponse) VisitCreateGroupMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteGroupMappingRequestObject struct {
	ProviderId uuid.UUID `json:"provider_id" binding:"required"`
	MappingId  uuid.UUID `json:"mapping_id" binding:"required"`
}

type DeleteGroupMappingResponseObject interface {
	VisitDeleteGroupMappingResponse(w http.ResponseWriter) error
}

type DeleteGroupMapping204Response struct {
}

func (response DeleteGroupMapping204Response) VisitDeleteGroupMappingResponse(w http.ResponseWriter) error {
	w.WriteHeader(204)
	return nil
}

type DeleteGroupMapping403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response DeleteGroupMapping403ApplicationProblemPlusJSONResponse) VisitDeleteGroupMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteGroupMapping404ApplicationProblemPlusJSONResponse Problem

func (response DeleteGroupMapping404ApplicationProblemPlusJSONResponse) VisitDeleteGroupMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteGroupMappingdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response DeleteGroupMappingdefaultApplicationProblemPlusJSONResponse) VisitDeleteGroupMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type TestIdentityProviderRequestObject struct {
	ProviderId uuid.UUID `json:"provider_id" binding:"required"`
}

type TestIdentityProviderResponseObject interface {
	VisitTestIdentityProviderResponse(w http.ResponseWriter) error
}

type TestIdentityProvider200JSONResponse IdentityProviderTest

func (response TestIdentityProvider200JSONResponse) VisitTestIdentityProviderResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type TestIdentityProvider403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response TestIdentityProvider403ApplicationProblemPlusJSONResponse) VisitTestIdentityProviderResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type TestIdentityProvider404ApplicationProblemPlusJSONResponse Problem

func (response TestIdentityProvider404ApplicationProblemPlusJSONResponse) VisitTestIdentityProviderResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type TestIdentityProviderdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response TestIdentityProviderdefaultApplicationProblemPlusJSONResponse) VisitTestIdentityProviderResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type GetInvitationPreviewRequestObject struct {
	Token string `json:"token" binding:"required"`
}

type GetInvitationPreviewResponseObject interface {
	VisitGetInvitationPreviewResponse(w http.ResponseWriter) error
}

type GetInvitationPreview200JSONResponse InvitationPreview

func (response GetInvitationPreview200JSONResponse) VisitGetInvitationPreviewResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type GetInvitationPreview404ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response GetInvitationPreview404ApplicationProblemPlusJSONResponse) VisitGetInvitationPreviewResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type GetInvitationPreview410ApplicationProblemPlusJSONResponse Problem

func (response GetInvitationPreview410ApplicationProblemPlusJSONResponse) VisitGetInvitationPreviewResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(410)
	_, err := buf.WriteTo(w)
	return err
}

type GetInvitationPreviewdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response GetInvitationPreviewdefaultApplicationProblemPlusJSONResponse) VisitGetInvitationPreviewResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type AcceptInvitationRequestObject struct {
	Token string `json:"token" binding:"required"`
	Body  *AcceptInvitationJSONRequestBody
}

type AcceptInvitationResponseObject interface {
	VisitAcceptInvitationResponse(w http.ResponseWriter) error
}

type AcceptInvitation200JSONResponse AuthResponse

func (response AcceptInvitation200JSONResponse) VisitAcceptInvitationResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type AcceptInvitation401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response AcceptInvitation401ApplicationProblemPlusJSONResponse) VisitAcceptInvitationResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type AcceptInvitation404ApplicationProblemPlusJSONResponse Problem

func (response AcceptInvitation404ApplicationProblemPlusJSONResponse) VisitAcceptInvitationResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type AcceptInvitation409ApplicationProblemPlusJSONResponse Problem

func (response AcceptInvitation409ApplicationProblemPlusJSONResponse) VisitAcceptInvitationResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(409)
	_, err := buf.WriteTo(w)
	return err
}

type AcceptInvitation410ApplicationProblemPlusJSONResponse Problem

func (response AcceptInvitation410ApplicationProblemPlusJSONResponse) VisitAcceptInvitationResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(410)
	_, err := buf.WriteTo(w)
	return err
}

type AcceptInvitation422ApplicationProblemPlusJSONResponse Problem

func (response AcceptInvitation422ApplicationProblemPlusJSONResponse) VisitAcceptInvitationResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type AcceptInvitation429ApplicationProblemPlusJSONResponse Problem

func (response AcceptInvitation429ApplicationProblemPlusJSONResponse) VisitAcceptInvitationResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(429)
	_, err := buf.WriteTo(w)
	return err
}

type AcceptInvitationdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response AcceptInvitationdefaultApplicationProblemPlusJSONResponse) VisitAcceptInvitationResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type ListInvitationsRequestObject struct {
	Params ListInvitationsParams
}

type ListInvitationsResponseObject interface {
	VisitListInvitationsResponse(w http.ResponseWriter) error
}

type ListInvitations200JSONResponse struct {
	Data []Invitation `json:"data" binding:"required"`
}

func (response ListInvitations200JSONResponse) VisitListInvitationsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type ListInvitations403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response ListInvitations403ApplicationProblemPlusJSONResponse) VisitListInvitationsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type ListInvitationsdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response ListInvitationsdefaultApplicationProblemPlusJSONResponse) VisitListInvitationsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type CreateInvitationRequestObject struct {
	Body *CreateInvitationJSONRequestBody
}

type CreateInvitationResponseObject interface {
	VisitCreateInvitationResponse(w http.ResponseWriter) error
}

type CreateInvitation201JSONResponse InvitationCreated

func (response CreateInvitation201JSONResponse) VisitCreateInvitationResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	_, err := buf.WriteTo(w)
	return err
}

type CreateInvitation403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response CreateInvitation403ApplicationProblemPlusJSONResponse) VisitCreateInvitationResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type CreateInvitation409ApplicationProblemPlusJSONResponse Problem

func (response CreateInvitation409ApplicationProblemPlusJSONResponse) VisitCreateInvitationResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(409)
	_, err := buf.WriteTo(w)
	return err
}

type CreateInvitation422ApplicationProblemPlusJSONResponse Problem

func (response CreateInvitation422ApplicationProblemPlusJSONResponse) VisitCreateInvitationResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type CreateInvitationdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response CreateInvitationdefaultApplicationProblemPlusJSONResponse) VisitCreateInvitationResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type RevokeInvitationRequestObject struct {
	InvitationId uuid.UUID `json:"invitation_id" binding:"required"`
}

type RevokeInvitationResponseObject interface {
	VisitRevokeInvitationResponse(w http.ResponseWriter) error
}

type RevokeInvitation204Response struct {
}

func (response RevokeInvitation204Response) VisitRevokeInvitationResponse(w http.ResponseWriter) error {
	w.WriteHeader(204)
	return nil
}

type RevokeInvitation403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response RevokeInvitation403ApplicationProblemPlusJSONResponse) VisitRevokeInvitationResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type RevokeInvitation404ApplicationProblemPlusJSONResponse Problem

func (response RevokeInvitation404ApplicationProblemPlusJSONResponse) VisitRevokeInvitationResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type RevokeInvitation409ApplicationProblemPlusJSONResponse Problem

func (response RevokeInvitation409ApplicationProblemPlusJSONResponse) VisitRevokeInvitationResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(409)
	_, err := buf.WriteTo(w)
	return err
}

type RevokeInvitationdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response RevokeInvitationdefaultApplicationProblemPlusJSONResponse) VisitRevokeInvitationResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type ListMappingsRequestObject struct {
	Params ListMappingsParams
}

type ListMappingsResponseObject interface {
	VisitListMappingsResponse(w http.ResponseWriter) error
}

type ListMappings200JSONResponse struct {
	Data []Mapping `json:"data" binding:"required"`
}

func (response ListMappings200JSONResponse) VisitListMappingsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type ListMappings401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response ListMappings401ApplicationProblemPlusJSONResponse) VisitListMappingsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type ListMappings403ApplicationProblemPlusJSONResponse Problem

func (response ListMappings403ApplicationProblemPlusJSONResponse) VisitListMappingsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type ListMappingsdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response ListMappingsdefaultApplicationProblemPlusJSONResponse) VisitListMappingsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type CreateMappingRequestObject struct {
	Body *CreateMappingJSONRequestBody
}

type CreateMappingResponseObject interface {
	VisitCreateMappingResponse(w http.ResponseWriter) error
}

type CreateMapping201JSONResponse Mapping

func (response CreateMapping201JSONResponse) VisitCreateMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	_, err := buf.WriteTo(w)
	return err
}

type CreateMapping401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response CreateMapping401ApplicationProblemPlusJSONResponse) VisitCreateMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type CreateMapping403ApplicationProblemPlusJSONResponse Problem

func (response CreateMapping403ApplicationProblemPlusJSONResponse) VisitCreateMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type CreateMapping422ApplicationProblemPlusJSONResponse Problem

func (response CreateMapping422ApplicationProblemPlusJSONResponse) VisitCreateMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type CreateMappingdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response CreateMappingdefaultApplicationProblemPlusJSONResponse) VisitCreateMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteMappingRequestObject struct {
	MappingId uuid.UUID `json:"mapping_id" binding:"required"`
}

type DeleteMappingResponseObject interface {
	VisitDeleteMappingResponse(w http.ResponseWriter) error
}

type DeleteMapping204Response struct {
}

func (response DeleteMapping204Response) VisitDeleteMappingResponse(w http.ResponseWriter) error {
	w.WriteHeader(204)
	return nil
}

type DeleteMapping401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response DeleteMapping401ApplicationProblemPlusJSONResponse) VisitDeleteMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteMapping403ApplicationProblemPlusJSONResponse Problem

func (response DeleteMapping403ApplicationProblemPlusJSONResponse) VisitDeleteMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteMapping404ApplicationProblemPlusJSONResponse Problem

func (response DeleteMapping404ApplicationProblemPlusJSONResponse) VisitDeleteMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteMappingdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response DeleteMappingdefaultApplicationProblemPlusJSONResponse) VisitDeleteMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type GetMappingRequestObject struct {
	MappingId uuid.UUID `json:"mapping_id" binding:"required"`
}

type GetMappingResponseObject interface {
	VisitGetMappingResponse(w http.ResponseWriter) error
}

type GetMapping200JSONResponse Mapping

func (response GetMapping200JSONResponse) VisitGetMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type GetMapping401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response GetMapping401ApplicationProblemPlusJSONResponse) VisitGetMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type GetMapping403ApplicationProblemPlusJSONResponse Problem

func (response GetMapping403ApplicationProblemPlusJSONResponse) VisitGetMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type GetMapping404ApplicationProblemPlusJSONResponse Problem

func (response GetMapping404ApplicationProblemPlusJSONResponse) VisitGetMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type GetMappingdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response GetMappingdefaultApplicationProblemPlusJSONResponse) VisitGetMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateMappingRequestObject struct {
	MappingId uuid.UUID `json:"mapping_id" binding:"required"`
	Body      *UpdateMappingJSONRequestBody
}

type UpdateMappingResponseObject interface {
	VisitUpdateMappingResponse(w http.ResponseWriter) error
}

type UpdateMapping200JSONResponse Mapping

func (response UpdateMapping200JSONResponse) VisitUpdateMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateMapping401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response UpdateMapping401ApplicationProblemPlusJSONResponse) VisitUpdateMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateMapping403ApplicationProblemPlusJSONResponse Problem

func (response UpdateMapping403ApplicationProblemPlusJSONResponse) VisitUpdateMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateMapping404ApplicationProblemPlusJSONResponse Problem

func (response UpdateMapping404ApplicationProblemPlusJSONResponse) VisitUpdateMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateMapping422ApplicationProblemPlusJSONResponse Problem

func (response UpdateMapping422ApplicationProblemPlusJSONResponse) VisitUpdateMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateMappingdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response UpdateMappingdefaultApplicationProblemPlusJSONResponse) VisitUpdateMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type PublishMappingRequestObject struct {
	MappingId uuid.UUID `json:"mapping_id" binding:"required"`
}

type PublishMappingResponseObject interface {
	VisitPublishMappingResponse(w http.ResponseWriter) error
}

type PublishMapping201JSONResponse MappingVersion

func (response PublishMapping201JSONResponse) VisitPublishMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	_, err := buf.WriteTo(w)
	return err
}

type PublishMapping401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response PublishMapping401ApplicationProblemPlusJSONResponse) VisitPublishMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type PublishMapping403ApplicationProblemPlusJSONResponse Problem

func (response PublishMapping403ApplicationProblemPlusJSONResponse) VisitPublishMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type PublishMapping404ApplicationProblemPlusJSONResponse Problem

func (response PublishMapping404ApplicationProblemPlusJSONResponse) VisitPublishMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type PublishMapping422ApplicationProblemPlusJSONResponse Problem

func (response PublishMapping422ApplicationProblemPlusJSONResponse) VisitPublishMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type PublishMappingdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response PublishMappingdefaultApplicationProblemPlusJSONResponse) VisitPublishMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type TestMappingRequestObject struct {
	MappingId uuid.UUID `json:"mapping_id" binding:"required"`
	Body      *TestMappingJSONRequestBody
}

type TestMappingResponseObject interface {
	VisitTestMappingResponse(w http.ResponseWriter) error
}

type TestMapping200JSONResponse MappingTestResult

func (response TestMapping200JSONResponse) VisitTestMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type TestMapping401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response TestMapping401ApplicationProblemPlusJSONResponse) VisitTestMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type TestMapping403ApplicationProblemPlusJSONResponse Problem

func (response TestMapping403ApplicationProblemPlusJSONResponse) VisitTestMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type TestMapping404ApplicationProblemPlusJSONResponse Problem

func (response TestMapping404ApplicationProblemPlusJSONResponse) VisitTestMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type TestMapping422ApplicationProblemPlusJSONResponse Problem

func (response TestMapping422ApplicationProblemPlusJSONResponse) VisitTestMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type TestMappingdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response TestMappingdefaultApplicationProblemPlusJSONResponse) VisitTestMappingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type ListMappingTestsRequestObject struct {
	MappingId uuid.UUID `json:"mapping_id" binding:"required"`
}

type ListMappingTestsResponseObject interface {
	VisitListMappingTestsResponse(w http.ResponseWriter) error
}

type ListMappingTests200JSONResponse struct {
	Data []MappingTest `json:"data" binding:"required"`
}

func (response ListMappingTests200JSONResponse) VisitListMappingTestsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type ListMappingTests401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response ListMappingTests401ApplicationProblemPlusJSONResponse) VisitListMappingTestsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type ListMappingTests403ApplicationProblemPlusJSONResponse Problem

func (response ListMappingTests403ApplicationProblemPlusJSONResponse) VisitListMappingTestsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type ListMappingTests404ApplicationProblemPlusJSONResponse Problem

func (response ListMappingTests404ApplicationProblemPlusJSONResponse) VisitListMappingTestsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type ListMappingTestsdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response ListMappingTestsdefaultApplicationProblemPlusJSONResponse) VisitListMappingTestsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type CreateMappingTestRequestObject struct {
	MappingId uuid.UUID `json:"mapping_id" binding:"required"`
	Body      *CreateMappingTestJSONRequestBody
}

type CreateMappingTestResponseObject interface {
	VisitCreateMappingTestResponse(w http.ResponseWriter) error
}

type CreateMappingTest201JSONResponse MappingTest

func (response CreateMappingTest201JSONResponse) VisitCreateMappingTestResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	_, err := buf.WriteTo(w)
	return err
}

type CreateMappingTest401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response CreateMappingTest401ApplicationProblemPlusJSONResponse) VisitCreateMappingTestResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type CreateMappingTest403ApplicationProblemPlusJSONResponse Problem

func (response CreateMappingTest403ApplicationProblemPlusJSONResponse) VisitCreateMappingTestResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type CreateMappingTest422ApplicationProblemPlusJSONResponse Problem

func (response CreateMappingTest422ApplicationProblemPlusJSONResponse) VisitCreateMappingTestResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type CreateMappingTestdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response CreateMappingTestdefaultApplicationProblemPlusJSONResponse) VisitCreateMappingTestResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteMappingTestRequestObject struct {
	MappingId uuid.UUID `json:"mapping_id" binding:"required"`
	TestId    uuid.UUID `json:"test_id" binding:"required"`
}

type DeleteMappingTestResponseObject interface {
	VisitDeleteMappingTestResponse(w http.ResponseWriter) error
}

type DeleteMappingTest204Response struct {
}

func (response DeleteMappingTest204Response) VisitDeleteMappingTestResponse(w http.ResponseWriter) error {
	w.WriteHeader(204)
	return nil
}

type DeleteMappingTest401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response DeleteMappingTest401ApplicationProblemPlusJSONResponse) VisitDeleteMappingTestResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteMappingTest403ApplicationProblemPlusJSONResponse Problem

func (response DeleteMappingTest403ApplicationProblemPlusJSONResponse) VisitDeleteMappingTestResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteMappingTest404ApplicationProblemPlusJSONResponse Problem

func (response DeleteMappingTest404ApplicationProblemPlusJSONResponse) VisitDeleteMappingTestResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteMappingTestdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response DeleteMappingTestdefaultApplicationProblemPlusJSONResponse) VisitDeleteMappingTestResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateMappingTestRequestObject struct {
	MappingId uuid.UUID `json:"mapping_id" binding:"required"`
	TestId    uuid.UUID `json:"test_id" binding:"required"`
	Body      *UpdateMappingTestJSONRequestBody
}

type UpdateMappingTestResponseObject interface {
	VisitUpdateMappingTestResponse(w http.ResponseWriter) error
}

type UpdateMappingTest200JSONResponse MappingTest

func (response UpdateMappingTest200JSONResponse) VisitUpdateMappingTestResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateMappingTest401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response UpdateMappingTest401ApplicationProblemPlusJSONResponse) VisitUpdateMappingTestResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateMappingTest403ApplicationProblemPlusJSONResponse Problem

func (response UpdateMappingTest403ApplicationProblemPlusJSONResponse) VisitUpdateMappingTestResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateMappingTest404ApplicationProblemPlusJSONResponse Problem

func (response UpdateMappingTest404ApplicationProblemPlusJSONResponse) VisitUpdateMappingTestResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateMappingTest422ApplicationProblemPlusJSONResponse Problem

func (response UpdateMappingTest422ApplicationProblemPlusJSONResponse) VisitUpdateMappingTestResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateMappingTestdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response UpdateMappingTestdefaultApplicationProblemPlusJSONResponse) VisitUpdateMappingTestResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type ListMappingVersionsRequestObject struct {
	MappingId uuid.UUID `json:"mapping_id" binding:"required"`
}

type ListMappingVersionsResponseObject interface {
	VisitListMappingVersionsResponse(w http.ResponseWriter) error
}

type ListMappingVersions200JSONResponse struct {
	Data []MappingVersion `json:"data" binding:"required"`
}

func (response ListMappingVersions200JSONResponse) VisitListMappingVersionsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type ListMappingVersions401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response ListMappingVersions401ApplicationProblemPlusJSONResponse) VisitListMappingVersionsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type ListMappingVersions403ApplicationProblemPlusJSONResponse Problem

func (response ListMappingVersions403ApplicationProblemPlusJSONResponse) VisitListMappingVersionsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type ListMappingVersions404ApplicationProblemPlusJSONResponse Problem

func (response ListMappingVersions404ApplicationProblemPlusJSONResponse) VisitListMappingVersionsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type ListMappingVersionsdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response ListMappingVersionsdefaultApplicationProblemPlusJSONResponse) VisitListMappingVersionsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type GetMeRequestObject struct {
}

type GetMeResponseObject interface {
	VisitGetMeResponse(w http.ResponseWriter) error
}

type GetMe200JSONResponse Me

func (response GetMe200JSONResponse) VisitGetMeResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type GetMe401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response GetMe401ApplicationProblemPlusJSONResponse) VisitGetMeResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type GetMedefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response GetMedefaultApplicationProblemPlusJSONResponse) VisitGetMeResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type GetMyPermissionsRequestObject struct {
}

type GetMyPermissionsResponseObject interface {
	VisitGetMyPermissionsResponse(w http.ResponseWriter) error
}

type GetMyPermissions200JSONResponse EffectivePermissions

func (response GetMyPermissions200JSONResponse) VisitGetMyPermissionsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type GetMyPermissionsdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response GetMyPermissionsdefaultApplicationProblemPlusJSONResponse) VisitGetMyPermissionsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type ListMySessionsRequestObject struct {
}

type ListMySessionsResponseObject interface {
	VisitListMySessionsResponse(w http.ResponseWriter) error
}

type ListMySessions200JSONResponse struct {
	Data []SessionInfo `json:"data" binding:"required"`
}

func (response ListMySessions200JSONResponse) VisitListMySessionsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type ListMySessions401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response ListMySessions401ApplicationProblemPlusJSONResponse) VisitListMySessionsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type ListMySessionsdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response ListMySessionsdefaultApplicationProblemPlusJSONResponse) VisitListMySessionsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type RevokeMySessionRequestObject struct {
	SessionId uuid.UUID `json:"session_id" binding:"required"`
}

type RevokeMySessionResponseObject interface {
	VisitRevokeMySessionResponse(w http.ResponseWriter) error
}

type RevokeMySession204Response struct {
}

func (response RevokeMySession204Response) VisitRevokeMySessionResponse(w http.ResponseWriter) error {
	w.WriteHeader(204)
	return nil
}

type RevokeMySession401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response RevokeMySession401ApplicationProblemPlusJSONResponse) VisitRevokeMySessionResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type RevokeMySession404ApplicationProblemPlusJSONResponse Problem

func (response RevokeMySession404ApplicationProblemPlusJSONResponse) VisitRevokeMySessionResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type RevokeMySessiondefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response RevokeMySessiondefaultApplicationProblemPlusJSONResponse) VisitRevokeMySessionResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type CreateOrganizationRequestObject struct {
	Body *CreateOrganizationJSONRequestBody
}

type CreateOrganizationResponseObject interface {
	VisitCreateOrganizationResponse(w http.ResponseWriter) error
}

type CreateOrganization201JSONResponse AuthResponse

func (response CreateOrganization201JSONResponse) VisitCreateOrganizationResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	_, err := buf.WriteTo(w)
	return err
}

type CreateOrganization409ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response CreateOrganization409ApplicationProblemPlusJSONResponse) VisitCreateOrganizationResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(409)
	_, err := buf.WriteTo(w)
	return err
}

type CreateOrganization422ApplicationProblemPlusJSONResponse Problem

func (response CreateOrganization422ApplicationProblemPlusJSONResponse) VisitCreateOrganizationResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type CreateOrganization429ApplicationProblemPlusJSONResponse Problem

func (response CreateOrganization429ApplicationProblemPlusJSONResponse) VisitCreateOrganizationResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(429)
	_, err := buf.WriteTo(w)
	return err
}

type CreateOrganizationdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response CreateOrganizationdefaultApplicationProblemPlusJSONResponse) VisitCreateOrganizationResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type GetCurrentOrganizationRequestObject struct {
}

type GetCurrentOrganizationResponseObject interface {
	VisitGetCurrentOrganizationResponse(w http.ResponseWriter) error
}

type GetCurrentOrganization200JSONResponse Organization

func (response GetCurrentOrganization200JSONResponse) VisitGetCurrentOrganizationResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type GetCurrentOrganization401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response GetCurrentOrganization401ApplicationProblemPlusJSONResponse) VisitGetCurrentOrganizationResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type GetCurrentOrganization403ApplicationProblemPlusJSONResponse Problem

func (response GetCurrentOrganization403ApplicationProblemPlusJSONResponse) VisitGetCurrentOrganizationResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type GetCurrentOrganizationdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response GetCurrentOrganizationdefaultApplicationProblemPlusJSONResponse) VisitGetCurrentOrganizationResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateCurrentOrganizationRequestObject struct {
	Body *UpdateCurrentOrganizationJSONRequestBody
}

type UpdateCurrentOrganizationResponseObject interface {
	VisitUpdateCurrentOrganizationResponse(w http.ResponseWriter) error
}

type UpdateCurrentOrganization200JSONResponse Organization

func (response UpdateCurrentOrganization200JSONResponse) VisitUpdateCurrentOrganizationResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateCurrentOrganization401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response UpdateCurrentOrganization401ApplicationProblemPlusJSONResponse) VisitUpdateCurrentOrganizationResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateCurrentOrganization403ApplicationProblemPlusJSONResponse Problem

func (response UpdateCurrentOrganization403ApplicationProblemPlusJSONResponse) VisitUpdateCurrentOrganizationResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateCurrentOrganization422ApplicationProblemPlusJSONResponse Problem

func (response UpdateCurrentOrganization422ApplicationProblemPlusJSONResponse) VisitUpdateCurrentOrganizationResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateCurrentOrganizationdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response UpdateCurrentOrganizationdefaultApplicationProblemPlusJSONResponse) VisitUpdateCurrentOrganizationResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type ListPlansRequestObject struct {
}

type ListPlansResponseObject interface {
	VisitListPlansResponse(w http.ResponseWriter) error
}

type ListPlans200JSONResponse struct {
	Data     []Plan              `json:"data" binding:"required"`
	Features []FeatureDefinition `json:"features" binding:"required"`
}

func (response ListPlans200JSONResponse) VisitListPlansResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type ListPlansdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response ListPlansdefaultApplicationProblemPlusJSONResponse) VisitListPlansResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type ListResourceTypesRequestObject struct {
}

type ListResourceTypesResponseObject interface {
	VisitListResourceTypesResponse(w http.ResponseWriter) error
}

type ListResourceTypes200JSONResponse struct {
	Data []ResourceType `json:"data" binding:"required"`
}

func (response ListResourceTypes200JSONResponse) VisitListResourceTypesResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type ListResourceTypes403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response ListResourceTypes403ApplicationProblemPlusJSONResponse) VisitListResourceTypesResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type ListResourceTypesdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response ListResourceTypesdefaultApplicationProblemPlusJSONResponse) VisitListResourceTypesResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type ListRoleBindingsRequestObject struct {
	Params ListRoleBindingsParams
}

type ListRoleBindingsResponseObject interface {
	VisitListRoleBindingsResponse(w http.ResponseWriter) error
}

type ListRoleBindings200JSONResponse struct {
	Data []RoleBinding `json:"data" binding:"required"`
}

func (response ListRoleBindings200JSONResponse) VisitListRoleBindingsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type ListRoleBindings403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response ListRoleBindings403ApplicationProblemPlusJSONResponse) VisitListRoleBindingsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type ListRoleBindingsdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response ListRoleBindingsdefaultApplicationProblemPlusJSONResponse) VisitListRoleBindingsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type CreateRoleBindingRequestObject struct {
	Body *CreateRoleBindingJSONRequestBody
}

type CreateRoleBindingResponseObject interface {
	VisitCreateRoleBindingResponse(w http.ResponseWriter) error
}

type CreateRoleBinding201JSONResponse RoleBinding

func (response CreateRoleBinding201JSONResponse) VisitCreateRoleBindingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	_, err := buf.WriteTo(w)
	return err
}

type CreateRoleBinding403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response CreateRoleBinding403ApplicationProblemPlusJSONResponse) VisitCreateRoleBindingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type CreateRoleBinding409ApplicationProblemPlusJSONResponse Problem

func (response CreateRoleBinding409ApplicationProblemPlusJSONResponse) VisitCreateRoleBindingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(409)
	_, err := buf.WriteTo(w)
	return err
}

type CreateRoleBinding422ApplicationProblemPlusJSONResponse Problem

func (response CreateRoleBinding422ApplicationProblemPlusJSONResponse) VisitCreateRoleBindingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type CreateRoleBindingdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response CreateRoleBindingdefaultApplicationProblemPlusJSONResponse) VisitCreateRoleBindingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteRoleBindingRequestObject struct {
	BindingId uuid.UUID `json:"binding_id" binding:"required"`
}

type DeleteRoleBindingResponseObject interface {
	VisitDeleteRoleBindingResponse(w http.ResponseWriter) error
}

type DeleteRoleBinding204Response struct {
}

func (response DeleteRoleBinding204Response) VisitDeleteRoleBindingResponse(w http.ResponseWriter) error {
	w.WriteHeader(204)
	return nil
}

type DeleteRoleBinding403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response DeleteRoleBinding403ApplicationProblemPlusJSONResponse) VisitDeleteRoleBindingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteRoleBinding404ApplicationProblemPlusJSONResponse Problem

func (response DeleteRoleBinding404ApplicationProblemPlusJSONResponse) VisitDeleteRoleBindingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteRoleBinding409ApplicationProblemPlusJSONResponse Problem

func (response DeleteRoleBinding409ApplicationProblemPlusJSONResponse) VisitDeleteRoleBindingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(409)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteRoleBindingdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response DeleteRoleBindingdefaultApplicationProblemPlusJSONResponse) VisitDeleteRoleBindingResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type ListRolesRequestObject struct {
}

type ListRolesResponseObject interface {
	VisitListRolesResponse(w http.ResponseWriter) error
}

type ListRoles200JSONResponse struct {
	Data []Role `json:"data" binding:"required"`
}

func (response ListRoles200JSONResponse) VisitListRolesResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type ListRoles403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response ListRoles403ApplicationProblemPlusJSONResponse) VisitListRolesResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type ListRolesdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response ListRolesdefaultApplicationProblemPlusJSONResponse) VisitListRolesResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type CreateRoleRequestObject struct {
	Body *CreateRoleJSONRequestBody
}

type CreateRoleResponseObject interface {
	VisitCreateRoleResponse(w http.ResponseWriter) error
}

type CreateRole201JSONResponse Role

func (response CreateRole201JSONResponse) VisitCreateRoleResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	_, err := buf.WriteTo(w)
	return err
}

type CreateRole403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response CreateRole403ApplicationProblemPlusJSONResponse) VisitCreateRoleResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type CreateRole409ApplicationProblemPlusJSONResponse Problem

func (response CreateRole409ApplicationProblemPlusJSONResponse) VisitCreateRoleResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(409)
	_, err := buf.WriteTo(w)
	return err
}

type CreateRole422ApplicationProblemPlusJSONResponse Problem

func (response CreateRole422ApplicationProblemPlusJSONResponse) VisitCreateRoleResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type CreateRoledefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response CreateRoledefaultApplicationProblemPlusJSONResponse) VisitCreateRoleResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteRoleRequestObject struct {
	RoleId uuid.UUID `json:"role_id" binding:"required"`
}

type DeleteRoleResponseObject interface {
	VisitDeleteRoleResponse(w http.ResponseWriter) error
}

type DeleteRole204Response struct {
}

func (response DeleteRole204Response) VisitDeleteRoleResponse(w http.ResponseWriter) error {
	w.WriteHeader(204)
	return nil
}

type DeleteRole403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response DeleteRole403ApplicationProblemPlusJSONResponse) VisitDeleteRoleResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteRole404ApplicationProblemPlusJSONResponse Problem

func (response DeleteRole404ApplicationProblemPlusJSONResponse) VisitDeleteRoleResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteRole409ApplicationProblemPlusJSONResponse Problem

func (response DeleteRole409ApplicationProblemPlusJSONResponse) VisitDeleteRoleResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(409)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteRoledefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response DeleteRoledefaultApplicationProblemPlusJSONResponse) VisitDeleteRoleResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type GetRoleRequestObject struct {
	RoleId uuid.UUID `json:"role_id" binding:"required"`
}

type GetRoleResponseObject interface {
	VisitGetRoleResponse(w http.ResponseWriter) error
}

type GetRole200JSONResponse Role

func (response GetRole200JSONResponse) VisitGetRoleResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type GetRole404ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response GetRole404ApplicationProblemPlusJSONResponse) VisitGetRoleResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type GetRoledefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response GetRoledefaultApplicationProblemPlusJSONResponse) VisitGetRoleResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateRoleRequestObject struct {
	RoleId uuid.UUID `json:"role_id" binding:"required"`
	Body   *UpdateRoleJSONRequestBody
}

type UpdateRoleResponseObject interface {
	VisitUpdateRoleResponse(w http.ResponseWriter) error
}

type UpdateRole200JSONResponse Role

func (response UpdateRole200JSONResponse) VisitUpdateRoleResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateRole403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response UpdateRole403ApplicationProblemPlusJSONResponse) VisitUpdateRoleResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateRole404ApplicationProblemPlusJSONResponse Problem

func (response UpdateRole404ApplicationProblemPlusJSONResponse) VisitUpdateRoleResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateRole409ApplicationProblemPlusJSONResponse Problem

func (response UpdateRole409ApplicationProblemPlusJSONResponse) VisitUpdateRoleResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(409)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateRole422ApplicationProblemPlusJSONResponse Problem

func (response UpdateRole422ApplicationProblemPlusJSONResponse) VisitUpdateRoleResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateRoledefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response UpdateRoledefaultApplicationProblemPlusJSONResponse) VisitUpdateRoleResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type ListServiceAccountsRequestObject struct {
}

type ListServiceAccountsResponseObject interface {
	VisitListServiceAccountsResponse(w http.ResponseWriter) error
}

type ListServiceAccounts200JSONResponse struct {
	Data []ServiceAccount `json:"data" binding:"required"`
}

func (response ListServiceAccounts200JSONResponse) VisitListServiceAccountsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type ListServiceAccounts403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response ListServiceAccounts403ApplicationProblemPlusJSONResponse) VisitListServiceAccountsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type ListServiceAccountsdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response ListServiceAccountsdefaultApplicationProblemPlusJSONResponse) VisitListServiceAccountsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type CreateServiceAccountRequestObject struct {
	Body *CreateServiceAccountJSONRequestBody
}

type CreateServiceAccountResponseObject interface {
	VisitCreateServiceAccountResponse(w http.ResponseWriter) error
}

type CreateServiceAccount201JSONResponse ServiceAccount

func (response CreateServiceAccount201JSONResponse) VisitCreateServiceAccountResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	_, err := buf.WriteTo(w)
	return err
}

type CreateServiceAccount403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response CreateServiceAccount403ApplicationProblemPlusJSONResponse) VisitCreateServiceAccountResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type CreateServiceAccount409ApplicationProblemPlusJSONResponse Problem

func (response CreateServiceAccount409ApplicationProblemPlusJSONResponse) VisitCreateServiceAccountResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(409)
	_, err := buf.WriteTo(w)
	return err
}

type CreateServiceAccount422ApplicationProblemPlusJSONResponse Problem

func (response CreateServiceAccount422ApplicationProblemPlusJSONResponse) VisitCreateServiceAccountResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type CreateServiceAccountdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response CreateServiceAccountdefaultApplicationProblemPlusJSONResponse) VisitCreateServiceAccountResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteServiceAccountRequestObject struct {
	ServiceAccountId uuid.UUID `json:"service_account_id" binding:"required"`
}

type DeleteServiceAccountResponseObject interface {
	VisitDeleteServiceAccountResponse(w http.ResponseWriter) error
}

type DeleteServiceAccount204Response struct {
}

func (response DeleteServiceAccount204Response) VisitDeleteServiceAccountResponse(w http.ResponseWriter) error {
	w.WriteHeader(204)
	return nil
}

type DeleteServiceAccount403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response DeleteServiceAccount403ApplicationProblemPlusJSONResponse) VisitDeleteServiceAccountResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteServiceAccount404ApplicationProblemPlusJSONResponse Problem

func (response DeleteServiceAccount404ApplicationProblemPlusJSONResponse) VisitDeleteServiceAccountResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteServiceAccountdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response DeleteServiceAccountdefaultApplicationProblemPlusJSONResponse) VisitDeleteServiceAccountResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type GetServiceAccountRequestObject struct {
	ServiceAccountId uuid.UUID `json:"service_account_id" binding:"required"`
}

type GetServiceAccountResponseObject interface {
	VisitGetServiceAccountResponse(w http.ResponseWriter) error
}

type GetServiceAccount200JSONResponse ServiceAccount

func (response GetServiceAccount200JSONResponse) VisitGetServiceAccountResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type GetServiceAccount403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response GetServiceAccount403ApplicationProblemPlusJSONResponse) VisitGetServiceAccountResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type GetServiceAccount404ApplicationProblemPlusJSONResponse Problem

func (response GetServiceAccount404ApplicationProblemPlusJSONResponse) VisitGetServiceAccountResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type GetServiceAccountdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response GetServiceAccountdefaultApplicationProblemPlusJSONResponse) VisitGetServiceAccountResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateServiceAccountRequestObject struct {
	ServiceAccountId uuid.UUID `json:"service_account_id" binding:"required"`
	Body             *UpdateServiceAccountJSONRequestBody
}

type UpdateServiceAccountResponseObject interface {
	VisitUpdateServiceAccountResponse(w http.ResponseWriter) error
}

type UpdateServiceAccount200JSONResponse ServiceAccount

func (response UpdateServiceAccount200JSONResponse) VisitUpdateServiceAccountResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateServiceAccount403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response UpdateServiceAccount403ApplicationProblemPlusJSONResponse) VisitUpdateServiceAccountResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateServiceAccount404ApplicationProblemPlusJSONResponse Problem

func (response UpdateServiceAccount404ApplicationProblemPlusJSONResponse) VisitUpdateServiceAccountResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateServiceAccount409ApplicationProblemPlusJSONResponse Problem

func (response UpdateServiceAccount409ApplicationProblemPlusJSONResponse) VisitUpdateServiceAccountResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(409)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateServiceAccount422ApplicationProblemPlusJSONResponse Problem

func (response UpdateServiceAccount422ApplicationProblemPlusJSONResponse) VisitUpdateServiceAccountResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateServiceAccountdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response UpdateServiceAccountdefaultApplicationProblemPlusJSONResponse) VisitUpdateServiceAccountResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type ListApiKeysRequestObject struct {
	ServiceAccountId uuid.UUID `json:"service_account_id" binding:"required"`
}

type ListApiKeysResponseObject interface {
	VisitListApiKeysResponse(w http.ResponseWriter) error
}

type ListApiKeys200JSONResponse struct {
	Data []ApiKey `json:"data" binding:"required"`
}

func (response ListApiKeys200JSONResponse) VisitListApiKeysResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type ListApiKeys403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response ListApiKeys403ApplicationProblemPlusJSONResponse) VisitListApiKeysResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type ListApiKeys404ApplicationProblemPlusJSONResponse Problem

func (response ListApiKeys404ApplicationProblemPlusJSONResponse) VisitListApiKeysResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type ListApiKeysdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response ListApiKeysdefaultApplicationProblemPlusJSONResponse) VisitListApiKeysResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type CreateApiKeyRequestObject struct {
	ServiceAccountId uuid.UUID `json:"service_account_id" binding:"required"`
	Body             *CreateApiKeyJSONRequestBody
}

type CreateApiKeyResponseObject interface {
	VisitCreateApiKeyResponse(w http.ResponseWriter) error
}

type CreateApiKey201JSONResponse ApiKeyCreated

func (response CreateApiKey201JSONResponse) VisitCreateApiKeyResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	_, err := buf.WriteTo(w)
	return err
}

type CreateApiKey403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response CreateApiKey403ApplicationProblemPlusJSONResponse) VisitCreateApiKeyResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type CreateApiKey404ApplicationProblemPlusJSONResponse Problem

func (response CreateApiKey404ApplicationProblemPlusJSONResponse) VisitCreateApiKeyResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type CreateApiKey409ApplicationProblemPlusJSONResponse Problem

func (response CreateApiKey409ApplicationProblemPlusJSONResponse) VisitCreateApiKeyResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(409)
	_, err := buf.WriteTo(w)
	return err
}

type CreateApiKey422ApplicationProblemPlusJSONResponse Problem

func (response CreateApiKey422ApplicationProblemPlusJSONResponse) VisitCreateApiKeyResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type CreateApiKeydefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response CreateApiKeydefaultApplicationProblemPlusJSONResponse) VisitCreateApiKeyResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type RevokeApiKeyRequestObject struct {
	ServiceAccountId uuid.UUID `json:"service_account_id" binding:"required"`
	KeyId            uuid.UUID `json:"key_id" binding:"required"`
}

type RevokeApiKeyResponseObject interface {
	VisitRevokeApiKeyResponse(w http.ResponseWriter) error
}

type RevokeApiKey204Response struct {
}

func (response RevokeApiKey204Response) VisitRevokeApiKeyResponse(w http.ResponseWriter) error {
	w.WriteHeader(204)
	return nil
}

type RevokeApiKey403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response RevokeApiKey403ApplicationProblemPlusJSONResponse) VisitRevokeApiKeyResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type RevokeApiKey404ApplicationProblemPlusJSONResponse Problem

func (response RevokeApiKey404ApplicationProblemPlusJSONResponse) VisitRevokeApiKeyResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type RevokeApiKey409ApplicationProblemPlusJSONResponse Problem

func (response RevokeApiKey409ApplicationProblemPlusJSONResponse) VisitRevokeApiKeyResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(409)
	_, err := buf.WriteTo(w)
	return err
}

type RevokeApiKeydefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response RevokeApiKeydefaultApplicationProblemPlusJSONResponse) VisitRevokeApiKeyResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type TryServiceEndpointRequestObject struct {
	EndpointId uuid.UUID `json:"endpoint_id" binding:"required"`
	Body       *TryServiceEndpointJSONRequestBody
}

type TryServiceEndpointResponseObject interface {
	VisitTryServiceEndpointResponse(w http.ResponseWriter) error
}

type TryServiceEndpoint200JSONResponse ServiceTryResult

func (response TryServiceEndpoint200JSONResponse) VisitTryServiceEndpointResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type TryServiceEndpoint401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response TryServiceEndpoint401ApplicationProblemPlusJSONResponse) VisitTryServiceEndpointResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type TryServiceEndpoint403ApplicationProblemPlusJSONResponse Problem

func (response TryServiceEndpoint403ApplicationProblemPlusJSONResponse) VisitTryServiceEndpointResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type TryServiceEndpoint404ApplicationProblemPlusJSONResponse Problem

func (response TryServiceEndpoint404ApplicationProblemPlusJSONResponse) VisitTryServiceEndpointResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type TryServiceEndpoint422ApplicationProblemPlusJSONResponse Problem

func (response TryServiceEndpoint422ApplicationProblemPlusJSONResponse) VisitTryServiceEndpointResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type TryServiceEndpointdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response TryServiceEndpointdefaultApplicationProblemPlusJSONResponse) VisitTryServiceEndpointResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type ListServicesRequestObject struct {
	Params ListServicesParams
}

type ListServicesResponseObject interface {
	VisitListServicesResponse(w http.ResponseWriter) error
}

type ListServices200JSONResponse struct {
	Data []Service `json:"data" binding:"required"`
}

func (response ListServices200JSONResponse) VisitListServicesResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type ListServices401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response ListServices401ApplicationProblemPlusJSONResponse) VisitListServicesResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type ListServices403ApplicationProblemPlusJSONResponse Problem

func (response ListServices403ApplicationProblemPlusJSONResponse) VisitListServicesResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type ListServicesdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response ListServicesdefaultApplicationProblemPlusJSONResponse) VisitListServicesResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type CreateServiceRequestObject struct {
	Body *CreateServiceJSONRequestBody
}

type CreateServiceResponseObject interface {
	VisitCreateServiceResponse(w http.ResponseWriter) error
}

type CreateService201JSONResponse Service

func (response CreateService201JSONResponse) VisitCreateServiceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	_, err := buf.WriteTo(w)
	return err
}

type CreateService401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response CreateService401ApplicationProblemPlusJSONResponse) VisitCreateServiceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type CreateService403ApplicationProblemPlusJSONResponse Problem

func (response CreateService403ApplicationProblemPlusJSONResponse) VisitCreateServiceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type CreateService422ApplicationProblemPlusJSONResponse Problem

func (response CreateService422ApplicationProblemPlusJSONResponse) VisitCreateServiceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type CreateServicedefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response CreateServicedefaultApplicationProblemPlusJSONResponse) VisitCreateServiceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteServiceRequestObject struct {
	ServiceId uuid.UUID `json:"service_id" binding:"required"`
}

type DeleteServiceResponseObject interface {
	VisitDeleteServiceResponse(w http.ResponseWriter) error
}

type DeleteService204Response struct {
}

func (response DeleteService204Response) VisitDeleteServiceResponse(w http.ResponseWriter) error {
	w.WriteHeader(204)
	return nil
}

type DeleteService401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response DeleteService401ApplicationProblemPlusJSONResponse) VisitDeleteServiceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteService403ApplicationProblemPlusJSONResponse Problem

func (response DeleteService403ApplicationProblemPlusJSONResponse) VisitDeleteServiceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteService404ApplicationProblemPlusJSONResponse Problem

func (response DeleteService404ApplicationProblemPlusJSONResponse) VisitDeleteServiceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteServicedefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response DeleteServicedefaultApplicationProblemPlusJSONResponse) VisitDeleteServiceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type GetServiceRequestObject struct {
	ServiceId uuid.UUID `json:"service_id" binding:"required"`
}

type GetServiceResponseObject interface {
	VisitGetServiceResponse(w http.ResponseWriter) error
}

type GetService200JSONResponse Service

func (response GetService200JSONResponse) VisitGetServiceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type GetService401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response GetService401ApplicationProblemPlusJSONResponse) VisitGetServiceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type GetService403ApplicationProblemPlusJSONResponse Problem

func (response GetService403ApplicationProblemPlusJSONResponse) VisitGetServiceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type GetService404ApplicationProblemPlusJSONResponse Problem

func (response GetService404ApplicationProblemPlusJSONResponse) VisitGetServiceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type GetServicedefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response GetServicedefaultApplicationProblemPlusJSONResponse) VisitGetServiceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateServiceRequestObject struct {
	ServiceId uuid.UUID `json:"service_id" binding:"required"`
	Body      *UpdateServiceJSONRequestBody
}

type UpdateServiceResponseObject interface {
	VisitUpdateServiceResponse(w http.ResponseWriter) error
}

type UpdateService200JSONResponse Service

func (response UpdateService200JSONResponse) VisitUpdateServiceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateService401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response UpdateService401ApplicationProblemPlusJSONResponse) VisitUpdateServiceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateService403ApplicationProblemPlusJSONResponse Problem

func (response UpdateService403ApplicationProblemPlusJSONResponse) VisitUpdateServiceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateService404ApplicationProblemPlusJSONResponse Problem

func (response UpdateService404ApplicationProblemPlusJSONResponse) VisitUpdateServiceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateServicedefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response UpdateServicedefaultApplicationProblemPlusJSONResponse) VisitUpdateServiceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type ListServiceEndpointsRequestObject struct {
	ServiceId uuid.UUID `json:"service_id" binding:"required"`
}

type ListServiceEndpointsResponseObject interface {
	VisitListServiceEndpointsResponse(w http.ResponseWriter) error
}

type ListServiceEndpoints200JSONResponse struct {
	Data []ServiceEndpoint `json:"data" binding:"required"`
}

func (response ListServiceEndpoints200JSONResponse) VisitListServiceEndpointsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type ListServiceEndpoints401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response ListServiceEndpoints401ApplicationProblemPlusJSONResponse) VisitListServiceEndpointsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type ListServiceEndpoints403ApplicationProblemPlusJSONResponse Problem

func (response ListServiceEndpoints403ApplicationProblemPlusJSONResponse) VisitListServiceEndpointsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type ListServiceEndpoints404ApplicationProblemPlusJSONResponse Problem

func (response ListServiceEndpoints404ApplicationProblemPlusJSONResponse) VisitListServiceEndpointsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type ListServiceEndpointsdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response ListServiceEndpointsdefaultApplicationProblemPlusJSONResponse) VisitListServiceEndpointsResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteServiceEnvironmentRequestObject struct {
	ServiceId     uuid.UUID `json:"service_id" binding:"required"`
	EnvironmentId uuid.UUID `json:"environment_id" binding:"required"`
}

type DeleteServiceEnvironmentResponseObject interface {
	VisitDeleteServiceEnvironmentResponse(w http.ResponseWriter) error
}

type DeleteServiceEnvironment204Response struct {
}

func (response DeleteServiceEnvironment204Response) VisitDeleteServiceEnvironmentResponse(w http.ResponseWriter) error {
	w.WriteHeader(204)
	return nil
}

type DeleteServiceEnvironment401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response DeleteServiceEnvironment401ApplicationProblemPlusJSONResponse) VisitDeleteServiceEnvironmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteServiceEnvironment403ApplicationProblemPlusJSONResponse Problem

func (response DeleteServiceEnvironment403ApplicationProblemPlusJSONResponse) VisitDeleteServiceEnvironmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteServiceEnvironment404ApplicationProblemPlusJSONResponse Problem

func (response DeleteServiceEnvironment404ApplicationProblemPlusJSONResponse) VisitDeleteServiceEnvironmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type DeleteServiceEnvironmentdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response DeleteServiceEnvironmentdefaultApplicationProblemPlusJSONResponse) VisitDeleteServiceEnvironmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type PutServiceEnvironmentRequestObject struct {
	ServiceId     uuid.UUID `json:"service_id" binding:"required"`
	EnvironmentId uuid.UUID `json:"environment_id" binding:"required"`
	Body          *PutServiceEnvironmentJSONRequestBody
}

type PutServiceEnvironmentResponseObject interface {
	VisitPutServiceEnvironmentResponse(w http.ResponseWriter) error
}

type PutServiceEnvironment200JSONResponse ServiceEnvironment

func (response PutServiceEnvironment200JSONResponse) VisitPutServiceEnvironmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type PutServiceEnvironment401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response PutServiceEnvironment401ApplicationProblemPlusJSONResponse) VisitPutServiceEnvironmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type PutServiceEnvironment403ApplicationProblemPlusJSONResponse Problem

func (response PutServiceEnvironment403ApplicationProblemPlusJSONResponse) VisitPutServiceEnvironmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type PutServiceEnvironment404ApplicationProblemPlusJSONResponse Problem

func (response PutServiceEnvironment404ApplicationProblemPlusJSONResponse) VisitPutServiceEnvironmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type PutServiceEnvironment422ApplicationProblemPlusJSONResponse Problem

func (response PutServiceEnvironment422ApplicationProblemPlusJSONResponse) VisitPutServiceEnvironmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type PutServiceEnvironmentdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response PutServiceEnvironmentdefaultApplicationProblemPlusJSONResponse) VisitPutServiceEnvironmentResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type ImportOpenAPIRequestObject struct {
	ServiceId uuid.UUID `json:"service_id" binding:"required"`
	Params    ImportOpenAPIParams
	Body      *ImportOpenAPIJSONRequestBody
}

type ImportOpenAPIResponseObject interface {
	VisitImportOpenAPIResponse(w http.ResponseWriter) error
}

type ImportOpenAPI200JSONResponse OpenAPIImportResult

func (response ImportOpenAPI200JSONResponse) VisitImportOpenAPIResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type ImportOpenAPI401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response ImportOpenAPI401ApplicationProblemPlusJSONResponse) VisitImportOpenAPIResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type ImportOpenAPI403ApplicationProblemPlusJSONResponse Problem

func (response ImportOpenAPI403ApplicationProblemPlusJSONResponse) VisitImportOpenAPIResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type ImportOpenAPI404ApplicationProblemPlusJSONResponse Problem

func (response ImportOpenAPI404ApplicationProblemPlusJSONResponse) VisitImportOpenAPIResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type ImportOpenAPI422ApplicationProblemPlusJSONResponse Problem

func (response ImportOpenAPI422ApplicationProblemPlusJSONResponse) VisitImportOpenAPIResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type ImportOpenAPIdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response ImportOpenAPIdefaultApplicationProblemPlusJSONResponse) VisitImportOpenAPIResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type GetSystemInfoRequestObject struct {
}

type GetSystemInfoResponseObject interface {
	VisitGetSystemInfoResponse(w http.ResponseWriter) error
}

type GetSystemInfo200JSONResponse SystemInfo

func (response GetSystemInfo200JSONResponse) VisitGetSystemInfoResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type GetSystemInfodefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response GetSystemInfodefaultApplicationProblemPlusJSONResponse) VisitGetSystemInfoResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type ListUsersRequestObject struct {
}

type ListUsersResponseObject interface {
	VisitListUsersResponse(w http.ResponseWriter) error
}

type ListUsers200JSONResponse struct {
	Data []Member `json:"data" binding:"required"`
}

func (response ListUsers200JSONResponse) VisitListUsersResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type ListUsers403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response ListUsers403ApplicationProblemPlusJSONResponse) VisitListUsersResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type ListUsersdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response ListUsersdefaultApplicationProblemPlusJSONResponse) VisitListUsersResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type RemoveUserRequestObject struct {
	UserId uuid.UUID `json:"user_id" binding:"required"`
}

type RemoveUserResponseObject interface {
	VisitRemoveUserResponse(w http.ResponseWriter) error
}

type RemoveUser204Response struct {
}

func (response RemoveUser204Response) VisitRemoveUserResponse(w http.ResponseWriter) error {
	w.WriteHeader(204)
	return nil
}

type RemoveUser403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response RemoveUser403ApplicationProblemPlusJSONResponse) VisitRemoveUserResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type RemoveUser404ApplicationProblemPlusJSONResponse Problem

func (response RemoveUser404ApplicationProblemPlusJSONResponse) VisitRemoveUserResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type RemoveUser409ApplicationProblemPlusJSONResponse Problem

func (response RemoveUser409ApplicationProblemPlusJSONResponse) VisitRemoveUserResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(409)
	_, err := buf.WriteTo(w)
	return err
}

type RemoveUserdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response RemoveUserdefaultApplicationProblemPlusJSONResponse) VisitRemoveUserResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type GetUserRequestObject struct {
	UserId uuid.UUID `json:"user_id" binding:"required"`
}

type GetUserResponseObject interface {
	VisitGetUserResponse(w http.ResponseWriter) error
}

type GetUser200JSONResponse Member

func (response GetUser200JSONResponse) VisitGetUserResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type GetUser403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response GetUser403ApplicationProblemPlusJSONResponse) VisitGetUserResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type GetUser404ApplicationProblemPlusJSONResponse Problem

func (response GetUser404ApplicationProblemPlusJSONResponse) VisitGetUserResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type GetUserdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response GetUserdefaultApplicationProblemPlusJSONResponse) VisitGetUserResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateUserRequestObject struct {
	UserId uuid.UUID `json:"user_id" binding:"required"`
	Body   *UpdateUserJSONRequestBody
}

type UpdateUserResponseObject interface {
	VisitUpdateUserResponse(w http.ResponseWriter) error
}

type UpdateUser200JSONResponse Member

func (response UpdateUser200JSONResponse) VisitUpdateUserResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateUser403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response UpdateUser403ApplicationProblemPlusJSONResponse) VisitUpdateUserResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateUser404ApplicationProblemPlusJSONResponse Problem

func (response UpdateUser404ApplicationProblemPlusJSONResponse) VisitUpdateUserResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateUser409ApplicationProblemPlusJSONResponse Problem

func (response UpdateUser409ApplicationProblemPlusJSONResponse) VisitUpdateUserResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(409)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateUser422ApplicationProblemPlusJSONResponse Problem

func (response UpdateUser422ApplicationProblemPlusJSONResponse) VisitUpdateUserResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateUserdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response UpdateUserdefaultApplicationProblemPlusJSONResponse) VisitUpdateUserResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type ListWorkspacesRequestObject struct {
	Params ListWorkspacesParams
}

type ListWorkspacesResponseObject interface {
	VisitListWorkspacesResponse(w http.ResponseWriter) error
}

type ListWorkspaces200JSONResponse struct {
	Data []Workspace `json:"data" binding:"required"`
}

func (response ListWorkspaces200JSONResponse) VisitListWorkspacesResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type ListWorkspaces401ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response ListWorkspaces401ApplicationProblemPlusJSONResponse) VisitListWorkspacesResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(401)
	_, err := buf.WriteTo(w)
	return err
}

type ListWorkspacesdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response ListWorkspacesdefaultApplicationProblemPlusJSONResponse) VisitListWorkspacesResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type CreateWorkspaceRequestObject struct {
	Body *CreateWorkspaceJSONRequestBody
}

type CreateWorkspaceResponseObject interface {
	VisitCreateWorkspaceResponse(w http.ResponseWriter) error
}

type CreateWorkspace201JSONResponse Workspace

func (response CreateWorkspace201JSONResponse) VisitCreateWorkspaceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	_, err := buf.WriteTo(w)
	return err
}

type CreateWorkspace402ApplicationProblemPlusJSONResponse struct {
	PlanProblemApplicationProblemPlusJSONResponse
}

func (response CreateWorkspace402ApplicationProblemPlusJSONResponse) VisitCreateWorkspaceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(402)
	_, err := buf.WriteTo(w)
	return err
}

type CreateWorkspace403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response CreateWorkspace403ApplicationProblemPlusJSONResponse) VisitCreateWorkspaceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type CreateWorkspace409ApplicationProblemPlusJSONResponse Problem

func (response CreateWorkspace409ApplicationProblemPlusJSONResponse) VisitCreateWorkspaceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(409)
	_, err := buf.WriteTo(w)
	return err
}

type CreateWorkspace422ApplicationProblemPlusJSONResponse Problem

func (response CreateWorkspace422ApplicationProblemPlusJSONResponse) VisitCreateWorkspaceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type CreateWorkspacedefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response CreateWorkspacedefaultApplicationProblemPlusJSONResponse) VisitCreateWorkspaceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type GetWorkspaceRequestObject struct {
	Workspace WorkspaceSlug `json:"workspace" binding:"required"`
}

type GetWorkspaceResponseObject interface {
	VisitGetWorkspaceResponse(w http.ResponseWriter) error
}

type GetWorkspace200JSONResponse Workspace

func (response GetWorkspace200JSONResponse) VisitGetWorkspaceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type GetWorkspace404ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response GetWorkspace404ApplicationProblemPlusJSONResponse) VisitGetWorkspaceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type GetWorkspacedefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response GetWorkspacedefaultApplicationProblemPlusJSONResponse) VisitGetWorkspaceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateWorkspaceRequestObject struct {
	Workspace WorkspaceSlug `json:"workspace" binding:"required"`
	Body      *UpdateWorkspaceJSONRequestBody
}

type UpdateWorkspaceResponseObject interface {
	VisitUpdateWorkspaceResponse(w http.ResponseWriter) error
}

type UpdateWorkspace200JSONResponse Workspace

func (response UpdateWorkspace200JSONResponse) VisitUpdateWorkspaceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateWorkspace402ApplicationProblemPlusJSONResponse struct {
	PlanProblemApplicationProblemPlusJSONResponse
}

func (response UpdateWorkspace402ApplicationProblemPlusJSONResponse) VisitUpdateWorkspaceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(402)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateWorkspace403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response UpdateWorkspace403ApplicationProblemPlusJSONResponse) VisitUpdateWorkspaceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateWorkspace404ApplicationProblemPlusJSONResponse Problem

func (response UpdateWorkspace404ApplicationProblemPlusJSONResponse) VisitUpdateWorkspaceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateWorkspace422ApplicationProblemPlusJSONResponse Problem

func (response UpdateWorkspace422ApplicationProblemPlusJSONResponse) VisitUpdateWorkspaceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type UpdateWorkspacedefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response UpdateWorkspacedefaultApplicationProblemPlusJSONResponse) VisitUpdateWorkspaceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type ArchiveWorkspaceRequestObject struct {
	Workspace WorkspaceSlug `json:"workspace" binding:"required"`
}

type ArchiveWorkspaceResponseObject interface {
	VisitArchiveWorkspaceResponse(w http.ResponseWriter) error
}

type ArchiveWorkspace200JSONResponse Workspace

func (response ArchiveWorkspace200JSONResponse) VisitArchiveWorkspaceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type ArchiveWorkspace403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response ArchiveWorkspace403ApplicationProblemPlusJSONResponse) VisitArchiveWorkspaceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type ArchiveWorkspace404ApplicationProblemPlusJSONResponse Problem

func (response ArchiveWorkspace404ApplicationProblemPlusJSONResponse) VisitArchiveWorkspaceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type ArchiveWorkspace409ApplicationProblemPlusJSONResponse Problem

func (response ArchiveWorkspace409ApplicationProblemPlusJSONResponse) VisitArchiveWorkspaceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(409)
	_, err := buf.WriteTo(w)
	return err
}

type ArchiveWorkspacedefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response ArchiveWorkspacedefaultApplicationProblemPlusJSONResponse) VisitArchiveWorkspaceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type ListWorkspaceMembersRequestObject struct {
	Workspace WorkspaceSlug `json:"workspace" binding:"required"`
}

type ListWorkspaceMembersResponseObject interface {
	VisitListWorkspaceMembersResponse(w http.ResponseWriter) error
}

type ListWorkspaceMembers200JSONResponse struct {
	Data []Member `json:"data" binding:"required"`
}

func (response ListWorkspaceMembers200JSONResponse) VisitListWorkspaceMembersResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type ListWorkspaceMembers404ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response ListWorkspaceMembers404ApplicationProblemPlusJSONResponse) VisitListWorkspaceMembersResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type ListWorkspaceMembersdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response ListWorkspaceMembersdefaultApplicationProblemPlusJSONResponse) VisitListWorkspaceMembersResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type AddWorkspaceMemberRequestObject struct {
	Workspace WorkspaceSlug `json:"workspace" binding:"required"`
	Body      *AddWorkspaceMemberJSONRequestBody
}

type AddWorkspaceMemberResponseObject interface {
	VisitAddWorkspaceMemberResponse(w http.ResponseWriter) error
}

type AddWorkspaceMember204Response struct {
}

func (response AddWorkspaceMember204Response) VisitAddWorkspaceMemberResponse(w http.ResponseWriter) error {
	w.WriteHeader(204)
	return nil
}

type AddWorkspaceMember403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response AddWorkspaceMember403ApplicationProblemPlusJSONResponse) VisitAddWorkspaceMemberResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type AddWorkspaceMember404ApplicationProblemPlusJSONResponse Problem

func (response AddWorkspaceMember404ApplicationProblemPlusJSONResponse) VisitAddWorkspaceMemberResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type AddWorkspaceMember409ApplicationProblemPlusJSONResponse Problem

func (response AddWorkspaceMember409ApplicationProblemPlusJSONResponse) VisitAddWorkspaceMemberResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(409)
	_, err := buf.WriteTo(w)
	return err
}

type AddWorkspaceMember422ApplicationProblemPlusJSONResponse Problem

func (response AddWorkspaceMember422ApplicationProblemPlusJSONResponse) VisitAddWorkspaceMemberResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(422)
	_, err := buf.WriteTo(w)
	return err
}

type AddWorkspaceMemberdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response AddWorkspaceMemberdefaultApplicationProblemPlusJSONResponse) VisitAddWorkspaceMemberResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type RemoveWorkspaceMemberRequestObject struct {
	Workspace WorkspaceSlug `json:"workspace" binding:"required"`
	UserId    uuid.UUID     `json:"user_id" binding:"required"`
}

type RemoveWorkspaceMemberResponseObject interface {
	VisitRemoveWorkspaceMemberResponse(w http.ResponseWriter) error
}

type RemoveWorkspaceMember204Response struct {
}

func (response RemoveWorkspaceMember204Response) VisitRemoveWorkspaceMemberResponse(w http.ResponseWriter) error {
	w.WriteHeader(204)
	return nil
}

type RemoveWorkspaceMember403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response RemoveWorkspaceMember403ApplicationProblemPlusJSONResponse) VisitRemoveWorkspaceMemberResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type RemoveWorkspaceMember404ApplicationProblemPlusJSONResponse Problem

func (response RemoveWorkspaceMember404ApplicationProblemPlusJSONResponse) VisitRemoveWorkspaceMemberResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type RemoveWorkspaceMember409ApplicationProblemPlusJSONResponse Problem

func (response RemoveWorkspaceMember409ApplicationProblemPlusJSONResponse) VisitRemoveWorkspaceMemberResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(409)
	_, err := buf.WriteTo(w)
	return err
}

type RemoveWorkspaceMemberdefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response RemoveWorkspaceMemberdefaultApplicationProblemPlusJSONResponse) VisitRemoveWorkspaceMemberResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}

type RestoreWorkspaceRequestObject struct {
	Workspace WorkspaceSlug `json:"workspace" binding:"required"`
}

type RestoreWorkspaceResponseObject interface {
	VisitRestoreWorkspaceResponse(w http.ResponseWriter) error
}

type RestoreWorkspace200JSONResponse Workspace

func (response RestoreWorkspace200JSONResponse) VisitRestoreWorkspaceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, err := buf.WriteTo(w)
	return err
}

type RestoreWorkspace402ApplicationProblemPlusJSONResponse struct {
	PlanProblemApplicationProblemPlusJSONResponse
}

func (response RestoreWorkspace402ApplicationProblemPlusJSONResponse) VisitRestoreWorkspaceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(402)
	_, err := buf.WriteTo(w)
	return err
}

type RestoreWorkspace403ApplicationProblemPlusJSONResponse struct {
	ProblemApplicationProblemPlusJSONResponse
}

func (response RestoreWorkspace403ApplicationProblemPlusJSONResponse) VisitRestoreWorkspaceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(403)
	_, err := buf.WriteTo(w)
	return err
}

type RestoreWorkspace404ApplicationProblemPlusJSONResponse Problem

func (response RestoreWorkspace404ApplicationProblemPlusJSONResponse) VisitRestoreWorkspaceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(404)
	_, err := buf.WriteTo(w)
	return err
}

type RestoreWorkspace409ApplicationProblemPlusJSONResponse Problem

func (response RestoreWorkspace409ApplicationProblemPlusJSONResponse) VisitRestoreWorkspaceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(409)
	_, err := buf.WriteTo(w)
	return err
}

type RestoreWorkspacedefaultApplicationProblemPlusJSONResponse struct {
	Body       Problem
	StatusCode int
}

func (response RestoreWorkspacedefaultApplicationProblemPlusJSONResponse) VisitRestoreWorkspaceResponse(w http.ResponseWriter) error {

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response.Body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(response.StatusCode)
	_, err := buf.WriteTo(w)
	return err
}
