package services

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	authzsvc "github.com/nuriansyah/hugimethara/internal/modules/authorization/services"
	auditmodel "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/model"
	auditsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/services"
	identitymodel "github.com/nuriansyah/hugimethara/internal/modules/identity/model"
	identitydto "github.com/nuriansyah/hugimethara/internal/modules/identity/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/identity/repository/commands"
	"github.com/nuriansyah/hugimethara/internal/modules/identity/repository/queries"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

func getServiceAccount(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (identitymodel.ServiceAccount, error) {
	var out []identitymodel.ServiceAccount
	if err := tx.SelectContext(ctx, &out, queries.SQL.Get("ListServiceAccounts"), id); err != nil {
		return identitymodel.ServiceAccount{}, err
	}
	if len(out) == 0 {
		return identitymodel.ServiceAccount{}, identitymodel.ErrServiceAccountNotFound
	}
	return out[0], nil
}

// ListServiceAccounts lists the organization's service accounts (service_account:read).
func (s *IdentityService) ListServiceAccounts(ctx context.Context) ([]identitymodel.ServiceAccount, error) {
	p, err := s.require(ctx, "read", identitymodel.ServiceAccountResource, "")
	if err != nil {
		return nil, err
	}
	out := []identitymodel.ServiceAccount{}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &out, queries.SQL.Get("ListServiceAccounts"), nil)
	})
	return out, err
}

// GetServiceAccount returns one service account (service_account:read).
func (s *IdentityService) GetServiceAccount(ctx context.Context, id uuid.UUID) (identitymodel.ServiceAccount, error) {
	p, err := s.require(ctx, "read", identitymodel.ServiceAccountResource, id.String())
	if err != nil {
		return identitymodel.ServiceAccount{}, err
	}
	var sa identitymodel.ServiceAccount
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		var err error
		sa, err = getServiceAccount(ctx, tx, id)
		return err
	})
	return sa, err
}

// CreateServiceAccount creates a service account with no roles (service_account:create).
func (s *IdentityService) CreateServiceAccount(ctx context.Context, in identitydto.ServiceAccountInput) (identitymodel.ServiceAccount, error) {
	p, err := s.require(ctx, "create", identitymodel.ServiceAccountResource, "")
	if err != nil {
		return identitymodel.ServiceAccount{}, err
	}
	if err := identitymodel.ValidateNameDesc(&in.Name, &in.Description, "Service account"); err != nil {
		return identitymodel.ServiceAccount{}, err
	}
	id := uuid.Must(uuid.NewV7())
	var sa identitymodel.ServiceAccount
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertServiceAccount"), id, p.OrgID, in.Name, in.Description, p.ID); err != nil {
			if db.IsUniqueViolation(err) {
				return identitymodel.ErrServiceAccountName
			}
			return err
		}
		var err error
		if sa, err = getServiceAccount(ctx, tx, id); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "service_account.created",
			ResourceType: identitymodel.ServiceAccountResource, ResourceID: id.String(), Details: map[string]any{"name": in.Name}})
	})
	return sa, err
}

