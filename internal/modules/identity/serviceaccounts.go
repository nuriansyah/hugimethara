package identity

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/modules/authorization"
	"github.com/nuriansyah/hugimethara/internal/modules/governance/audit"
	"github.com/nuriansyah/hugimethara/internal/platform/database"
	"github.com/nuriansyah/hugimethara/internal/platform/httpx"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

// ServiceAccount is a non-human identity that authenticates with API keys.
type ServiceAccount struct {
	ID             uuid.UUID    `db:"id"`
	Name           string       `db:"name"`
	Description    string       `db:"description"`
	DisabledAt     sql.NullTime `db:"disabled_at"`
	ActiveKeyCount int64        `db:"active_key_count"`
	CreatedAt      time.Time    `db:"created_at"`
	UpdatedAt      time.Time    `db:"updated_at"`
}

// APIKey describes a key; the secret itself is never stored or listed.
type APIKey struct {
	ID         uuid.UUID    `db:"id"`
	Name       string       `db:"name"`
	Prefix     string       `db:"prefix"`
	CreatedAt  time.Time    `db:"created_at"`
	ExpiresAt  sql.NullTime `db:"expires_at"`
	LastUsedAt sql.NullTime `db:"last_used_at"`
	RevokedAt  sql.NullTime `db:"revoked_at"`
}

// ServiceAccountInput creates or updates a service account.
type ServiceAccountInput struct {
	Name        string
	Description string
	Disabled    bool // update only
}

// MaxActiveKeys bounds the active keys per service account (rotation needs two).
const MaxActiveKeys = 10

const apiKeyPrefix = "hugi_"

var (
	errServiceAccountNotFound = httpx.NotFound("Service account not found.")
	errServiceAccountName     = httpx.NewProblem(http.StatusConflict, "name_taken", "Conflict", "A service account with this name already exists.")
	errInvalidAPIKey          = errors.New("identity: invalid api key")
)

// ErrAPIKeyRejected is the 401 for a missing, malformed, unknown, revoked or
// expired key, or a disabled account. The cases are not distinguished.
var ErrAPIKeyRejected = httpx.NewProblem(http.StatusUnauthorized, "invalid_api_key", "Authentication failed",
	"The API key is invalid, expired or revoked, or its service account is disabled.")

func getServiceAccount(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (ServiceAccount, error) {
	var out []ServiceAccount
	if err := tx.SelectContext(ctx, &out, q.Get("ListServiceAccounts"), id); err != nil {
		return ServiceAccount{}, err
	}
	if len(out) == 0 {
		return ServiceAccount{}, errServiceAccountNotFound
	}
	return out[0], nil
}

// ListServiceAccounts lists the organization's service accounts (service_account:read).
func (s *Service) ListServiceAccounts(ctx context.Context) ([]ServiceAccount, error) {
	p, err := s.require(ctx, "read", ServiceAccountResource, "")
	if err != nil {
		return nil, err
	}
	out := []ServiceAccount{}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error { return tx.SelectContext(ctx, &out, q.Get("ListServiceAccounts"), nil) })
	return out, err
}

// GetServiceAccount returns one service account (service_account:read).
func (s *Service) GetServiceAccount(ctx context.Context, id uuid.UUID) (ServiceAccount, error) {
	p, err := s.require(ctx, "read", ServiceAccountResource, id.String())
	if err != nil {
		return ServiceAccount{}, err
	}
	var sa ServiceAccount
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		var err error
		sa, err = getServiceAccount(ctx, tx, id)
		return err
	})
	return sa, err
}

// CreateServiceAccount creates a service account with no roles (service_account:create).
func (s *Service) CreateServiceAccount(ctx context.Context, in ServiceAccountInput) (ServiceAccount, error) {
	p, err := s.require(ctx, "create", ServiceAccountResource, "")
	if err != nil {
		return ServiceAccount{}, err
	}
	if err := validateNameDesc(&in.Name, &in.Description, "Service account"); err != nil {
		return ServiceAccount{}, err
	}
	id := uuid.Must(uuid.NewV7())
	var sa ServiceAccount
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, q.Get("InsertServiceAccount"), id, p.OrgID, in.Name, in.Description, p.ID); err != nil {
			if db.IsUniqueViolation(err) {
				return errServiceAccountName
			}
			return err
		}
		var err error
		if sa, err = getServiceAccount(ctx, tx, id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "service_account.created",
			ResourceType: ServiceAccountResource, ResourceID: id.String(), Details: map[string]any{"name": in.Name}})
	})
	return sa, err
}

