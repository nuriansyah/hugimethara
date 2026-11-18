// Package api assembles the HTTP router for hugi-api: shared middleware plus
// the OpenAPI-generated strict server, implemented by per-domain handlers.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/nuriansyah/hugimethara/backend/internal/api/apigen"
	"github.com/nuriansyah/hugimethara/backend/internal/audit"
	"github.com/nuriansyah/hugimethara/backend/internal/authz"
	"github.com/nuriansyah/hugimethara/backend/internal/identity"
	"github.com/nuriansyah/hugimethara/backend/internal/organization"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/buildinfo"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/health"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/httpx"
	"github.com/nuriansyah/hugimethara/backend/internal/principal"
)

// MaxRequestBody is the default request body cap for API routes.
const MaxRequestBody = 1 << 20

// Deps are the collaborators the router needs.
type Deps struct {
	Env            string
	TrustedOrigins []string // browser origins allowed for CORS and state-changing requests
	TrustedProxies []netip.Prefix
	Health         *health.Handler
	Registerer     prometheus.Registerer

	Identity     *identity.HTTP
	Organization *organization.HTTP
	Audit        *audit.HTTP
}

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

	srv := &server{
		system: system{env: d.Env, health: d.Health},
		HTTP:   d.Identity, orgHTTP: orgHTTP{d.Organization}, auditHTTP: auditHTTP{d.Audit},
	}
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

// server implements apigen.StrictServerInterface by embedding one handler per
// domain; each domain owns its slice of the interface.
type server struct {
	system
	*identity.HTTP
	orgHTTP
	auditHTTP
}

type orgHTTP struct{ *organization.HTTP }
type auditHTTP struct{ *audit.HTTP }

var _ apigen.StrictServerInterface = (*server)(nil)

type system struct {
	env    string
	health *health.Handler
}

func (s system) GetLiveness(context.Context, apigen.GetLivenessRequestObject) (apigen.GetLivenessResponseObject, error) {
	return apigen.GetLiveness200JSONResponse{Status: apigen.HealthReportStatusOk}, nil
}

func (s system) GetReadiness(ctx context.Context, _ apigen.GetReadinessRequestObject) (apigen.GetReadinessResponseObject, error) {
	rep := s.health.Evaluate(ctx)
	out, err := toAPIReport(rep)
	if err != nil {
		return nil, err
	}
	if !rep.OK() {
		return apigen.GetReadiness503JSONResponse(out), nil
	}
	return apigen.GetReadiness200JSONResponse(out), nil
}

func (s system) GetSystemInfo(context.Context, apigen.GetSystemInfoRequestObject) (apigen.GetSystemInfoResponseObject, error) {
	return apigen.GetSystemInfo200JSONResponse{
		Name:        "Hugimethara",
		Version:     buildinfo.Version,
		Commit:      buildinfo.Commit,
		GoVersion:   buildinfo.GoVersion(),
		Environment: apigen.SystemInfoEnvironment(s.env),
		ApiVersion:  "v1",
	}, nil
}

// toAPIReport converts via JSON: both types share the documented wire shape.
func toAPIReport(r health.Report) (apigen.HealthReport, error) {
	var out apigen.HealthReport
	b, err := json.Marshal(r)
	if err != nil {
		return out, err
	}
	return out, json.Unmarshal(b, &out)
}
