package handlers

import (
	"context"
	"encoding/json"
	"net/netip"

	webhookhandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/webhook"
	workflowhandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/workflow"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/nuriansyah/hugimethara/internal/infrastructure/observability/health"
	"github.com/nuriansyah/hugimethara/internal/platform/config/buildinfo"
	"github.com/nuriansyah/hugimethara/internal/transport/http/dto"
	compositionhandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/api_composition"
	audithandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/audit"
	authzhandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/authorization"
	connectorhandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/connector"
	enthandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/entitlement"
	identityhandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/identity"
	mappinghandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/mapping"
	orghandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/organization"
	secretshandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/secrets"
	servicereghandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/service_registry"
)

type Deps struct {
	Env            string
	TrustedOrigins []string
	TrustedProxies []netip.Prefix
	Health         *health.Handler
	Registerer     prometheus.Registerer

	Identity     *identityhandler.HTTP
	Organization *orghandler.HTTP
	Audit        *audithandler.HTTP
	Entitlement  *enthandler.HTTP
	Access       *authzhandler.HTTP
	Secrets      *secretshandler.HTTP
	Connectors   *connectorhandler.HTTP
	Services     *servicereghandler.HTTP
	Mappings     *mappinghandler.HTTP
	Compositions *compositionhandler.HTTP
	Workflows    *workflowhandler.HTTP
	Webhooks     *webhookhandler.HTTP
}

func NewServer(d Deps) any {
	return &server{
		system: system{env: d.Env, health: d.Health},
		HTTP:   d.Identity, orgHTTP: orgHTTP{d.Organization}, auditHTTP: auditHTTP{d.Audit},
		entHTTP: entHTTP{d.Entitlement}, accessHTTP: accessHTTP{d.Access},
		secretHTTP:      secretHTTP{d.Secrets},
		connectorHTTP:   connectorHTTP{d.Connectors},
		serviceHTTP:     serviceHTTP{d.Services},
		mappingHTTP:     mappingHTTP{d.Mappings},
		compositionHTTP: compositionHTTP{d.Compositions},
		workflowHTTP:    workflowHTTP{d.Workflows},
		webhookHTTP:     webhookHTTP{d.Webhooks},
	}
}

type server struct {
	system
	*identityhandler.HTTP
	orgHTTP
	auditHTTP
	entHTTP
	accessHTTP
	secretHTTP
	connectorHTTP
	serviceHTTP
	mappingHTTP
	compositionHTTP
	workflowHTTP
	webhookHTTP
}

type orgHTTP struct{ *orghandler.HTTP }
type auditHTTP struct{ *audithandler.HTTP }
type entHTTP struct{ *enthandler.HTTP }
type accessHTTP struct{ *authzhandler.HTTP }
type secretHTTP struct{ *secretshandler.HTTP }
type connectorHTTP struct{ *connectorhandler.HTTP }
type serviceHTTP struct{ *servicereghandler.HTTP }
type mappingHTTP struct{ *mappinghandler.HTTP }
type compositionHTTP struct{ *compositionhandler.HTTP }
type workflowHTTP struct{ *workflowhandler.HTTP }
type webhookHTTP struct{ *webhookhandler.HTTP }

type system struct {
	env    string
	health *health.Handler
}

// @Summary Liveness probe
// @ID getLiveness
// @Tags system
// @Success 200 {object} dto.HealthReport "Process is alive."
// @Produce json
// @Router /healthz [get]
func (s system) GetLiveness(context.Context, dto.GetLivenessRequestObject) (dto.GetLivenessResponseObject, error) {
	return dto.GetLiveness200JSONResponse{Status: dto.HealthReportStatusOk}, nil
}

// @Summary Readiness probe (dependencies and migration version)
// @ID getReadiness
// @Tags system
// @Success 200 {object} dto.HealthReport "All dependencies ready."
// @Failure 503 {object} dto.HealthReport "At least one dependency is not ready."
// @Produce json
// @Router /readyz [get]
func (s system) GetReadiness(ctx context.Context, _ dto.GetReadinessRequestObject) (dto.GetReadinessResponseObject, error) {
	rep := s.health.Evaluate(ctx)
	out, err := toAPIReport(rep)
	if err != nil {
		return nil, err
	}
	if !rep.OK() {
		return dto.GetReadiness503JSONResponse(out), nil
	}
	return dto.GetReadiness200JSONResponse(out), nil
}

// @Summary Build and deployment information
// @ID getSystemInfo
// @Tags system
// @Success 200 {object} dto.SystemInfo "Build and deployment information."
// @Produce json
// @Router /v1/hugimethara/system/info [get]
func (s system) GetSystemInfo(context.Context, dto.GetSystemInfoRequestObject) (dto.GetSystemInfoResponseObject, error) {
	return dto.GetSystemInfo200JSONResponse{
		Name:        "Hugimethara",
		Version:     buildinfo.Version,
		Commit:      buildinfo.Commit,
		GoVersion:   buildinfo.GoVersion(),
		Environment: dto.SystemInfoEnvironment(s.env),
		ApiVersion:  "v1",
	}, nil
}

func toAPIReport(r health.Report) (dto.HealthReport, error) {
	var out dto.HealthReport
	b, err := json.Marshal(r)
	if err != nil {
		return out, err
	}
	return out, json.Unmarshal(b, &out)
}
