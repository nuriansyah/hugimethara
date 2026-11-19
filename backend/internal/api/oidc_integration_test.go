//go:build integration

package api_test

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// mockIdP is a minimal OIDC provider: discovery, authorization, token and
// JWKS. It signs RS256 ID tokens and enforces PKCE and the nonce.
type mockIdP struct {
	srv  *httptest.Server
	key  *rsa.PrivateKey
	mu   sync.Mutex
	code map[string]mockCode

	claims          mockClaims
	signWithOther   bool
	lastVerifier    string
	lastRedirectURI string
}

type mockCode struct {
	Nonce       string
	Challenge   string
	RedirectURI string
	ClientID    string
}

type mockClaims struct {
	Email         string
	Name          string
	Groups        []string
	EmailVerified *bool
}

func newMockIdP(t *testing.T) *mockIdP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	idp := &mockIdP{key: key, code: map[string]mockCode{}, claims: mockClaims{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                idp.srv.URL,
			"authorization_endpoint":                idp.srv.URL + "/authorize",
			"token_endpoint":                        idp.srv.URL + "/token",
			"jwks_uri":                              idp.srv.URL + "/jwks",
			"response_types_supported":              []string{"code"},
			"subject_types_supported":               []string{"public"},
			"id_token_signing_alg_values_supported": []string{"RS256"},
			"scopes_supported":                      []string{"openid", "email", "profile"},
			"token_endpoint_auth_methods_supported": []string{"none", "client_secret_basic"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		e := big.NewInt(int64(key.PublicKey.E)).Bytes()
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]any{{
			"kty": "RSA", "kid": "test", "use": "sig", "alg": "RS256",
			"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(e),
		}}})
	})
	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		code := "code-" + fmt.Sprint(time.Now().UnixNano())
		idp.mu.Lock()
		idp.code[code] = mockCode{Nonce: q.Get("nonce"), Challenge: q.Get("code_challenge"),
			RedirectURI: q.Get("redirect_uri"), ClientID: q.Get("client_id")}
		idp.lastRedirectURI = q.Get("redirect_uri")
		idp.mu.Unlock()
		loc, _ := url.Parse(q.Get("redirect_uri"))
		qq := loc.Query()
		qq.Set("code", code)
		qq.Set("state", q.Get("state"))
		loc.RawQuery = qq.Encode()
		http.Redirect(w, r, loc.String(), http.StatusFound)
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		idp.mu.Lock()
		rec, ok := idp.code[r.PostForm.Get("code")]
		delete(idp.code, r.PostForm.Get("code"))
		idp.lastVerifier = r.PostForm.Get("code_verifier")
		broken := idp.signWithOther
		claims := idp.claims
		idp.mu.Unlock()
		if !ok {
			http.Error(w, "unknown code", http.StatusBadRequest)
			return
		}
		sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
		if base64.RawURLEncoding.EncodeToString(sum[:]) != rec.Challenge {
			http.Error(w, "pkce mismatch", http.StatusBadRequest)
			return
		}
		now := time.Now()
		payload := map[string]any{
			"iss": idp.srv.URL, "sub": "subject-" + claims.Email, "aud": rec.ClientID,
			"exp": now.Add(5 * time.Minute).Unix(), "iat": now.Unix(), "nonce": rec.Nonce,
			"email": claims.Email, "name": claims.Name, "groups": claims.Groups,
		}
		if claims.EmailVerified != nil {
			payload["email_verified"] = *claims.EmailVerified
		}
		signKey := idp.key
		if broken {
			other, _ := rsa.GenerateKey(rand.Reader, 2048)
			signKey = other
		}
		header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","kid":"test","typ":"JWT"}`))
		body, _ := json.Marshal(payload)
		signing := header + "." + base64.RawURLEncoding.EncodeToString(body)
		digest := sha256.Sum256([]byte(signing))
		sig, err := rsa.SignPKCS1v15(rand.Reader, signKey, crypto.SHA256, digest[:])
		if err != nil {
			http.Error(w, "sign", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "access", "token_type": "Bearer", "expires_in": 300,
			"id_token": signing + "." + base64.RawURLEncoding.EncodeToString(sig),
		})
	})
	idp.srv = httptest.NewServer(mux)
	t.Cleanup(idp.srv.Close)
	return idp
}

