package services

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
	"golang.org/x/oauth2"

	"github.com/nuriansyah/hugimethara/internal/infrastructure/persistence/outbox"
	"github.com/nuriansyah/hugimethara/internal/infrastructure/security/ratelimit"
	authzmodel "github.com/nuriansyah/hugimethara/internal/modules/authorization/model"
	authzsvc "github.com/nuriansyah/hugimethara/internal/modules/authorization/services"
	auditmodel "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/model"
	auditsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/services"
	identitymodel "github.com/nuriansyah/hugimethara/internal/modules/identity/model"
	identitydto "github.com/nuriansyah/hugimethara/internal/modules/identity/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/identity/repository"
	"github.com/nuriansyah/hugimethara/internal/modules/identity/repository/commands"
	"github.com/nuriansyah/hugimethara/internal/modules/identity/repository/queries"
	orgmodel "github.com/nuriansyah/hugimethara/internal/modules/organization/model"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

var (
	ruleOIDCStart    = ratelimit.Rule{Name: "oidc_start_ip", Max: 30, Window: 5 * time.Minute}
	ruleOIDCCallback = ratelimit.Rule{Name: "oidc_callback_ip", Max: 60, Window: 5 * time.Minute}
	ruleOIDCLookup   = ratelimit.Rule{Name: "oidc_lookup_ip", Max: 120, Window: 5 * time.Minute}
)

// RedirectURI is the callback registered at every identity provider.
func (s *IdentityService) RedirectURI() string {
	return strings.TrimSuffix(s.cfg.PublicURL, "/") + "/api/v1/hugimethara/auth/oidc/callback"
}

// SSOURL is the browser URL that starts sign-in through this provider.
func (s *IdentityService) SSOURL(id uuid.UUID) string {
	return strings.TrimSuffix(s.cfg.PublicURL, "/") + "/api/v1/hugimethara/auth/oidc/" + id.String() + "/start"
}

type discoveryEntry struct {
	provider *oidc.Provider
	expires  time.Time
}

func (s *IdentityService) oidcClient() *http.Client {
	return &http.Client{Timeout: identitymodel.OidcHTTPTimeout}
}

func (s *IdentityService) discover(ctx context.Context, issuer string, fresh bool) (*oidc.Provider, error) {
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
	s.discovery[issuer] = discoveryEntry{provider: p, expires: s.now().Add(identitymodel.OidcDiscoveryTTL)}
	s.discoveryMu.Unlock()
	return p, nil
}

func (s *IdentityService) forgetDiscovery(issuer string) {
	s.discoveryMu.Lock()
	delete(s.discovery, strings.TrimSpace(issuer))
	s.discoveryMu.Unlock()
}

// ListIdentityProviders lists configured providers (identity_provider:read).
func (s *IdentityService) ListIdentityProviders(ctx context.Context) ([]identitymodel.IdentityProvider, error) {
	p, err := s.require(ctx, "read", identitymodel.IdentityProviderResource, "")
	if err != nil {
		return nil, err
	}
	var rows []repository.ProviderRow
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, queries.SQL.Get("ListIdentityProviders"))
	})
	if err != nil {
		return nil, err
	}
	out := make([]identitymodel.IdentityProvider, len(rows))
	for i, r := range rows {
		out[i] = r.Model()
	}
	return out, nil
}

// GetIdentityProvider returns one provider (identity_provider:read).
func (s *IdentityService) GetIdentityProvider(ctx context.Context, id uuid.UUID) (identitymodel.IdentityProvider, error) {
	p, err := s.require(ctx, "read", identitymodel.IdentityProviderResource, id.String())
	if err != nil {
		return identitymodel.IdentityProvider{}, err
	}
	return s.getProvider(ctx, p, id)
}

func (s *IdentityService) getProvider(ctx context.Context, p principal.Principal, id uuid.UUID) (identitymodel.IdentityProvider, error) {
	var row repository.ProviderRow
	err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.GetContext(ctx, &row, queries.SQL.Get("GetIdentityProvider"), id)
	})
	if db.IsNoRows(err) {
		return identitymodel.IdentityProvider{}, identitymodel.ErrProviderNotFound
	}
	if err != nil {
		return identitymodel.IdentityProvider{}, err
	}
	return row.Model(), nil
}

func (s *IdentityService) sealSecret(secret string) ([]byte, error) {
	if secret == "" {
		return nil, nil
	}
	if s.cfg.Sealer == nil {
		return nil, errors.New("identity: master key is not configured")
	}
	return s.cfg.Sealer.Seal([]byte(secret))
}

