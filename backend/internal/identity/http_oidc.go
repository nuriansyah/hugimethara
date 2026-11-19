package identity

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nuriansyah/hugimethara/backend/internal/api/apigen"
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

func (h *HTTP) ListOidcProviders(ctx context.Context, req apigen.ListOidcProvidersRequestObject) (apigen.ListOidcProvidersResponseObject, error) {
	options, err := h.svc.OIDCProviderOptions(ctx, req.Params.Organization)
	if err != nil {
		return nil, err
	}
	out := apigen.ListOidcProviders200JSONResponse{Data: make([]apigen.OidcProviderOption, len(options))}
	for i, o := range options {
		out.Data[i] = apigen.OidcProviderOption{Id: o.ID, Name: o.Name, Preset: apigen.IdentityProviderPreset(o.Preset)}
	}
	return out, nil
}

func (h *HTTP) StartOidcLogin(ctx context.Context, req apigen.StartOidcLoginRequestObject) (apigen.StartOidcLoginResponseObject, error) {
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

func (h *HTTP) CompleteOidcLogin(ctx context.Context, req apigen.CompleteOidcLoginRequestObject) (apigen.CompleteOidcLoginResponseObject, error) {
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

func (h *HTTP) apiProvider(p IdentityProvider) apigen.IdentityProvider {
	cm := apigen.ClaimMapping{Email: p.ClaimMapping.Email, Name: p.ClaimMapping.Name}
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
	return apigen.IdentityProvider{
		Id: p.ID, Name: p.Name, Protocol: apigen.IdentityProviderProtocol(p.Protocol),
		Preset: apigen.IdentityProviderPreset(p.Preset), Issuer: p.Issuer, ClientId: p.ClientID,
		HasClientSecret: p.HasClientSecret, Scopes: scopes, ClaimMapping: cm, JitProvisioning: p.JITProvisioning,
		AllowedDomains: domains, Enabled: p.Enabled, RedirectUri: h.svc.RedirectURI(), SsoUrl: h.svc.SSOURL(p.ID),
		GroupMappingCount: p.GroupMappingCount, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func apiGroupMapping(m GroupMapping) apigen.GroupMapping {
	return apigen.GroupMapping{
		Id: m.ID, ExternalGroup: m.ExternalGroup, GroupId: m.GroupID, GroupName: m.GroupName,
		GroupSource: apigen.GroupMappingGroupSource(m.GroupSource), CreatedAt: m.CreatedAt,
	}
}

func claimMappingFromAPI(cm *apigen.ClaimMapping) *ClaimMapping {
	if cm == nil {
		return nil
	}
	out := ClaimMapping{Email: cm.Email, Name: cm.Name}
	if cm.Groups != nil {
		out.Groups = *cm.Groups
	}
	return &out
}

func (h *HTTP) ListIdentityProviders(ctx context.Context, _ apigen.ListIdentityProvidersRequestObject) (apigen.ListIdentityProvidersResponseObject, error) {
	providers, err := h.svc.ListIdentityProviders(ctx)
	if err != nil {
		return nil, err
	}
	out := apigen.ListIdentityProviders200JSONResponse{Data: make([]apigen.IdentityProvider, len(providers))}
	for i, p := range providers {
		out.Data[i] = h.apiProvider(p)
	}
	return out, nil
}

func (h *HTTP) CreateIdentityProvider(ctx context.Context, req apigen.CreateIdentityProviderRequestObject) (apigen.CreateIdentityProviderResponseObject, error) {
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
	return apigen.CreateIdentityProvider201JSONResponse(h.apiProvider(p)), nil
}

func (h *HTTP) GetIdentityProvider(ctx context.Context, req apigen.GetIdentityProviderRequestObject) (apigen.GetIdentityProviderResponseObject, error) {
	p, err := h.svc.GetIdentityProvider(ctx, req.ProviderId)
	if err != nil {
		return nil, err
	}
	return apigen.GetIdentityProvider200JSONResponse(h.apiProvider(p)), nil
}

func (h *HTTP) UpdateIdentityProvider(ctx context.Context, req apigen.UpdateIdentityProviderRequestObject) (apigen.UpdateIdentityProviderResponseObject, error) {
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
	return apigen.UpdateIdentityProvider200JSONResponse(h.apiProvider(p)), nil
}

func (h *HTTP) DeleteIdentityProvider(ctx context.Context, req apigen.DeleteIdentityProviderRequestObject) (apigen.DeleteIdentityProviderResponseObject, error) {
	if err := h.svc.DeleteIdentityProvider(ctx, req.ProviderId); err != nil {
		return nil, err
	}
	return apigen.DeleteIdentityProvider204Response{}, nil
}

func (h *HTTP) TestIdentityProvider(ctx context.Context, req apigen.TestIdentityProviderRequestObject) (apigen.TestIdentityProviderResponseObject, error) {
	res, err := h.svc.TestIdentityProvider(ctx, req.ProviderId)
	if err != nil {
		return nil, err
	}
	out := apigen.IdentityProviderTest{
		Ok: res.OK, LatencyMs: res.Latency.Milliseconds(), CheckedAt: res.CheckedAt,
		Issuer: strPtr(res.Issuer), AuthorizationEndpoint: strPtr(res.AuthorizationEndpoint),
		TokenEndpoint: strPtr(res.TokenEndpoint), JwksUri: strPtr(res.JWKSUri), Error: strPtr(res.Error),
	}
	return apigen.TestIdentityProvider200JSONResponse(out), nil
}

// Group mappings --------------------------------------------------------------------

func (h *HTTP) ListGroupMappings(ctx context.Context, req apigen.ListGroupMappingsRequestObject) (apigen.ListGroupMappingsResponseObject, error) {
	mappings, err := h.svc.ListGroupMappings(ctx, req.ProviderId)
	if err != nil {
		return nil, err
	}
	out := apigen.ListGroupMappings200JSONResponse{Data: make([]apigen.GroupMapping, len(mappings))}
	for i, m := range mappings {
		out.Data[i] = apiGroupMapping(m)
	}
	return out, nil
}

func (h *HTTP) CreateGroupMapping(ctx context.Context, req apigen.CreateGroupMappingRequestObject) (apigen.CreateGroupMappingResponseObject, error) {
	if req.Body == nil {
		return nil, bodyRequired()
	}
	m, err := h.svc.CreateGroupMapping(ctx, req.ProviderId, req.Body.ExternalGroup, req.Body.GroupId)
	if err != nil {
		return nil, err
	}
	return apigen.CreateGroupMapping201JSONResponse(apiGroupMapping(m)), nil
}

func (h *HTTP) DeleteGroupMapping(ctx context.Context, req apigen.DeleteGroupMappingRequestObject) (apigen.DeleteGroupMappingResponseObject, error) {
	if err := h.svc.DeleteGroupMapping(ctx, req.ProviderId, req.MappingId); err != nil {
		return nil, err
	}
	return apigen.DeleteGroupMapping204Response{}, nil
}

// clearOIDCState expires the flow cookie.
func (c Cookies) clearOIDCState() *http.Cookie {
	//nolint:gosec // G124: HttpOnly + Secure (except plain-HTTP development) + SameSite=Lax.
	return &http.Cookie{
		Name: c.OIDCStateName(), Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(0, 0),
		HttpOnly: true, Secure: c.Secure, SameSite: http.SameSiteLaxMode,
	}
}
