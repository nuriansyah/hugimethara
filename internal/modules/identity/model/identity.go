package model

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/nuriansyah/hugimethara/internal/platform/errors"
)

// Errors returned to clients.
var (
	ErrInvalidCredentials = apperrors.NewProblem(http.StatusUnauthorized, "invalid_credentials", "Sign-in failed",
		"The email or password is incorrect, or the account is temporarily locked after repeated failures.")
	ErrEmailTaken = apperrors.NewProblem(http.StatusConflict, "email_taken", "Email already registered",
		"An account with this email already exists. Sign in instead.")
	ErrNoActiveOrganization = apperrors.NewProblem(http.StatusForbidden, "no_active_organization", "No active organization",
		"Your account is not an active member of any organization.")
	ErrNoSession = errors.New("identity: no valid session")
)

// User is a person's identity.
type User struct {
	ID          uuid.UUID
	Email       string
	DisplayName string
	Status      string
}

// OrgSummary identifies an organization a user may sign in to.
type OrgSummary struct {
	ID   uuid.UUID
	Slug string
	Name string
}

// SessionGrant is the result of signup/login: secrets for the cookies plus context.
type SessionGrant struct {
	Token         string // session cookie value (shown once)
	CSRFToken     string // CSRF cookie value
	SessionID     uuid.UUID
	ExpiresAt     time.Time
	IdleExpiresAt time.Time
	User          User
	Organization  OrgSummary
}

// OrganizationChoiceRequired is returned when the user must pick an organization.
type OrganizationChoiceRequired struct{ Choices []OrgSummary }

func (e *OrganizationChoiceRequired) Error() string { return "organization choice required" }

// Problem renders the error with the list of organizations to choose from.
func (e *OrganizationChoiceRequired) Problem() *apperrors.Problem {
	p := apperrors.NewProblem(http.StatusConflict, "organization_required", "Choose an organization",
		"Your account belongs to several organizations. Choose one to sign in to.")
	choices := make([]map[string]any, len(e.Choices))
	for i, c := range e.Choices {
		choices[i] = map[string]any{"slug": c.Slug, "name": c.Name}
	}
	p.Extensions = map[string]any{"organizations": choices}
	return p
}

// VerifyCSRF compares the presented token with the session's stored hash.
func VerifyCSRF(presented string, storedHash []byte) bool {
	if presented == "" || len(storedHash) == 0 {
		return false
	}
	return subtle.ConstantTimeCompare(HashToken(presented), storedHash) == 1
}

// Member is the caller's identity within their current organization.
type Member struct {
	User     User
	IsOwner  bool
	JoinedAt time.Time
}

// SessionInfo describes one of the caller's active sessions.
type SessionInfo struct {
	ID            uuid.UUID
	Current       bool
	CreatedAt     time.Time
	LastSeenAt    time.Time
	IdleExpiresAt time.Time
	ExpiresAt     time.Time
	IP            string
	UserAgent     string
}

// errNotAUser answers person-only endpoints called with an API key.
var ErrNotAUser = apperrors.NewProblem(http.StatusForbidden, "not_a_user", "Not available",
	"This endpoint is about the signed-in person and is not available to service accounts.")

func NormalizeEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }

func ValidEmail(e string) bool {
	if len(e) > 254 || strings.ContainsAny(e, " \t\r\n<>") {
		return false
	}
	a, err := mail.ParseAddress(e)
	return err == nil && a.Address == e && strings.Contains(e[strings.LastIndex(e, "@"):], ".")
}

func Digest(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:16])
}

func MinTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