func (s *IdentityService) openSecret(sealed []byte) (string, error) {
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
func (s *IdentityService) CreateIdentityProvider(ctx context.Context, in identitydto.CreateIdentityProviderInput) (identitymodel.IdentityProvider, error) {
	p, err := s.require(ctx, "create", identitymodel.IdentityProviderResource, "")
	if err != nil {
		return identitymodel.IdentityProvider{}, err
	}
	claim := identitymodel.DefaultClaimMapping()
	if in.ClaimMapping != nil {
		claim = *in.ClaimMapping
	}
	scopes := in.Scopes
	if scopes == nil {
		scopes = []string{"openid", "email", "profile"}
	}
	jit := in.JITProvisioning == nil || *in.JITProvisioning
	enabled := in.Enabled == nil || *in.Enabled
	n, err := identitymodel.NormalizeProvider(identitymodel.NormalizedProvider{
		Name: in.Name, Preset: in.Preset, Issuer: in.Issuer, ClientID: in.ClientID,
		Scopes: scopes, ClaimMapping: claim, AllowedDomains: in.AllowedDomains,
	})
	if err != nil {
		return identitymodel.IdentityProvider{}, err
	}
	secret := ""
	if in.ClientSecret != nil {
		secret = *in.ClientSecret
	}
	if len(secret) > 4096 {
		return identitymodel.IdentityProvider{}, problem.Validation(problem.FieldError{Field: "client_secret", Code: "invalid_length", Message: "Client secret must be at most 4096 characters."})
	}
	sealed, err := s.sealSecret(secret)
	if err != nil {
		return identitymodel.IdentityProvider{}, err
	}
	claims, err := json.Marshal(n.ClaimMapping)
	if err != nil {
		return identitymodel.IdentityProvider{}, err
	}
	id := uuid.Must(uuid.NewV7())
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		_, err := tx.ExecContext(ctx, commands.SQL.Get("InsertIdentityProvider"), id, p.OrgID, n.Name, n.Preset, n.Issuer,
			n.ClientID, sealed, n.Scopes, claims, jit, n.AllowedDomains, enabled, p.ID)
		if db.IsUniqueViolation(err) {
			return identitymodel.ErrProviderName
		}
		if err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "identity_provider.created",
			ResourceType: identitymodel.IdentityProviderResource, ResourceID: id.String(),
			Details: map[string]any{"name": n.Name, "preset": n.Preset, "issuer": n.Issuer, "enabled": enabled}})
	})
	if err != nil {
		return identitymodel.IdentityProvider{}, err
	}
	return s.getProvider(ctx, p, id)
}

// UpdateIdentityProvider changes a provider (identity_provider:update).
func (s *IdentityService) UpdateIdentityProvider(ctx context.Context, id uuid.UUID, in identitydto.UpdateIdentityProviderInput) (identitymodel.IdentityProvider, error) {
	p, err := s.require(ctx, "update", identitymodel.IdentityProviderResource, id.String())
	if err != nil {
		return identitymodel.IdentityProvider{}, err
	}
	cur, err := s.getProvider(ctx, p, id)
	if err != nil {
		return identitymodel.IdentityProvider{}, err
	}
	n := identitymodel.NormalizedProvider{
		Name: cur.Name, Preset: cur.Preset, Issuer: cur.Issuer, ClientID: cur.ClientID,
		Scopes: cur.Scopes, ClaimMapping: cur.ClaimMapping, AllowedDomains: cur.AllowedDomains,
	}
	if in.Name != nil {
		n.Name = *in.Name
	}
	if in.Preset != nil {
		n.Preset = *in.Preset
	}
	if in.Issuer != nil {
		n.Issuer = *in.Issuer
	}
	if in.ClientID != nil {
		n.ClientID = *in.ClientID
	}
	if in.Scopes != nil {
		n.Scopes = *in.Scopes
	}
	if in.ClaimMapping != nil {
		n.ClaimMapping = *in.ClaimMapping
	}
	if in.AllowedDomains != nil {
		n.AllowedDomains = *in.AllowedDomains
	}
	n, err = identitymodel.NormalizeProvider(n)
	if err != nil {
		return identitymodel.IdentityProvider{}, err
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
			return identitymodel.IdentityProvider{}, problem.Validation(problem.FieldError{Field: "client_secret", Code: "invalid_length", Message: "Client secret must be at most 4096 characters."})
		}
		if sealed, err = s.sealSecret(secret); err != nil {
			return identitymodel.IdentityProvider{}, err
		}
	}
	claims, err := json.Marshal(n.ClaimMapping)
	if err != nil {
		return identitymodel.IdentityProvider{}, err
	}
	out, err := s.updateProviderTx(ctx, p, id, n, sealed, setSecret, jit, enabled, claims, cur)
	if err != nil {
		return identitymodel.IdentityProvider{}, err
	}
	if cur.Issuer != n.Issuer {
		s.forgetDiscovery(cur.Issuer)
	}
	s.forgetDiscovery(n.Issuer)
	return out, nil
}

