package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadDefaults(t *testing.T) {
	c, err := Load("hugi-test")
	require.NoError(t, err)
	require.Equal(t, "development", c.Env)
	require.Equal(t, "hugi-test", c.ServiceName)
	require.Equal(t, ":8080", c.HTTPAddr)
}

func TestLoadRequiresDependencies(t *testing.T) {
	_, err := Load("hugi-test", NeedDatabase, NeedRedis, NeedNATS)
	require.Error(t, err)
	for _, name := range []string{"HUGI_DATABASE_URL", "HUGI_REDIS_URL", "HUGI_NATS_URL"} {
		require.Contains(t, err.Error(), name)
	}
}

func TestInvalidURLDoesNotLeakValue(t *testing.T) {
	t.Setenv("HUGI_DATABASE_URL", "mysql://user:s3cret@host/db")
	_, err := Load("hugi-test", NeedDatabase)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "s3cret")
}

func TestRejectsBadValues(t *testing.T) {
	t.Setenv("HUGI_ENV", "prod")
	t.Setenv("HUGI_LOG_LEVEL", "verbose")
	t.Setenv("HUGI_CORS_ALLOWED_ORIGINS", "http://localhost:3000,http://x.test/path")
	_, err := Load("hugi-test")
	require.Error(t, err)
	msg := err.Error()
	require.True(t, strings.Contains(msg, "HUGI_ENV") && strings.Contains(msg, "HUGI_LOG_LEVEL"))
	require.Contains(t, msg, "x.test/path")
	require.NotContains(t, msg, "localhost:3000\"")
}
