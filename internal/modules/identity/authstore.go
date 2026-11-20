package identity

import (
	"context"
	"database/sql"
	"embed"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/modules/organization"
	"github.com/nuriansyah/hugimethara/internal/platform/database"
)

//go:embed queries.sql
var queryFS embed.FS

var q = db.MustLoad(queryFS, "queries.sql")

type loginRecord struct {
	UserID         uuid.UUID    `db:"user_id"`
	UserStatus     string       `db:"user_status"`
	DisplayName    string       `db:"display_name"`
	PasswordHash   string       `db:"password_hash"`
	FailedAttempts int32        `db:"failed_attempts"`
	LockedUntil    sql.NullTime `db:"locked_until"`
}

func loginLookup(ctx context.Context, tx *sqlx.Tx, email string) (loginRecord, error) {
	var r loginRecord
	err := tx.GetContext(ctx, &r, q.Get("LoginLookup"), email)
	return r, err
}

type orgChoice struct {
	ID       uuid.UUID `db:"org_id"`
	Slug     string    `db:"slug"`
	Name     string    `db:"name"`
	RawSet   []byte    `db:"settings"`
	Settings organization.Settings
}

func userOrganizations(ctx context.Context, tx *sqlx.Tx, userID uuid.UUID) ([]orgChoice, error) {
	var out []orgChoice
	if err := tx.SelectContext(ctx, &out, q.Get("UserOrganizations"), userID); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Settings = organization.ParseSettings(out[i].RawSet)
	}
	return out, nil
}

func recordAttempt(ctx context.Context, tx *sqlx.Tx, userID uuid.UUID, success bool, maxFailures int, lockFor time.Duration) error {
	_, err := tx.ExecContext(ctx, q.Get("RecordLoginAttempt"), userID, success, maxFailures, lockFor.Seconds())
	return err
}

type resolvedSession struct {
	SessionID        uuid.UUID    `db:"session_id"`
	OrgID            uuid.UUID    `db:"org_id"`
	UserID           uuid.UUID    `db:"user_id"`
	CSRFHash         []byte       `db:"csrf_hash"`
	IdleExpiresAt    time.Time    `db:"idle_expires_at"`
	ExpiresAt        time.Time    `db:"expires_at"`
	RevokedAt        sql.NullTime `db:"revoked_at"`
	LastSeenAt       time.Time    `db:"last_seen_at"`
	UserStatus       string       `db:"user_status"`
	MembershipStatus string       `db:"membership_status"`
	OrgStatus        string       `db:"org_status"`
	RawSettings      []byte       `db:"org_settings"`
	OrgSettings      organization.Settings
}

func resolveSession(ctx context.Context, tx *sqlx.Tx, tokenHash []byte) (resolvedSession, error) {
	var s resolvedSession
	err := tx.GetContext(ctx, &s, q.Get("ResolveSession"), tokenHash)
	s.OrgSettings = organization.ParseSettings(s.RawSettings)
	return s, err
}

type memberRow struct {
	ID          uuid.UUID `db:"id"`
	Email       string    `db:"email"`
	DisplayName string    `db:"display_name"`
	Status      string    `db:"status"`
	JoinedAt    time.Time `db:"joined_at"`
}

type sessionRow struct {
	ID            uuid.UUID      `db:"id"`
	CreatedAt     time.Time      `db:"created_at"`
	LastSeenAt    time.Time      `db:"last_seen_at"`
	IdleExpiresAt time.Time      `db:"idle_expires_at"`
	ExpiresAt     time.Time      `db:"expires_at"`
	IP            sql.NullString `db:"ip"`
	UserAgent     sql.NullString `db:"user_agent"`
}
