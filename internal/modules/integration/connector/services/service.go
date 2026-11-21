package services

import (
	"context"

	"github.com/google/uuid"

	connectormodel "github.com/nuriansyah/hugimethara/internal/modules/integration/connector/model"
	connectordto "github.com/nuriansyah/hugimethara/internal/modules/integration/connector/model/dto"
)

// Code shape: transport adapters and other modules depend on these use-case
// interfaces (ports), never on ConnectorService directly (DIP); each interface
// groups one capability so consumers can depend on the narrowest one (ISP).

// Service is the complete use-case surface of the module.
type Service interface {
	Create(ctx context.Context, in connectordto.CreateInput) (connectormodel.Instance, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (connectormodel.Instance, error)
	Health(ctx context.Context, id uuid.UUID) ([]connectormodel.Health, error)
	List(ctx context.Context) ([]connectormodel.Instance, error)
	ListDefinitions(ctx context.Context) ([]connectormodel.Definition, error)
	ListVersions(ctx context.Context, definitionID uuid.UUID) ([]connectormodel.Version, error)
	Test(ctx context.Context, id uuid.UUID) (connectormodel.TestResult, error)
	Update(ctx context.Context, id uuid.UUID, in connectordto.UpdateInput) (connectormodel.Instance, error)
}

var _ Service = (*ConnectorService)(nil)
