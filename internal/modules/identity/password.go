package identity

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"

	"github.com/nuriansyah/hugimethara/internal/platform/httpx"
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

// Hasher hashes and verifies passwords as PHC strings:
// $argon2id$v=19$m=65536,t=3,p=2$<salt>$<key>
type Hasher struct {
	P     Argon2Params
	dummy string // hash used to equalize timing for unknown accounts
}

func NewHasher(p Argon2Params) *Hasher {
	h := &Hasher{P: p}
	h.dummy, _ = h.Hash("dummy-password-for-timing-equalization")
	return h
}

// Hash returns the PHC encoding of password.
func (h *Hasher) Hash(password string) (string, error) {
	salt := make([]byte, h.P.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, h.P.Iterations, h.P.MemoryKiB, h.P.Parallelism, h.P.KeyLen)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version,
		h.P.MemoryKiB, h.P.Iterations, h.P.Parallelism, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

var errBadHash = errors.New("identity: malformed password hash")

// Verify reports whether password matches encoded, in constant time.
func (h *Hasher) Verify(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errBadHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, errBadHash
	}
	var p Argon2Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.MemoryKiB, &p.Iterations, &p.Parallelism); err != nil {
		return false, errBadHash
	}
	b64 := base64.RawStdEncoding
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return false, errBadHash
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil || len(want) < 16 || len(want) > 128 || len(salt) < 8 {
		return false, errBadHash
	}
	got := argon2.IDKey([]byte(password), salt, p.Iterations, p.MemoryKiB, p.Parallelism, uint32(len(want))) //nolint:gosec // G115: len bounded to 128 above
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// BurnTime performs one verification against a dummy hash so that requests
// for unknown accounts take as long as requests for real ones.
func (h *Hasher) BurnTime(password string) { _, _ = h.Verify(password, h.dummy) }

// Password policy (NIST SP 800-63B style: length over composition rules).
const (
	MinPasswordLen = 12
	MaxPasswordLen = 128
)

// ValidatePassword returns field errors for an unacceptable password.
func ValidatePassword(field, password, email string) []httpx.FieldError {
	n := utf8.RuneCountInString(password)
	switch {
	case n < MinPasswordLen:
		return []httpx.FieldError{{Field: field, Code: "too_short", Message: fmt.Sprintf("Use at least %d characters.", MinPasswordLen)}}
	case n > MaxPasswordLen:
		return []httpx.FieldError{{Field: field, Code: "too_long", Message: fmt.Sprintf("Use at most %d characters.", MaxPasswordLen)}}
	case email != "" && strings.EqualFold(password, email):
		return []httpx.FieldError{{Field: field, Code: "matches_email", Message: "The password cannot be your email address."}}
	}
	return nil
}