// UpdateServiceAccount renames, re-describes, disables or re-enables a
// service account (service_account:update). Disabling stops its keys at once.
func (s *IdentityService) UpdateServiceAccount(ctx context.Context, id uuid.UUID, in identitydto.ServiceAccountInput) (identitymodel.ServiceAccount, error) {
	p, err := s.require(ctx, "update", identitymodel.ServiceAccountResource, id.String())
	if err != nil {
		return identitymodel.ServiceAccount{}, err
	}
	if err := identitymodel.ValidateNameDesc(&in.Name, &in.Description, "Service account"); err != nil {
		return identitymodel.ServiceAccount{}, err
	}
	if in.Disabled && p.Type == principal.ServiceAccount && p.ID == id {
		return identitymodel.ServiceAccount{}, problem.NewProblem(http.StatusConflict, "cannot_disable_self", "Conflict", "A service account cannot disable itself.")
	}
	var sa identitymodel.ServiceAccount
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		before, err := getServiceAccount(ctx, tx, id)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("UpdateServiceAccount"), id, in.Name, in.Description, in.Disabled); err != nil {
			if db.IsUniqueViolation(err) {
				return identitymodel.ErrServiceAccountName
			}
			return err
		}
		if before.DisabledAt.Valid != in.Disabled {
			if err := authzsvc.BumpVersion(ctx, tx, p.OrgID); err != nil {
				return err
			}
		}
		if sa, err = getServiceAccount(ctx, tx, id); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "service_account.updated",
			ResourceType: identitymodel.ServiceAccountResource, ResourceID: id.String(), Details: map[string]any{
				"from": map[string]any{"name": before.Name, "disabled": before.DisabledAt.Valid},
				"to":   map[string]any{"name": sa.Name, "disabled": sa.DisabledAt.Valid}}})
	})
	if err == nil {
		s.acl.Invalidate(ctx, p.OrgID)
	}
	return sa, err
}

// DeleteServiceAccount deletes a service account with its keys and role
// assignments (service_account:delete).
func (s *IdentityService) DeleteServiceAccount(ctx context.Context, id uuid.UUID) error {
	p, err := s.require(ctx, "delete", identitymodel.ServiceAccountResource, id.String())
	if err != nil {
		return err
	}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		sa, err := getServiceAccount(ctx, tx, id)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("DeleteServiceAccount"), id); err != nil {
			return err
		}
		if err := authzsvc.BumpVersion(ctx, tx, p.OrgID); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "service_account.deleted",
			ResourceType: identitymodel.ServiceAccountResource, ResourceID: id.String(), Details: map[string]any{"name": sa.Name}})
	})
	if err == nil {
		s.acl.Invalidate(ctx, p.OrgID)
	}
	return err
}

// ListAPIKeys lists a service account's keys (service_account:read).
func (s *IdentityService) ListAPIKeys(ctx context.Context, accountID uuid.UUID) ([]identitymodel.APIKey, error) {
	p, err := s.require(ctx, "read", identitymodel.ServiceAccountResource, accountID.String())
	if err != nil {
		return nil, err
	}
	out := []identitymodel.APIKey{}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		if _, err := getServiceAccount(ctx, tx, accountID); err != nil {
			return err
		}
		return tx.SelectContext(ctx, &out, queries.SQL.Get("ListAPIKeys"), accountID)
	})
	return out, err
}

// CreateAPIKey issues a key for a service account (service_account:manage_keys).
// The returned secret is shown once.
func (s *IdentityService) CreateAPIKey(ctx context.Context, accountID uuid.UUID, name string, expiresAt *time.Time) (identitymodel.APIKey, string, error) {
	p, err := s.require(ctx, "manage_keys", identitymodel.ServiceAccountResource, accountID.String())
	if err != nil {
		return identitymodel.APIKey{}, "", err
	}
	name = strings.TrimSpace(name)
	var errs []problem.FieldError
	if n := len([]rune(name)); n < 1 || n > 100 {
		errs = append(errs, problem.FieldError{Field: "name", Code: "invalid_length", Message: "Key name must be 1–100 characters."})
	}
	if expiresAt != nil && !expiresAt.After(s.now()) {
		errs = append(errs, problem.FieldError{Field: "expires_at", Code: "in_past", Message: "Expiry must be in the future."})
	}
	if len(errs) > 0 {
		return identitymodel.APIKey{}, "", problem.Validation(errs...)
	}
	secret, prefix := identitymodel.NewAPIKey()
	id := uuid.Must(uuid.NewV7())
	var k identitymodel.APIKey
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		sa, err := getServiceAccount(ctx, tx, accountID)
		if err != nil {
			return err
		}
		if sa.DisabledAt.Valid {
			return problem.NewProblem(http.StatusConflict, "service_account_disabled", "Conflict", "Enable the service account before creating keys.")
		}
		var active int64
		if err := tx.GetContext(ctx, &active, queries.SQL.Get("CountActiveAPIKeys"), accountID); err != nil {
			return err
		}
		if active >= identitymodel.MaxActiveKeys {
			return problem.NewProblem(http.StatusConflict, "too_many_keys", "Conflict", "A service account can have at most 10 active keys. Revoke one first.")
		}
		var exp any
		if expiresAt != nil {
			exp = *expiresAt
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertAPIKey"), id, p.OrgID, accountID, name, prefix, identitymodel.HashToken(secret), p.ID, exp); err != nil {
			return err
		}
		if err := tx.GetContext(ctx, &k, queries.SQL.Get("GetAPIKey"), id); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "service_account.key_created",
			ResourceType: identitymodel.ServiceAccountResource, ResourceID: accountID.String(),
			Details: map[string]any{"key_id": id, "prefix": prefix, "name": name, "expires_at": exp}})
	})
	if err != nil {
		return identitymodel.APIKey{}, "", err
	}
	return k, secret, nil
}

