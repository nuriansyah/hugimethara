package services

import (
	"context"

	"github.com/google/uuid"

	mappingmodel "github.com/nuriansyah/hugimethara/internal/modules/integration/mapping/model"
	"github.com/nuriansyah/hugimethara/internal/modules/integration/mapping/model/dto"
)

// Code shape: transport adapters and other modules depend on these use-case
// interfaces (ports), never on MappingService directly (DIP); each interface
// groups one capability so consumers can depend on the narrowest one (ISP).

// Service is the complete use-case surface of the module.
type Service interface {
	Create(ctx context.Context, in dto.CreateInput) (mappingmodel.Mapping, error)
	CreateTest(ctx context.Context, id uuid.UUID, in dto.TestCreateInput) (mappingmodel.MappingTest, error)
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteTest(ctx context.Context, mappingID uuid.UUID, testID uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (mappingmodel.Mapping, error)
	List(ctx context.Context, workspaceID uuid.UUID) ([]mappingmodel.Mapping, error)
	ListTests(ctx context.Context, id uuid.UUID) ([]mappingmodel.MappingTest, error)
	ListVersions(ctx context.Context, id uuid.UUID) ([]mappingmodel.MappingVersion, error)
	Publish(ctx context.Context, id uuid.UUID) (mappingmodel.MappingVersion, error)
	Test(ctx context.Context, id uuid.UUID, input any) (mappingmodel.TestResult, error)
	Update(ctx context.Context, id uuid.UUID, in dto.UpdateInput) (mappingmodel.Mapping, error)
	UpdateTest(ctx context.Context, mappingID uuid.UUID, testID uuid.UUID, in dto.TestUpdateInput) (mappingmodel.MappingTest, error)
}

var _ Service = (*MappingService)(nil)