func (s *IdentityService) updateProviderTx(ctx context.Context, p principal.Principal, id uuid.UUID, n identitymodel.NormalizedProvider,
	sealed []byte, setSecret, jit, enabled bool, claims []byte, before identitymodel.IdentityProvider) (identitymodel.IdentityProvider, error) {
	err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		res, err := tx.ExecContext(ctx, commands.SQL.Get("UpdateIdentityProvider"), id, n.Name, n.Preset, n.Issuer, n.ClientID,
			setSecret, sealed, n.Scopes, claims, jit, n.AllowedDomains, enabled)
		if err != nil {
			if db.IsUniqueViolation(err) {
				return identitymodel.ErrProviderName
			}
			return err
		}
		if rows, _ := res.RowsAffected(); rows == 0 {
			return identitymodel.ErrProviderNotFound
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "identity_provider.updated",
			ResourceType: identitymodel.IdentityProviderResource, ResourceID: id.String(),
			Details: map[string]any{"name": n.Name, "enabled": enabled, "secret_changed": setSecret,
				"issuer_changed": before.Issuer != n.Issuer}})
	})
	if err != nil {
		return identitymodel.IdentityProvider{}, err
	}
	return s.getProvider(ctx, p, id)
}

// DeleteIdentityProvider removes a provider with its links and mappings
// (identity_provider:delete). Memberships in IdP-managed groups are cleared.
func (s *IdentityService) DeleteIdentityProvider(ctx context.Context, id uuid.UUID) error {
	p, err := s.require(ctx, "delete", identitymodel.IdentityProviderResource, id.String())
	if err != nil {
		return err
	}
	var issuer string
	var cleared int64
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		var row repository.ProviderRow
		if err := tx.GetContext(ctx, &row, queries.SQL.Get("GetIdentityProvider"), id); err != nil {
			if db.IsNoRows(err) {
				return identitymodel.ErrProviderNotFound
			}
			return err
		}
		issuer = row.Issuer
		res, err := tx.ExecContext(ctx, commands.SQL.Get("ClearMappedGroupMembers"), id)
		if err != nil {
			return err
		}
		cleared, _ = res.RowsAffected()
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("DeleteIdentityProvider"), id); err != nil {
			return err
		}
		if cleared > 0 {
			if err := authzsvc.BumpVersion(ctx, tx, p.OrgID); err != nil {
				return err
			}
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "identity_provider.deleted",
			ResourceType: identitymodel.IdentityProviderResource, ResourceID: id.String(),
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
func (s *IdentityService) TestIdentityProvider(ctx context.Context, id uuid.UUID) (identitymodel.IdentityProviderTest, error) {
	p, err := s.require(ctx, "test", identitymodel.IdentityProviderResource, id.String())
	if err != nil {
		return identitymodel.IdentityProviderTest{}, err
	}
	prov, err := s.getProvider(ctx, p, id)
	if err != nil {
		return identitymodel.IdentityProviderTest{}, err
	}
	start := time.Now()
	out := identitymodel.IdentityProviderTest{CheckedAt: s.now()}
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
func (s *IdentityService) ListGroupMappings(ctx context.Context, id uuid.UUID) ([]identitymodel.GroupMapping, error) {
	p, err := s.require(ctx, "read", identitymodel.IdentityProviderResource, id.String())
	if err != nil {
		return nil, err
	}
	out := []identitymodel.GroupMapping{}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		var exists bool
		if err := tx.GetContext(ctx, &exists, "SELECT EXISTS (SELECT 1 FROM identity_providers WHERE id = $1)", id); err != nil {
			return err
		}
		if !exists {
			return identitymodel.ErrProviderNotFound
		}
		return tx.SelectContext(ctx, &out, queries.SQL.Get("ListGroupMappings"), id)
	})
	return out, err
}

