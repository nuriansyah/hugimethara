package model

import (
	"database/sql"

	"github.com/google/uuid"
)

type LoginRecord struct {
	UserID         uuid.UUID    `db:"user_id"`
	UserStatus     string       `db:"user_status"`
	DisplayName    string       `db:"display_name"`
	PasswordHash   string       `db:"password_hash"`
	FailedAttempts int32        `db:"failed_attempts"`
	LockedUntil    sql.NullTime `db:"locked_until"`
}
