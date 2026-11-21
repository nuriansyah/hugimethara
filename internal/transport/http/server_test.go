package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/nuriansyah/hugimethara/internal/infrastructure/observability/health"
	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/transport/http/dto"
	identityhandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/identity"
	"github.com/nuriansyah/hugimethara/internal/transport/http/middleware"
)

func newTestRouter(t *testing.T, checks map[string]health.Check) *httptest.Server {
	t.Helper()
	h := health.New(0)
	for n, c := range checks {
		h.Add(n, c)
	}
	router, err := NewRouter(Deps{
		Env: "test", Health: h, Registerer: prometheus.NewRegistry(),
		Identity: identityhandler.NewHTTP(nil, identityhandler.Cookies{}), // no cookie sent => service never called
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
	require.NotEmpty(t, resp.Header.Get(middleware.RequestIDHeader))
	require.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))

	var info dto.SystemInfo
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&info))
	require.Equal(t, "Hugimethara", info.Name)
	require.Equal(t, dto.SystemInfoEnvironment("test"), info.Environment)
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
	var rep dto.HealthReport
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&rep))
	require.Equal(t, dto.HealthReportStatusFail, rep.Status)
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
	var p problem.Problem
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&p))
	require.Equal(t, "not_found", p.Code)
	require.NotEmpty(t, p.RequestID)
}

func TestProtectedOperationsRequireSession(t *testing.T) {
	srv := newTestRouter(t, nil)
	workspaceID := uuid.New()
	executionID := uuid.New()
	for _, path := range []string{
		"/v1/hugimethara/me",
		"/v1/hugimethara/organizations/current",
		"/v1/hugimethara/audit-events",
		"/v1/hugimethara/api-compositions?workspace_id=" + workspaceID.String(),
		"/v1/hugimethara/api-composition-executions/" + executionID.String(),
		"/v1/hugimethara/approval-tasks",
		"/v1/hugimethara/workflow-schedules?workspace_id=" + workspaceID.String(),
		"/v1/hugimethara/webhooks?workspace_id=" + workspaceID.String(),
		"/v1/hugimethara/webhook-deliveries?workspace_id=" + workspaceID.String(),
		"/v1/hugimethara/event-types",
	} {
		resp, err := srv.Client().Get(srv.URL + path)
		require.NoError(t, err)
		_ = resp.Body.Close()
		require.Equal(t, 401, resp.StatusCode, path)
	}
}

func TestPublicOperationsAreExplicit(t *testing.T) {
	public, err := PublicOperations()
	require.NoError(t, err)
	require.Equal(t, map[string]bool{
		"GetLiveness": true, "GetReadiness": true, "GetSystemInfo": true,
		"CreateOrganization": true, "Login": true, "Logout": true,
		"GetInvitationPreview": true, "AcceptInvitation": true, // S4: invitation links (token-gated, rate-limited)
		"ListOidcProviders": true, "StartOidcLogin": true, "CompleteOidcLogin": true, // S5: SSO before a session exists
	}, public, "adding a public operation is a security decision: update this test deliberately")
	// S10 added 14 workflow operations, S11 six more, S12 nineteen webhook operations. The webhook
	// ingress (/hooks/{public_id}) is deliberately outside this table: its credential is the signature.
	// S14 added getOverview; the OpenAPI document (/v1/hugimethara/openapi.*) is static and also outside.
	require.Len(t, routes, 167)
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

func TestCORSPreflightStillAnswered(t *testing.T) {
	h := health.New(0)
	router, err := NewRouter(Deps{Env: "test", Health: h, Registerer: prometheus.NewRegistry(),
		TrustedOrigins: []string{"https://app.example"},
		Identity:       identityhandler.NewHTTP(nil, identityhandler.Cookies{})})
	require.NoError(t, err)
	srv := httptest.NewServer(router)
	defer srv.Close()
	req, _ := http.NewRequest("OPTIONS", srv.URL+"/v1/hugimethara/workflows", nil)
	req.Header.Set("Origin", "https://app.example")
	req.Header.Set("Access-Control-Request-Method", "POST")
	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Less(t, resp.StatusCode, 300)
	require.Equal(t, "https://app.example", resp.Header.Get("Access-Control-Allow-Origin"))
}

// The served contract documents exactly the operation table (S14: "backend
// APIs documented"): every route is in the spec and every spec path is routed.
func TestOpenAPIContractMatchesTheOperationTable(t *testing.T) {
	srv := newTestRouter(t, nil)
	resp, err := srv.Client().Get(srv.URL + OpenAPIPath + ".json")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, 200, resp.StatusCode)
	require.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	var spec struct {
		OpenAPI string                               `json:"openapi"`
		Paths   map[string]map[string]map[string]any `json:"paths"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	require.Equal(t, "3.0.3", spec.OpenAPI)

	documented := map[string]string{}
	for path, methods := range spec.Paths {
		for method, op := range methods {
			id, _ := op["operationId"].(string)
			documented[strings.ToUpper(method)+" "+path] = id
		}
	}
	routed := map[string]bool{"POST " + IngressPath: true}
	for _, r := range routes {
		key := r.method + " " + r.path
		routed[key] = true
		require.Equal(t, r.operation, documented[key], "%s is routed but not documented as %s", key, r.operation)
	}
	for key := range documented {
		require.True(t, routed[key], "%s is documented but not routed", key)
	}

	yamlResp, err := srv.Client().Get(srv.URL + OpenAPIPath + ".yaml")
	require.NoError(t, err)
	_ = yamlResp.Body.Close()
	require.Equal(t, 200, yamlResp.StatusCode)
}
