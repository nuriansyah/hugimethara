package identity

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
	"golang.org/x/oauth2"

	"github.com/nuriansyah/hugimethara/internal/modules/authorization"
	"github.com/nuriansyah/hugimethara/internal/modules/governance/audit"
	"github.com/nuriansyah/hugimethara/internal/modules/organization"
	"github.com/nuriansyah/hugimethara/internal/modules/workflow/event"
	"github.com/nuriansyah/hugimethara/internal/platform/database"
	"github.com/nuriansyah/hugimethara/internal/platform/httpx"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
	"github.com/nuriansyah/hugimethara/internal/platform/ratelimit"
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
	oidcStateTTL     = 10 * time.Minute
	oidcHTTPTimeout  = 15 * time.Second
	oidcDiscoveryTTL = 15 * time.Minute
	oidcMaxStates    = 8 * 1024
)

var (
	ruleOIDCStart    = ratelimit.Rule{Name: "oidc_start_ip", Max: 30, Window: 5 * time.Minute}
	ruleOIDCCallback = ratelimit.Rule{Name: "oidc_callback_ip", Max: 60, Window: 5 * time.Minute}
	ruleOIDCLookup   = ratelimit.Rule{Name: "oidc_lookup_ip", Max: 120, Window: 5 * time.Minute}
)

var (
	errProviderNotFound = httpx.NotFound("Identity provider not found.")
	errProviderName     = httpx.NewProblem(http.StatusConflict, "name_taken", "Conflict",
		"An identity provider with this name already exists.")
	errMappingExists = httpx.NewProblem(http.StatusConflict, "mapping_exists", "Conflict",
		"This external group is already mapped for this provider.")
	errMappingGroup = httpx.NewProblem(http.StatusConflict, "group_already_mapped", "Conflict",
		"This group is already mapped to an external group.")
	errGroupNameTaken = httpx.NewProblem(http.StatusConflict, "group_name_taken", "Conflict",
		"A group with this name already exists. Choose it explicitly instead.")
)

// ClaimMapping says which ID-token claims carry the user's email, display
// name and groups. Groups may be empty to disable group synchronization.
type ClaimMapping struct {
	Email  string `json:"email"`
	Name   string `json:"name"`
	Groups string `json:"groups,omitempty"`
}

func defaultClaimMapping() ClaimMapping {
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

type providerRow struct {
	ID                uuid.UUID `db:"id"`
	Name              string    `db:"name"`
	Protocol          string    `db:"protocol"`
	Preset            string    `db:"preset"`
	Issuer            string    `db:"issuer"`
	ClientID          string    `db:"client_id"`
	HasClientSecret   bool      `db:"has_client_secret"`
	Scopes            []string  `db:"scopes"`
	ClaimMapping      []byte    `db:"claim_mapping"`
	JITProvisioning   bool      `db:"jit_provisioning"`
	AllowedDomains    []string  `db:"allowed_domains"`
	Enabled           bool      `db:"enabled"`
	GroupMappingCount int64     `db:"group_mapping_count"`
	CreatedAt         time.Time `db:"created_at"`
	UpdatedAt         time.Time `db:"updated_at"`
}

func (r providerRow) model() IdentityProvider {
	p := IdentityProvider{
		ID: r.ID, Name: r.Name, Protocol: r.Protocol, Preset: r.Preset, Issuer: r.Issuer,
		ClientID: r.ClientID, HasClientSecret: r.HasClientSecret, Scopes: r.Scopes,
		JITProvisioning: r.JITProvisioning, AllowedDomains: r.AllowedDomains, Enabled: r.Enabled,
		GroupMappingCount: r.GroupMappingCount, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
	if len(r.ClaimMapping) > 0 {
		_ = json.Unmarshal(r.ClaimMapping, &p.ClaimMapping)
	}
	if p.ClaimMapping.Email == "" {
		p.ClaimMapping = defaultClaimMapping()
	}
	if p.Scopes == nil {
		p.Scopes = []string{}
	}
	if p.AllowedDomains == nil {
		p.AllowedDomains = []string{}
	}
	return p
}

// RedirectURI is the callback registered at every identity provider.
func (s *Service) RedirectURI() string {
	return strings.TrimSuffix(s.cfg.PublicURL, "/") + "/api/v1/hugimethara/auth/oidc/callback"
}

// SSOURL is the browser URL that starts sign-in through this provider.
func (s *Service) SSOURL(id uuid.UUID) string {
	return strings.TrimSuffix(s.cfg.PublicURL, "/") + "/api/v1/hugimethara/auth/oidc/" + id.String() + "/start"
}

// CreateIdentityProviderInput is a new provider configuration.
type CreateIdentityProviderInput struct {
	Name            string
	Preset          string
	Issuer          string
	ClientID        string
	ClientSecret    *string
	Scopes          []string
	ClaimMapping    *ClaimMapping
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
	ClaimMapping    *ClaimMapping
	JITProvisioning *bool
	AllowedDomains  *[]string
	Enabled         *bool
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

type discoveryEntry struct {
	provider *oidc.Provider
	expires  time.Time
}

func (s *Service) oidcClient() *http.Client {
	return &http.Client{Timeout: oidcHTTPTimeout}
}

func (s *Service) discover(ctx context.Context, issuer string, fresh bool) (*oidc.Provider, error) {
	issuer = strings.TrimSpace(issuer)
	if !fresh {
		s.discoveryMu.Lock()
		entry, ok := s.discovery[issuer]
		s.discoveryMu.Unlock()
		if ok && s.now().Before(entry.expires) {
			return entry.provider, nil
		}
	}
	ctx = oidc.ClientContext(ctx, s.oidcClient())
	p, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, err
	}
	s.discoveryMu.Lock()
	s.discovery[issuer] = discoveryEntry{provider: p, expires: s.now().Add(oidcDiscoveryTTL)}
	s.discoveryMu.Unlock()
	return p, nil
}

func (s *Service) forgetDiscovery(issuer string) {
	s.discoveryMu.Lock()
	delete(s.discovery, strings.TrimSpace(issuer))
	s.discoveryMu.Unlock()
}

// ListIdentityProviders lists configured providers (identity_provider:read).
func (s *Service) ListIdentityProviders(ctx context.Context) ([]IdentityProvider, error) {
	p, err := s.require(ctx, "read", IdentityProviderResource, "")
	if err != nil {
		return nil, err
	}
	var rows []providerRow
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, q.Get("ListIdentityProviders"))
	})
	if err != nil {
		return nil, err
	}
	out := make([]IdentityProvider, len(rows))
	for i, r := range rows {
		out[i] = r.model()
	}
	return out, nil
}

