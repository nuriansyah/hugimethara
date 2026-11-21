package services

import (
	"context"

	"github.com/google/uuid"

	secretsmodel "github.com/nuriansyah/hugimethara/internal/modules/integration/secrets/model"
	"github.com/nuriansyah/hugimethara/internal/modules/integration/secrets/model/dto"
)

// Code shape: transport adapters and other modules depend on these use-case
// interfaces (ports), never on SecretsService directly (DIP); each interface
// groups one capability so consumers can depend on the narrowest one (ISP).

// Service is the complete use-case surface of the module.
type Service interface {
	Create(ctx context.Context, in dto.CreateInput) (secretsmodel.Credential, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (secretsmodel.Credential, error)
	List(ctx context.Context) ([]secretsmodel.Credential, error)
	// Resolve decrypts a credential for a trusted data-plane operation. Callers
	// must zero the returned buffer as soon as the connector has finished.
	Resolve(ctx context.Context, id uuid.UUID) ([]byte, error)
	Update(ctx context.Context, id uuid.UUID, in dto.UpdateInput) (secretsmodel.Credential, error)
}

var _ Service = (*SecretsService)(nil)
