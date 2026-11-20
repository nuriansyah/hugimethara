// Package secretbox seals small values with AES-256-GCM. It protects data the
// platform needs to read back (identity-provider client secrets and the OIDC
// state cookie) until the envelope-encryption vault replaces it in S6.
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
)

// KeySize is the required key length in bytes (AES-256).
const KeySize = 32

// Key seals and opens values with one AES-256-GCM key.
type Key struct {
	aead cipher.AEAD
}

// DeriveKey turns a passphrase into a key. Use it only for development
// defaults; production deployments supply a random 32-byte key.
func DeriveKey(passphrase string) []byte {
	sum := sha256.Sum256([]byte(passphrase))
	return sum[:]
}

// New validates key and returns a sealer. Nonces are random per value and
// prepended to the ciphertext, so the same plaintext seals differently each time.
func New(key []byte) (*Key, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("secretbox: key must be %d bytes", KeySize)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("secretbox: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secretbox: %w", err)
	}
	return &Key{aead: aead}, nil
}

// Seal encrypts and authenticates plaintext. The returned value is nonce||ciphertext.
func (k *Key) Seal(plaintext []byte) ([]byte, error) {
	return k.SealWithAAD(plaintext, nil)
}

// SealWithAAD binds ciphertext to caller-provided context such as tenant and
// record identifiers, preventing valid ciphertext from being moved elsewhere.
func (k *Key) SealWithAAD(plaintext, aad []byte) ([]byte, error) {
	nonce := make([]byte, k.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("secretbox: nonce: %w", err)
	}
	return k.aead.Seal(nonce, nonce, plaintext, aad), nil
}

// Open decrypts and authenticates a value produced by Seal.
func (k *Key) Open(sealed []byte) ([]byte, error) {
	return k.OpenWithAAD(sealed, nil)
}

// OpenWithAAD authenticates both ciphertext and its caller-provided context.
func (k *Key) OpenWithAAD(sealed, aad []byte) ([]byte, error) {
	if len(sealed) < k.aead.NonceSize() {
		return nil, errors.New("secretbox: value is truncated")
	}
	nonce, ciphertext := sealed[:k.aead.NonceSize()], sealed[k.aead.NonceSize():]
	plaintext, err := k.aead.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, errors.New("secretbox: value failed authentication")
	}
	return plaintext, nil
}