// GetIdentityProvider returns one provider (identity_provider:read).
func (s *Service) GetIdentityProvider(ctx context.Context, id uuid.UUID) (IdentityProvider, error) {
	p, err := s.require(ctx, "read", IdentityProviderResource, id.String())
	if err != nil {
		return IdentityProvider{}, err
	}
	return s.getProvider(ctx, p, id)
}

func (s *Service) getProvider(ctx context.Context, p principal.Principal, id uuid.UUID) (IdentityProvider, error) {
	var row providerRow
	err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.GetContext(ctx, &row, q.Get("GetIdentityProvider"), id)
	})
	if db.IsNoRows(err) {
		return IdentityProvider{}, errProviderNotFound
	}
	if err != nil {
		return IdentityProvider{}, err
	}
	return row.model(), nil
}

type normalizedProvider struct {
	name           string
	preset         string
	issuer         string
	clientID       string
	scopes         []string
	claimMapping   ClaimMapping
	allowedDomains []string
}

func normalizeProvider(n normalizedProvider) (normalizedProvider, error) {
	n.name = strings.TrimSpace(n.name)
	n.issuer = strings.TrimSpace(n.issuer)
	n.clientID = strings.TrimSpace(n.clientID)
	if n.preset == "" {
		n.preset = PresetGeneric
	}
	n.scopes = normalizeScopes(n.scopes)
	n.allowedDomains = normalizeDomains(n.allowedDomains)
	n.claimMapping = normalizeClaimMapping(n.claimMapping)

	var errs []httpx.FieldError
	if l := utf8.RuneCountInString(n.name); l < 1 || l > 100 {
		errs = append(errs, httpx.FieldError{Field: "name", Code: "invalid_length", Message: "Name must be 1–100 characters."})
	}
	switch n.preset {
	case PresetEntra, PresetGoogle, PresetKeycloak, PresetGeneric:
	default:
		errs = append(errs, httpx.FieldError{Field: "preset", Code: "invalid", Message: "Preset must be entra, google, keycloak or generic."})
	}
	if err := validIssuer(n.issuer); err != nil {
		errs = append(errs, httpx.FieldError{Field: "issuer", Code: "invalid_url", Message: err.Error()})
	}
	if l := len(n.clientID); l < 1 || l > 255 {
		errs = append(errs, httpx.FieldError{Field: "client_id", Code: "invalid_length", Message: "Client ID must be 1–255 characters."})
	}
	if len(n.scopes) == 0 {
		errs = append(errs, httpx.FieldError{Field: "scopes", Code: "required", Message: "At least the openid scope is required."})
	}
	for _, d := range n.allowedDomains {
		if !validDomain(d) {
			errs = append(errs, httpx.FieldError{Field: "allowed_domains", Code: "invalid_domain", Message: "Allowed domains must be plain email domains, e.g. example.com."})
			break
		}
	}
	for _, f := range []struct{ field, value string }{
		{"claim_mapping.email", n.claimMapping.Email},
		{"claim_mapping.name", n.claimMapping.Name},
		{"claim_mapping.groups", n.claimMapping.Groups},
	} {
		if l := len(f.value); l > 200 || strings.ContainsAny(f.value, " \t\r\n") {
			errs = append(errs, httpx.FieldError{Field: f.field, Code: "invalid", Message: "Claim names must be at most 200 characters without spaces."})
		}
	}
	if len(errs) > 0 {
		return n, httpx.Validation(errs...)
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

func (s *Service) sealSecret(secret string) ([]byte, error) {
	if secret == "" {
		return nil, nil
	}
	if s.cfg.Sealer == nil {
		return nil, errors.New("identity: master key is not configured")
	}
	return s.cfg.Sealer.Seal([]byte(secret))
}

func (s *Service) openSecret(sealed []byte) (string, error) {
	if len(sealed) == 0 {
		return "", nil
	}
	if s.cfg.Sealer == nil {
		return "", errors.New("identity: master key is not configured")
	}
	plain, err := s.cfg.Sealer.Open(sealed)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// CreateIdentityProvider stores a provider (identity_provider:create).
func (s *Service) CreateIdentityProvider(ctx context.Context, in CreateIdentityProviderInput) (IdentityProvider, error) {
	p, err := s.require(ctx, "create", IdentityProviderResource, "")
	if err != nil {
		return IdentityProvider{}, err
	}
	claim := defaultClaimMapping()
	if in.ClaimMapping != nil {
		claim = *in.ClaimMapping
	}
	scopes := in.Scopes
	if scopes == nil {
		scopes = []string{"openid", "email", "profile"}
	}
	jit := in.JITProvisioning == nil || *in.JITProvisioning
	enabled := in.Enabled == nil || *in.Enabled
	n, err := normalizeProvider(normalizedProvider{
		name: in.Name, preset: in.Preset, issuer: in.Issuer, clientID: in.ClientID,
		scopes: scopes, claimMapping: claim, allowedDomains: in.AllowedDomains,
	})
	if err != nil {
		return IdentityProvider{}, err
	}
	secret := ""
	if in.ClientSecret != nil {
		secret = *in.ClientSecret
	}
	if len(secret) > 4096 {
		return IdentityProvider{}, httpx.Validation(httpx.FieldError{Field: "client_secret", Code: "invalid_length", Message: "Client secret must be at most 4096 characters."})
	}
	sealed, err := s.sealSecret(secret)
	if err != nil {
		return IdentityProvider{}, err
	}
	claims, err := json.Marshal(n.claimMapping)
	if err != nil {
		return IdentityProvider{}, err
	}
	id := uuid.Must(uuid.NewV7())
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		_, err := tx.ExecContext(ctx, q.Get("InsertIdentityProvider"), id, p.OrgID, n.name, n.preset, n.issuer,
			n.clientID, sealed, n.scopes, claims, jit, n.allowedDomains, enabled, p.ID)
		if db.IsUniqueViolation(err) {
			return errProviderName
		}
		if err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "identity_provider.created",
			ResourceType: IdentityProviderResource, ResourceID: id.String(),
			Details: map[string]any{"name": n.name, "preset": n.preset, "issuer": n.issuer, "enabled": enabled}})
	})
	if err != nil {
		return IdentityProvider{}, err
	}
	return s.getProvider(ctx, p, id)
}

