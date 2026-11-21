package services

import (
	"context"

	"github.com/google/uuid"

	compositionmodel "github.com/nuriansyah/hugimethara/internal/modules/api_composition/model"
	compositiondto "github.com/nuriansyah/hugimethara/internal/modules/api_composition/model/dto"
)

// Code shape: transport adapters and other modules depend on these use-case
// interfaces (ports), never on CompositionService directly (DIP); each interface
// groups one capability so consumers can depend on the narrowest one (ISP).

// Service is the complete use-case surface of the module.
type Service interface {
	Create(ctx context.Context, in compositiondto.CreateInput) (compositionmodel.Composition, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Execute(ctx context.Context, id uuid.UUID, in compositiondto.ExecuteInput) (compositionmodel.Execution, error)
	Get(ctx context.Context, id uuid.UUID) (compositionmodel.Composition, error)
	GetExecution(ctx context.Context, id uuid.UUID) (compositionmodel.Execution, error)
	List(ctx context.Context, workspaceID uuid.UUID) ([]compositionmodel.Composition, error)
	ListExecutions(ctx context.Context, workspaceID uuid.UUID, compositionID *uuid.UUID) ([]compositionmodel.Execution, error)
	ListVersions(ctx context.Context, id uuid.UUID) ([]compositionmodel.Version, error)
	Publish(ctx context.Context, id uuid.UUID) (compositionmodel.Version, error)
	Update(ctx context.Context, id uuid.UUID, in compositiondto.UpdateInput) (compositionmodel.Composition, error)
	Validate(ctx context.Context, id uuid.UUID) error
}

var _ Service = (*CompositionService)(nil)
