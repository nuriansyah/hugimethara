package identity

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nuriansyah/hugimethara/internal/transport/http/dto"
)

// oidcRedirect writes a 302 with cookies; it satisfies both browser-facing
// response interfaces (start and callback).
type oidcRedirect struct {
	location string
	cookies  []*http.Cookie
}

func (r oidcRedirect) visit(w http.ResponseWriter) error {
	for _, c := range r.cookies {
		http.SetCookie(w, c)
	}
	w.Header().Set("Location", r.location)
	w.WriteHeader(http.StatusFound)
	return nil
}

func (r oidcRedirect) VisitStartOidcLoginResponse(w http.ResponseWriter) error {
	return r.visit(w)
}

func (r oidcRedirect) VisitCompleteOidcLoginResponse(w http.ResponseWriter) error {
	return r.visit(w)
}

func (h *HTTP) loginFailure(code string) string {
	return strings.TrimSuffix(h.svc.cfg.PublicURL, "/") + "/login?sso_error=" + url.QueryEscape(code)
}

// @Summary Enabled SSO providers of an organization (public)
// @ID listOidcProviders
// @Tags auth
// @Param organization query string true "Organization slug."
// @Success 200 {object} dto.ListOidcProviders200JSONResponse "Enabled identity providers."
// @Failure 429 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Router /v1/hugimethara/auth/oidc/providers [get]
func (h *HTTP) ListOidcProviders(ctx context.Context, req dto.ListOidcProvidersRequestObject) (dto.ListOidcProvidersResponseObject, error) {
	options, err := h.svc.OIDCProviderOptions(ctx, req.Params.Organization)
	if err != nil {
		return nil, err
	}
	out := dto.ListOidcProviders200JSONResponse{Data: make([]dto.OidcProviderOption, len(options))}
	for i, o := range options {
		out.Data[i] = dto.OidcProviderOption{Id: o.ID, Name: o.Name, Preset: dto.IdentityProviderPreset(o.Preset)}
	}
	return out, nil
}

// @Summary Begin an OIDC sign-in (public)
// @ID startOidcLogin
// @Tags auth
// @Param idp_id path string true "idp_id"
// @Param redirect query string false "redirect"
// @Success 302 "Redirect to the identity provider (or to the sign-in page on failure)."
// @Failure 429 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Router /v1/hugimethara/auth/oidc/{idp_id}/start [get]
func (h *HTTP) StartOidcLogin(ctx context.Context, req dto.StartOidcLoginRequestObject) (dto.StartOidcLoginResponseObject, error) {
	redirect := ""
	if req.Params.Redirect != nil {
		redirect = *req.Params.Redirect
	}
	start, err := h.svc.StartOIDCLogin(ctx, req.IdpId, redirect)
	if err != nil {
		if code, ok := SSOErrorCode(err); ok {
			return oidcRedirect{location: h.loginFailure(code)}, nil
		}
		return nil, err
	}
	return oidcRedirect{
		location: start.AuthURL,
		cookies:  []*http.Cookie{h.cookies.oidcState(start.SealedState, start.Expires)},
	}, nil
}

// @Summary Finish an OIDC sign-in (public)
// @ID completeOidcLogin
// @Tags auth
// @Param code query string false "code"
// @Param state query string false "state"
// @Param error query string false "error"
// @Param error_description query string false "error_description"
// @Success 302 "Redirect to the web app; session cookies are set on success."
// @Failure 429 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Router /v1/hugimethara/auth/oidc/callback [get]
func (h *HTTP) CompleteOidcLogin(ctx context.Context, req dto.CompleteOidcLoginRequestObject) (dto.CompleteOidcLoginResponseObject, error) {
	params := OIDCCallbackParams{}
	if req.Params.Code != nil {
		params.Code = *req.Params.Code
	}
	if req.Params.State != nil {
		params.State = *req.Params.State
	}
	if req.Params.Error != nil {
		params.Error = *req.Params.Error
	}
	if req.Params.ErrorDescription != nil {
		params.ErrorDescription = *req.Params.ErrorDescription
	}
	clear := h.cookies.clearOIDCState()
	result, err := h.svc.CompleteOIDCLogin(ctx, oidcStateCookieFrom(ctx), params)
	if err != nil {
		if code, ok := SSOErrorCode(err); ok {
			return oidcRedirect{location: h.loginFailure(code), cookies: []*http.Cookie{clear}}, nil
		}
		return nil, err
	}
	cookies := append(h.cookies.issue(result.Grant), clear)
	return oidcRedirect{
		location: strings.TrimSuffix(h.svc.cfg.PublicURL, "/") + result.Redirect,
		cookies:  cookies,
	}, nil
}

// Identity provider configuration ---------------------------------------------------

