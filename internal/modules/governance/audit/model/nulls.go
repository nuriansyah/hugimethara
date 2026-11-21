package model

import (
	"time"

	"github.com/google/uuid"
)

// nullable helpers for query parameters.
func NullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func NullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

func NullUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}