// UpdateIdentityProvider changes a provider (identity_provider:update).
func (s *Service) UpdateIdentityProvider(ctx context.Context, id uuid.UUID, in UpdateIdentityProviderInput) (IdentityProvider, error) {
	p, err := s.require(ctx, "update", IdentityProviderResource, id.String())
	if err != nil {
		return IdentityProvider{}, err
	}
	cur, err := s.getProvider(ctx, p, id)
	if err != nil {
		return IdentityProvider{}, err
	}
	n := normalizedProvider{
		name: cur.Name, preset: cur.Preset, issuer: cur.Issuer, clientID: cur.ClientID,
		scopes: cur.Scopes, claimMapping: cur.ClaimMapping, allowedDomains: cur.AllowedDomains,
	}
	if in.Name != nil {
		n.name = *in.Name
	}
	if in.Preset != nil {
		n.preset = *in.Preset
	}
	if in.Issuer != nil {
		n.issuer = *in.Issuer
	}
	if in.ClientID != nil {
		n.clientID = *in.ClientID
	}
	if in.Scopes != nil {
		n.scopes = *in.Scopes
	}
	if in.ClaimMapping != nil {
		n.claimMapping = *in.ClaimMapping
	}
	if in.AllowedDomains != nil {
		n.allowedDomains = *in.AllowedDomains
	}
	n, err = normalizeProvider(n)
	if err != nil {
		return IdentityProvider{}, err
	}
	jit, enabled := cur.JITProvisioning, cur.Enabled
	if in.JITProvisioning != nil {
		jit = *in.JITProvisioning
	}
	if in.Enabled != nil {
		enabled = *in.Enabled
	}

	var sealed []byte
	setSecret := in.ClientSecret != nil
	if setSecret {
		secret := *in.ClientSecret
		if len(secret) > 4096 {
			return IdentityProvider{}, httpx.Validation(httpx.FieldError{Field: "client_secret", Code: "invalid_length", Message: "Client secret must be at most 4096 characters."})
		}
		if sealed, err = s.sealSecret(secret); err != nil {
			return IdentityProvider{}, err
		}
	}
	claims, err := json.Marshal(n.claimMapping)
	if err != nil {
		return IdentityProvider{}, err
	}
	out, err := s.updateProviderTx(ctx, p, id, n, sealed, setSecret, jit, enabled, claims, cur)
	if err != nil {
		return IdentityProvider{}, err
	}
	if cur.Issuer != n.issuer {
		s.forgetDiscovery(cur.Issuer)
	}
	s.forgetDiscovery(n.issuer)
	return out, nil
}

func (s *Service) updateProviderTx(ctx context.Context, p principal.Principal, id uuid.UUID, n normalizedProvider,
	sealed []byte, setSecret, jit, enabled bool, claims []byte, before IdentityProvider) (IdentityProvider, error) {
	err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		res, err := tx.ExecContext(ctx, q.Get("UpdateIdentityProvider"), id, n.name, n.preset, n.issuer, n.clientID,
			setSecret, sealed, n.scopes, claims, jit, n.allowedDomains, enabled)
		if err != nil {
			if db.IsUniqueViolation(err) {
				return errProviderName
			}
			return err
		}
		if rows, _ := res.RowsAffected(); rows == 0 {
			return errProviderNotFound
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "identity_provider.updated",
			ResourceType: IdentityProviderResource, ResourceID: id.String(),
			Details: map[string]any{"name": n.name, "enabled": enabled, "secret_changed": setSecret,
				"issuer_changed": before.Issuer != n.issuer}})
	})
	if err != nil {
		return IdentityProvider{}, err
	}
	return s.getProvider(ctx, p, id)
}

// DeleteIdentityProvider removes a provider with its links and mappings
// (identity_provider:delete). Memberships in IdP-managed groups are cleared.
func (s *Service) DeleteIdentityProvider(ctx context.Context, id uuid.UUID) error {
	p, err := s.require(ctx, "delete", IdentityProviderResource, id.String())
	if err != nil {
		return err
	}
	var issuer string
	var cleared int64
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		var row providerRow
		if err := tx.GetContext(ctx, &row, q.Get("GetIdentityProvider"), id); err != nil {
			if db.IsNoRows(err) {
				return errProviderNotFound
			}
			return err
		}
		issuer = row.Issuer
		res, err := tx.ExecContext(ctx, q.Get("ClearMappedGroupMembers"), id)
		if err != nil {
			return err
		}
		cleared, _ = res.RowsAffected()
		if _, err := tx.ExecContext(ctx, q.Get("DeleteIdentityProvider"), id); err != nil {
			return err
		}
		if cleared > 0 {
			if err := authz.BumpVersion(ctx, tx, p.OrgID); err != nil {
				return err
			}
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "identity_provider.deleted",
			ResourceType: IdentityProviderResource, ResourceID: id.String(),
			Details: map[string]any{"name": row.Name, "cleared_group_members": cleared}})
	})
	if err != nil {
		return err
	}
	s.forgetDiscovery(issuer)
	if cleared > 0 {
		s.acl.Invalidate(ctx, p.OrgID)
	}
	return nil
}

