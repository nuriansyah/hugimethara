package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedactsSensitiveKeysAndAddsRequestID(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, "debug", "json", "svc", "test")
	ctx := WithRequestID(context.Background(), "req-1")

	log.InfoContext(ctx, "hello",
		"client_secret", "abc", "Authorization", "Bearer x", "db_password", "p", "user", "alice")

	var rec map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &rec))
	require.Equal(t, Redacted, rec["client_secret"])
	require.Equal(t, Redacted, rec["Authorization"])
	require.Equal(t, Redacted, rec["db_password"])
	require.Equal(t, "alice", rec["user"])
	require.Equal(t, "req-1", rec["request_id"])
	require.Equal(t, "svc", rec["service"])
	require.NotContains(t, buf.String(), "Bearer x")
}

func TestRedactsInsideGroups(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, "info", "json", "svc", "test")
	log.Info("x", "conn", map[string]string{"k": "v"})
	log.WithGroup("cfg").Info("y", "api_key", "k1")
	require.NotContains(t, buf.String(), "k1")
}