func (m *mockIdP) setClaims(c mockClaims) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.claims = c
}

func (m *mockIdP) breakSignature(broken bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.signWithOther = broken
}

func noFollow(c *client) *http.Client {
	return &http.Client{Jar: c.jar, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
}

func (e *env) oidcStart(t *testing.T, c *client, providerID, redirect string) (*url.URL, string) {
	t.Helper()
	hc := noFollow(c)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
		e.srv.URL+base+"/auth/oidc/"+providerID+"/start?redirect="+url.QueryEscape(redirect), nil)
	require.NoError(t, err)
	res, err := hc.Do(req)
	require.NoError(t, err)
	_ = res.Body.Close()
	require.Equal(t, http.StatusFound, res.StatusCode)
	loc := res.Header.Get("Location")
	u, err := url.Parse(loc)
	require.NoError(t, err)
	return u, loc
}

// oidcLogin runs the whole browser flow and returns the callback redirect
// location (success lands on the web app, failures carry sso_error).
func (e *env) oidcLogin(t *testing.T, c *client, providerID, redirect string) string {
	t.Helper()
	authURL, _ := e.oidcStart(t, c, providerID, redirect)
	hc := noFollow(c)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, authURL.String(), nil)
	require.NoError(t, err)
	res, err := hc.Do(req)
	require.NoError(t, err)
	_ = res.Body.Close()
	require.Equal(t, http.StatusFound, res.StatusCode)
	loc, err := url.Parse(res.Header.Get("Location"))
	require.NoError(t, err)
	code := loc.Query().Get("code")
	state := loc.Query().Get("state")
	require.NotEmpty(t, code)

	cbURL := e.srv.URL + base + "/auth/oidc/callback?code=" + url.QueryEscape(code) + "&state=" + url.QueryEscape(state)
	req, err = http.NewRequestWithContext(context.Background(), http.MethodGet, cbURL, nil)
	require.NoError(t, err)
	res, err = hc.Do(req)
	require.NoError(t, err)
	_ = res.Body.Close()
	require.Equal(t, http.StatusFound, res.StatusCode)
	return res.Header.Get("Location")
}

func ssoError(location string) string {
	u, err := url.Parse(location)
	if err != nil {
		return ""
	}
	return u.Query().Get("sso_error")
}

func (e *env) createProvider(t *testing.T, owner *client, idp *mockIdP, extra map[string]any) string {
	t.Helper()
	body := map[string]any{
		"name": "Acme SSO", "preset": "generic", "issuer": idp.srv.URL, "client_id": "hugi-test",
		"scopes":          []string{"openid", "email", "profile"},
		"claim_mapping":   map[string]any{"email": "email", "name": "name", "groups": "groups"},
		"allowed_domains": []string{"acme.test"},
	}
	for k, v := range extra {
		body[k] = v
	}
	r := owner.do("POST", base+"/identity/providers", body, true)
	require.Equal(t, 201, r.Status, r.Body)
	return r.Body["id"].(string)
}

