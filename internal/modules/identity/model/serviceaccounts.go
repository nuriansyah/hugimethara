package model

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/nuriansyah/hugimethara/internal/platform/errors"
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

// MaxActiveKeys bounds the active keys per service account (rotation needs two).
const MaxActiveKeys = 10

const apiKeyPrefix = "hugi_"

var (
	ErrServiceAccountNotFound = apperrors.NotFound("Service account not found.")
	ErrServiceAccountName     = apperrors.NewProblem(http.StatusConflict, "name_taken", "Conflict", "A service account with this name already exists.")
	ErrInvalidAPIKey          = errors.New("identity: invalid api key")
)

// ErrAPIKeyRejected is the 401 for a missing, malformed, unknown, revoked or
// expired key, or a disabled account. The cases are not distinguished.
var ErrAPIKeyRejected = apperrors.NewProblem(http.StatusUnauthorized, "invalid_api_key", "Authentication failed",
	"The API key is invalid, expired or revoked, or its service account is disabled.")

// newAPIKey returns "hugi_<12 lowercase alphanumerics>_<64 hex>" and its prefix.
func NewAPIKey() (key, prefix string) {
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
func ParseAPIKey(key string) (string, bool) {
	rest, ok := strings.CutPrefix(key, apiKeyPrefix)
	if !ok || len(rest) != 12+1+64 || rest[12] != '_' {
		return "", false
	}
	return rest[:12], true
}

type ResolvedKey struct {
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
