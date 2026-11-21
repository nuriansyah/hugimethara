package services

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	authzmodel "github.com/nuriansyah/hugimethara/internal/modules/authorization/model"
	authzsvc "github.com/nuriansyah/hugimethara/internal/modules/authorization/services"
	auditmodel "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/model"
	auditsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/services"
	secretsmodel "github.com/nuriansyah/hugimethara/internal/modules/integration/secrets/model"
	"github.com/nuriansyah/hugimethara/internal/modules/integration/secrets/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/integration/secrets/repository"
	"github.com/nuriansyah/hugimethara/internal/modules/integration/secrets/repository/commands"
	"github.com/nuriansyah/hugimethara/internal/modules/integration/secrets/repository/queries"
	secretbox "github.com/nuriansyah/hugimethara/internal/platform/crypto"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	apperrors "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

type SecretsService struct {
	pool   *sqlx.DB
	az     authzsvc.Authorizer
	master *secretbox.Key
	kekID  string
}

func NewService(pool *sqlx.DB, az authzsvc.Authorizer, masterKey []byte, kekID string) (*SecretsService, error) {
	master, err := secretbox.New(masterKey)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(kekID) == "" {
		return nil, errors.New("secret: KEK id is required")
	}
	return &SecretsService{pool: pool, az: az, master: master, kekID: kekID}, nil
}

func (s *SecretsService) require(ctx context.Context, action, id string) (principal.Principal, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return principal.Principal{}, authzmodel.Unauthenticated()
	}
	if _, err := authzsvc.Require(ctx, s.az, action, authzmodel.Resource{Type: secretsmodel.Resource, OrgID: p.OrgID, ID: id}); err != nil {
		return principal.Principal{}, err
	}
	return p, nil
}

func (s *SecretsService) tenantTx(ctx context.Context, p principal.Principal, fn func(*sqlx.Tx) error) error {
	return db.InTenantTx(ctx, s.pool, p.OrgID, fn)
}

func (s *SecretsService) List(ctx context.Context) ([]secretsmodel.Credential, error) {
	p, err := s.require(ctx, "read", "")
	if err != nil {
		return nil, err
	}
	out := []secretsmodel.Credential{}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &out, queries.SQL.Get("ListCredentials"))
	})
	return out, err
}

func (s *SecretsService) Get(ctx context.Context, id uuid.UUID) (secretsmodel.Credential, error) {
	p, err := s.require(ctx, "read", id.String())
	if err != nil {
		return secretsmodel.Credential{}, err
	}
	var out secretsmodel.Credential
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		var row repository.CredentialRow
		if err := tx.GetContext(ctx, &row, queries.SQL.Get("GetCredential"), id); err != nil {
			return secretsmodel.TranslateNotFound(err)
		}
		out = row.Credential
		return nil
	})
	return out, err
}

func (s *SecretsService) Create(ctx context.Context, in dto.CreateInput) (secretsmodel.Credential, error) {
	p, err := s.require(ctx, "create", "")
	if err != nil {
		return secretsmodel.Credential{}, err
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Kind = strings.TrimSpace(in.Kind)
	if err := secretsmodel.ValidateInput(in.Name, "name", 100); err != nil {
		return secretsmodel.Credential{}, err
	}
	if err := secretsmodel.ValidateInput(in.Kind, "kind", 64); err != nil {
		return secretsmodel.Credential{}, err
	}
	if err := secretsmodel.ValidateInput(in.Value, "value", 65536); err != nil {
		return secretsmodel.Credential{}, err
	}

	var out secretsmodel.Credential
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		id := uuid.Must(uuid.NewV7())
		dekID, dek, err := s.getOrCreateDEK(ctx, tx, p.OrgID)
		if err != nil {
			return err
		}
		defer secretsmodel.Zero(dek)
		ciphertext, err := sealValue(dek, p.OrgID, id, []byte(in.Value))
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertCredential"), p.OrgID, id,
			secretsmodel.NullableUUID(in.EnvironmentID), in.Kind, in.Name, dekID, ciphertext, secretsmodel.Hint(in.Value)); err != nil {
			if db.IsUniqueViolation(err) {
				return apperrors.NewProblem(http.StatusConflict, "credential_name_taken", "Conflict",
					"A credential with this name already exists in this scope.")
			}
			if db.IsForeignKeyViolation(err, "secrets_org_id_environment_id_fkey") {
				return apperrors.Validation(apperrors.FieldError{Field: "environment_id", Code: "not_found",
					Message: "The environment does not exist in this organization."})
			}
			return err
		}
		if err := tx.GetContext(ctx, &out, queries.SQL.Get("GetCredentialMetadata"), id); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "secret.created",
			ResourceType: secretsmodel.Resource, ResourceID: id.String(), Details: map[string]any{"name": in.Name, "kind": in.Kind}})
	})
	return out, err
}

