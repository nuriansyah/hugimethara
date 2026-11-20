// Package secret owns encrypted credential metadata and tenant-scoped
// envelope encryption. Plaintext is exposed only through Resolve.
package secret

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/modules/authorization"
	"github.com/nuriansyah/hugimethara/internal/modules/governance/audit"
	"github.com/nuriansyah/hugimethara/internal/platform/crypto"
	"github.com/nuriansyah/hugimethara/internal/platform/database"
	"github.com/nuriansyah/hugimethara/internal/platform/httpx"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

const Resource = "secret"

var errNotFound = httpx.NotFound("Credential not found.")

type Credential struct {
	ID            uuid.UUID  `db:"id"`
	EnvironmentID *uuid.UUID `db:"environment_id"`
	Kind          string     `db:"kind"`
	Name          string     `db:"name"`
	Hint          string     `db:"hint"`
	Version       int64      `db:"version"`
	CreatedAt     time.Time  `db:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at"`
}

type credentialRow struct {
	Credential
	Ciphertext []byte    `db:"ciphertext"`
	DEKID      uuid.UUID `db:"dek_id"`
}

type dataKeyRow struct {
	ID      uuid.UUID `db:"id"`
	KEKID   string    `db:"kek_id"`
	Wrapped []byte    `db:"wrapped_key"`
}

type CreateInput struct {
	Name          string
	Kind          string
	Value         string
	EnvironmentID *uuid.UUID
}

type UpdateInput struct {
	Name  *string
	Value *string
}

type Service struct {
	pool   *sqlx.DB
	az     authz.Authorizer
	master *secretbox.Key
	kekID  string
}

func NewService(pool *sqlx.DB, az authz.Authorizer, masterKey []byte, kekID string) (*Service, error) {
	master, err := secretbox.New(masterKey)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(kekID) == "" {
		return nil, errors.New("secret: KEK id is required")
	}
	return &Service{pool: pool, az: az, master: master, kekID: kekID}, nil
}

func (s *Service) require(ctx context.Context, action, id string) (principal.Principal, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return principal.Principal{}, authz.Unauthenticated()
	}
	if _, err := authz.Require(ctx, s.az, action, authz.Resource{Type: Resource, OrgID: p.OrgID, ID: id}); err != nil {
		return principal.Principal{}, err
	}
	return p, nil
}

func (s *Service) tenantTx(ctx context.Context, p principal.Principal, fn func(*sqlx.Tx) error) error {
	return db.InTenantTx(ctx, s.pool, p.OrgID, fn)
}

func (s *Service) List(ctx context.Context) ([]Credential, error) {
	p, err := s.require(ctx, "read", "")
	if err != nil {
		return nil, err
	}
	out := []Credential{}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &out, q.Get("ListCredentials"))
	})
	return out, err
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (Credential, error) {
	p, err := s.require(ctx, "read", id.String())
	if err != nil {
		return Credential{}, err
	}
	var out Credential
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		var row credentialRow
		if err := tx.GetContext(ctx, &row, q.Get("GetCredential"), id); err != nil {
			return translateNotFound(err)
		}
		out = row.Credential
		return nil
	})
	return out, err
}