// TestIdentityProvider probes discovery and signing keys (identity_provider:test).
func (s *Service) TestIdentityProvider(ctx context.Context, id uuid.UUID) (IdentityProviderTest, error) {
	p, err := s.require(ctx, "test", IdentityProviderResource, id.String())
	if err != nil {
		return IdentityProviderTest{}, err
	}
	prov, err := s.getProvider(ctx, p, id)
	if err != nil {
		return IdentityProviderTest{}, err
	}
	start := time.Now()
	out := IdentityProviderTest{CheckedAt: s.now()}
	discovered, err := s.discover(ctx, prov.Issuer, true)
	if err != nil {
		s.log.WarnContext(ctx, "identity provider discovery failed", "provider_id", id, "err", err)
		out.Error = "The discovery document could not be fetched or its issuer does not match."
		out.Latency = time.Since(start)
		return out, nil
	}
	out.Issuer = prov.Issuer
	var meta struct {
		Issuer                string `json:"issuer"`
		AuthorizationEndpoint string `json:"authorization_endpoint"`
		TokenEndpoint         string `json:"token_endpoint"`
		JWKSUri               string `json:"jwks_uri"`
	}
	if err := discovered.Claims(&meta); err != nil {
		s.log.WarnContext(ctx, "identity provider metadata decode failed", "provider_id", id, "err", err)
		out.Error = "The discovery document has an unexpected shape."
		out.Latency = time.Since(start)
		return out, nil
	}
	out.AuthorizationEndpoint, out.TokenEndpoint, out.JWKSUri = meta.AuthorizationEndpoint, meta.TokenEndpoint, meta.JWKSUri
	if err := fetchKeys(ctx, s.oidcClient(), meta.JWKSUri); err != nil {
		s.log.WarnContext(ctx, "identity provider JWKS fetch failed", "provider_id", id, "err", err)
		out.Error = "The signing keys (JWKS) could not be fetched."
		out.Latency = time.Since(start)
		return out, nil
	}
	out.OK = true
	out.Latency = time.Since(start)
	return out, nil
}

func fetchKeys(ctx context.Context, client *http.Client, jwksURI string) error {
	if jwksURI == "" {
		return errors.New("missing jwks_uri")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURI, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks: status %d", resp.StatusCode)
	}
	_, err = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	return err
}

// ListGroupMappings lists mappings for a provider (identity_provider:read).
func (s *Service) ListGroupMappings(ctx context.Context, id uuid.UUID) ([]GroupMapping, error) {
	p, err := s.require(ctx, "read", IdentityProviderResource, id.String())
	if err != nil {
		return nil, err
	}
	out := []GroupMapping{}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		var exists bool
		if err := tx.GetContext(ctx, &exists, "SELECT EXISTS (SELECT 1 FROM identity_providers WHERE id = $1)", id); err != nil {
			return err
		}
		if !exists {
			return errProviderNotFound
		}
		return tx.SelectContext(ctx, &out, q.Get("ListGroupMappings"), id)
	})
	return out, err
}

// CreateGroupMapping maps an external group to an internal group
// (identity_provider:update). Without groupID, an IdP-managed group is created.
func (s *Service) CreateGroupMapping(ctx context.Context, id uuid.UUID, external string, groupID *uuid.UUID) (GroupMapping, error) {
	p, err := s.require(ctx, "update", IdentityProviderResource, id.String())
	if err != nil {
		return GroupMapping{}, err
	}
	external = strings.TrimSpace(external)
	if l := utf8.RuneCountInString(external); l < 1 || l > 200 {
		return GroupMapping{}, httpx.Validation(httpx.FieldError{Field: "external_group", Code: "invalid_length", Message: "External group must be 1–200 characters."})
	}
	var out GroupMapping
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		var exists bool
		if err := tx.GetContext(ctx, &exists, "SELECT EXISTS (SELECT 1 FROM identity_providers WHERE id = $1)", id); err != nil {
			return err
		}
		if !exists {
			return errProviderNotFound
		}
		var target uuid.UUID
		if groupID == nil {
			var found uuid.UUID
			err := tx.GetContext(ctx, &found, q.Get("GroupIDByNameAndSource"), external, "idp")
			switch {
			case err == nil:
				target = found
			case db.IsNoRows(err):
				if utf8.RuneCountInString(external) > 100 {
					return httpx.Validation(httpx.FieldError{Field: "external_group", Code: "group_name_too_long",
						Message: "IdP group names longer than 100 characters need an existing group."})
				}
				target = uuid.Must(uuid.NewV7())
				if _, err := tx.ExecContext(ctx, q.Get("InsertIdPGroup"), target, p.OrgID, external, nil); err != nil {
					if db.IsUniqueViolation(err) {
						return errGroupNameTaken
					}
					return err
				}
			default:
				return err
			}
		} else {
			target = *groupID
		}
		if _, err := getGroup(ctx, tx, target); err != nil {
			return err
		}
		if err := s.acl.CheckCanHandOut(ctx, tx, p, authz.PrincipalRef{Type: authz.PrincipalGroup, ID: target}); err != nil {
			return err
		}
		mappingID := uuid.Must(uuid.NewV7())
		if _, err := tx.ExecContext(ctx, q.Get("InsertGroupMapping"), mappingID, p.OrgID, id, external, target, p.ID); err != nil {
			if db.IsUniqueViolation(err) {
				return errMappingConflict(err)
			}
			return err
		}
		return tx.GetContext(ctx, &out, q.Get("GetGroupMapping"), mappingID, id)
	})
	if err != nil {
		return GroupMapping{}, err
	}
	return out, nil
}

func errMappingConflict(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && strings.Contains(pgErr.ConstraintName, "external_group") {
		return errMappingExists
	}
	return errMappingGroup
}