// UpdateServiceAccount renames, re-describes, disables or re-enables a
// service account (service_account:update). Disabling stops its keys at once.
func (s *Service) UpdateServiceAccount(ctx context.Context, id uuid.UUID, in ServiceAccountInput) (ServiceAccount, error) {
	p, err := s.require(ctx, "update", ServiceAccountResource, id.String())
	if err != nil {
		return ServiceAccount{}, err
	}
	if err := validateNameDesc(&in.Name, &in.Description, "Service account"); err != nil {
		return ServiceAccount{}, err
	}
	if in.Disabled && p.Type == principal.ServiceAccount && p.ID == id {
		return ServiceAccount{}, httpx.NewProblem(http.StatusConflict, "cannot_disable_self", "Conflict", "A service account cannot disable itself.")
	}
	var sa ServiceAccount
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		before, err := getServiceAccount(ctx, tx, id)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, q.Get("UpdateServiceAccount"), id, in.Name, in.Description, in.Disabled); err != nil {
			if db.IsUniqueViolation(err) {
				return errServiceAccountName
			}
			return err
		}
		if before.DisabledAt.Valid != in.Disabled {
			if err := authz.BumpVersion(ctx, tx, p.OrgID); err != nil {
				return err
			}
		}
		if sa, err = getServiceAccount(ctx, tx, id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "service_account.updated",
			ResourceType: ServiceAccountResource, ResourceID: id.String(), Details: map[string]any{
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
func (s *Service) DeleteServiceAccount(ctx context.Context, id uuid.UUID) error {
	p, err := s.require(ctx, "delete", ServiceAccountResource, id.String())
	if err != nil {
		return err
	}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		sa, err := getServiceAccount(ctx, tx, id)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, q.Get("DeleteServiceAccount"), id); err != nil {
			return err
		}
		if err := authz.BumpVersion(ctx, tx, p.OrgID); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "service_account.deleted",
			ResourceType: ServiceAccountResource, ResourceID: id.String(), Details: map[string]any{"name": sa.Name}})
	})
	if err == nil {
		s.acl.Invalidate(ctx, p.OrgID)
	}
	return err
}

// ListAPIKeys lists a service account's keys (service_account:read).
func (s *Service) ListAPIKeys(ctx context.Context, accountID uuid.UUID) ([]APIKey, error) {
	p, err := s.require(ctx, "read", ServiceAccountResource, accountID.String())
	if err != nil {
		return nil, err
	}
	out := []APIKey{}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		if _, err := getServiceAccount(ctx, tx, accountID); err != nil {
			return err
		}
		return tx.SelectContext(ctx, &out, q.Get("ListAPIKeys"), accountID)
	})
	return out, err
}

// newAPIKey returns "hugi_<12 lowercase alphanumerics>_<64 hex>" and its prefix.
func newAPIKey() (key, prefix string) {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 12+32)
	if _, err := rand.Read(b); err != nil {
		panic("identity: crypto/rand failed: " + err.Error())
	}
	p := make([]byte, 12)
	for i := range p {
		p[i] = alphabet[int(b[i])%len(alphabet)]
	}
	prefix = string(p)
	return apiKeyPrefix + prefix + "_" + hex.EncodeToString(b[12:]), prefix
}

// parseAPIKey extracts the prefix from a presented key.
func parseAPIKey(key string) (string, bool) {
	rest, ok := strings.CutPrefix(key, apiKeyPrefix)
	if !ok || len(rest) != 12+1+64 || rest[12] != '_' {
		return "", false
	}
	return rest[:12], true
}

