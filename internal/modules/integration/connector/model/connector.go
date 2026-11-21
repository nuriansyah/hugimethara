package model

import (
	"database/sql"
	"errors"

	"github.com/google/uuid"

	apperrors "github.com/nuriansyah/hugimethara/internal/platform/errors"
)

var ErrNotFound = apperrors.NotFound("The requested connector resource does not exist.")

func TranslateNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func Validation(field, code, message string) error {
	return apperrors.Validation(apperrors.FieldError{Field: field, Code: code, Message: message})
}

func NullableUUID(v *uuid.UUID) any {
	if v == nil || *v == uuid.Nil {
		return nil
	}
	return *v
}

func NullableJSON(v []byte) any {
	if v == nil {
		return nil
	}
	return v
}

func ValidRefKey(s string) bool {
	if len(s) < 2 || len(s) > 64 || s[0] < 'a' || s[0] > 'z' {
		return false
	}
	for _, r := range s[1:] {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' && r != '-' {
			return false
		}
	}
	return true
}