func TestOIDCJITLoginAndGroupSync(t *testing.T) {
	e := setup(t)
	idp := newMockIdP(t)
	owner := e.client()
	r := owner.do("POST", base+"/organizations", signupBody("acme", "alice@acme.test"), false)
	require.Equal(t, 201, r.Status, r.Body)

	providerID := e.createProvider(t, owner, idp, nil)

	r = owner.do("POST", base+"/groups", map[string]any{"name": "Engineers"}, true)
	require.Equal(t, 201, r.Status, r.Body)
	groupID := r.Body["id"].(string)
	r = owner.do("POST", base+"/identity/providers/"+providerID+"/group-mappings",
		map[string]any{"external_group": "eng", "group_id": groupID}, true)
	require.Equal(t, 201, r.Status, r.Body)
	require.Equal(t, "Engineers", r.Body["group_name"])

	verified := true
	idp.setClaims(mockClaims{Email: "bob@acme.test", Name: "Bob", Groups: []string{"eng"}, EmailVerified: &verified})
	bob := e.client()
	loc := e.oidcLogin(t, bob, providerID, "/welcome")
	require.Equal(t, "http://localhost:3000/welcome", loc)
	me := bob.do("GET", base+"/me", nil, false)
	require.Equal(t, 200, me.Status, me.Body)
	require.Equal(t, "bob@acme.test", me.Body["user"].(map[string]any)["email"])
	require.Equal(t, false, me.Body["membership"].(map[string]any)["is_owner"])

	members := data(owner.do("GET", base+"/groups/"+groupID+"/members", nil, false))
	require.Len(t, members, 1)
	require.Equal(t, "bob@acme.test", members[0]["email"])

	// Next sign-in without the group removes the mapped membership.
	idp.setClaims(mockClaims{Email: "bob@acme.test", Name: "Bob", EmailVerified: &verified})
	bob2 := e.client()
	require.Equal(t, "http://localhost:3000/", e.oidcLogin(t, bob2, providerID, ""))
	require.Empty(t, data(owner.do("GET", base+"/groups/"+groupID+"/members", nil, false)))

	// JIT membership gets the baseline member role: it may read groups but not configure SSO.
	require.Equal(t, 403, bob2.do("GET", base+"/identity/providers", nil, false).Status)
	require.Contains(t, auditActions(t, owner, "?action=user"), "user.joined:success")
}

func TestOIDCProviderSecurityAndLifecycle(t *testing.T) {
	e := setup(t)
	idp := newMockIdP(t)
	owner := e.client()
	require.Equal(t, 201, owner.do("POST", base+"/organizations", signupBody("acme", "alice@acme.test"), false).Status)

	r := owner.do("POST", base+"/identity/providers", map[string]any{
		"name": "Acme SSO", "preset": "entra", "issuer": idp.srv.URL, "client_id": "hugi-test",
		"client_secret": "super-secret", "allowed_domains": []string{"acme.test"},
	}, true)
	require.Equal(t, 201, r.Status, r.Body)
	providerID := r.Body["id"].(string)
	require.Equal(t, true, r.Body["has_client_secret"])
	require.NotContains(t, r.Body, "client_secret")
	require.Contains(t, r.Body["redirect_uri"], "/api/v1/hugimethara/auth/oidc/callback")
	require.Contains(t, r.Body["sso_url"], "/start")

	// The probe reaches discovery and JWKS.
	r = owner.do("POST", base+"/identity/providers/"+providerID+"/test", nil, true)
	require.Equal(t, 200, r.Status, r.Body)
	require.Equal(t, true, r.Body["ok"], r.Body)
	require.NotEmpty(t, r.Body["jwks_uri"])

	// Other organizations cannot see or change the provider.
	other := e.client()
	require.Equal(t, 201, other.do("POST", base+"/organizations", signupBody("other", "o@other.test"), false).Status)
	require.Equal(t, 404, other.do("GET", base+"/identity/providers/"+providerID, nil, false).Status)
	require.Equal(t, 404, other.do("POST", base+"/identity/providers/"+providerID+"/test", nil, true).Status)

	// Disabling hides it from the public sign-in hint and blocks new flows.
	r = owner.do("PATCH", base+"/identity/providers/"+providerID, map[string]any{"enabled": false}, true)
	require.Equal(t, 200, r.Status, r.Body)
	anon := e.client()
	r = anon.do("GET", base+"/auth/oidc/providers?organization=acme", nil, false)
	require.Equal(t, 200, r.Status, r.Body)
	require.Empty(t, r.Body["data"])
	_, loc := e.oidcStart(t, anon, providerID, "")
	require.Equal(t, "provider_unavailable", ssoError(loc))

	// Delete is immediate and audited.
	require.Equal(t, 204, owner.do("DELETE", base+"/identity/providers/"+providerID, nil, true).Status)
	require.Equal(t, 404, owner.do("GET", base+"/identity/providers/"+providerID, nil, false).Status)
	require.Contains(t, auditActions(t, owner, "?action=identity_provider"), "identity_provider.deleted:success")
}