func (s *SecretsService) Update(ctx context.Context, id uuid.UUID, in dto.UpdateInput) (secretsmodel.Credential, error) {
	p, err := s.require(ctx, "update", id.String())
	if err != nil {
		return secretsmodel.Credential{}, err
	}
	if in.Name == nil && in.Value == nil {
		return secretsmodel.Credential{}, apperrors.Validation(apperrors.FieldError{Field: "body", Code: "empty",
			Message: "At least one field must be provided."})
	}
	if in.Name != nil {
		*in.Name = strings.TrimSpace(*in.Name)
		if err := secretsmodel.ValidateInput(*in.Name, "name", 100); err != nil {
			return secretsmodel.Credential{}, err
		}
	}
	if in.Value != nil {
		if err := secretsmodel.ValidateInput(*in.Value, "value", 65536); err != nil {
			return secretsmodel.Credential{}, err
		}
	}
	var out secretsmodel.Credential
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		var current repository.CredentialRow
		if err := tx.GetContext(ctx, &current, queries.SQL.Get("GetCredential"), id); err != nil {
			return secretsmodel.TranslateNotFound(err)
		}
		var dekID any
		var ciphertext []byte
		var secretHint any
		if in.Value != nil {
			dekIDValue, dek, err := s.getOrCreateDEK(ctx, tx, p.OrgID)
			if err != nil {
				return err
			}
			defer secretsmodel.Zero(dek)
			ciphertext, err = sealValue(dek, p.OrgID, id, []byte(*in.Value))
			if err != nil {
				return err
			}
			dekID, secretHint = dekIDValue, secretsmodel.Hint(*in.Value)
		}
		result, err := tx.ExecContext(ctx, commands.SQL.Get("UpdateCredential"), id, in.Name, dekID, ciphertext, secretHint)
		if err != nil {
			if db.IsUniqueViolation(err) {
				return apperrors.NewProblem(http.StatusConflict, "credential_name_taken", "Conflict",
					"A credential with this name already exists in this scope.")
			}
			return err
		}
		if n, _ := result.RowsAffected(); n == 0 {
			return secretsmodel.ErrNotFound
		}
		if err := tx.GetContext(ctx, &out, queries.SQL.Get("GetCredentialMetadata"), id); err != nil {
			return err
		}
		details := map[string]any{}
		if in.Name != nil {
			details["name"] = *in.Name
		}
		if in.Value != nil {
			details["value_rotated"] = true
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "secret.updated",
			ResourceType: secretsmodel.Resource, ResourceID: id.String(), Details: details})
	})
	return out, err
}

func (s *SecretsService) Delete(ctx context.Context, id uuid.UUID) error {
	p, err := s.require(ctx, "delete", id.String())
	if err != nil {
		return err
	}
	return s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		var name string
		if err := tx.GetContext(ctx, &name, `SELECT name FROM secrets WHERE id = $1`, id); err != nil {
			return secretsmodel.TranslateNotFound(err)
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("DeleteCredential"), id); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "secret.deleted",
			ResourceType: secretsmodel.Resource, ResourceID: id.String(), Details: map[string]any{"name": name}})
	})
}

