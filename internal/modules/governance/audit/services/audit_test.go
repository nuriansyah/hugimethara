package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	logging "github.com/nuriansyah/hugimethara/internal/platform/logger"
)

func TestRedactNested(t *testing.T) {
	out := redact(map[string]any{
		"name":   "Acme",
		"secret": "x",
		"config": map[string]any{"client_secret": "y", "region": "eu"},
	})
	require.Equal(t, "Acme", out["name"])
	require.Equal(t, logging.Redacted, out["secret"])
	nested := out["config"].(map[string]any)
	require.Equal(t, logging.Redacted, nested["client_secret"])
	require.Equal(t, "eu", nested["region"])
	require.NotNil(t, redact(nil))
}
