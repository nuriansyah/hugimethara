package db

import (
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// ErrNoRows is returned by single-row reads that find nothing.
var ErrNoRows = sql.ErrNoRows

// IsNoRows reports whether err means "no row found".
func IsNoRows(err error) bool { return errors.Is(err, sql.ErrNoRows) }

// IsUniqueViolation reports a unique-constraint violation (SQLSTATE 23505),
// optionally only for the named constraint.
func IsUniqueViolation(err error, constraint ...string) bool { return isCode(err, "23505", constraint) }

// IsForeignKeyViolation reports a foreign-key violation (SQLSTATE 23503).
func IsForeignKeyViolation(err error, constraint ...string) bool {
	return isCode(err, "23503", constraint)
}

func isCode(err error, code string, constraint []string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != code {
		return false
	}
	if len(constraint) == 0 {
		return true
	}
	for _, c := range constraint {
		if pgErr.ConstraintName == c {
			return true
		}
	}
	return false
}
