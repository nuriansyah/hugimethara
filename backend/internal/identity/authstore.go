package identity

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/nuriansyah/hugimethara/backend/internal/organization"
)

// Hand-written calls to the pre-tenant SECURITY DEFINER functions from
// migration 00003 (sqlc cannot infer their result columns).


type loginRecord struct {
	UserID         uuid.UUID
	UserStatus     string
	DisplayName    string
	PasswordHash   string
	FailedAttempts int32
	LockedUntil    pgtype.Timestamptz
}

func loginLookup(ctx context.Context, q pgx.Tx, email string) (loginRecord, error) {
	var r loginRecord
	err := q.QueryRow(ctx, `SELECT user_id, user_status, display_name, password_hash, failed_attempts, locked_until
		FROM hugi_auth_login_lookup($1)`, email).
		Scan(&r.UserID, &r.UserStatus, &r.DisplayName, &r.PasswordHash, &r.FailedAttempts, &r.LockedUntil)
	return r, err
}

type orgChoice struct {
	ID       uuid.UUID
	Slug     string
	Name     string
	Settings organization.Settings
}

func userOrganizations(ctx context.Context, q pgx.Tx, userID uuid.UUID) ([]orgChoice, error) {
	rows, err := q.Query(ctx, `SELECT org_id, slug::text, name, settings FROM hugi_auth_user_orgs($1)`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (orgChoice, error) {
		var c orgChoice
		var raw []byte
		err := r.Scan(&c.ID, &c.Slug, &c.Name, &raw)
		c.Settings = organization.ParseSettings(raw)
		return c, err
	})
}

func recordAttempt(ctx context.Context, q pgx.Tx, userID uuid.UUID, success bool, maxFailures int, lockFor time.Duration) error {
	_, err := q.Exec(ctx, `SELECT hugi_auth_record_attempt($1, $2, $3, $4)`, userID, success, maxFailures, lockFor)
	return err
}

type resolvedSession struct {
	SessionID        uuid.UUID
	OrgID            uuid.UUID
	UserID           uuid.UUID
	CSRFHash         []byte
	IdleExpiresAt    time.Time
	ExpiresAt        time.Time
	RevokedAt        pgtype.Timestamptz
	LastSeenAt       time.Time
	UserStatus       string
	MembershipStatus string
	IsOwner          bool
	OrgStatus        string
	OrgSettings      organization.Settings
}

func resolveSession(ctx context.Context, q pgx.Tx, tokenHash []byte) (resolvedSession, error) {
	var s resolvedSession
	var settings []byte
	err := q.QueryRow(ctx, `SELECT session_id, org_id, user_id, csrf_hash, idle_expires_at, expires_at, revoked_at,
		last_seen_at, user_status, membership_status, is_owner, org_status, org_settings
		FROM hugi_auth_resolve_session($1)`, tokenHash).
		Scan(&s.SessionID, &s.OrgID, &s.UserID, &s.CSRFHash, &s.IdleExpiresAt, &s.ExpiresAt, &s.RevokedAt,
			&s.LastSeenAt, &s.UserStatus, &s.MembershipStatus, &s.IsOwner, &s.OrgStatus, &settings)
	s.OrgSettings = organization.ParseSettings(settings)
	return s, err
}
