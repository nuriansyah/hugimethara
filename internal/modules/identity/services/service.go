package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	identitymodel "github.com/nuriansyah/hugimethara/internal/modules/identity/model"
	identitydto "github.com/nuriansyah/hugimethara/internal/modules/identity/model/dto"
	orgmodel "github.com/nuriansyah/hugimethara/internal/modules/organization/model"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

// Code shape: transport adapters and other modules depend on these use-case
// interfaces (ports), never on IdentityService directly (DIP); each interface
// groups one capability so consumers can depend on the narrowest one (ISP).

// GroupsUseCases is the groups capability of the module.
type GroupsUseCases interface {
	// AddGroupMember adds an organization member to a group (group:manage_members).
	// Joining a group hands out the group's roles, so the caller must be able to
	// grant every one of them (no escalation through groups).
	AddGroupMember(ctx context.Context, groupID uuid.UUID, userID uuid.UUID) error
	// CreateGroup creates an internal group (group:create).
	CreateGroup(ctx context.Context, in identitydto.GroupInput) (identitymodel.Group, error)
	// DeleteGroup deletes a group with its memberships and role assignments (group:delete).
	DeleteGroup(ctx context.Context, id uuid.UUID) error
	// GetGroup returns one group (group:read).
	GetGroup(ctx context.Context, id uuid.UUID) (identitymodel.Group, error)
	// ListGroupMembers lists a group's members (group:read).
	ListGroupMembers(ctx context.Context, id uuid.UUID) ([]identitymodel.GroupMember, error)
	// ListGroups lists the organization's groups (group:read).
	ListGroups(ctx context.Context) ([]identitymodel.Group, error)
	// RemoveGroupMember removes a member from a group (group:manage_members).
	RemoveGroupMember(ctx context.Context, groupID uuid.UUID, userID uuid.UUID) error
	// UpdateGroup renames or re-describes a group (group:update).
	UpdateGroup(ctx context.Context, id uuid.UUID, in identitydto.GroupInput) (identitymodel.Group, error)
}

// InvitationsUseCases is the invitations capability of the module.
type InvitationsUseCases interface {
	// AcceptInvitation joins the invited organization and signs in to it. The
	// invitee proves who they are with a session for the invited address, the
	// password of the existing account, or by creating the account.
	AcceptInvitation(ctx context.Context, token string, in identitydto.AcceptInput) (identitymodel.SessionGrant, error)
	// CreateInvitation invites an email address (user:invite). Each workspace
	// grant needs workspace:manage_members there and an organization role needs
	// role:bind; in every case the caller can only hand out permissions they hold
	// at that scope. Re-inviting an address replaces its open invitation.
	CreateInvitation(ctx context.Context, in identitydto.InvitationInput) (identitydto.CreatedInvitation, error)
	// ListInvitations lists invitations, open ones only unless all (user:invite).
	ListInvitations(ctx context.Context, all bool) ([]identitymodel.Invitation, error)
	// PreviewInvitation describes an invitation link (public; rate-limited by IP).
	PreviewInvitation(ctx context.Context, token string) (identitymodel.InvitationPreview, error)
	// RevokeInvitation revokes an open invitation (user:invite).
	RevokeInvitation(ctx context.Context, id uuid.UUID) error
}

// MembersUseCases is the members capability of the module.
type MembersUseCases interface {
	// GetMember returns one member with their groups (user:read).
	GetMember(ctx context.Context, id uuid.UUID) (identitymodel.OrgMember, error)
	// ListMembers lists the organization's members (user:read).
	ListMembers(ctx context.Context) ([]identitymodel.OrgMember, error)
	// RemoveMember removes a member from the organization (user:remove): their
	// role assignments, group memberships and sessions here end. The global
	// account is kept (it may belong to other organizations).
	RemoveMember(ctx context.Context, id uuid.UUID) error
	// SetMemberStatus suspends or reactivates a member (user:update). Suspension
	// ends the member's sessions in this organization and removes their access
	// (grants require an active membership). Nobody can suspend themselves or the
	// last owner.
	SetMemberStatus(ctx context.Context, id uuid.UUID, status string) (identitymodel.OrgMember, error)
}

