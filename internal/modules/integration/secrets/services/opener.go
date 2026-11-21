package services

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	secretsmodel "github.com/nuriansyah/hugimethara/internal/modules/integration/secrets/model"
	"github.com/nuriansyah/hugimethara/internal/modules/integration/secrets/repository"
	"github.com/nuriansyah/hugimethara/internal/modules/integration/secrets/repository/queries"
	secretbox "github.com/nuriansyah/hugimethara/internal/platform/crypto"
)

// Opener decrypts credentials for trusted data-plane code that already holds
// a tenant-bound transaction, such as the workflow engine resolving the
// credential of a connector instance a published workflow references. It
// performs no principal authorization: the caller vouches that the tenant's
// own configuration references the credential. Callers zero the returned
// buffer as soon as the call that needed it has finished.
type Opener struct {
	master *secretbox.Key
	kekID  string
}

// NewOpener builds an Opener for the configured master key.
func NewOpener(masterKey []byte, kekID string) (*Opener, error) {
	master, err := secretbox.New(masterKey)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(kekID) == "" {
		return nil, errors.New("secret: KEK id is required")
	}
	return &Opener{master: master, kekID: kekID}, nil
}

// Open decrypts credential id of org inside tx (bound to org).
func (o *Opener) Open(ctx context.Context, tx *sqlx.Tx, org, id uuid.UUID) ([]byte, error) {
	var row struct {
		DEKID      uuid.UUID `db:"dek_id"`
		Ciphertext []byte    `db:"ciphertext"`
	}
	if err := tx.GetContext(ctx, &row, queries.SQL.Get("OpenCredential"), id, org); err != nil {
		return nil, secretsmodel.TranslateNotFound(err)
	}
	var key repository.DataKeyRow
	if err := tx.GetContext(ctx, &key, queries.SQL.Get("GetDataKey"), org); err != nil {
		return nil, err
	}
	if key.ID != row.DEKID {
		return nil, errors.New("secret: credential references an unknown data key")
	}
	dek, err := unwrapDEK(o.master, o.kekID, org, key)
	if err != nil {
		return nil, err
	}
	defer secretsmodel.Zero(dek)
	return openValue(dek, org, id, row.Ciphertext)
}