// DeleteGroupMapping removes a mapping and clears IdP-managed memberships it
// owned (identity_provider:update).
func (s *Service) DeleteGroupMapping(ctx context.Context, id, mappingID uuid.UUID) error {
	p, err := s.require(ctx, "update", IdentityProviderResource, id.String())
	if err != nil {
		return err
	}
	var groupID uuid.UUID
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		var row struct {
			GroupID uuid.UUID `db:"group_id"`
		}
		if err := tx.GetContext(ctx, &row, q.Get("GetGroupMapping"), mappingID, id); err != nil {
			if db.IsNoRows(err) {
				return errMappingNotFound
			}
			return err
		}
		groupID = row.GroupID
		if _, err := tx.ExecContext(ctx, q.Get("DeleteGroupMapping"), mappingID, id); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, q.Get("ClearIdPGroupMembers"), groupID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			if err := authz.BumpVersion(ctx, tx, p.OrgID); err != nil {
				return err
			}
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "group_mapping.deleted",
			ResourceType: IdentityProviderResource, ResourceID: id.String(),
			Details: map[string]any{"mapping_id": mappingID, "group_id": groupID}})
	})
	if err != nil {
		return err
	}
	s.acl.Invalidate(ctx, p.OrgID)
	return nil
}

var errMappingNotFound = httpx.NotFound("Group mapping not found.")

// OIDCProviderOptions lists enabled providers for sign-in (public).
func (s *Service) OIDCProviderOptions(ctx context.Context, slug string) ([]OIDCProviderOption, error) {
	ci := httpx.ClientInfoFrom(ctx)
	if err := s.limiter.Allow(ctx, ruleOIDCLookup, ci.IP.String()); err != nil {
		return nil, err
	}
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" || len(slug) > 40 {
		return []OIDCProviderOption{}, nil
	}
	var rows []struct {
		ProviderID uuid.UUID `db:"provider_id"`
		Name       string    `db:"provider_name"`
		Preset     string    `db:"preset"`
	}
	err := db.InTx(ctx, s.pool, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, q.Get("OIDCProviders"), slug)
	})
	if err != nil {
		return nil, err
	}
	out := make([]OIDCProviderOption, len(rows))
	for i, r := range rows {
		out[i] = OIDCProviderOption{ID: r.ProviderID, Name: r.Name, Preset: r.Preset}
	}
	return out, nil
}

