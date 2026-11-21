package services

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/nuriansyah/hugimethara/internal/infrastructure/security/egress"
)

func loopback(t *testing.T) egress.Policy {
	t.Helper()
	p, err := egress.ParsePolicy([]string{"127.0.0.1/32"})
	require.NoError(t, err)
	return p
}

func toolBinding(key, url string) Binding {
	return Binding{RefKey: key + "-prod", Key: key, Operations: []string{OperationTriggerWorkflow},
		Config: map[string]any{"webhook_url": url}, AuthMethod: "none", TimeoutMs: 2000, Enabled: true}
}

func invokeCode(t *testing.T, err error) (string, bool) {
	t.Helper()
	var ie *InvokeError
	require.True(t, errors.As(err, &ie), "expected an InvokeError, got %v", err)
	return ie.Code, ie.Retryable
}

func TestTriggerWorkflowPostsInputWithAuthAndCorrelation(t *testing.T) {
	var got struct {
		header http.Header
		body   map[string]any
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		got.header = r.Header.Clone()
		raw, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(raw, &got.body))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"accepted":true,"run_id":"r-1"}`))
	}))
	defer srv.Close()

	rt := NewRuntime(nil)
	for key, header := range map[string]string{"n8n": "Authorization", "make": "X-Make-Apikey"} {
		b := toolBinding(key, srv.URL+"/webhook/lead")
		b.AuthMethod, b.Credential = "header", []byte("Bearer s3cret")
		res, err := rt.Invoke(context.Background(), b, InvokeRequest{Operation: OperationTriggerWorkflow,
			Input: map[string]any{"lead": "ada@example.com"}, IdempotencyKey: "exec:node::1",
			Headers: map[string]string{"X-Hugi-Execution-Id": "exec"}, Egress: loopback(t)})
		require.NoError(t, err)
		require.Equal(t, 200, res.Status)
		require.Equal(t, map[string]any{"accepted": true, "run_id": "r-1"}, res.Body)
		require.Equal(t, "Bearer s3cret", got.header.Get(header))
		require.Equal(t, "exec:node::1", got.header.Get("Idempotency-Key"))
		require.Equal(t, "exec", got.header.Get("X-Hugi-Execution-Id"))
		require.Equal(t, "ada@example.com", got.body["lead"])
	}

	// A custom header name overrides the tool default.
	b := toolBinding("n8n", srv.URL)
	b.AuthMethod, b.Credential, b.Config["auth_header"] = "header", []byte("tok"), "X-N8N-Key"
	_, err := rt.Invoke(context.Background(), b, InvokeRequest{Operation: OperationTriggerWorkflow, Egress: loopback(t)})
	require.NoError(t, err)
	require.Equal(t, "tok", got.header.Get("X-N8N-Key"))
	require.Empty(t, got.header.Get("Authorization"))

	// Header auth without a credential is a configuration error, not a call.
	b.Credential = nil
	_, err = rt.Invoke(context.Background(), b, InvokeRequest{Operation: OperationTriggerWorkflow, Egress: loopback(t)})
	code, retry := invokeCode(t, err)
	require.Equal(t, CodeMisconfigured, code)
	require.False(t, retry)

	b.Credential = []byte("x")
	b.Release()
	require.Nil(t, b.Credential)
}

func TestTriggerWorkflowWhenTheToolIsUnreachable(t *testing.T) {
	// A port that was just closed: connection refused.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())

	rt := NewRuntime(nil)
	_, err = rt.Invoke(context.Background(), toolBinding("zapier", "http://"+addr+"/hooks/catch/1/abc"),
		InvokeRequest{Operation: OperationTriggerWorkflow, Egress: loopback(t)})
	code, retry := invokeCode(t, err)
	require.Equal(t, CodeUnavailable, code)
	require.True(t, retry)
	require.NotContains(t, err.Error(), addr, "messages never carry the webhook URL")

	// A tool that accepts the connection but never answers: the instance timeout applies.
	done := make(chan struct{})
	hang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-done:
		}
	}))
	defer hang.Close()
	defer close(done)
	b := toolBinding("make", hang.URL)
	b.TimeoutMs = 150
	started := time.Now()
	_, err = rt.Invoke(context.Background(), b, InvokeRequest{Operation: OperationTriggerWorkflow, Egress: loopback(t)})
	code, retry = invokeCode(t, err)
	require.Equal(t, CodeTimeout, code)
	require.True(t, retry)
	require.Less(t, time.Since(started), 2*time.Second)
}

func TestTriggerWorkflowClassifiesUpstreamAnswersAndGuards(t *testing.T) {
	status := http.StatusInternalServerError
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte("nope"))
	}))
	defer srv.Close()
	rt := NewRuntime(nil)
	call := func(b Binding, op string, p egress.Policy) (InvokeResult, error) {
		return rt.Invoke(context.Background(), b, InvokeRequest{Operation: op, Egress: p})
	}

	res, err := call(toolBinding("n8n", srv.URL), OperationTriggerWorkflow, loopback(t))
	code, retry := invokeCode(t, err)
	require.Equal(t, CodeUpstreamError, code)
	require.True(t, retry)
	require.Equal(t, 500, res.Status)
	require.Equal(t, "nope", res.Body)

	status = http.StatusTooManyRequests
	_, err = call(toolBinding("n8n", srv.URL), OperationTriggerWorkflow, loopback(t))
	code, retry = invokeCode(t, err)
	require.Equal(t, CodeUpstreamError, code)
	require.True(t, retry)

	status = http.StatusNotFound // e.g. an n8n workflow that is not active
	_, err = call(toolBinding("n8n", srv.URL), OperationTriggerWorkflow, loopback(t))
	code, retry = invokeCode(t, err)
	require.Equal(t, CodeUpstreamReject, code)
	require.False(t, retry)

	blocked, err := egress.ParsePolicy(nil)
	require.NoError(t, err)
	_, err = call(toolBinding("n8n", srv.URL), OperationTriggerWorkflow, blocked)
	code, retry = invokeCode(t, err)
	require.Equal(t, CodeEgressBlocked, code)
	require.False(t, retry)

	disabled := toolBinding("n8n", srv.URL)
	disabled.Enabled = false
	_, err = call(disabled, OperationTriggerWorkflow, loopback(t))
	code, _ = invokeCode(t, err)
	require.Equal(t, CodeDisabled, code)

	_, err = call(toolBinding("n8n", srv.URL), "delete_everything", loopback(t))
	code, _ = invokeCode(t, err)
	require.Equal(t, CodeUnsupported, code)

	slack := toolBinding("slack", srv.URL)
	_, err = call(slack, OperationTriggerWorkflow, loopback(t))
	code, _ = invokeCode(t, err)
	require.Equal(t, CodeUnsupported, code)

	bad := toolBinding("zapier", "ftp://example.com/x")
	_, err = call(bad, OperationTriggerWorkflow, loopback(t))
	code, _ = invokeCode(t, err)
	require.Equal(t, CodeMisconfigured, code)
}

func TestSupportsOnlyTheExternalToolOperations(t *testing.T) {
	for _, k := range InvocableDefinitions() {
		require.True(t, Supports(k, OperationTriggerWorkflow))
	}
	require.False(t, Supports("http_rest", "request"))
	require.Equal(t, []string{"a", "b"}, OperationKeys([]byte(`[{"key":"a"},{"name":"x"},{"key":"b"}]`)))
	require.Empty(t, OperationKeys([]byte(`not json`)))
}