// CreateGroupMapping maps an external group to an internal group
// (identity_provider:update). Without groupID, an IdP-managed group is created.
func (s *IdentityService) CreateGroupMapping(ctx context.Context, id uuid.UUID, external string, groupID *uuid.UUID) (identitymodel.GroupMapping, error) {
	p, err := s.require(ctx, "update", identitymodel.IdentityProviderResource, id.String())
	if err != nil {
		return identitymodel.GroupMapping{}, err
	}
	external = strings.TrimSpace(external)
	if l := utf8.RuneCountInString(external); l < 1 || l > 200 {
		return identitymodel.GroupMapping{}, problem.Validation(problem.FieldError{Field: "external_group", Code: "invalid_length", Message: "External group must be 1–200 characters."})
	}
	var out identitymodel.GroupMapping
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		var exists bool
		if err := tx.GetContext(ctx, &exists, "SELECT EXISTS (SELECT 1 FROM identity_providers WHERE id = $1)", id); err != nil {
			return err
		}
		if !exists {
			return identitymodel.ErrProviderNotFound
		}
		var target uuid.UUID
		if groupID == nil {
			var found uuid.UUID
			err := tx.GetContext(ctx, &found, queries.SQL.Get("GroupIDByNameAndSource"), external, "idp")
			switch {
			case err == nil:
				target = found
			case db.IsNoRows(err):
				if utf8.RuneCountInString(external) > 100 {
					return problem.Validation(problem.FieldError{Field: "external_group", Code: "group_name_too_long",
						Message: "IdP group names longer than 100 characters need an existing group."})
				}
				target = uuid.Must(uuid.NewV7())
				if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertIdPGroup"), target, p.OrgID, external, nil); err != nil {
					if db.IsUniqueViolation(err) {
						return identitymodel.ErrGroupNameTaken
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
		if err := s.acl.CheckCanHandOut(ctx, tx, p, authzmodel.PrincipalRef{Type: authzmodel.PrincipalGroup, ID: target}); err != nil {
			return err
		}
		mappingID := uuid.Must(uuid.NewV7())
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertGroupMapping"), mappingID, p.OrgID, id, external, target, p.ID); err != nil {
			if db.IsUniqueViolation(err) {
				return errMappingConflict(err)
			}
			return err
		}
		return tx.GetContext(ctx, &out, queries.SQL.Get("GetGroupMapping"), mappingID, id)
	})
	if err != nil {
		return identitymodel.GroupMapping{}, err
	}
	return out, nil
}

func errMappingConflict(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && strings.Contains(pgErr.ConstraintName, "external_group") {
		return identitymodel.ErrMappingExists
	}
	return identitymodel.ErrMappingGroup
}

// DeleteGroupMapping removes a mapping and clears IdP-managed memberships it
// owned (identity_provider:update).
func (s *IdentityService) DeleteGroupMapping(ctx context.Context, id, mappingID uuid.UUID) error {
	p, err := s.require(ctx, "update", identitymodel.IdentityProviderResource, id.String())
	if err != nil {
		return err
	}
	var groupID uuid.UUID
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		var row identitymodel.GroupMapping
		if err := tx.GetContext(ctx, &row, queries.SQL.Get("GetGroupMapping"), mappingID, id); err != nil {
			if db.IsNoRows(err) {
				return identitymodel.ErrMappingNotFound
			}
			return err
		}
		groupID = row.GroupID
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("DeleteGroupMapping"), mappingID, id); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, commands.SQL.Get("ClearIdPGroupMembers"), groupID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			if err := authzsvc.BumpVersion(ctx, tx, p.OrgID); err != nil {
				return err
			}
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "group_mapping.deleted",
			ResourceType: identitymodel.IdentityProviderResource, ResourceID: id.String(),
			Details: map[string]any{"mapping_id": mappingID, "group_id": groupID}})
	})
	if err != nil {
		return err
	}
	s.acl.Invalidate(ctx, p.OrgID)
	return nil
}

// OIDCProviderOptions lists enabled providers for sign-in (public).
func (s *IdentityService) OIDCProviderOptions(ctx context.Context, slug string) ([]identitymodel.OIDCProviderOption, error) {
	ci := principal.ClientInfoFrom(ctx)
	if err := s.limiter.Allow(ctx, ruleOIDCLookup, ci.IP.String()); err != nil {
		return nil, err
	}
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" || len(slug) > 40 {
		return []identitymodel.OIDCProviderOption{}, nil
	}
	var rows []struct {
		ProviderID uuid.UUID `db:"provider_id"`
		Name       string    `db:"provider_name"`
		Preset     string    `db:"preset"`
	}
	err := db.InTx(ctx, s.pool, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, queries.SQL.Get("OIDCProviders"), slug)
	})
	if err != nil {
		return nil, err
	}
	out := make([]identitymodel.OIDCProviderOption, len(rows))
	for i, r := range rows {
		out[i] = identitymodel.OIDCProviderOption{ID: r.ProviderID, Name: r.Name, Preset: r.Preset}
	}
	return out, nil
}