// oidcState is sealed into a short-lived cookie. The browser never sees its
// contents (beyond the opaque value), and the cookie binds the callback to
// the browser that started the flow.
type oidcState struct {
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

func randomString(bytes int) string {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		panic("identity: crypto/rand failed: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func safeRedirect(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "/", nil
	}
	if len(raw) > 512 || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") ||
		strings.ContainsAny(raw, "\\\r\n") {
		return "", httpx.Validation(httpx.FieldError{Field: "redirect", Code: "invalid", Message: "Redirect must be a relative path on this site."})
	}
	return raw, nil
}

type oidcProviderAuth struct {
	ProviderID      uuid.UUID `db:"provider_id"`
	OrgID           uuid.UUID `db:"org_id"`
	OrgSlug         string    `db:"org_slug"`
	OrgName         string    `db:"org_name"`
	OrgSettings     []byte    `db:"org_settings"`
	OrgStatus       string    `db:"org_status"`
	ProviderName    string    `db:"provider_name"`
	Preset          string    `db:"preset"`
	Issuer          string    `db:"issuer"`
	ClientID        string    `db:"client_id"`
	ClientSecret    []byte    `db:"client_secret"`
	Scopes          []string  `db:"scopes"`
	ClaimMapping    []byte    `db:"claim_mapping"`
	JITProvisioning bool      `db:"jit_provisioning"`
	AllowedDomains  []string  `db:"allowed_domains"`
	Enabled         bool      `db:"enabled"`
}

func (r oidcProviderAuth) provider() IdentityProvider {
	p := IdentityProvider{
		ID: r.ProviderID, Name: r.ProviderName, Protocol: "oidc", Preset: r.Preset, Issuer: r.Issuer,
		ClientID: r.ClientID, HasClientSecret: len(r.ClientSecret) > 0, Scopes: r.Scopes,
		JITProvisioning: r.JITProvisioning, AllowedDomains: r.AllowedDomains, Enabled: r.Enabled,
	}
	if len(r.ClaimMapping) > 0 {
		_ = json.Unmarshal(r.ClaimMapping, &p.ClaimMapping)
	}
	if p.ClaimMapping.Email == "" {
		p.ClaimMapping = defaultClaimMapping()
	}
	return p
}

func (s *Service) providerAuthByID(ctx context.Context, id uuid.UUID) (oidcProviderAuth, error) {
	var row oidcProviderAuth
	err := db.InTx(ctx, s.pool, func(tx *sqlx.Tx) error {
		return tx.GetContext(ctx, &row, q.Get("OIDCProvider"), id)
	})
	if db.IsNoRows(err) {
		return row, errProviderNotFound
	}
	return row, err
}

// ssoError is an expected sign-in failure that becomes a redirect with a
// stable code the web app can explain. Unexpected errors stay problems.
type ssoError struct{ code string }

func (e *ssoError) Error() string { return "identity: sso " + e.code }

func ssoFail(code string) error { return &ssoError{code: code} }

// SSOErrorCode extracts the redirect code from an expected sign-in failure.
func SSOErrorCode(err error) (string, bool) {
	var e *ssoError
	if errors.As(err, &e) {
		return e.code, true
	}
	return "", false
}

// StartOIDCLogin creates the authorization redirect and sealed state cookie
// (public). The caller sets the cookie; the state binds the callback to this
// browser.
func (s *Service) StartOIDCLogin(ctx context.Context, idpID uuid.UUID, redirect string) (OIDCStart, error) {
	ci := httpx.ClientInfoFrom(ctx)
	if err := s.limiter.Allow(ctx, ruleOIDCStart, ci.IP.String()); err != nil {
		return OIDCStart{}, err
	}
	redirect, err := safeRedirect(redirect)
	if err != nil {
		return OIDCStart{}, err
	}
	auth, err := s.providerAuthByID(ctx, idpID)
	if err != nil {
		if db.IsNoRows(err) {
			return OIDCStart{}, ssoFail("provider_unavailable")
		}
		return OIDCStart{}, err
	}
	prov := auth.provider()
	if !prov.Enabled || auth.OrgStatus != "active" {
		return OIDCStart{}, ssoFail("provider_unavailable")
	}
	discovered, err := s.discover(ctx, prov.Issuer, false)
	if err != nil {
		s.log.WarnContext(ctx, "oidc start discovery failed", "provider_id", idpID, "err", err)
		return OIDCStart{}, ssoFail("provider_unavailable")
	}
	secret, err := s.openSecret(auth.ClientSecret)
	if err != nil {
		return OIDCStart{}, err
	}
	state := oidcState{
		IDP: prov.ID, Org: auth.OrgID, State: randomString(32), Nonce: randomString(32),
		Verifier: randomString(48), Redirect: redirect, Expires: s.now().Add(oidcStateTTL),
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return OIDCStart{}, err
	}
	if s.cfg.Sealer == nil {
		return OIDCStart{}, errors.New("identity: master key is not configured")
	}
	sealed, err := s.cfg.Sealer.Seal(raw)
	if err != nil {
		return OIDCStart{}, err
	}
	if len(sealed) > oidcMaxStates {
		return OIDCStart{}, errors.New("identity: sealed OIDC state too large")
	}
	cfg := &oauth2.Config{
		ClientID: prov.ClientID, ClientSecret: secret, Scopes: prov.Scopes, RedirectURL: s.RedirectURI(),
		Endpoint: discovered.Endpoint(),
	}
	authURL := cfg.AuthCodeURL(state.State,
		oidc.Nonce(state.Nonce),
		oauth2.S256ChallengeOption(state.Verifier),
	)
	return OIDCStart{
		AuthURL:     authURL,
		SealedState: base64.RawURLEncoding.EncodeToString(sealed),
		Expires:     state.Expires,
	}, nil
}

// OIDCCallbackParams carries the provider's callback query parameters.
type OIDCCallbackParams struct {
	Code             string
	State            string
	Error            string
	ErrorDescription string
}

// OIDCCompletion is a successful sign-in.
type OIDCCompletion struct {
	Grant    SessionGrant
	Redirect string
}

// CompleteOIDCLogin validates the callback, links or provisions the user,
// applies group mappings and issues a session (public).
func (s *Service) CompleteOIDCLogin(ctx context.Context, sealedCookie string, in OIDCCallbackParams) (OIDCCompletion, error) {
	ci := httpx.ClientInfoFrom(ctx)
	if err := s.limiter.Allow(ctx, ruleOIDCCallback, ci.IP.String()); err != nil {
		return OIDCCompletion{}, err
	}
	if sealedCookie == "" {
		return OIDCCompletion{}, ssoFail("state_missing")
	}
	sealed, err := base64.RawURLEncoding.DecodeString(sealedCookie)
	if err != nil || s.cfg.Sealer == nil {
		return OIDCCompletion{}, ssoFail("state_invalid")
	}
	raw, err := s.cfg.Sealer.Open(sealed)
	if err != nil {
		return OIDCCompletion{}, ssoFail("state_invalid")
	}
	var state oidcState
	if err := json.Unmarshal(raw, &state); err != nil || s.now().After(state.Expires) {
		return OIDCCompletion{}, ssoFail("state_expired")
	}
	if len(in.State) > 512 || subtle.ConstantTimeCompare([]byte(in.State), []byte(state.State)) != 1 {
		return OIDCCompletion{}, ssoFail("state_invalid")
	}
	if in.Error != "" {
		s.log.InfoContext(ctx, "identity provider returned an error", "error", in.Error, "description", in.ErrorDescription)
		if in.Error == "access_denied" {
			return OIDCCompletion{}, ssoFail("access_denied")
		}
		return OIDCCompletion{}, ssoFail("idp_error")
	}
	if in.Code == "" || len(in.Code) > 4096 {
		return OIDCCompletion{}, ssoFail("idp_error")
	}

	auth, err := s.providerAuthByID(ctx, state.IDP)
	if err != nil {
		if db.IsNoRows(err) {
			return OIDCCompletion{}, ssoFail("provider_unavailable")
		}
		return OIDCCompletion{}, err
	}
	prov := auth.provider()
	if !prov.Enabled || auth.OrgStatus != "active" {
		return OIDCCompletion{}, ssoFail("provider_unavailable")
	}
	discovered, err := s.discover(ctx, prov.Issuer, false)
	if err != nil {
		s.log.WarnContext(ctx, "oidc callback discovery failed", "provider_id", state.IDP, "err", err)
		return OIDCCompletion{}, ssoFail("provider_unavailable")
	}
	secret, err := s.openSecret(auth.ClientSecret)
	if err != nil {
		return OIDCCompletion{}, err
	}
	cfg := &oauth2.Config{
		ClientID: prov.ClientID, ClientSecret: secret, Scopes: prov.Scopes, RedirectURL: s.RedirectURI(),
		Endpoint: discovered.Endpoint(),
	}
	exchangeCtx := oidc.ClientContext(ctx, s.oidcClient())
	token, err := cfg.Exchange(exchangeCtx, in.Code, oauth2.VerifierOption(state.Verifier))
	if err != nil {
		s.log.WarnContext(ctx, "oidc code exchange failed", "provider_id", state.IDP, "err", err)
		return OIDCCompletion{}, ssoFail("idp_error")
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		s.log.WarnContext(ctx, "oidc token response has no id_token", "provider_id", state.IDP)
		return OIDCCompletion{}, ssoFail("idp_error")
	}
	idToken, err := discovered.Verifier(&oidc.Config{ClientID: prov.ClientID}).Verify(exchangeCtx, rawIDToken)
	if err != nil {
		s.log.WarnContext(ctx, "oidc id token verification failed", "provider_id", state.IDP, "err", err)
		return OIDCCompletion{}, ssoFail("invalid_token")
	}
	if subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(state.Nonce)) != 1 {
		s.log.WarnContext(ctx, "oidc nonce mismatch", "provider_id", state.IDP)
		return OIDCCompletion{}, ssoFail("invalid_token")
	}

	var rawClaims map[string]any
	if err := idToken.Claims(&rawClaims); err != nil {
		return OIDCCompletion{}, ssoFail("invalid_token")
	}
	claims := parseClaims(rawClaims, prov.ClaimMapping, idToken.Subject)
	if err := s.ensureClaimAllowed(prov, claims); err != nil {
		return OIDCCompletion{}, err
	}

	org := OrgSummary{ID: auth.OrgID, Slug: auth.OrgSlug, Name: auth.OrgName}
	settings := organization.ParseSettings(auth.OrgSettings)

	grant, err := s.linkOrProvision(ctx, prov, org, settings, idToken.Subject, claims)
	if err != nil {
		return OIDCCompletion{}, err
	}
	return OIDCCompletion{Grant: grant, Redirect: state.Redirect}, nil
}

