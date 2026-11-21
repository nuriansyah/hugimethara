package repository

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	identitymodel "github.com/nuriansyah/hugimethara/internal/modules/identity/model"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
)

type ProviderRow struct {
	ID                uuid.UUID    `db:"id"`
	Name              string       `db:"name"`
	Protocol          string       `db:"protocol"`
	Preset            string       `db:"preset"`
	Issuer            string       `db:"issuer"`
	ClientID          string       `db:"client_id"`
	HasClientSecret   bool         `db:"has_client_secret"`
	Scopes            db.TextArray `db:"scopes"`
	ClaimMapping      []byte       `db:"claim_mapping"`
	JITProvisioning   bool         `db:"jit_provisioning"`
	AllowedDomains    db.TextArray `db:"allowed_domains"`
	Enabled           bool         `db:"enabled"`
	GroupMappingCount int64        `db:"group_mapping_count"`
	CreatedAt         time.Time    `db:"created_at"`
	UpdatedAt         time.Time    `db:"updated_at"`
}

func (r ProviderRow) Model() identitymodel.IdentityProvider {
	p := identitymodel.IdentityProvider{
		ID: r.ID, Name: r.Name, Protocol: r.Protocol, Preset: r.Preset, Issuer: r.Issuer,
		ClientID: r.ClientID, HasClientSecret: r.HasClientSecret, Scopes: r.Scopes,
		JITProvisioning: r.JITProvisioning, AllowedDomains: r.AllowedDomains, Enabled: r.Enabled,
		GroupMappingCount: r.GroupMappingCount, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
	if len(r.ClaimMapping) > 0 {
		_ = json.Unmarshal(r.ClaimMapping, &p.ClaimMapping)
	}
	if p.ClaimMapping.Email == "" {
		p.ClaimMapping = identitymodel.DefaultClaimMapping()
	}
	if p.Scopes == nil {
		p.Scopes = []string{}
	}
	if p.AllowedDomains == nil {
		p.AllowedDomains = []string{}
	}
	return p
}

type IdentityLinkRow struct {
	LinkID           uuid.UUID    `db:"link_id"`
	UserID           uuid.UUID    `db:"user_id"`
	Email            string       `db:"email"`
	DisplayName      string       `db:"display_name"`
	UserStatus       string       `db:"user_status"`
	MembershipStatus string       `db:"membership_status"`
	LastLoginAt      sql.NullTime `db:"last_login_at"`
}

type OidcProviderAuth struct {
	ProviderID      uuid.UUID    `db:"provider_id"`
	OrgID           uuid.UUID    `db:"org_id"`
	OrgSlug         string       `db:"org_slug"`
	OrgName         string       `db:"org_name"`
	OrgSettings     []byte       `db:"org_settings"`
	OrgStatus       string       `db:"org_status"`
	ProviderName    string       `db:"provider_name"`
	Preset          string       `db:"preset"`
	Issuer          string       `db:"issuer"`
	ClientID        string       `db:"client_id"`
	ClientSecret    []byte       `db:"client_secret"`
	Scopes          db.TextArray `db:"scopes"`
	ClaimMapping    []byte       `db:"claim_mapping"`
	JITProvisioning bool         `db:"jit_provisioning"`
	AllowedDomains  db.TextArray `db:"allowed_domains"`
	Enabled         bool         `db:"enabled"`
}

func (r OidcProviderAuth) Provider() identitymodel.IdentityProvider {
	p := identitymodel.IdentityProvider{
		ID: r.ProviderID, Name: r.ProviderName, Protocol: "oidc", Preset: r.Preset, Issuer: r.Issuer,
		ClientID: r.ClientID, HasClientSecret: len(r.ClientSecret) > 0, Scopes: r.Scopes,
		JITProvisioning: r.JITProvisioning, AllowedDomains: r.AllowedDomains, Enabled: r.Enabled,
	}
	if len(r.ClaimMapping) > 0 {
		_ = json.Unmarshal(r.ClaimMapping, &p.ClaimMapping)
	}
	if p.ClaimMapping.Email == "" {
		p.ClaimMapping = identitymodel.DefaultClaimMapping()
	}
	return p
}
