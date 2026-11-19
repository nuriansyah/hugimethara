package secretbox

import (
	"bytes"
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSealOpenRoundTrip(t *testing.T) {
	key, err := New(bytes.Repeat([]byte{7}, KeySize))
	require.NoError(t, err)

	first, err := key.Seal([]byte("client-secret"))
	require.NoError(t, err)
	second, err := key.Seal([]byte("client-secret"))
	require.NoError(t, err)
	require.NotEqual(t, first, second, "random nonce per value")

	got, err := key.Open(first)
	require.NoError(t, err)
	require.Equal(t, "client-secret", string(got))
}

func TestOpenRejectsTamperingAndWrongKey(t *testing.T) {
	key, err := New(bytes.Repeat([]byte{7}, KeySize))
	require.NoError(t, err)
	sealed, err := key.Seal([]byte("value"))
	require.NoError(t, err)

	tampered := append([]byte(nil), sealed...)
	tampered[len(tampered)-1] ^= 1
	_, err = key.Open(tampered)
	require.Error(t, err)

	other, err := New(bytes.Repeat([]byte{9}, KeySize))
	require.NoError(t, err)
	_, err = other.Open(sealed)
	require.Error(t, err)

	_, err = key.Open(sealed[:3])
	require.Error(t, err)
}

func TestNewRejectsBadKeySize(t *testing.T) {
	_, err := New([]byte("short"))
	require.Error(t, err)
}

func TestDeriveKey(t *testing.T) {
	a := DeriveKey("master")
	b := DeriveKey("master")
	c := DeriveKey("other")
	require.Equal(t, a, b)
	require.NotEqual(t, a, c)
	require.Len(t, a, KeySize)
	require.Equal(t, sha256.Sum256([]byte("master")), [32]byte(a))
}