type oidcClaims struct {
	Email         string
	EmailVerified *bool
	DisplayName   string
	Groups        []string
}

func parseClaims(raw map[string]any, mapping ClaimMapping, subject string) oidcClaims {
	c := oidcClaims{Email: normalizeEmail(claimString(raw, mapping.Email))}
	if c.Email == "" {
		c.Email = normalizeEmail(claimString(raw, "preferred_username"))
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

func (s *Service) ensureClaimAllowed(prov IdentityProvider, c oidcClaims) error {
	if c.EmailVerified != nil && !*c.EmailVerified {
		return ssoFail("email_unverified")
	}
	if c.Email == "" || !validEmail(c.Email) {
		return ssoFail("email_missing")
	}
	if len(prov.AllowedDomains) > 0 {
		at := strings.LastIndex(c.Email, "@")
		if at < 0 || !domainAllowed(c.Email[at+1:], prov.AllowedDomains) {
			return ssoFail("email_domain_not_allowed")
		}
	}
	return nil
}

func domainAllowed(domain string, allowed []string) bool {
	for _, d := range allowed {
		if strings.EqualFold(domain, d) {
			return true
		}
	}
	return false
}

type identityLinkRow struct {
	LinkID           uuid.UUID    `db:"link_id"`
	UserID           uuid.UUID    `db:"user_id"`
	Email            string       `db:"email"`
	DisplayName      string       `db:"display_name"`
	UserStatus       string       `db:"user_status"`
	MembershipStatus string       `db:"membership_status"`
	LastLoginAt      sql.NullTime `db:"last_login_at"`
}

func (s *Service) identityLink(ctx context.Context, idpID uuid.UUID, subject string) (identityLinkRow, bool, error) {
	var row identityLinkRow
	err := db.InTx(ctx, s.pool, func(tx *sqlx.Tx) error {
		return tx.GetContext(ctx, &row, q.Get("OIDCIdentityLink"), idpID, subject)
	})
	if db.IsNoRows(err) {
		return row, false, nil
	}
	if err != nil {
		return row, false, err
	}
	return row, true, nil
}

func (s *Service) userByEmail(ctx context.Context, email string) (User, bool, error) {
	var row struct {
		UserID      uuid.UUID `db:"user_id"`
		DisplayName string    `db:"display_name"`
		Status      string    `db:"status"`
	}
	err := db.InTx(ctx, s.pool, func(tx *sqlx.Tx) error {
		return tx.GetContext(ctx, &row, q.Get("OIDCUserByEmail"), email)
	})
	if db.IsNoRows(err) {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, err
	}
	return User{ID: row.UserID, Email: email, DisplayName: row.DisplayName, Status: row.Status}, true, nil
}

func (s *Service) linkOrProvision(ctx context.Context, prov IdentityProvider, org OrgSummary, settings organization.Settings,
	subject string, c oidcClaims) (SessionGrant, error) {
	if link, ok, err := s.identityLink(ctx, prov.ID, subject); err != nil {
		return SessionGrant{}, err
	} else if ok {
		if link.UserStatus != "active" {
			return SessionGrant{}, ssoFail("account_disabled")
		}
		if link.MembershipStatus != MembershipActive {
			return SessionGrant{}, ssoFail("membership_suspended")
		}
		user := User{ID: link.UserID, Email: link.Email, DisplayName: link.DisplayName, Status: link.UserStatus}
		return s.finishOIDCLogin(ctx, prov, org, settings, user, link.LinkID, c.Groups)
	}

	existing, found, err := s.userByEmail(ctx, c.Email)
	if err != nil {
		return SessionGrant{}, err
	}
	if found && existing.Status != "active" {
		return SessionGrant{}, ssoFail("account_disabled")
	}
	if found {
		// Never auto-link an account that is not already a member: a
		// misconfigured or malicious IdP must not take over an account whose
		// email merely matches.
		var status string
		if err := db.InTx(ctx, s.pool, func(tx *sqlx.Tx) error {
			return tx.GetContext(ctx, &status, q.Get("GetMembershipStatusAny"), existing.ID, org.ID)
		}); err != nil {
			return SessionGrant{}, err
		}
		if status == "" {
			return SessionGrant{}, ssoFail("account_exists")
		}
		if status != MembershipActive {
			return SessionGrant{}, ssoFail("membership_suspended")
		}
		return s.provisionAndLogin(ctx, prov, org, settings, existing, subject, c.Groups, false)
	}

	if !prov.JITProvisioning {
		return SessionGrant{}, ssoFail("no_account")
	}
	user := User{ID: uuid.Must(uuid.NewV7()), Email: c.Email, DisplayName: c.DisplayName, Status: "active"}
	return s.provisionAndLogin(ctx, prov, org, settings, user, subject, c.Groups, true)
}

func (s *Service) provisionAndLogin(ctx context.Context, prov IdentityProvider, org OrgSummary, settings organization.Settings,
	user User, subject string, groups []string, create bool) (SessionGrant, error) {
	var grant SessionGrant
	var linkID = uuid.Must(uuid.NewV7())
	changed := false
	err := db.InTenantTx(ctx, s.pool, org.ID, func(tx *sqlx.Tx) error {
		if create {
			if _, err := tx.ExecContext(ctx, q.Get("InsertUser"), user.ID, user.Email, user.DisplayName); err != nil {
				if db.IsUniqueViolation(err) {
					return ssoFail("account_exists")
				}
				return err
			}
		}
		res, err := tx.ExecContext(ctx, q.Get("InsertMembershipIfAbsent"), org.ID, user.ID)
		if err != nil {
			return err
		}
		added, _ := res.RowsAffected()
		var status string
		if err := tx.GetContext(ctx, &status, q.Get("GetMembershipStatus"), user.ID); err != nil {
			return err
		}
		if status != MembershipActive {
			return ssoFail("membership_suspended")
		}
		if added > 0 {
			if err := authz.BindRoleKeyInTx(ctx, tx, org.ID,
				authz.PrincipalRef{Type: authz.PrincipalUser, ID: user.ID},
				authz.MemberRoleKey, authz.ScopeRef{Type: authz.ScopeOrg}, uuid.Nil); err != nil {
				return err
			}
			actor := &audit.Actor{Type: string(principal.User), ID: user.ID}
			if err := audit.Record(ctx, tx, audit.Event{OrgID: org.ID, Action: "user.joined", ResourceType: UserResource,
				ResourceID: user.ID.String(), Actor: actor,
				Details: map[string]any{"source": "oidc", "identity_provider_id": prov.ID, "new_account": create}}); err != nil {
				return err
			}
			if _, err := event.Publish(ctx, tx, event.Event{OrgID: org.ID, Subject: "hugi.events.user.joined",
				Payload: map[string]any{"user_id": user.ID, "identity_provider_id": prov.ID}}); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, q.Get("InsertIdentityLink"), linkID, org.ID, prov.ID, subject, user.ID); err != nil {
			if db.IsUniqueViolation(err) {
				return ssoFail("account_conflict")
			}
			return err
		}
		if changed, err = s.syncIdPGroups(ctx, tx, org.ID, prov.ID, user.ID, groups); err != nil {
			return err
		}
		grant, err = s.issueSession(ctx, tx, user, org, settings, "oidc")
		return err
	})
	if err != nil {
		return SessionGrant{}, err
	}
	if changed || create {
		s.acl.Invalidate(ctx, org.ID)
	}
	return grant, nil
}

func (s *Service) finishOIDCLogin(ctx context.Context, prov IdentityProvider, org OrgSummary, settings organization.Settings,
	user User, linkID uuid.UUID, groups []string) (SessionGrant, error) {
	var grant SessionGrant
	var changed bool
	err := db.InTenantTx(ctx, s.pool, org.ID, func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, q.Get("TouchIdentityLink"), linkID); err != nil {
			return err
		}
		var err error
		if changed, err = s.syncIdPGroups(ctx, tx, org.ID, prov.ID, user.ID, groups); err != nil {
			return err
		}
		grant, err = s.issueSession(ctx, tx, user, org, settings, "oidc")
		return err
	})
	if err != nil {
		return SessionGrant{}, err
	}
	if changed {
		s.acl.Invalidate(ctx, org.ID)
	}
	return grant, nil
}

