package identity

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/nuriansyah/hugimethara/backend/internal/platform/httpx"
)

var cheap = Argon2Params{MemoryKiB: 1024, Iterations: 1, Parallelism: 1, SaltLen: 16, KeyLen: 32}

func TestPasswordHashRoundTrip(t *testing.T) {
	h := NewHasher(cheap)
	enc, err := h.Hash("correct horse battery")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(enc, "$argon2id$v=19$m=1024,t=1,p=1$"))

	ok, err := h.Verify("correct horse battery", enc)
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = h.Verify("correct horse batterY", enc)
	require.NoError(t, err)
	require.False(t, ok)

	enc2, _ := h.Hash("correct horse battery")
	require.NotEqual(t, enc, enc2, "salts must differ")

	// Hashes made with other parameters still verify (params are read from the hash).
	ok, err = NewHasher(Argon2Params{MemoryKiB: 2048, Iterations: 2, Parallelism: 1, SaltLen: 16, KeyLen: 32}).Verify("correct horse battery", enc)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestVerifyRejectsMalformedHashes(t *testing.T) {
	h := NewHasher(cheap)
	for _, bad := range []string{"", "plaintext", "$bcrypt$x$y$z$w", "$argon2id$v=18$m=1,t=1,p=1$AAAA$AAAA", "$argon2id$v=19$m=1024,t=1,p=1$AAAAAAAAAAA$AA"} {
		_, err := h.Verify("x", bad)
		require.Error(t, err, bad)
	}
}

func TestValidatePassword(t *testing.T) {
	require.Nil(t, ValidatePassword("p", "correct horse battery", "a@b.test"))
	require.Equal(t, "too_short", ValidatePassword("p", "short", "")[0].Code)
	require.Equal(t, "too_long", ValidatePassword("p", strings.Repeat("x", 129), "")[0].Code)
	require.Equal(t, "matches_email", ValidatePassword("p", "Someone@Example.test", "someone@example.test")[0].Code)
	require.Nil(t, ValidatePassword("p", "ééééééééééé1", ""), "length counts runes, not bytes")
}

func TestTokensAndCSRF(t *testing.T) {
	tok, h := newToken()
	require.Len(t, tok, 43)
	require.Equal(t, h, hashToken(tok))
	other, _ := newToken()
	require.NotEqual(t, tok, other)

	require.True(t, VerifyCSRF(tok, h))
	require.False(t, VerifyCSRF(other, h))
	require.False(t, VerifyCSRF("", h))
	require.False(t, VerifyCSRF(tok, nil))
}

func TestChooseOrg(t *testing.T) {
	a := orgChoice{ID: uuid.New(), Slug: "a", Name: "A"}
	b := orgChoice{ID: uuid.New(), Slug: "b", Name: "B"}

	_, err := chooseOrg(nil, "")
	require.ErrorIs(t, err, ErrNoActiveOrganization)

	got, err := chooseOrg([]orgChoice{a}, "")
	require.NoError(t, err)
	require.Equal(t, "a", got.Slug)

	_, err = chooseOrg([]orgChoice{a, b}, "")
	var choice *OrganizationChoiceRequired
	require.True(t, errors.As(err, &choice))
	require.Len(t, choice.Choices, 2)
	p := choice.Problem()
	require.Equal(t, 409, p.Status)
	require.Len(t, p.Extensions["organizations"], 2)

	got, err = chooseOrg([]orgChoice{a, b}, "b")
	require.NoError(t, err)
	require.Equal(t, b.ID, got.ID)

	// Unknown slug: offer the real choices rather than failing opaquely.
	_, err = chooseOrg([]orgChoice{a, b}, "zzz")
	require.True(t, errors.As(err, &choice))
}

func TestValidEmail(t *testing.T) {
	for _, ok := range []string{"a@b.co", "first.last+tag@sub.example.test"} {
		require.True(t, validEmail(ok), ok)
	}
	for _, bad := range []string{"", "a@b", "a b@c.d", "<a@b.c>", "Name <a@b.c>", "a@", strings.Repeat("a", 250) + "@b.co"} {
		require.False(t, validEmail(bad), bad)
	}
}

func TestProblemsAreHTTPProblems(t *testing.T) {
	var p *httpx.Problem
	require.True(t, errors.As(error(ErrInvalidCredentials), &p))
	require.Equal(t, 401, p.Status)
}
