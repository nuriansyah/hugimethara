package handlers

import (
	"context"
	"encoding/json"
	"net/netip"

	authz "github.com/nuriansyah/hugimethara/internal/modules/authorization"
	"github.com/nuriansyah/hugimethara/internal/modules/governance/audit"
	"github.com/nuriansyah/hugimethara/internal/modules/governance/entitlement"
	"github.com/nuriansyah/hugimethara/internal/modules/identity"
	"github.com/nuriansyah/hugimethara/internal/modules/integration/composition"
	"github.com/nuriansyah/hugimethara/internal/modules/integration/connector"
	"github.com/nuriansyah/hugimethara/internal/modules/integration/mapping"
	secretvault "github.com/nuriansyah/hugimethara/internal/modules/integration/secrets"
	"github.com/nuriansyah/hugimethara/internal/modules/integration/service_registry"
	"github.com/nuriansyah/hugimethara/internal/modules/organization"
	"github.com/nuriansyah/hugimethara/internal/platform/buildinfo"
	"github.com/nuriansyah/hugimethara/internal/platform/health"
	"github.com/nuriansyah/hugimethara/internal/transport/http/dto"
	"github.com/prometheus/client_golang/prometheus"
)

type Deps struct {
	Env            string
	TrustedOrigins []string
	TrustedProxies []netip.Prefix
	Health         *health.Handler
	Registerer     prometheus.Registerer

	Identity     *identity.HTTP
	Organization *organization.HTTP
	Audit        *audit.HTTP
	Entitlement  *entitlement.HTTP
	Access       *authz.HTTP
	Secrets      *secretvault.HTTP
	Connectors   *connector.HTTP
	Services     *servicereg.HTTP
	Mappings     *mapping.HTTP
	Compositions *composition.HTTP
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
	}
}

type server struct {
	system
	*identity.HTTP
	orgHTTP
	auditHTTP
	entHTTP
	accessHTTP
	secretHTTP
	connectorHTTP
	serviceHTTP
	mappingHTTP
	compositionHTTP
}

type orgHTTP struct{ *organization.HTTP }
type auditHTTP struct{ *audit.HTTP }
type entHTTP struct{ *entitlement.HTTP }
type accessHTTP struct{ *authz.HTTP }
type secretHTTP struct{ *secretvault.HTTP }
type connectorHTTP struct{ *connector.HTTP }
type serviceHTTP struct{ *servicereg.HTTP }
type mappingHTTP struct{ *mapping.HTTP }
type compositionHTTP struct{ *composition.HTTP }

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