func (h *HTTP) apiProvider(p IdentityProvider) dto.IdentityProvider {
	cm := dto.ClaimMapping{Email: p.ClaimMapping.Email, Name: p.ClaimMapping.Name}
	if p.ClaimMapping.Groups != "" {
		groups := p.ClaimMapping.Groups
		cm.Groups = &groups
	}
	scopes := p.Scopes
	if scopes == nil {
		scopes = []string{}
	}
	domains := p.AllowedDomains
	if domains == nil {
		domains = []string{}
	}
	return dto.IdentityProvider{
		Id: p.ID, Name: p.Name, Protocol: dto.IdentityProviderProtocol(p.Protocol),
		Preset: dto.IdentityProviderPreset(p.Preset), Issuer: p.Issuer, ClientId: p.ClientID,
		HasClientSecret: p.HasClientSecret, Scopes: scopes, ClaimMapping: cm, JitProvisioning: p.JITProvisioning,
		AllowedDomains: domains, Enabled: p.Enabled, RedirectUri: h.svc.RedirectURI(), SsoUrl: h.svc.SSOURL(p.ID),
		GroupMappingCount: p.GroupMappingCount, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func apiGroupMapping(m GroupMapping) dto.GroupMapping {
	return dto.GroupMapping{
		Id: m.ID, ExternalGroup: m.ExternalGroup, GroupId: m.GroupID, GroupName: m.GroupName,
		GroupSource: dto.GroupMappingGroupSource(m.GroupSource), CreatedAt: m.CreatedAt,
	}
}

func claimMappingFromAPI(cm *dto.ClaimMapping) *ClaimMapping {
	if cm == nil {
		return nil
	}
	out := ClaimMapping{Email: cm.Email, Name: cm.Name}
	if cm.Groups != nil {
		out.Groups = *cm.Groups
	}
	return &out
}

// @Summary OIDC identity providers
// @ID listIdentityProviders
// @Tags identity
// @Success 200 {object} dto.ListIdentityProviders200JSONResponse "Providers."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/identity/providers [get]
func (h *HTTP) ListIdentityProviders(ctx context.Context, _ dto.ListIdentityProvidersRequestObject) (dto.ListIdentityProvidersResponseObject, error) {
	providers, err := h.svc.ListIdentityProviders(ctx)
	if err != nil {
		return nil, err
	}
	out := dto.ListIdentityProviders200JSONResponse{Data: make([]dto.IdentityProvider, len(providers))}
	for i, p := range providers {
		out.Data[i] = h.apiProvider(p)
	}
	return out, nil
}

// @Summary Configure an OIDC identity provider
// @ID createIdentityProvider
// @Tags identity
// @Param body body dto.IdentityProviderCreate true "Request body"
// @Success 201 {object} dto.IdentityProvider "Created."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/identity/providers [post]
func (h *HTTP) CreateIdentityProvider(ctx context.Context, req dto.CreateIdentityProviderRequestObject) (dto.CreateIdentityProviderResponseObject, error) {
	if req.Body == nil {
		return nil, bodyRequired()
	}
	b := req.Body
	in := CreateIdentityProviderInput{
		Name: b.Name, Preset: string(b.Preset), Issuer: b.Issuer, ClientID: b.ClientId,
		ClientSecret: b.ClientSecret, ClaimMapping: claimMappingFromAPI(b.ClaimMapping),
		JITProvisioning: b.JitProvisioning, Enabled: b.Enabled,
	}
	if b.Scopes != nil {
		in.Scopes = *b.Scopes
	}
	if b.AllowedDomains != nil {
		in.AllowedDomains = *b.AllowedDomains
	}
	p, err := h.svc.CreateIdentityProvider(ctx, in)
	if err != nil {
		return nil, err
	}
	return dto.CreateIdentityProvider201JSONResponse(h.apiProvider(p)), nil
}

// @Summary One identity provider
// @ID getIdentityProvider
// @Tags identity
// @Param provider_id path string true "provider_id"
// @Success 200 {object} dto.IdentityProvider "The provider."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/identity/providers/{provider_id} [get]
func (h *HTTP) GetIdentityProvider(ctx context.Context, req dto.GetIdentityProviderRequestObject) (dto.GetIdentityProviderResponseObject, error) {
	p, err := h.svc.GetIdentityProvider(ctx, req.ProviderId)
	if err != nil {
		return nil, err
	}
	return dto.GetIdentityProvider200JSONResponse(h.apiProvider(p)), nil
}

// @Summary Update an identity provider
// @ID updateIdentityProvider
// @Tags identity
// @Param provider_id path string true "provider_id"
// @Param body body dto.IdentityProviderUpdate true "Request body"
// @Success 200 {object} dto.IdentityProvider "Updated."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/identity/providers/{provider_id} [patch]
func (h *HTTP) UpdateIdentityProvider(ctx context.Context, req dto.UpdateIdentityProviderRequestObject) (dto.UpdateIdentityProviderResponseObject, error) {
	if req.Body == nil {
		return nil, bodyRequired()
	}
	b := req.Body
	in := UpdateIdentityProviderInput{
		Name: b.Name, ClientID: b.ClientId, ClientSecret: b.ClientSecret,
		ClaimMapping: claimMappingFromAPI(b.ClaimMapping), JITProvisioning: b.JitProvisioning, Enabled: b.Enabled,
	}
	if b.Preset != nil {
		preset := string(*b.Preset)
		in.Preset = &preset
	}
	in.Issuer = b.Issuer
	in.Scopes = b.Scopes
	in.AllowedDomains = b.AllowedDomains
	p, err := h.svc.UpdateIdentityProvider(ctx, req.ProviderId, in)
	if err != nil {
		return nil, err
	}
	return dto.UpdateIdentityProvider200JSONResponse(h.apiProvider(p)), nil
}

// @Summary Delete an identity provider, its links and group mappings
// @ID deleteIdentityProvider
// @Tags identity
// @Param provider_id path string true "provider_id"
// @Success 204 "Deleted."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/identity/providers/{provider_id} [delete]
func (h *HTTP) DeleteIdentityProvider(ctx context.Context, req dto.DeleteIdentityProviderRequestObject) (dto.DeleteIdentityProviderResponseObject, error) {
	if err := h.svc.DeleteIdentityProvider(ctx, req.ProviderId); err != nil {
		return nil, err
	}
	return dto.DeleteIdentityProvider204Response{}, nil
}

// @Summary Check discovery and signing-key reachability
// @ID testIdentityProvider
// @Tags identity
// @Param provider_id path string true "provider_id"
// @Success 200 {object} dto.IdentityProviderTest "The probe result (ok=false carries a sanitized error)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/identity/providers/{provider_id}/test [post]
func (h *HTTP) TestIdentityProvider(ctx context.Context, req dto.TestIdentityProviderRequestObject) (dto.TestIdentityProviderResponseObject, error) {
	res, err := h.svc.TestIdentityProvider(ctx, req.ProviderId)
	if err != nil {
		return nil, err
	}
	out := dto.IdentityProviderTest{
		Ok: res.OK, LatencyMs: res.Latency.Milliseconds(), CheckedAt: res.CheckedAt,
		Issuer: strPtr(res.Issuer), AuthorizationEndpoint: strPtr(res.AuthorizationEndpoint),
		TokenEndpoint: strPtr(res.TokenEndpoint), JwksUri: strPtr(res.JWKSUri), Error: strPtr(res.Error),
	}
	return dto.TestIdentityProvider200JSONResponse(out), nil
}

// Group mappings --------------------------------------------------------------------

// @Summary External-group to internal-group mappings
// @ID listGroupMappings
// @Tags identity
// @Param provider_id path string true "provider_id"
// @Success 200 {object} dto.ListGroupMappings200JSONResponse "Mappings."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/identity/providers/{provider_id}/group-mappings [get]
func (h *HTTP) ListGroupMappings(ctx context.Context, req dto.ListGroupMappingsRequestObject) (dto.ListGroupMappingsResponseObject, error) {
	mappings, err := h.svc.ListGroupMappings(ctx, req.ProviderId)
	if err != nil {
		return nil, err
	}
	out := dto.ListGroupMappings200JSONResponse{Data: make([]dto.GroupMapping, len(mappings))}
	for i, m := range mappings {
		out.Data[i] = apiGroupMapping(m)
	}
	return out, nil
}

// @Summary Map an IdP group to an internal group
// @ID createGroupMapping
// @Tags identity
// @Param provider_id path string true "provider_id"
// @Param body body dto.GroupMappingCreate true "Request body"
// @Success 201 {object} dto.GroupMapping "Created."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/identity/providers/{provider_id}/group-mappings [post]
func (h *HTTP) CreateGroupMapping(ctx context.Context, req dto.CreateGroupMappingRequestObject) (dto.CreateGroupMappingResponseObject, error) {
	if req.Body == nil {
		return nil, bodyRequired()
	}
	m, err := h.svc.CreateGroupMapping(ctx, req.ProviderId, req.Body.ExternalGroup, req.Body.GroupId)
	if err != nil {
		return nil, err
	}
	return dto.CreateGroupMapping201JSONResponse(apiGroupMapping(m)), nil
}

// @Summary Remove a group mapping
// @ID deleteGroupMapping
// @Tags identity
// @Param provider_id path string true "provider_id"
// @Param mapping_id path string true "mapping_id"
// @Success 204 "Removed."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/identity/providers/{provider_id}/group-mappings/{mapping_id} [delete]
func (h *HTTP) DeleteGroupMapping(ctx context.Context, req dto.DeleteGroupMappingRequestObject) (dto.DeleteGroupMappingResponseObject, error) {
	if err := h.svc.DeleteGroupMapping(ctx, req.ProviderId, req.MappingId); err != nil {
		return nil, err
	}
	return dto.DeleteGroupMapping204Response{}, nil
}

// clearOIDCState expires the flow cookie.
func (c Cookies) clearOIDCState() *http.Cookie {
	//nolint:gosec // G124: HttpOnly + Secure (except plain-HTTP development) + SameSite=Lax.
	return &http.Cookie{
		Name: c.OIDCStateName(), Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(0, 0),
		HttpOnly: true, Secure: c.Secure, SameSite: http.SameSiteLaxMode,
	}
}