func TestOIDCLinkingRules(t *testing.T) {
	e := setup(t)
	idp := newMockIdP(t)
	owner := e.client()
	require.Equal(t, 201, owner.do("POST", base+"/organizations", signupBody("acme", "alice@acme.test"), false).Status)

	// Pre-invited Bob becomes a member, so SSO may link him even with JIT off.
	token, _ := invite(t, owner, map[string]any{"email": "bob@acme.test"})
	bob := e.client()
	require.Equal(t, 200, bob.do("POST", base+"/invitation-tokens/"+token+"/accept",
		map[string]any{"display_name": "Bob", "password": "correct horse battery"}, false).Status)

	providerID := e.createProvider(t, owner, idp, map[string]any{"jit_provisioning": false})
	verified := true
	idp.setClaims(mockClaims{Email: "bob@acme.test", Name: "Bob", EmailVerified: &verified})
	linked := e.client()
	require.Equal(t, "http://localhost:3000/", e.oidcLogin(t, linked, providerID, ""))
	require.Equal(t, 200, linked.do("GET", base+"/me", nil, false).Status)

	// JIT off: an unknown email cannot create an account.
	idp.setClaims(mockClaims{Email: "eve@acme.test", Name: "Eve", EmailVerified: &verified})
	require.Equal(t, "no_account", ssoError(e.oidcLogin(t, e.client(), providerID, "")))

	// JIT on, but the email belongs to an account outside this organization:
	// linking would be a cross-tenant takeover, so it is refused.
	require.Equal(t, 201, e.client().do("POST", base+"/organizations", signupBody("victim", "carol@acme.test"), false).Status)
	require.Equal(t, 200, owner.do("PATCH", base+"/identity/providers/"+providerID,
		map[string]any{"jit_provisioning": true}, true).Status)
	idp.setClaims(mockClaims{Email: "carol@acme.test", Name: "Carol", EmailVerified: &verified})
	require.Equal(t, "account_exists", ssoError(e.oidcLogin(t, e.client(), providerID, "")))

	// Domain allow-list and explicit unverified email are enforced.
	idp.setClaims(mockClaims{Email: "dave@other.test", Name: "Dave", EmailVerified: &verified})
	require.Equal(t, "email_domain_not_allowed", ssoError(e.oidcLogin(t, e.client(), providerID, "")))
	no := false
	idp.setClaims(mockClaims{Email: "dave@acme.test", Name: "Dave", EmailVerified: &no})
	require.Equal(t, "email_unverified", ssoError(e.oidcLogin(t, e.client(), providerID, "")))

	// A tampered state and a token signed by another key both fail closed.
	sc := e.client()
	_, _ = e.oidcStart(t, sc, providerID, "")
	cbURL := e.srv.URL + base + "/auth/oidc/callback?code=x&state=bogus"
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, cbURL, nil)
	res, err := noFollow(sc).Do(req)
	require.NoError(t, err)
	_ = res.Body.Close()
	require.Equal(t, "state_invalid", ssoError(res.Header.Get("Location")))

	idp.setClaims(mockClaims{Email: "new@acme.test", Name: "New", EmailVerified: &verified})
	idp.breakSignature(true)
	require.Equal(t, "invalid_token", ssoError(e.oidcLogin(t, e.client(), providerID, "")))
}

func TestOIDCPublicDiscoveryIsScoped(t *testing.T) {
	e := setup(t)
	idp := newMockIdP(t)
	owner := e.client()
	require.Equal(t, 201, owner.do("POST", base+"/organizations", signupBody("acme", "alice@acme.test"), false).Status)
	providerID := e.createProvider(t, owner, idp, nil)

	r := e.client().do("GET", base+"/auth/oidc/providers?organization=acme", nil, false)
	require.Equal(t, 200, r.Status, r.Body)
	require.Len(t, r.Body["data"], 1)
	opt := r.Body["data"].([]any)[0].(map[string]any)
	require.Equal(t, providerID, opt["id"])
	require.Equal(t, "Acme SSO", opt["name"])

	r = e.client().do("GET", base+"/auth/oidc/providers?organization=nope", nil, false)
	require.Equal(t, 200, r.Status)
	require.Empty(t, r.Body["data"])

	// Slug matching is exact; the public surface never exposes the issuer.
	for _, e := range r.Body["data"].([]any) {
		require.NotContains(t, e.(map[string]any), "issuer")
	}
}
