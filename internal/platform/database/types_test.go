package db

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTextArrayScansPostgresTextForm(t *testing.T) {
	var a TextArray
	require.NoError(t, a.Scan(`{openid,email,"with space"}`))
	require.Equal(t, TextArray{"openid", "email", "with space"}, a)
	require.NoError(t, a.Scan([]byte(`{}`)))
	require.Empty(t, a)
	require.NoError(t, a.Scan(nil))
	require.Nil(t, a)
}
