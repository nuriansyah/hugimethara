package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/nuriansyah/hugimethara/backend/internal/api/apigen"
	"github.com/nuriansyah/hugimethara/backend/internal/identity"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/health"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/httpx"
)

func newTestRouter(t *testing.T, checks map[string]health.Check) *httptest.Server {
	t.Helper()
	h := health.New(0)
	for n, c := range checks {
		h.Add(n, c)
	}
	router, err := NewRouter(Deps{
		Env: "test", Health: h, Registerer: prometheus.NewRegistry(),
		Identity: identity.NewHTTP(nil, identity.Cookies{}), // no cookie sent => service never called
	})
	require.NoError(t, err)
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv
}

func TestSystemInfo(t *testing.T) {
	srv := newTestRouter(t, nil)
	resp, err := srv.Client().Get(srv.URL + "/v1/hugimethara/system/info")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, 200, resp.StatusCode)
	require.NotEmpty(t, resp.Header.Get(httpx.RequestIDHeader))
	require.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))

	var info apigen.SystemInfo
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&info))
	require.Equal(t, "Hugimethara", info.Name)
	require.Equal(t, apigen.SystemInfoEnvironment("test"), info.Environment)
	require.Equal(t, "v1", info.ApiVersion)
}

func TestReadinessReflectsChecks(t *testing.T) {
	srv := newTestRouter(t, map[string]health.Check{
		"postgres": func(context.Context) error { return nil },
		"nats":     func(context.Context) error { return errors.New("not connected") },
	})
	resp, err := srv.Client().Get(srv.URL + "/readyz")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, 503, resp.StatusCode)
	var rep apigen.HealthReport
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&rep))
	require.Equal(t, apigen.HealthReportStatusFail, rep.Status)
	require.Equal(t, "not connected", *(*rep.Checks)["nats"].Error)

	resp2, err := srv.Client().Get(srv.URL + "/healthz")
	require.NoError(t, err)
	_ = resp2.Body.Close()
	require.Equal(t, 200, resp2.StatusCode)
}

func TestUnknownRouteIsProblem(t *testing.T) {
	srv := newTestRouter(t, nil)
	resp, err := srv.Client().Get(srv.URL + "/v1/hugimethara/nope")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, 404, resp.StatusCode)
	require.Equal(t, "application/problem+json", resp.Header.Get("Content-Type"))
	var p httpx.Problem
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&p))
	require.Equal(t, "not_found", p.Code)
	require.NotEmpty(t, p.RequestID)
}

func TestProtectedOperationsRequireSession(t *testing.T) {
	srv := newTestRouter(t, nil)
	for _, path := range []string{"/v1/hugimethara/me", "/v1/hugimethara/organizations/current", "/v1/hugimethara/audit-events"} {
		resp, err := srv.Client().Get(srv.URL + path)
		require.NoError(t, err)
		_ = resp.Body.Close()
		require.Equal(t, 401, resp.StatusCode, path)
	}
}

func TestPublicOperationsComeFromSpec(t *testing.T) {
	public, err := PublicOperations()
	require.NoError(t, err)
	require.Equal(t, map[string]bool{
		"GetLiveness": true, "GetReadiness": true, "GetSystemInfo": true,
		"CreateOrganization": true, "Login": true, "Logout": true,
		"GetInvitationPreview": true, "AcceptInvitation": true, // S4: invitation links (token-gated, rate-limited)
		"ListOidcProviders": true, "StartOidcLogin": true, "CompleteOidcLogin": true, // S5: SSO before a session exists
	}, public, "adding a public operation is a security decision: update this test deliberately")
}

func TestCrossSiteStateChangeRejected(t *testing.T) {
	srv := newTestRouter(t, nil)
	req, _ := http.NewRequest("POST", srv.URL+"/v1/hugimethara/auth/login", strings.NewReader(`{}`))
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Content-Type", "application/json")
	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, 403, resp.StatusCode)
}

func TestRequireSessionMatchesGeneratedOperationNames(t *testing.T) {
	mw := requireSession(map[string]bool{"login": true})
	called := false
	next := func(context.Context, http.ResponseWriter, *http.Request, any) (any, error) {
		called = true
		return nil, nil
	}

	_, err := mw(next, "Login")(context.Background(), nil, nil, nil)
	require.NoError(t, err)
	require.True(t, called, "public op must pass without a session")

	called = false
	_, err = mw(next, "GetMe")(context.Background(), nil, nil, nil)
	require.Error(t, err)
	require.False(t, called, "protected op must not run without a session")
}