func (s *IdentityService) providerAuthByID(ctx context.Context, id uuid.UUID) (repository.OidcProviderAuth, error) {
	var row repository.OidcProviderAuth
	err := db.InTx(ctx, s.pool, func(tx *sqlx.Tx) error {
		return tx.GetContext(ctx, &row, queries.SQL.Get("OIDCProvider"), id)
	})
	if db.IsNoRows(err) {
		return row, identitymodel.ErrProviderNotFound
	}
	return row, err
}

// StartOIDCLogin creates the authorization redirect and sealed state cookie
// (public). The caller sets the cookie; the state binds the callback to this
// browser.
func (s *IdentityService) StartOIDCLogin(ctx context.Context, idpID uuid.UUID, redirect string) (identitymodel.OIDCStart, error) {
	ci := principal.ClientInfoFrom(ctx)
	if err := s.limiter.Allow(ctx, ruleOIDCStart, ci.IP.String()); err != nil {
		return identitymodel.OIDCStart{}, err
	}
	redirect, err := identitymodel.SafeRedirect(redirect)
	if err != nil {
		return identitymodel.OIDCStart{}, err
	}
	auth, err := s.providerAuthByID(ctx, idpID)
	if err != nil {
		if db.IsNoRows(err) {
			return identitymodel.OIDCStart{}, identitymodel.SsoFail("provider_unavailable")
		}
		return identitymodel.OIDCStart{}, err
	}
	prov := auth.Provider()
	if !prov.Enabled || auth.OrgStatus != "active" {
		return identitymodel.OIDCStart{}, identitymodel.SsoFail("provider_unavailable")
	}
	discovered, err := s.discover(ctx, prov.Issuer, false)
	if err != nil {
		s.log.WarnContext(ctx, "oidc start discovery failed", "provider_id", idpID, "err", err)
		return identitymodel.OIDCStart{}, identitymodel.SsoFail("provider_unavailable")
	}
	secret, err := s.openSecret(auth.ClientSecret)
	if err != nil {
		return identitymodel.OIDCStart{}, err
	}
	state := identitymodel.OidcState{
		IDP: prov.ID, Org: auth.OrgID, State: identitymodel.RandomString(32), Nonce: identitymodel.RandomString(32),
		Verifier: identitymodel.RandomString(48), Redirect: redirect, Expires: s.now().Add(identitymodel.OidcStateTTL),
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return identitymodel.OIDCStart{}, err
	}
	if s.cfg.Sealer == nil {
		return identitymodel.OIDCStart{}, errors.New("identity: master key is not configured")
	}
	sealed, err := s.cfg.Sealer.Seal(raw)
	if err != nil {
		return identitymodel.OIDCStart{}, err
	}
	if len(sealed) > identitymodel.OidcMaxStates {
		return identitymodel.OIDCStart{}, errors.New("identity: sealed OIDC state too large")
	}
	cfg := &oauth2.Config{
		ClientID: prov.ClientID, ClientSecret: secret, Scopes: prov.Scopes, RedirectURL: s.RedirectURI(),
		Endpoint: discovered.Endpoint(),
	}
	authURL := cfg.AuthCodeURL(state.State,
		oidc.Nonce(state.Nonce),
		oauth2.S256ChallengeOption(state.Verifier),
	)
	return identitymodel.OIDCStart{
		AuthURL:     authURL,
		SealedState: base64.RawURLEncoding.EncodeToString(sealed),
		Expires:     state.Expires,
	}, nil
}

