package identity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// newToken returns a 256-bit random URL-safe token and its SHA-256 digest.
// Only the digest is stored; the token is given to the client once.
func newToken() (string, []byte) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("identity: crypto/rand failed: " + err.Error()) // unrecoverable
	}
	t := base64.RawURLEncoding.EncodeToString(b)
	return t, hashToken(t)
}

func hashToken(t string) []byte {
	sum := sha256.Sum256([]byte(t))
	return sum[:]
}
