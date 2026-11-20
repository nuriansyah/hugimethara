package handlers

import (
	"context"
	"encoding/json"
	"net/netip"

	authz "github.com/nuriansyah/hugimethara/internal/modules/authorization"
	"github.com/nuriansyah/hugimethara/internal/modules/governance/audit"
	"github.com/nuriansyah/hugimethara/internal/modules/governance/entitlement"
	"github.com/nuriansyah/hugimethara/internal/modules/identity"
	"github.com/nuriansyah/hugimethara/internal/modules/integration/connector"
	"github.com/nuriansyah/hugimethara/internal/modules/integration/mapping"
	secretvault "github.com/nuriansyah/hugimethara/internal/modules/integration/secrets"
	"github.com/nuriansyah/hugimethara/internal/modules/integration/service_registry"
	"github.com/nuriansyah/hugimethara/internal/modules/organization"
	"github.com/nuriansyah/hugimethara/internal/platform/buildinfo"
	"github.com/nuriansyah/hugimethara/internal/platform/health"
	"github.com/nuriansyah/hugimethara/internal/transport/http/apigen"
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
}

func NewServer(d Deps) apigen.StrictServerInterface {
	return &server{
		system: system{env: d.Env, health: d.Health},
		HTTP:   d.Identity, orgHTTP: orgHTTP{d.Organization}, auditHTTP: auditHTTP{d.Audit},
		entHTTP: entHTTP{d.Entitlement}, accessHTTP: accessHTTP{d.Access},
		secretHTTP:    secretHTTP{d.Secrets},
		connectorHTTP: connectorHTTP{d.Connectors},
		serviceHTTP:   serviceHTTP{d.Services},
		mappingHTTP:   mappingHTTP{d.Mappings},
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
}

type orgHTTP struct{ *organization.HTTP }
type auditHTTP struct{ *audit.HTTP }
type entHTTP struct{ *entitlement.HTTP }
type accessHTTP struct{ *authz.HTTP }
type secretHTTP struct{ *secretvault.HTTP }
type connectorHTTP struct{ *connector.HTTP }
type serviceHTTP struct{ *servicereg.HTTP }
type mappingHTTP struct{ *mapping.HTTP }

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

func toAPIReport(r health.Report) (apigen.HealthReport, error) {
	var out apigen.HealthReport
	b, err := json.Marshal(r)
	if err != nil {
		return out, err
	}
	return out, json.Unmarshal(b, &out)
}
