package connector

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nuriansyah/hugimethara/backend/internal/platform/egress"
)

func TestValidateConfigUsesSchemaAndRejectsInlineCredentials(t *testing.T) {
	schema := []byte(`{"type":"object","properties":{"port":{"type":"integer","minimum":1,"maximum":65535},"host":{"type":"string"}},"required":["host","port"],"additionalProperties":false}`)
	require.NoError(t, validateConfig(schema, []byte(`{"host":"db.example","port":5432}`)))
	require.Error(t, validateConfig(schema, []byte(`{"host":"db.example","port":70000}`)))
	require.Error(t, validateConfig(schema, []byte(`{"host":"db.example"}`)))
	require.Error(t, validateConfig(schema, []byte(`{"host":"db.example","port":5432,"password":"do-not-return"}`)))
}

func TestSanitizeConfigNeverReturnsSensitiveFields(t *testing.T) {
	config := sanitizeConfig([]byte(`{"host":"db.example","auth":{"api_token":"secret-value","mode":"bearer"},"webhook_url":"https://hooks.example/secret","signed_endpoint":"https://s3.example/file?sig=hidden","items":[{"password":"other-secret","label":"ok"}]}`))
	require.JSONEq(t, `{"host":"db.example","auth":{"mode":"bearer"},"items":[{"label":"ok"}]}`, string(config))
	require.NotContains(t, string(config), "secret-value")
	require.NotContains(t, string(config), "other-secret")
	require.NotContains(t, string(config), "sig=hidden")
}

func TestEndpointFromConfigRejectsUserInfoAndSupportsQueryCredentials(t *testing.T) {
	_, err := endpointFromConfig([]byte(`{"base_url":"https://user:pass@example.com/health"}`))
	require.Error(t, err)
	_, err = endpointFromConfig([]byte(`{"webhook_url":"https://example.com/health?token=hidden"}`))
	require.NoError(t, err)
	_, err = endpointFromConfig([]byte(`{"host":"db.example","port":5432}`))
	require.Error(t, err)
}

func TestCheckEndpointUsesEgressPolicy(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		require.Equal(t, http.MethodHead, r.Method)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	target, err := endpointFromConfig(mustJSON(t, map[string]any{"base_url": server.URL}))
	require.NoError(t, err)
	allowed, err := egress.ParsePolicy([]string{"127.0.0.1/32"})
	require.NoError(t, err)
	result := checkEndpoint(context.Background(), target, allowed, 1000)
	require.True(t, result.OK)
	require.Equal(t, "", result.ErrorCode)
	require.True(t, called)

	blocked, err := egress.ParsePolicy(nil)
	require.NoError(t, err)
	result = checkEndpoint(context.Background(), target, blocked, 1000)
	require.False(t, result.OK)
	require.Equal(t, "destination_blocked", result.ErrorCode)
	require.NotContains(t, result.Message, server.URL)
}

func TestTCPReachabilityIsGuardedByEgressPolicy(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })
	host, portText, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	port, err := strconv.Atoi(portText)
	require.NoError(t, err)
	config := mustJSON(t, map[string]any{"host": host, "port": port})
	address, err := tcpEndpointFromConfig(config)
	require.NoError(t, err)

	allowed, err := egress.ParsePolicy([]string{"127.0.0.1"})
	require.NoError(t, err)
	result := checkTCP(context.Background(), address, allowed, 1000)
	require.True(t, result.OK)

	blocked, err := egress.ParsePolicy(nil)
	require.NoError(t, err)
	result = checkTCP(context.Background(), address, blocked, 1000)
	require.False(t, result.OK)
	require.Equal(t, "destination_blocked", result.ErrorCode)
}

func TestEnvironmentAllowlistBuildsAnEnvironmentSpecificPolicy(t *testing.T) {
	development, err := parseEnvironmentAllowlist([]byte(`["10.10.0.0/16"]`))
	require.NoError(t, err)
	production, err := parseEnvironmentAllowlist([]byte(`["192.168.0.0/16"]`))
	require.NoError(t, err)

	require.True(t, development.Permits(netip.MustParseAddr("10.10.4.2")))
	require.False(t, development.Permits(netip.MustParseAddr("192.168.4.2")))
	require.True(t, production.Permits(netip.MustParseAddr("192.168.4.2")))
	require.False(t, production.Permits(netip.MustParseAddr("10.10.4.2")))
}

func TestEnvironmentAllowlistFailsClosed(t *testing.T) {
	_, err := parseEnvironmentAllowlist([]byte(`not-json`))
	require.Error(t, err)
	_, err = parseEnvironmentAllowlist([]byte(`null`))
	require.Error(t, err)
	_, err = parseEnvironmentAllowlist([]byte(`["not-a-cidr"]`))
	require.Error(t, err)
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	return encoded
}
