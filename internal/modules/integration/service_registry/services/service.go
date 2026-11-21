package services

import (
	"context"

	"github.com/google/uuid"

	serviceregmodel "github.com/nuriansyah/hugimethara/internal/modules/integration/service_registry/model"
	serviceregdto "github.com/nuriansyah/hugimethara/internal/modules/integration/service_registry/model/dto"
)

// Code shape: transport adapters and other modules depend on these use-case
// interfaces (ports), never on ServiceRegistryService directly (DIP); each interface
// groups one capability so consumers can depend on the narrowest one (ISP).

// Service is the complete use-case surface of the module.
type Service interface {
	Create(ctx context.Context, in serviceregdto.CreateInput) (serviceregmodel.RegisteredService, error)
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteEnvironment(ctx context.Context, serviceID uuid.UUID, environmentID uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (serviceregmodel.RegisteredService, error)
	ImportOpenAPI(ctx context.Context, serviceID uuid.UUID, in serviceregdto.ImportInput) (serviceregmodel.ImportResult, error)
	List(ctx context.Context, workspaceID uuid.UUID) ([]serviceregmodel.RegisteredService, error)
	ListEndpoints(ctx context.Context, serviceID uuid.UUID) ([]serviceregmodel.Endpoint, error)
	PutEnvironment(ctx context.Context, serviceID uuid.UUID, environmentID uuid.UUID, baseURL string, connectorID *uuid.UUID) (serviceregmodel.Environment, error)
	TryEndpoint(ctx context.Context, endpointID uuid.UUID, in serviceregdto.ServiceTryInput) (serviceregmodel.ServiceTryResult, error)
	Update(ctx context.Context, id uuid.UUID, in serviceregdto.UpdateInput) (serviceregmodel.RegisteredService, error)
}

var _ Service = (*ServiceRegistryService)(nil)