func (s *Service) Create(ctx context.Context, in CreateInput) (Credential, error) {
	p, err := s.require(ctx, "create", "")
	if err != nil {
		return Credential{}, err
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Kind = strings.TrimSpace(in.Kind)
	if err := validateInput(in.Name, "name", 100); err != nil {
		return Credential{}, err
	}
	if err := validateInput(in.Kind, "kind", 64); err != nil {
		return Credential{}, err
	}
	if err := validateInput(in.Value, "value", 65536); err != nil {
		return Credential{}, err
	}

	var out Credential
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		id := uuid.Must(uuid.NewV7())
		dekID, dek, err := s.getOrCreateDEK(ctx, tx, p.OrgID)
		if err != nil {
			return err
		}
		defer zero(dek)
		ciphertext, err := sealValue(dek, p.OrgID, id, []byte(in.Value))
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, q.Get("InsertCredential"), p.OrgID, id,
			nullableUUID(in.EnvironmentID), in.Kind, in.Name, dekID, ciphertext, hint(in.Value)); err != nil {
			if db.IsUniqueViolation(err) {
				return httpx.NewProblem(http.StatusConflict, "credential_name_taken", "Conflict",
					"A credential with this name already exists in this scope.")
			}
			if db.IsForeignKeyViolation(err, "secrets_org_id_environment_id_fkey") {
				return httpx.Validation(httpx.FieldError{Field: "environment_id", Code: "not_found",
					Message: "The environment does not exist in this organization."})
			}
			return err
		}
		if err := tx.GetContext(ctx, &out, q.Get("GetCredentialMetadata"), id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "secret.created",
			ResourceType: Resource, ResourceID: id.String(), Details: map[string]any{"name": in.Name, "kind": in.Kind}})
	})
	return out, err
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (Credential, error) {
	p, err := s.require(ctx, "update", id.String())
	if err != nil {
		return Credential{}, err
	}
	if in.Name == nil && in.Value == nil {
		return Credential{}, httpx.Validation(httpx.FieldError{Field: "body", Code: "empty",
			Message: "At least one field must be provided."})
	}
	if in.Name != nil {
		*in.Name = strings.TrimSpace(*in.Name)
		if err := validateInput(*in.Name, "name", 100); err != nil {
			return Credential{}, err
		}
	}
	if in.Value != nil {
		if err := validateInput(*in.Value, "value", 65536); err != nil {
			return Credential{}, err
		}
	}
	var out Credential
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		var current credentialRow
		if err := tx.GetContext(ctx, &current, q.Get("GetCredential"), id); err != nil {
			return translateNotFound(err)
		}
		var dekID any
		var ciphertext []byte
		var secretHint any
		if in.Value != nil {
			dekIDValue, dek, err := s.getOrCreateDEK(ctx, tx, p.OrgID)
			if err != nil {
				return err
			}
			defer zero(dek)
			ciphertext, err = sealValue(dek, p.OrgID, id, []byte(*in.Value))
			if err != nil {
				return err
			}
			dekID, secretHint = dekIDValue, hint(*in.Value)
		}
		result, err := tx.ExecContext(ctx, q.Get("UpdateCredential"), id, in.Name, dekID, ciphertext, secretHint)
		if err != nil {
			if db.IsUniqueViolation(err) {
				return httpx.NewProblem(http.StatusConflict, "credential_name_taken", "Conflict",
					"A credential with this name already exists in this scope.")
			}
			return err
		}
		if n, _ := result.RowsAffected(); n == 0 {
			return errNotFound
		}
		if err := tx.GetContext(ctx, &out, q.Get("GetCredentialMetadata"), id); err != nil {
			return err
		}
		details := map[string]any{}
		if in.Name != nil {
			details["name"] = *in.Name
		}
		if in.Value != nil {
			details["value_rotated"] = true
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "secret.updated",
			ResourceType: Resource, ResourceID: id.String(), Details: details})
	})
	return out, err
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	p, err := s.require(ctx, "delete", id.String())
	if err != nil {
		return err
	}
	return s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		var name string
		if err := tx.GetContext(ctx, &name, `SELECT name FROM secrets WHERE id = $1`, id); err != nil {
			return translateNotFound(err)
		}
		if _, err := tx.ExecContext(ctx, q.Get("DeleteCredential"), id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "secret.deleted",
			ResourceType: Resource, ResourceID: id.String(), Details: map[string]any{"name": name}})
	})
}

