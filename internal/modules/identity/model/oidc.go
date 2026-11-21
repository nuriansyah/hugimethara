package model

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
)

// IdentityProviderResource is the RBAC resource type for OIDC configuration.
const IdentityProviderResource = "identity_provider"

// IdentityProviderPresets are the supported configuration presets. The preset
// only changes UI guidance today; every provider speaks plain OIDC.
const (
	PresetEntra    = "entra"
	PresetGoogle   = "google"
	PresetKeycloak = "keycloak"
	PresetGeneric  = "generic"
)

// OIDC flow limits.
const (
	OidcStateTTL     = 10 * time.Minute
	OidcHTTPTimeout  = 15 * time.Second
	OidcDiscoveryTTL = 15 * time.Minute
	OidcMaxStates    = 8 * 1024
)

var (
	ErrProviderNotFound = problem.NotFound("Identity provider not found.")
	ErrProviderName     = problem.NewProblem(http.StatusConflict, "name_taken", "Conflict",
		"An identity provider with this name already exists.")
	ErrMappingExists = problem.NewProblem(http.StatusConflict, "mapping_exists", "Conflict",
		"This external group is already mapped for this provider.")
	ErrMappingGroup = problem.NewProblem(http.StatusConflict, "group_already_mapped", "Conflict",
		"This group is already mapped to an external group.")
	ErrGroupNameTaken = problem.NewProblem(http.StatusConflict, "group_name_taken", "Conflict",
		"A group with this name already exists. Choose it explicitly instead.")
)

// ClaimMapping says which ID-token claims carry the user's email, display
// name and groups. Groups may be empty to disable group synchronization.
type ClaimMapping struct {
	Email  string `json:"email"`
	Name   string `json:"name"`
	Groups string `json:"groups,omitempty"`
}

func DefaultClaimMapping() ClaimMapping {
	return ClaimMapping{Email: "email", Name: "name", Groups: "groups"}
}