// RevokeAPIKey revokes a key (service_account:manage_keys). Effective on the next request.
func (s *IdentityService) RevokeAPIKey(ctx context.Context, accountID, keyID uuid.UUID) error {
	p, err := s.require(ctx, "manage_keys", identitymodel.ServiceAccountResource, accountID.String())
	if err != nil {
		return err
	}
	return s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		if _, err := getServiceAccount(ctx, tx, accountID); err != nil {
			return err
		}
		n, err := execRows(ctx, tx, commands.SQL.Get("RevokeAPIKey"), keyID, accountID)
		if err != nil {
			return err
		}
		if n == 0 {
			return problem.NotFound("Key not found or already revoked.")
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "service_account.key_revoked",
			ResourceType: identitymodel.ServiceAccountResource, ResourceID: accountID.String(), Details: map[string]any{"key_id": keyID}})
	})
}

// AuthenticateAPIKey resolves a presented API key into a service-account principal.
func (s *IdentityService) AuthenticateAPIKey(ctx context.Context, key string) (principal.Principal, error) {
	prefix, ok := identitymodel.ParseAPIKey(key)
	if !ok {
		return principal.Principal{}, identitymodel.ErrInvalidAPIKey
	}
	var rk identitymodel.ResolvedKey
	err := db.InTx(ctx, s.pool, func(tx *sqlx.Tx) error { return tx.GetContext(ctx, &rk, queries.SQL.Get("ResolveAPIKey"), prefix) })
	if db.IsNoRows(err) {
		return principal.Principal{}, identitymodel.ErrInvalidAPIKey
	}
	if err != nil {
		return principal.Principal{}, err
	}
	now := s.now()
	if subtle.ConstantTimeCompare(identitymodel.HashToken(key), rk.KeyHash) != 1 || rk.RevokedAt.Valid || rk.DisabledAt.Valid ||
		(rk.ExpiresAt.Valid && !now.Before(rk.ExpiresAt.Time)) || rk.OrgStatus != "active" {
		return principal.Principal{}, identitymodel.ErrInvalidAPIKey
	}
	if !rk.LastUsedAt.Valid || now.Sub(rk.LastUsedAt.Time) >= s.cfg.TouchInterval {
		err := db.InTenantTx(ctx, s.pool, rk.OrgID, func(tx *sqlx.Tx) error {
			_, err := tx.ExecContext(ctx, commands.SQL.Get("TouchAPIKey"), rk.KeyID)
			return err
		})
		if err != nil {
			s.log.WarnContext(ctx, "api key touch failed", "err", err) // non-fatal
		}
	}
	return principal.Principal{ID: rk.AccountID, Type: principal.ServiceAccount, OrgID: rk.OrgID}, nil
}