// CreateAPIKey issues a key for a service account (service_account:manage_keys).
// The returned secret is shown once.
func (s *Service) CreateAPIKey(ctx context.Context, accountID uuid.UUID, name string, expiresAt *time.Time) (APIKey, string, error) {
	p, err := s.require(ctx, "manage_keys", ServiceAccountResource, accountID.String())
	if err != nil {
		return APIKey{}, "", err
	}
	name = strings.TrimSpace(name)
	var errs []httpx.FieldError
	if n := len([]rune(name)); n < 1 || n > 100 {
		errs = append(errs, httpx.FieldError{Field: "name", Code: "invalid_length", Message: "Key name must be 1–100 characters."})
	}
	if expiresAt != nil && !expiresAt.After(s.now()) {
		errs = append(errs, httpx.FieldError{Field: "expires_at", Code: "in_past", Message: "Expiry must be in the future."})
	}
	if len(errs) > 0 {
		return APIKey{}, "", httpx.Validation(errs...)
	}
	secret, prefix := newAPIKey()
	id := uuid.Must(uuid.NewV7())
	var k APIKey
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		sa, err := getServiceAccount(ctx, tx, accountID)
		if err != nil {
			return err
		}
		if sa.DisabledAt.Valid {
			return httpx.NewProblem(http.StatusConflict, "service_account_disabled", "Conflict", "Enable the service account before creating keys.")
		}
		var active int64
		if err := tx.GetContext(ctx, &active, q.Get("CountActiveAPIKeys"), accountID); err != nil {
			return err
		}
		if active >= MaxActiveKeys {
			return httpx.NewProblem(http.StatusConflict, "too_many_keys", "Conflict", "A service account can have at most 10 active keys. Revoke one first.")
		}
		var exp any
		if expiresAt != nil {
			exp = *expiresAt
		}
		if _, err := tx.ExecContext(ctx, q.Get("InsertAPIKey"), id, p.OrgID, accountID, name, prefix, hashToken(secret), p.ID, exp); err != nil {
			return err
		}
		if err := tx.GetContext(ctx, &k, q.Get("GetAPIKey"), id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "service_account.key_created",
			ResourceType: ServiceAccountResource, ResourceID: accountID.String(),
			Details: map[string]any{"key_id": id, "prefix": prefix, "name": name, "expires_at": exp}})
	})
	if err != nil {
		return APIKey{}, "", err
	}
	return k, secret, nil
}

// RevokeAPIKey revokes a key (service_account:manage_keys). Effective on the next request.
func (s *Service) RevokeAPIKey(ctx context.Context, accountID, keyID uuid.UUID) error {
	p, err := s.require(ctx, "manage_keys", ServiceAccountResource, accountID.String())
	if err != nil {
		return err
	}
	return s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		if _, err := getServiceAccount(ctx, tx, accountID); err != nil {
			return err
		}
		n, err := execRows(ctx, tx, q.Get("RevokeAPIKey"), keyID, accountID)
		if err != nil {
			return err
		}
		if n == 0 {
			return httpx.NotFound("Key not found or already revoked.")
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "service_account.key_revoked",
			ResourceType: ServiceAccountResource, ResourceID: accountID.String(), Details: map[string]any{"key_id": keyID}})
	})
}

type resolvedKey struct {
	KeyID      uuid.UUID    `db:"key_id"`
	OrgID      uuid.UUID    `db:"org_id"`
	AccountID  uuid.UUID    `db:"service_account_id"`
	KeyHash    []byte       `db:"key_hash"`
	ExpiresAt  sql.NullTime `db:"expires_at"`
	RevokedAt  sql.NullTime `db:"revoked_at"`
	LastUsedAt sql.NullTime `db:"last_used_at"`
	DisabledAt sql.NullTime `db:"account_disabled_at"`
	OrgStatus  string       `db:"org_status"`
}

// AuthenticateAPIKey resolves a presented API key into a service-account principal.
func (s *Service) AuthenticateAPIKey(ctx context.Context, key string) (principal.Principal, error) {
	prefix, ok := parseAPIKey(key)
	if !ok {
		return principal.Principal{}, errInvalidAPIKey
	}
	var rk resolvedKey
	err := db.InTx(ctx, s.pool, func(tx *sqlx.Tx) error { return tx.GetContext(ctx, &rk, q.Get("ResolveAPIKey"), prefix) })
	if db.IsNoRows(err) {
		return principal.Principal{}, errInvalidAPIKey
	}
	if err != nil {
		return principal.Principal{}, err
	}
	now := s.now()
	if subtle.ConstantTimeCompare(hashToken(key), rk.KeyHash) != 1 || rk.RevokedAt.Valid || rk.DisabledAt.Valid ||
		(rk.ExpiresAt.Valid && !now.Before(rk.ExpiresAt.Time)) || rk.OrgStatus != "active" {
		return principal.Principal{}, errInvalidAPIKey
	}
	if !rk.LastUsedAt.Valid || now.Sub(rk.LastUsedAt.Time) >= s.cfg.TouchInterval {
		err := db.InTenantTx(ctx, s.pool, rk.OrgID, func(tx *sqlx.Tx) error {
			_, err := tx.ExecContext(ctx, q.Get("TouchAPIKey"), rk.KeyID)
			return err
		})
		if err != nil {
			s.log.WarnContext(ctx, "api key touch failed", "err", err) // non-fatal
		}
	}
	return principal.Principal{ID: rk.AccountID, Type: principal.ServiceAccount, OrgID: rk.OrgID}, nil
}