// CompleteOIDCLogin validates the callback, links or provisions the user,
// applies group mappings and issues a session (public).
func (s *IdentityService) CompleteOIDCLogin(ctx context.Context, sealedCookie string, in identitydto.OIDCCallbackParams) (identitymodel.OIDCCompletion, error) {
	ci := principal.ClientInfoFrom(ctx)
	if err := s.limiter.Allow(ctx, ruleOIDCCallback, ci.IP.String()); err != nil {
		return identitymodel.OIDCCompletion{}, err
	}
	if sealedCookie == "" {
		return identitymodel.OIDCCompletion{}, identitymodel.SsoFail("state_missing")
	}
	sealed, err := base64.RawURLEncoding.DecodeString(sealedCookie)
	if err != nil || s.cfg.Sealer == nil {
		return identitymodel.OIDCCompletion{}, identitymodel.SsoFail("state_invalid")
	}
	raw, err := s.cfg.Sealer.Open(sealed)
	if err != nil {
		return identitymodel.OIDCCompletion{}, identitymodel.SsoFail("state_invalid")
	}
	var state identitymodel.OidcState
	if err := json.Unmarshal(raw, &state); err != nil || s.now().After(state.Expires) {
		return identitymodel.OIDCCompletion{}, identitymodel.SsoFail("state_expired")
	}
	if len(in.State) > 512 || subtle.ConstantTimeCompare([]byte(in.State), []byte(state.State)) != 1 {
		return identitymodel.OIDCCompletion{}, identitymodel.SsoFail("state_invalid")
	}
	if in.Error != "" {
		s.log.InfoContext(ctx, "identity provider returned an error", "error", in.Error, "description", in.ErrorDescription)
		if in.Error == "access_denied" {
			return identitymodel.OIDCCompletion{}, identitymodel.SsoFail("access_denied")
		}
		return identitymodel.OIDCCompletion{}, identitymodel.SsoFail("idp_error")
	}
	if in.Code == "" || len(in.Code) > 4096 {
		return identitymodel.OIDCCompletion{}, identitymodel.SsoFail("idp_error")
	}

	auth, err := s.providerAuthByID(ctx, state.IDP)
	if err != nil {
		if db.IsNoRows(err) {
			return identitymodel.OIDCCompletion{}, identitymodel.SsoFail("provider_unavailable")
		}
		return identitymodel.OIDCCompletion{}, err
	}
	prov := auth.Provider()
	if !prov.Enabled || auth.OrgStatus != "active" {
		return identitymodel.OIDCCompletion{}, identitymodel.SsoFail("provider_unavailable")
	}
	discovered, err := s.discover(ctx, prov.Issuer, false)
	if err != nil {
		s.log.WarnContext(ctx, "oidc callback discovery failed", "provider_id", state.IDP, "err", err)
		return identitymodel.OIDCCompletion{}, identitymodel.SsoFail("provider_unavailable")
	}
	secret, err := s.openSecret(auth.ClientSecret)
	if err != nil {
		return identitymodel.OIDCCompletion{}, err
	}
	cfg := &oauth2.Config{
		ClientID: prov.ClientID, ClientSecret: secret, Scopes: prov.Scopes, RedirectURL: s.RedirectURI(),
		Endpoint: discovered.Endpoint(),
	}
	exchangeCtx := oidc.ClientContext(ctx, s.oidcClient())
	token, err := cfg.Exchange(exchangeCtx, in.Code, oauth2.VerifierOption(state.Verifier))
	if err != nil {
		s.log.WarnContext(ctx, "oidc code exchange failed", "provider_id", state.IDP, "err", err)
		return identitymodel.OIDCCompletion{}, identitymodel.SsoFail("idp_error")
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		s.log.WarnContext(ctx, "oidc token response has no id_token", "provider_id", state.IDP)
		return identitymodel.OIDCCompletion{}, identitymodel.SsoFail("idp_error")
	}
	idToken, err := discovered.Verifier(&oidc.Config{ClientID: prov.ClientID}).Verify(exchangeCtx, rawIDToken)
	if err != nil {
		s.log.WarnContext(ctx, "oidc id token verification failed", "provider_id", state.IDP, "err", err)
		return identitymodel.OIDCCompletion{}, identitymodel.SsoFail("invalid_token")
	}
	if subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(state.Nonce)) != 1 {
		s.log.WarnContext(ctx, "oidc nonce mismatch", "provider_id", state.IDP)
		return identitymodel.OIDCCompletion{}, identitymodel.SsoFail("invalid_token")
	}

	var rawClaims map[string]any
	if err := idToken.Claims(&rawClaims); err != nil {
		return identitymodel.OIDCCompletion{}, identitymodel.SsoFail("invalid_token")
	}
	claims := identitymodel.ParseClaims(rawClaims, prov.ClaimMapping, idToken.Subject)
	if err := s.ensureClaimAllowed(prov, claims); err != nil {
		return identitymodel.OIDCCompletion{}, err
	}

	org := identitymodel.OrgSummary{ID: auth.OrgID, Slug: auth.OrgSlug, Name: auth.OrgName}
	settings := orgmodel.ParseSettings(auth.OrgSettings)

	grant, err := s.linkOrProvision(ctx, prov, org, settings, idToken.Subject, claims)
	if err != nil {
		return identitymodel.OIDCCompletion{}, err
	}
	return identitymodel.OIDCCompletion{Grant: grant, Redirect: state.Redirect}, nil
}

