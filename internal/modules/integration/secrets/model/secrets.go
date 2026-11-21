package model

import (
	"database/sql"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	apperrors "github.com/nuriansyah/hugimethara/internal/platform/errors"
)

const Resource = "secret"

var ErrNotFound = apperrors.NotFound("Credential not found.")

type Credential struct {
	ID            uuid.UUID  `db:"id"`
	EnvironmentID *uuid.UUID `db:"environment_id"`
	Kind          string     `db:"kind"`
	Name          string     `db:"name"`
	Hint          string     `db:"hint"`
	Version       int64      `db:"version"`
	CreatedAt     time.Time  `db:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at"`
}

func KeyAAD(org, id uuid.UUID) []byte {
	return []byte(org.String() + "|" + id.String())
}

func ValidateInput(value, field string, max int) error {
	if strings.TrimSpace(value) == "" || !utf8.ValidString(value) || len(value) > max {
		return apperrors.Validation(apperrors.FieldError{Field: field, Code: "invalid",
			Message: "This field is required and must be valid UTF-8 within its size limit."})
	}
	return nil
}

func Hint(value string) string {
	runes := []rune(value)
	if len(runes) > 4 {
		runes = runes[len(runes)-4:]
	}
	return string(runes)
}

func NullableUUID(v *uuid.UUID) any {
	if v == nil || *v == uuid.Nil {
		return nil
	}
	return *v
}

func TranslateNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func Zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
