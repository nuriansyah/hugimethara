package repository

import (
	"github.com/google/uuid"

	"github.com/nuriansyah/hugimethara/internal/modules/integration/secrets/model"
)

type CredentialRow struct {
	model.Credential
	Ciphertext []byte    `db:"ciphertext"`
	DEKID      uuid.UUID `db:"dek_id"`
}

type DataKeyRow struct {
	ID      uuid.UUID `db:"id"`
	KEKID   string    `db:"kek_id"`
	Wrapped []byte    `db:"wrapped_key"`
}