func (s *IdentityService) ensureClaimAllowed(prov identitymodel.IdentityProvider, c identitymodel.OidcClaims) error {
	if c.EmailVerified != nil && !*c.EmailVerified {
		return identitymodel.SsoFail("email_unverified")
	}
	if c.Email == "" || !identitymodel.ValidEmail(c.Email) {
		return identitymodel.SsoFail("email_missing")
	}
	if len(prov.AllowedDomains) > 0 {
		at := strings.LastIndex(c.Email, "@")
		if at < 0 || !identitymodel.DomainAllowed(c.Email[at+1:], prov.AllowedDomains) {
			return identitymodel.SsoFail("email_domain_not_allowed")
		}
	}
	return nil
}

func (s *IdentityService) identityLink(ctx context.Context, idpID uuid.UUID, subject string) (repository.IdentityLinkRow, bool, error) {
	var row repository.IdentityLinkRow
	err := db.InTx(ctx, s.pool, func(tx *sqlx.Tx) error {
		return tx.GetContext(ctx, &row, queries.SQL.Get("OIDCIdentityLink"), idpID, subject)
	})
	if db.IsNoRows(err) {
		return row, false, nil
	}
	if err != nil {
		return row, false, err
	}
	return row, true, nil
}

func (s *IdentityService) userByEmail(ctx context.Context, email string) (identitymodel.User, bool, error) {
	var row struct {
		UserID      uuid.UUID `db:"user_id"`
		DisplayName string    `db:"display_name"`
		Status      string    `db:"status"`
	}
	err := db.InTx(ctx, s.pool, func(tx *sqlx.Tx) error {
		return tx.GetContext(ctx, &row, queries.SQL.Get("OIDCUserByEmail"), email)
	})
	if db.IsNoRows(err) {
		return identitymodel.User{}, false, nil
	}
	if err != nil {
		return identitymodel.User{}, false, err
	}
	return identitymodel.User{ID: row.UserID, Email: email, DisplayName: row.DisplayName, Status: row.Status}, true, nil
}

func (s *IdentityService) linkOrProvision(ctx context.Context, prov identitymodel.IdentityProvider, org identitymodel.OrgSummary, settings orgmodel.Settings,
	subject string, c identitymodel.OidcClaims) (identitymodel.SessionGrant, error) {
	if link, ok, err := s.identityLink(ctx, prov.ID, subject); err != nil {
		return identitymodel.SessionGrant{}, err
	} else if ok {
		if link.UserStatus != "active" {
			return identitymodel.SessionGrant{}, identitymodel.SsoFail("account_disabled")
		}
		if link.MembershipStatus != identitymodel.MembershipActive {
			return identitymodel.SessionGrant{}, identitymodel.SsoFail("membership_suspended")
		}
		user := identitymodel.User{ID: link.UserID, Email: link.Email, DisplayName: link.DisplayName, Status: link.UserStatus}
		return s.finishOIDCLogin(ctx, prov, org, settings, user, link.LinkID, c.Groups)
	}

	existing, found, err := s.userByEmail(ctx, c.Email)
	if err != nil {
		return identitymodel.SessionGrant{}, err
	}
	if found && existing.Status != "active" {
		return identitymodel.SessionGrant{}, identitymodel.SsoFail("account_disabled")
	}
	if found {
		// Never auto-link an account that is not already a member: a
		// misconfigured or malicious IdP must not take over an account whose
		// email merely matches.
		var status string
		if err := db.InTx(ctx, s.pool, func(tx *sqlx.Tx) error {
			return tx.GetContext(ctx, &status, queries.SQL.Get("GetMembershipStatusAny"), existing.ID, org.ID)
		}); err != nil {
			return identitymodel.SessionGrant{}, err
		}
		if status == "" {
			return identitymodel.SessionGrant{}, identitymodel.SsoFail("account_exists")
		}
		if status != identitymodel.MembershipActive {
			return identitymodel.SessionGrant{}, identitymodel.SsoFail("membership_suspended")
		}
		return s.provisionAndLogin(ctx, prov, org, settings, existing, subject, c.Groups, false)
	}

	if !prov.JITProvisioning {
		return identitymodel.SessionGrant{}, identitymodel.SsoFail("no_account")
	}
	user := identitymodel.User{ID: uuid.Must(uuid.NewV7()), Email: c.Email, DisplayName: c.DisplayName, Status: "active"}
	return s.provisionAndLogin(ctx, prov, org, settings, user, subject, c.Groups, true)
}

