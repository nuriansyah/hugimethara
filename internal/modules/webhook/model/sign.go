package model

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
)

// StandardSecretPrefix prefixes Standard Webhooks secrets.
const StandardSecretPrefix = "whsec_"

// StandardKey extracts the HMAC key from a "whsec_<base64>" secret.
func StandardKey(secret []byte) ([]byte, error) {
	s, ok := strings.CutPrefix(string(secret), StandardSecretPrefix)
	if !ok {
		return nil, errors.New("webhook: secret must start with " + StandardSecretPrefix)
	}
	key, err := base64.StdEncoding.DecodeString(s)
	if err != nil || len(key) < 24 || len(key) > 64 {
		return nil, errors.New("webhook: secret must be whsec_ followed by 24–64 base64-encoded bytes")
	}
	return key, nil
}

// GenerateSecret creates a signing secret for scheme: a Standard Webhooks
// "whsec_" secret for standard_webhooks (and outbound subscriptions), a
// 64-character hex string for generic HMAC and the external-tool token presets.
func GenerateSecret(scheme Scheme) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	if scheme == SchemeHMAC || scheme.IsToken() {
		return hex.EncodeToString(b), nil
	}
	return StandardSecretPrefix + base64.StdEncoding.EncodeToString(b), nil
}

// ValidateSecret checks a caller-supplied secret for scheme.
func ValidateSecret(scheme Scheme, secret string) error {
	switch {
	case len(secret) < 16:
		return invalidField("secret", "too_short", "The secret must be at least 16 characters.")
	case len(secret) > 512:
		return invalidField("secret", "too_long", "The secret must be at most 512 characters.")
	}
	if scheme == SchemeStandardWebhooks {
		if _, err := StandardKey([]byte(secret)); err != nil {
			return invalidField("secret", "invalid_secret", err.Error()+".")
		}
	}
	return nil
}

// SecretHint is the non-sensitive suffix shown in the UI.
func SecretHint(secret string) string {
	if len(secret) <= 8 {
		return ""
	}
	return "…" + secret[len(secret)-4:]
}

// NewPublicID returns the unguessable path segment of an ingress URL.
func NewPublicID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Signature is a Standard Webhooks signature header value ("v1,<base64>")
// over "<msgID>.<unix timestamp>.<body>".
func Signature(secret []byte, msgID string, at time.Time, body []byte) (string, error) {
	key, err := StandardKey(secret)
	if err != nil {
		return "", err
	}
	ts := strconv.FormatInt(at.Unix(), 10)
	return "v1," + base64.StdEncoding.EncodeToString(mac(key, []byte(msgID+"."+ts+"."), body)), nil
}

// SignedHeaders returns the Standard Webhooks headers for an outbound delivery.
func SignedHeaders(secret []byte, msgID string, at time.Time, body []byte) (map[string]string, error) {
	sig, err := Signature(secret, msgID, at, body)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"Webhook-Id":        msgID,
		"Webhook-Timestamp": strconv.FormatInt(at.Unix(), 10),
		"Webhook-Signature": sig,
	}, nil
}
