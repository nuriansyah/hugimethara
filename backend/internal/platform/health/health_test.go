package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReady(t *testing.T) {
	h := New(0)
	h.Add("postgres", func(context.Context) error { return nil })
	rec := httptest.NewRecorder()
	h.Ready(rec, httptest.NewRequest("GET", "/readyz", nil))
	require.Equal(t, 200, rec.Code)

	h.Add("nats", func(context.Context) error { return errors.New("not connected") })
	rec = httptest.NewRecorder()
	h.Ready(rec, httptest.NewRequest("GET", "/readyz", nil))
	require.Equal(t, 503, rec.Code)

	var rep Report
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rep))
	require.Equal(t, "fail", rep.Status)
	require.Equal(t, "ok", rep.Checks["postgres"].Status)
	require.Equal(t, "not connected", rep.Checks["nats"].Error)
}

func TestLive(t *testing.T) {
	rec := httptest.NewRecorder()
	New(0).Live(rec, httptest.NewRequest("GET", "/healthz", nil))
	require.Equal(t, 200, rec.Code)
}