func (s *IdentityService) provisionAndLogin(ctx context.Context, prov identitymodel.IdentityProvider, org identitymodel.OrgSummary, settings orgmodel.Settings,
	user identitymodel.User, subject string, groups []string, create bool) (identitymodel.SessionGrant, error) {
	var grant identitymodel.SessionGrant
	var linkID = uuid.Must(uuid.NewV7())
	changed := false
	err := db.InTenantTx(ctx, s.pool, org.ID, func(tx *sqlx.Tx) error {
		if create {
			if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertUser"), user.ID, user.Email, user.DisplayName); err != nil {
				if db.IsUniqueViolation(err) {
					return identitymodel.SsoFail("account_exists")
				}
				return err
			}
		}
		res, err := tx.ExecContext(ctx, commands.SQL.Get("InsertMembershipIfAbsent"), org.ID, user.ID)
		if err != nil {
			return err
		}
		added, _ := res.RowsAffected()
		var status string
		if err := tx.GetContext(ctx, &status, queries.SQL.Get("GetMembershipStatus"), user.ID); err != nil {
			return err
		}
		if status != identitymodel.MembershipActive {
			return identitymodel.SsoFail("membership_suspended")
		}
		if added > 0 {
			if err := authzsvc.BindRoleKeyInTx(ctx, tx, org.ID,
				authzmodel.PrincipalRef{Type: authzmodel.PrincipalUser, ID: user.ID},
				authzmodel.MemberRoleKey, authzmodel.ScopeRef{Type: authzmodel.ScopeOrg}, uuid.Nil); err != nil {
				return err
			}
			actor := &auditmodel.Actor{Type: string(principal.User), ID: user.ID}
			if err := auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: org.ID, Action: "user.joined", ResourceType: identitymodel.UserResource,
				ResourceID: user.ID.String(), Actor: actor,
				Details: map[string]any{"source": "oidc", "identity_provider_id": prov.ID, "new_account": create}}); err != nil {
				return err
			}
			if _, err := outbox.Publish(ctx, tx, outbox.Event{OrgID: org.ID, Subject: "hugi.events.user.joined",
				Payload: map[string]any{"user_id": user.ID, "identity_provider_id": prov.ID}}); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertIdentityLink"), linkID, org.ID, prov.ID, subject, user.ID); err != nil {
			if db.IsUniqueViolation(err) {
				return identitymodel.SsoFail("account_conflict")
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
		return identitymodel.SessionGrant{}, err
	}
	if changed || create {
		s.acl.Invalidate(ctx, org.ID)
	}
	return grant, nil
}

func (s *IdentityService) finishOIDCLogin(ctx context.Context, prov identitymodel.IdentityProvider, org identitymodel.OrgSummary, settings orgmodel.Settings,
	user identitymodel.User, linkID uuid.UUID, groups []string) (identitymodel.SessionGrant, error) {
	var grant identitymodel.SessionGrant
	var changed bool
	err := db.InTenantTx(ctx, s.pool, org.ID, func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("TouchIdentityLink"), linkID); err != nil {
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
		return identitymodel.SessionGrant{}, err
	}
	if changed {
		s.acl.Invalidate(ctx, org.ID)
	}
	return grant, nil
}

func (s *IdentityService) syncIdPGroups(ctx context.Context, tx *sqlx.Tx, orgID, idpID, userID uuid.UUID, claimed []string) (bool, error) {
	var mapped []struct {
		ID            uuid.UUID `db:"id"`
		ExternalGroup string    `db:"external_group"`
	}
	if err := tx.SelectContext(ctx, &mapped, queries.SQL.Get("MappedGroupIDs"), idpID); err != nil {
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
	if err := tx.SelectContext(ctx, &current, queries.SQL.Get("ListUserMappedGroupIDs"), idpID, userID); err != nil {
		return false, err
	}
	changed := false
	have := map[uuid.UUID]bool{}
	for _, g := range current {
		have[g] = true
		if !desired[g] {
			if _, err := tx.ExecContext(ctx, commands.SQL.Get("DeleteIdPGroupMember"), g, userID); err != nil {
				return false, err
			}
			changed = true
		}
	}
	for g := range desired {
		if !have[g] {
			if _, err := tx.ExecContext(ctx, commands.SQL.Get("SyncIdPGroupMembers"), userID, nil, g); err != nil {
				return false, err
			}
			changed = true
		}
	}
	if changed {
		if err := authzsvc.BumpVersion(ctx, tx, orgID); err != nil {
			return false, err
		}
	}
	return changed, nil
}