// Resolve decrypts a credential for a trusted data-plane operation. Callers
// must zero the returned buffer as soon as the connector has finished.
func (s *Service) Resolve(ctx context.Context, id uuid.UUID) ([]byte, error) {
	p, err := s.require(ctx, "use", id.String())
	if err != nil {
		return nil, err
	}
	var value []byte
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		var row credentialRow
		if err := tx.GetContext(ctx, &row, q.Get("GetCredential"), id); err != nil {
			return translateNotFound(err)
		}
		dek, err := s.loadDEK(ctx, tx, p.OrgID, row.DEKID)
		if err != nil {
			return err
		}
		defer zero(dek)
		value, err = openValue(dek, p.OrgID, id, row.Ciphertext)
		return err
	})
	return value, err
}

func (s *Service) getOrCreateDEK(ctx context.Context, tx *sqlx.Tx, org uuid.UUID) (uuid.UUID, []byte, error) {
	var existing dataKeyRow
	err := tx.GetContext(ctx, &existing, q.Get("GetDataKey"), org)
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
	wrapped, err := s.master.SealWithAAD(dek, keyAAD(org, id))
	if err != nil {
		zero(dek)
		return uuid.Nil, nil, err
	}
	if _, err := tx.ExecContext(ctx, q.Get("InsertDataKey"), org, id, s.kekID, wrapped); err != nil {
		zero(dek)
		return uuid.Nil, nil, err
	}
	if err := tx.GetContext(ctx, &existing, q.Get("GetDataKey"), org); err != nil {
		zero(dek)
		return uuid.Nil, nil, err
	}
	if existing.ID != id {
		zero(dek)
		dek, err = s.unwrapDEK(org, existing)
	}
	return existing.ID, dek, err
}

func (s *Service) loadDEK(ctx context.Context, tx *sqlx.Tx, org, id uuid.UUID) ([]byte, error) {
	var row dataKeyRow
	if err := tx.GetContext(ctx, &row, q.Get("GetDataKey"), org); err != nil {
		return nil, err
	}
	if row.ID != id {
		return nil, errors.New("secret: credential references an unknown data key")
	}
	return s.unwrapDEK(org, row)
}

func (s *Service) unwrapDEK(org uuid.UUID, row dataKeyRow) ([]byte, error) {
	if row.KEKID != s.kekID {
		return nil, fmt.Errorf("secret: configured KEK %q does not match stored KEK %q", s.kekID, row.KEKID)
	}
	dek, err := s.master.OpenWithAAD(row.Wrapped, keyAAD(org, row.ID))
	if err != nil {
		return nil, fmt.Errorf("secret: unwrap data key: %w", err)
	}
	if len(dek) != secretbox.KeySize {
		zero(dek)
		return nil, errors.New("secret: stored data key has invalid size")
	}
	return dek, nil
}

func keyAAD(org, id uuid.UUID) []byte {
	return []byte(org.String() + "|" + id.String())
}

func sealValue(dek []byte, org, id uuid.UUID, value []byte) ([]byte, error) {
	box, err := secretbox.New(dek)
	if err != nil {
		return nil, err
	}
	return box.SealWithAAD(value, keyAAD(org, id))
}

func openValue(dek []byte, org, id uuid.UUID, sealed []byte) ([]byte, error) {
	box, err := secretbox.New(dek)
	if err != nil {
		return nil, err
	}
	return box.OpenWithAAD(sealed, keyAAD(org, id))
}

func validateInput(value, field string, max int) error {
	if strings.TrimSpace(value) == "" || !utf8.ValidString(value) || len(value) > max {
		return httpx.Validation(httpx.FieldError{Field: field, Code: "invalid",
			Message: "This field is required and must be valid UTF-8 within its size limit."})
	}
	return nil
}

func hint(value string) string {
	runes := []rune(value)
	if len(runes) > 4 {
		runes = runes[len(runes)-4:]
	}
	return string(runes)
}

func nullableUUID(v *uuid.UUID) any {
	if v == nil || *v == uuid.Nil {
		return nil
	}
	return *v
}

func translateNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return errNotFound
	}
	return err
}

func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
