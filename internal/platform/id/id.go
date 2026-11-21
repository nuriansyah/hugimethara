// Package id generates entity identifiers.
//
// Identifiers are UUIDv7 (RFC 9562 §5.7): the leading 48 bits are a Unix
// millisecond timestamp, so new rows land at the right edge of B-tree indexes
// and identifiers sort by creation time, while the remaining 74 random bits keep
// them unguessable across tenants.
package id

import "github.com/google/uuid"

// Generator produces new identifiers; inject it where tests need determinism.
type Generator interface {
	New() uuid.UUID
}

// V7 is the production Generator.
type V7 struct{}

// New returns a new UUIDv7.
func (V7) New() uuid.UUID { return New() }

// New returns a new UUIDv7. It panics only if the system entropy source fails,
// which is unrecoverable for a server process.
func New() uuid.UUID { return uuid.Must(uuid.NewV7()) }

// Parse parses s as a UUID in any RFC 9562 textual form.
func Parse(s string) (uuid.UUID, error) { return uuid.Parse(s) }
