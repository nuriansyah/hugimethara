package services

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	identitymodel "github.com/nuriansyah/hugimethara/internal/modules/identity/model"
	"github.com/nuriansyah/hugimethara/internal/modules/identity/repository/queries"
	orgmodel "github.com/nuriansyah/hugimethara/internal/modules/organization/model"
)

func loginLookup(ctx context.Context, tx *sqlx.Tx, email string) (identitymodel.LoginRecord, error) {
	var r identitymodel.LoginRecord
	err := tx.GetContext(ctx, &r, queries.SQL.Get("LoginLookup"), email)
	return r, err
}

type orgChoice struct {
	ID       uuid.UUID `db:"org_id"`
	Slug     string    `db:"slug"`
	Name     string    `db:"name"`
	RawSet   []byte    `db:"settings"`
	Settings orgmodel.Settings
}

func userOrganizations(ctx context.Context, tx *sqlx.Tx, userID uuid.UUID) ([]orgChoice, error) {
	var out []orgChoice
	if err := tx.SelectContext(ctx, &out, queries.SQL.Get("UserOrganizations"), userID); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Settings = orgmodel.ParseSettings(out[i].RawSet)
	}
	return out, nil
}

func recordAttempt(ctx context.Context, tx *sqlx.Tx, userID uuid.UUID, success bool, maxFailures int, lockFor time.Duration) error {
	_, err := tx.ExecContext(ctx, queries.SQL.Get("RecordLoginAttempt"), userID, success, maxFailures, lockFor.Seconds())
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
	OrgSettings      orgmodel.Settings
}

func resolveSession(ctx context.Context, tx *sqlx.Tx, tokenHash []byte) (resolvedSession, error) {
	var s resolvedSession
	err := tx.GetContext(ctx, &s, queries.SQL.Get("ResolveSession"), tokenHash)
	s.OrgSettings = orgmodel.ParseSettings(s.RawSettings)
	return s, err
}
