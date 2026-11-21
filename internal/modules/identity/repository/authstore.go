package repository

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type MemberRow struct {
	ID          uuid.UUID `db:"id"`
	Email       string    `db:"email"`
	DisplayName string    `db:"display_name"`
	Status      string    `db:"status"`
	JoinedAt    time.Time `db:"joined_at"`
}

type SessionRow struct {
	ID            uuid.UUID      `db:"id"`
	CreatedAt     time.Time      `db:"created_at"`
	LastSeenAt    time.Time      `db:"last_seen_at"`
	IdleExpiresAt time.Time      `db:"idle_expires_at"`
	ExpiresAt     time.Time      `db:"expires_at"`
	IP            sql.NullString `db:"ip"`
	UserAgent     sql.NullString `db:"user_agent"`
}
