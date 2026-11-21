package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/platform/pagination"
)

func newRouter(t *testing.T) (*chi.Mux, *HTTPMetrics) {
	t.Helper()
	m := NewHTTPMetrics(prometheus.NewRegistry())
	r := chi.NewRouter()
	r.Use(RequestID, Observe(m), Recoverer)
	return r, m
}

func TestRequestIDGeneratedAndEchoed(t *testing.T) {
	r, _ := newRouter(t)
	r.Get("/x", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	require.Len(t, rec.Header().Get(RequestIDHeader), 36)

	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set(RequestIDHeader, "client-supplied-id-123")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, "client-supplied-id-123", rec.Header().Get(RequestIDHeader))

	req = httptest.NewRequest("GET", "/x", nil)
	req.Header.Set(RequestIDHeader, "bad id\nwith newline")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.NotContains(t, rec.Header().Get(RequestIDHeader), "bad")
}

func TestUnknownErrorIsOpaque500WithRequestID(t *testing.T) {
	r, _ := newRouter(t)
	r.Get("/boom", func(w http.ResponseWriter, req *http.Request) {
		problem.WriteError(w, req, errors.New("pq: password authentication failed for user hugi"))
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/boom", nil))

	require.Equal(t, 500, rec.Code)
	require.Equal(t, "application/problem+json", rec.Header().Get("Content-Type"))
	require.NotContains(t, rec.Body.String(), "password")
	var p problem.Problem
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &p))
	require.Equal(t, "internal_error", p.Code)
	require.Equal(t, rec.Header().Get(RequestIDHeader), p.RequestID)
}

func TestPanicRecoveredAndMetricsUseRoutePattern(t *testing.T) {
	r, m := newRouter(t)
	r.Get("/items/{id}", func(http.ResponseWriter, *http.Request) { panic("kaboom") })
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/items/42", nil))

	require.Equal(t, 500, rec.Code)
	require.Equal(t, 1.0, testutil.ToFloat64(m.requests.WithLabelValues("/items/{id}", "GET", "500")))
}

func TestValidationProblem(t *testing.T) {
	rec := httptest.NewRecorder()
	problem.WriteError(rec, httptest.NewRequest("GET", "/", nil),
		problem.Validation(problem.FieldError{Field: "name", Code: "required", Message: "Name is required."}))
	require.Equal(t, 422, rec.Code)
	var p problem.Problem
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &p))
	require.Equal(t, "https://docs.hugimethara.dev/errors/validation_failed", p.Type)
	require.Equal(t, "name", p.Errors[0].Field)
}

func TestCORSAllowlist(t *testing.T) {
	h := CORS([]string{"http://localhost:3000"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))

	pre := httptest.NewRequest("OPTIONS", "/x", nil)
	pre.Header.Set("Origin", "http://localhost:3000")
	pre.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, pre)
	require.Equal(t, 204, rec.Code)
	require.Equal(t, "http://localhost:3000", rec.Header().Get("Access-Control-Allow-Origin"))

	evil := httptest.NewRequest("GET", "/x", nil)
	evil.Header.Set("Origin", "https://evil.example")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, evil)
	require.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestParsePageAndSort(t *testing.T) {
	p, err := pagination.ParsePage(httptest.NewRequest("GET", "/?limit=10&cursor=abc", nil))
	require.NoError(t, err)
	require.Equal(t, pagination.Page{Limit: 10, Cursor: "abc"}, p)

	p, err = pagination.ParsePage(httptest.NewRequest("GET", "/", nil))
	require.NoError(t, err)
	require.Equal(t, pagination.DefaultPageLimit, p.Limit)

	for _, q := range []string{"0", "201", "x"} {
		_, err = pagination.ParsePage(httptest.NewRequest("GET", "/?limit="+q, nil))
		require.Error(t, err, q)
	}

	s, err := pagination.ParseSort(httptest.NewRequest("GET", "/?sort=-created_at,name", nil), "created_at", "name")
	require.NoError(t, err)
	require.Equal(t, []pagination.SortField{{Field: "created_at", Desc: true}, {Field: "name", Desc: false}}, s)
	_, err = pagination.ParseSort(httptest.NewRequest("GET", "/?sort=password", nil), "name")
	require.Error(t, err)

	type pos struct{ ID string }
	c, err := pagination.EncodeCursor(pos{ID: "a"})
	require.NoError(t, err)
	var out pos
	require.NoError(t, pagination.DecodeCursor(c, &out))
	require.Equal(t, "a", out.ID)
	require.Error(t, pagination.DecodeCursor("!!", &out))
}