// IdentityProvider is an organization's OIDC configuration. The client
// secret is never part of the model.
type IdentityProvider struct {
	ID                uuid.UUID
	Name              string
	Protocol          string
	Preset            string
	Issuer            string
	ClientID          string
	HasClientSecret   bool
	Scopes            []string
	ClaimMapping      ClaimMapping
	JITProvisioning   bool
	AllowedDomains    []string
	Enabled           bool
	GroupMappingCount int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// GroupMapping links one external IdP group to one internal group.
type GroupMapping struct {
	ID            uuid.UUID `db:"id"`
	ExternalGroup string    `db:"external_group"`
	GroupID       uuid.UUID `db:"group_id"`
	GroupName     string    `db:"group_name"`
	GroupSource   string    `db:"group_source"`
	CreatedAt     time.Time `db:"created_at"`
}

// IdentityProviderTest reports whether discovery and signing keys are reachable.
type IdentityProviderTest struct {
	OK                    bool
	Issuer                string
	AuthorizationEndpoint string
	TokenEndpoint         string
	JWKSUri               string
	Latency               time.Duration
	Error                 string
	CheckedAt             time.Time
}

// OIDCProviderOption is the public, minimal description used by the sign-in page.
type OIDCProviderOption struct {
	ID     uuid.UUID
	Name   string
	Preset string
}

// ProviderName returns the provider's display name; used in redirects.
func (p IdentityProvider) ProviderName() string { return p.Name }

type NormalizedProvider struct {
	Name           string       `json:"-"`
	Preset         string       `json:"-"`
	Issuer         string       `json:"-"`
	ClientID       string       `json:"-"`
	Scopes         []string     `json:"-"`
	ClaimMapping   ClaimMapping `json:"-"`
	AllowedDomains []string     `json:"-"`
}

func NormalizeProvider(n NormalizedProvider) (NormalizedProvider, error) {
	n.Name = strings.TrimSpace(n.Name)
	n.Issuer = strings.TrimSpace(n.Issuer)
	n.ClientID = strings.TrimSpace(n.ClientID)
	if n.Preset == "" {
		n.Preset = PresetGeneric
	}
	n.Scopes = normalizeScopes(n.Scopes)
	n.AllowedDomains = normalizeDomains(n.AllowedDomains)
	n.ClaimMapping = normalizeClaimMapping(n.ClaimMapping)

	var errs []problem.FieldError
	if l := utf8.RuneCountInString(n.Name); l < 1 || l > 100 {
		errs = append(errs, problem.FieldError{Field: "name", Code: "invalid_length", Message: "Name must be 1–100 characters."})
	}
	switch n.Preset {
	case PresetEntra, PresetGoogle, PresetKeycloak, PresetGeneric:
	default:
		errs = append(errs, problem.FieldError{Field: "preset", Code: "invalid", Message: "Preset must be entra, google, keycloak or generic."})
	}
	if err := validIssuer(n.Issuer); err != nil {
		errs = append(errs, problem.FieldError{Field: "issuer", Code: "invalid_url", Message: err.Error()})
	}
	if l := len(n.ClientID); l < 1 || l > 255 {
		errs = append(errs, problem.FieldError{Field: "client_id", Code: "invalid_length", Message: "Client ID must be 1–255 characters."})
	}
	if len(n.Scopes) == 0 {
		errs = append(errs, problem.FieldError{Field: "scopes", Code: "required", Message: "At least the openid scope is required."})
	}
	for _, d := range n.AllowedDomains {
		if !validDomain(d) {
			errs = append(errs, problem.FieldError{Field: "allowed_domains", Code: "invalid_domain", Message: "Allowed domains must be plain email domains, e.g. example.com."})
			break
		}
	}
	for _, f := range []struct{ field, value string }{
		{"claim_mapping.email", n.ClaimMapping.Email},
		{"claim_mapping.name", n.ClaimMapping.Name},
		{"claim_mapping.groups", n.ClaimMapping.Groups},
	} {
		if l := len(f.value); l > 200 || strings.ContainsAny(f.value, " \t\r\n") {
			errs = append(errs, problem.FieldError{Field: f.field, Code: "invalid", Message: "Claim names must be at most 200 characters without spaces."})
		}
	}
	if len(errs) > 0 {
		return n, problem.Validation(errs...)
	}
	return n, nil
}

func normalizeScopes(in []string) []string {
	out := make([]string, 0, len(in)+1)
	seen := map[string]bool{"openid": true}
	out = append(out, "openid")
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func normalizeDomains(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, d := range in {
		d = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(d), "@")))
		if d == "" || seen[d] {
			continue
		}
		seen[d] = true
		out = append(out, d)
	}
	return out
}

func normalizeClaimMapping(in ClaimMapping) ClaimMapping {
	in.Email = strings.TrimSpace(in.Email)
	in.Name = strings.TrimSpace(in.Name)
	in.Groups = strings.TrimSpace(in.Groups)
	if in.Email == "" {
		in.Email = "email"
	}
	if in.Name == "" {
		in.Name = "name"
	}
	return in
}

func validIssuer(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("issuer must be an absolute http(s) URL")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("issuer must not contain credentials, a query or a fragment")
	}
	return nil
}

func validDomain(d string) bool {
	if len(d) < 3 || len(d) > 255 || strings.ContainsAny(d, " /@\\\t\r\n") {
		return false
	}
	return strings.Contains(d, ".")
}

var ErrMappingNotFound = problem.NotFound("Group mapping not found.")

// oidcState is sealed into a short-lived cookie. The browser never sees its
// contents (beyond the opaque value), and the cookie binds the callback to
// the browser that started the flow.
type OidcState struct {
	IDP      uuid.UUID `json:"idp"`
	Org      uuid.UUID `json:"org"`
	State    string    `json:"state"`
	Nonce    string    `json:"nonce"`
	Verifier string    `json:"verifier"`
	Redirect string    `json:"redirect"`
	Expires  time.Time `json:"expires"`
}