func (s *Service) syncIdPGroups(ctx context.Context, tx *sqlx.Tx, orgID, idpID, userID uuid.UUID, claimed []string) (bool, error) {
	var mapped []struct {
		ID            uuid.UUID `db:"id"`
		ExternalGroup string    `db:"external_group"`
	}
	if err := tx.SelectContext(ctx, &mapped, q.Get("MappedGroupIDs"), idpID); err != nil {
		return false, err
	}
	if len(mapped) == 0 {
		return false, nil
	}
	desired := map[uuid.UUID]bool{}
	for _, m := range mapped {
		for _, ext := range claimed {
			if ext == m.ExternalGroup {
				desired[m.ID] = true
				break
			}
		}
	}
	var current []uuid.UUID
	if err := tx.SelectContext(ctx, &current, q.Get("ListUserMappedGroupIDs"), idpID, userID); err != nil {
		return false, err
	}
	changed := false
	have := map[uuid.UUID]bool{}
	for _, g := range current {
		have[g] = true
		if !desired[g] {
			if _, err := tx.ExecContext(ctx, q.Get("DeleteIdPGroupMember"), g, userID); err != nil {
				return false, err
			}
			changed = true
		}
	}
	for g := range desired {
		if !have[g] {
			if _, err := tx.ExecContext(ctx, q.Get("SyncIdPGroupMembers"), userID, nil, g); err != nil {
				return false, err
			}
			changed = true
		}
	}
	if changed {
		if err := authz.BumpVersion(ctx, tx, orgID); err != nil {
			return false, err
		}
	}
	return changed, nil
}

// OIDCStateCookie is the exported cookie name suffix for the flow cookie.
func (c Cookies) OIDCStateName() string { return c.prefix() + "hugi_oidc_state" }

func (c Cookies) oidcState(value string, expires time.Time) *http.Cookie {
	//nolint:gosec // G124: HttpOnly + Secure (except plain-HTTP development) + SameSite=Lax.
	return &http.Cookie{
		Name: c.OIDCStateName(), Value: value, Path: "/", Expires: expires, MaxAge: int(oidcStateTTL.Seconds()),
		HttpOnly: true, Secure: c.Secure, SameSite: http.SameSiteLaxMode,
	}
}

type oidcStateCookieKey struct{}

func withOIDCStateCookie(ctx context.Context, value string) context.Context {
	return context.WithValue(ctx, oidcStateCookieKey{}, value)
}

func oidcStateCookieFrom(ctx context.Context) string {
	v, _ := ctx.Value(oidcStateCookieKey{}).(string)
	return v
}
