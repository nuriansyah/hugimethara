// Package api assembles the HTTP router for hugi-api: shared middleware plus
// the OpenAPI-generated strict server, implemented by per-domain handlers.
package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/nuriansyah/hugimethara/internal/modules/authorization"
	"github.com/nuriansyah/hugimethara/internal/platform/httpx"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
	"github.com/nuriansyah/hugimethara/internal/transport/http/apigen"
	"github.com/nuriansyah/hugimethara/internal/transport/http/handlers"
)

// MaxRequestBody is the default request body cap for API routes.
const MaxRequestBody = 1 << 20

type Deps = handlers.Deps

// NewRouter returns the root handler for the API listener.
func NewRouter(d Deps) (http.Handler, error) {
	public, err := PublicOperations()
	if err != nil {
		return nil, err
	}

	r := chi.NewRouter()
	r.Use(
		httpx.RequestID,
		httpx.Observe(httpx.NewHTTPMetrics(d.Registerer)),
		httpx.Recoverer,
		httpx.SecurityHeaders,
		httpx.CORS(d.TrustedOrigins),
		httpx.ClientInfoMiddleware(d.TrustedProxies),
		httpx.OriginGuard(d.TrustedOrigins),
		httpx.MaxBody(MaxRequestBody),
		d.Identity.Middleware,
	)
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		httpx.WriteError(w, req, httpx.NotFound("No route matches "+req.Method+" "+req.URL.Path+"."))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		httpx.WriteError(w, req, httpx.NewProblem(http.StatusMethodNotAllowed, "method_not_allowed",
			"Method not allowed", req.Method+" is not supported on this route."))
	})

	srv := handlers.NewServer(d)
	strict := apigen.NewStrictHandlerWithOptions(srv,
		[]apigen.StrictMiddlewareFunc{requireSession(public)},
		apigen.StrictHTTPServerOptions{
			RequestErrorHandlerFunc: func(w http.ResponseWriter, req *http.Request, err error) {
				httpx.WriteError(w, req, httpx.BadRequest("The request body could not be parsed as JSON of the expected shape."))
			},
			ResponseErrorHandlerFunc: func(w http.ResponseWriter, req *http.Request, err error) {
				httpx.WriteError(w, req, err)
			},
		})
	return apigen.HandlerWithOptions(strict, apigen.ChiServerOptions{
		BaseRouter: r,
		ErrorHandlerFunc: func(w http.ResponseWriter, req *http.Request, err error) {
			httpx.WriteError(w, req, httpx.BadRequest(err.Error()))
		},
	}), nil
}

// PublicOperations lists operation ids the spec marks `security: []`.
// Every other operation requires an authenticated principal (deny by default).
func PublicOperations() (map[string]bool, error) {
	spec, err := apigen.GetSpec()
	if err != nil {
		return nil, fmt.Errorf("api: load embedded spec: %w", err)
	}
	public := map[string]bool{}
	for _, item := range spec.Paths.Map() {
		for _, op := range item.Operations() {
			if op.Security != nil && len(*op.Security) == 0 {
				public[op.OperationID] = true
			}
		}
	}
	return public, nil
}

// requireSession rejects unauthenticated calls to non-public operations. The
// generated code passes Go-cased operation names ("Login" for spec "login"),
// so matching is case-insensitive.
func requireSession(public map[string]bool) apigen.StrictMiddlewareFunc {
	folded := make(map[string]bool, len(public))
	for id := range public {
		folded[strings.ToLower(id)] = true
	}
	return func(f apigen.StrictHandlerFunc, operationID string) apigen.StrictHandlerFunc {
		if folded[strings.ToLower(operationID)] {
			return f
		}
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, req any) (any, error) {
			if _, ok := principal.From(ctx); !ok {
				return nil, authz.Unauthenticated()
			}
			return f(ctx, w, r, req)
		}
	}
}