// OIDCStart is a prepared authorization redirect.
type OIDCStart struct {
	AuthURL     string
	SealedState string
	Expires     time.Time
}

func RandomString(bytes int) string {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		panic("identity: crypto/rand failed: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func SafeRedirect(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "/", nil
	}
	if len(raw) > 512 || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") ||
		strings.ContainsAny(raw, "\\\r\n") {
		return "", problem.Validation(problem.FieldError{Field: "redirect", Code: "invalid", Message: "Redirect must be a relative path on this site."})
	}
	return raw, nil
}

// ssoError is an expected sign-in failure that becomes a redirect with a
// stable code the web app can explain. Unexpected errors stay problems.
type ssoError struct{ code string }

func (e *ssoError) Error() string { return "identity: sso " + e.code }

func SsoFail(code string) error { return &ssoError{code: code} }

// SSOErrorCode extracts the redirect code from an expected sign-in failure.
func SSOErrorCode(err error) (string, bool) {
	var e *ssoError
	if errors.As(err, &e) {
		return e.code, true
	}
	return "", false
}

// OIDCCompletion is a successful sign-in.
type OIDCCompletion struct {
	Grant    SessionGrant
	Redirect string
}

type OidcClaims struct {
	Email         string
	EmailVerified *bool
	DisplayName   string
	Groups        []string
}

func ParseClaims(raw map[string]any, mapping ClaimMapping, subject string) OidcClaims {
	c := OidcClaims{Email: NormalizeEmail(claimString(raw, mapping.Email))}
	if c.Email == "" {
		c.Email = NormalizeEmail(claimString(raw, "preferred_username"))
	}
	c.DisplayName = strings.TrimSpace(claimString(raw, mapping.Name))
	if c.DisplayName == "" {
		c.DisplayName = strings.TrimSpace(claimString(raw, "preferred_username"))
	}
	if c.DisplayName == "" && strings.Contains(c.Email, "@") {
		c.DisplayName = c.Email[:strings.Index(c.Email, "@")]
	}
	if c.DisplayName == "" {
		c.DisplayName = "User " + shortSubject(subject)
	}
	if utf8.RuneCountInString(c.DisplayName) > 100 {
		c.DisplayName = string([]rune(c.DisplayName)[:100])
	}
	if mapping.Groups != "" {
		c.Groups = claimStrings(raw[mapping.Groups])
	}
	if v, ok := raw["email_verified"]; ok {
		switch t := v.(type) {
		case bool:
			c.EmailVerified = &t
		case string:
			b := strings.EqualFold(t, "true")
			c.EmailVerified = &b
		}
	}
	return c
}

func shortSubject(subject string) string {
	if len(subject) > 8 {
		return subject[:8]
	}
	if subject == "" {
		return "unknown"
	}
	return subject
}

func claimString(raw map[string]any, name string) string {
	if name == "" {
		return ""
	}
	v, ok := raw[name]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func claimStrings(v any) []string {
	var out []string
	switch t := v.(type) {
	case []any:
		for _, e := range t {
			if s, ok := e.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
	case []string:
		for _, s := range t {
			if strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
	case string:
		out = strings.FieldsFunc(t, func(r rune) bool { return r == ' ' || r == ',' })
	}
	return out
}

func DomainAllowed(domain string, allowed []string) bool {
	for _, d := range allowed {
		if strings.EqualFold(domain, d) {
			return true
		}
	}
	return false
}

type oidcStateCookieKey struct{}

func WithOIDCStateCookie(ctx context.Context, value string) context.Context {
	return context.WithValue(ctx, oidcStateCookieKey{}, value)
}

func OidcStateCookieFrom(ctx context.Context) string {
	v, _ := ctx.Value(oidcStateCookieKey{}).(string)
	return v
}