// OIDCUseCases is the oidc capability of the module.
type OIDCUseCases interface {
	// CompleteOIDCLogin validates the callback, links or provisions the user,
	// applies group mappings and issues a session (public).
	CompleteOIDCLogin(ctx context.Context, sealedCookie string, in identitydto.OIDCCallbackParams) (identitymodel.OIDCCompletion, error)
	// CreateGroupMapping maps an external group to an internal group
	// (identity_provider:update). Without groupID, an IdP-managed group is created.
	CreateGroupMapping(ctx context.Context, id uuid.UUID, external string, groupID *uuid.UUID) (identitymodel.GroupMapping, error)
	// CreateIdentityProvider stores a provider (identity_provider:create).
	CreateIdentityProvider(ctx context.Context, in identitydto.CreateIdentityProviderInput) (identitymodel.IdentityProvider, error)
	// DeleteGroupMapping removes a mapping and clears IdP-managed memberships it
	// owned (identity_provider:update).
	DeleteGroupMapping(ctx context.Context, id uuid.UUID, mappingID uuid.UUID) error
	// DeleteIdentityProvider removes a provider with its links and mappings
	// (identity_provider:delete). Memberships in IdP-managed groups are cleared.
	DeleteIdentityProvider(ctx context.Context, id uuid.UUID) error
	// GetIdentityProvider returns one provider (identity_provider:read).
	GetIdentityProvider(ctx context.Context, id uuid.UUID) (identitymodel.IdentityProvider, error)
	// ListGroupMappings lists mappings for a provider (identity_provider:read).
	ListGroupMappings(ctx context.Context, id uuid.UUID) ([]identitymodel.GroupMapping, error)
	// ListIdentityProviders lists configured providers (identity_provider:read).
	ListIdentityProviders(ctx context.Context) ([]identitymodel.IdentityProvider, error)
	// OIDCProviderOptions lists enabled providers for sign-in (public).
	OIDCProviderOptions(ctx context.Context, slug string) ([]identitymodel.OIDCProviderOption, error)
	// RedirectURI is the callback registered at every identity provider.
	RedirectURI() string
	// SSOURL is the browser URL that starts sign-in through this provider.
	SSOURL(id uuid.UUID) string
	// StartOIDCLogin creates the authorization redirect and sealed state cookie
	// (public). The caller sets the cookie; the state binds the callback to this
	// browser.
	StartOIDCLogin(ctx context.Context, idpID uuid.UUID, redirect string) (identitymodel.OIDCStart, error)
	// TestIdentityProvider probes discovery and signing keys (identity_provider:test).
	TestIdentityProvider(ctx context.Context, id uuid.UUID) (identitymodel.IdentityProviderTest, error)
	// UpdateIdentityProvider changes a provider (identity_provider:update).
	UpdateIdentityProvider(ctx context.Context, id uuid.UUID, in identitydto.UpdateIdentityProviderInput) (identitymodel.IdentityProvider, error)
}

// ServiceaccountsUseCases is the serviceaccounts capability of the module.
type ServiceaccountsUseCases interface {
	// AuthenticateAPIKey resolves a presented API key into a service-account principal.
	AuthenticateAPIKey(ctx context.Context, key string) (principal.Principal, error)
	// CreateAPIKey issues a key for a service account (service_account:manage_keys).
	// The returned secret is shown once.
	CreateAPIKey(ctx context.Context, accountID uuid.UUID, name string, expiresAt *time.Time) (identitymodel.APIKey, string, error)
	// CreateServiceAccount creates a service account with no roles (service_account:create).
	CreateServiceAccount(ctx context.Context, in identitydto.ServiceAccountInput) (identitymodel.ServiceAccount, error)
	// DeleteServiceAccount deletes a service account with its keys and role
	// assignments (service_account:delete).
	DeleteServiceAccount(ctx context.Context, id uuid.UUID) error
	// GetServiceAccount returns one service account (service_account:read).
	GetServiceAccount(ctx context.Context, id uuid.UUID) (identitymodel.ServiceAccount, error)
	// ListAPIKeys lists a service account's keys (service_account:read).
	ListAPIKeys(ctx context.Context, accountID uuid.UUID) ([]identitymodel.APIKey, error)
	// ListServiceAccounts lists the organization's service accounts (service_account:read).
	ListServiceAccounts(ctx context.Context) ([]identitymodel.ServiceAccount, error)
	// RevokeAPIKey revokes a key (service_account:manage_keys). Effective on the next request.
	RevokeAPIKey(ctx context.Context, accountID uuid.UUID, keyID uuid.UUID) error
	// UpdateServiceAccount renames, re-describes, disables or re-enables a
	// service account (service_account:update). Disabling stops its keys at once.
	UpdateServiceAccount(ctx context.Context, id uuid.UUID, in identitydto.ServiceAccountInput) (identitymodel.ServiceAccount, error)
}

// Service is the complete use-case surface of the module.
type Service interface {
	GroupsUseCases
	InvitationsUseCases
	MembersUseCases
	OIDCUseCases
	ServiceaccountsUseCases
	// Authenticate resolves a session token into a principal. It returns the
	// session's CSRF hash for the caller to verify on unsafe requests.
	Authenticate(ctx context.Context, token string) (principal.Principal, []byte, error)
	// CurrentOrganization returns the caller's organization; identity owns the
	// session that names it, organization owns the record.
	CurrentOrganization(ctx context.Context) (orgmodel.Organization, error)
	// ListSessions returns the caller's active sessions in the current organization.
	ListSessions(ctx context.Context) ([]identitymodel.SessionInfo, error)
	// Login verifies credentials and creates a session in the chosen organization.
	Login(ctx context.Context, in identitydto.LoginInput) (identitymodel.SessionGrant, error)
	// Logout revokes the caller's current session.
	Logout(ctx context.Context) error
	// Me returns the caller's user and membership.
	Me(ctx context.Context) (identitymodel.Member, error)
	// PublicURL is the externally visible base URL used to build redirects.
	PublicURL() string
	// RevokeSession revokes one of the caller's own sessions.
	RevokeSession(ctx context.Context, id uuid.UUID) error
	// Signup registers a new organization with the caller as owner and signs them in.
	Signup(ctx context.Context, in identitydto.SignupInput) (identitymodel.SessionGrant, error)
}

var _ Service = (*IdentityService)(nil)
