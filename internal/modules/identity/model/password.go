package model

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
)

// Argon2Params are argon2id cost parameters.
type Argon2Params struct {
	MemoryKiB   uint32
	Iterations  uint32
	Parallelism uint8
	SaltLen     uint32
	KeyLen      uint32
}

// DefaultArgon2 follows OWASP guidance (docs/security/security-model.md).
var DefaultArgon2 = Argon2Params{MemoryKiB: 64 * 1024, Iterations: 3, Parallelism: 2, SaltLen: 16, KeyLen: 32}

var ErrBadHash = errors.New("identity: malformed password hash")

// Password policy (NIST SP 800-63B style: length over composition rules).
const (
	MinPasswordLen = 12
	MaxPasswordLen = 128
)

// ValidatePassword returns field errors for an unacceptable password.
func ValidatePassword(field, password, email string) []problem.FieldError {
	n := utf8.RuneCountInString(password)
	switch {
	case n < MinPasswordLen:
		return []problem.FieldError{{Field: field, Code: "too_short", Message: fmt.Sprintf("Use at least %d characters.", MinPasswordLen)}}
	case n > MaxPasswordLen:
		return []problem.FieldError{{Field: field, Code: "too_long", Message: fmt.Sprintf("Use at most %d characters.", MaxPasswordLen)}}
	case email != "" && strings.EqualFold(password, email):
		return []problem.FieldError{{Field: field, Code: "matches_email", Message: "The password cannot be your email address."}}
	}
	return nil
}