// Resolve decrypts a credential for a trusted data-plane operation. Callers
// must zero the returned buffer as soon as the connector has finished.
func (s *SecretsService) Resolve(ctx context.Context, id uuid.UUID) ([]byte, error) {
	p, err := s.require(ctx, "use", id.String())
	if err != nil {
		return nil, err
	}
	var value []byte
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		var row repository.CredentialRow
		if err := tx.GetContext(ctx, &row, queries.SQL.Get("GetCredential"), id); err != nil {
			return secretsmodel.TranslateNotFound(err)
		}
		dek, err := s.loadDEK(ctx, tx, p.OrgID, row.DEKID)
		if err != nil {
			return err
		}
		defer secretsmodel.Zero(dek)
		value, err = openValue(dek, p.OrgID, id, row.Ciphertext)
		return err
	})
	return value, err
}

func (s *SecretsService) getOrCreateDEK(ctx context.Context, tx *sqlx.Tx, org uuid.UUID) (uuid.UUID, []byte, error) {
	var existing repository.DataKeyRow
	err := tx.GetContext(ctx, &existing, queries.SQL.Get("GetDataKey"), org)
	if err == nil {
		dek, err := s.unwrapDEK(org, existing)
		return existing.ID, dek, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, nil, err
	}
	dek := make([]byte, secretbox.KeySize)
	if _, err := rand.Read(dek); err != nil {
		return uuid.Nil, nil, fmt.Errorf("secret: generate DEK: %w", err)
	}
	id := uuid.Must(uuid.NewV7())
	wrapped, err := s.master.SealWithAAD(dek, secretsmodel.KeyAAD(org, id))
	if err != nil {
		secretsmodel.Zero(dek)
		return uuid.Nil, nil, err
	}
	if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertDataKey"), org, id, s.kekID, wrapped); err != nil {
		secretsmodel.Zero(dek)
		return uuid.Nil, nil, err
	}
	if err := tx.GetContext(ctx, &existing, queries.SQL.Get("GetDataKey"), org); err != nil {
		secretsmodel.Zero(dek)
		return uuid.Nil, nil, err
	}
	if existing.ID != id {
		secretsmodel.Zero(dek)
		dek, err = s.unwrapDEK(org, existing)
	}
	return existing.ID, dek, err
}

func (s *SecretsService) loadDEK(ctx context.Context, tx *sqlx.Tx, org, id uuid.UUID) ([]byte, error) {
	var row repository.DataKeyRow
	if err := tx.GetContext(ctx, &row, queries.SQL.Get("GetDataKey"), org); err != nil {
		return nil, err
	}
	if row.ID != id {
		return nil, errors.New("secret: credential references an unknown data key")
	}
	return s.unwrapDEK(org, row)
}

func (s *SecretsService) unwrapDEK(org uuid.UUID, row repository.DataKeyRow) ([]byte, error) {
	if row.KEKID != s.kekID {
		return nil, fmt.Errorf("secret: configured KEK %q does not match stored KEK %q", s.kekID, row.KEKID)
	}
	dek, err := s.master.OpenWithAAD(row.Wrapped, secretsmodel.KeyAAD(org, row.ID))
	if err != nil {
		return nil, fmt.Errorf("secret: unwrap data key: %w", err)
	}
	if len(dek) != secretbox.KeySize {
		secretsmodel.Zero(dek)
		return nil, errors.New("secret: stored data key has invalid size")
	}
	return dek, nil
}

func sealValue(dek []byte, org, id uuid.UUID, value []byte) ([]byte, error) {
	box, err := secretbox.New(dek)
	if err != nil {
		return nil, err
	}
	return box.SealWithAAD(value, secretsmodel.KeyAAD(org, id))
}

func openValue(dek []byte, org, id uuid.UUID, sealed []byte) ([]byte, error) {
	box, err := secretbox.New(dek)
	if err != nil {
		return nil, err
	}
	return box.OpenWithAAD(sealed, secretsmodel.KeyAAD(org, id))
}
