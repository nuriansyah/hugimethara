package dto

import "github.com/nuriansyah/hugimethara/internal/modules/identity/model"

// CreateIdentityProviderInput is a new provider configuration.
type CreateIdentityProviderInput struct {
	Name            string
	Preset          string
	Issuer          string
	ClientID        string
	ClientSecret    *string
	Scopes          []string
	ClaimMapping    *model.ClaimMapping
	JITProvisioning *bool
	AllowedDomains  []string
	Enabled         *bool
}

// UpdateIdentityProviderInput changes a provider. Nil fields are kept; a
// non-nil empty client secret removes the stored secret.
type UpdateIdentityProviderInput struct {
	Name            *string
	Preset          *string
	Issuer          *string
	ClientID        *string
	ClientSecret    *string
	Scopes          *[]string
	ClaimMapping    *model.ClaimMapping
	JITProvisioning *bool
	AllowedDomains  *[]string
	Enabled         *bool
}

// OIDCCallbackParams carries the provider's callback query parameters.
type OIDCCallbackParams struct {
	Code             string
	State            string
	Error            string
	ErrorDescription string
}
