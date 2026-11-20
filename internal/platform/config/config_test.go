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
	require.Equal(t, "env-v1", c.MasterKeyID)
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

func TestMasterKey(t *testing.T) {
	c, err := Load("hugi-test")
	require.NoError(t, err)
	key, defaulted, err := c.MasterKeyBytes()
	require.NoError(t, err)
	require.True(t, defaulted)
	require.Len(t, key, 32)

	t.Setenv("HUGI_MASTER_KEY", "not base64!")
	_, err = Load("hugi-test")
	require.ErrorContains(t, err, "HUGI_MASTER_KEY")

	t.Setenv("HUGI_MASTER_KEY", "c2hvcnQ=") // "short"
	_, err = Load("hugi-test")
	require.ErrorContains(t, err, "HUGI_MASTER_KEY")

	// 32 bytes, base64.
	t.Setenv("HUGI_MASTER_KEY", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	c, err = Load("hugi-test")
	require.NoError(t, err)
	key, defaulted, err = c.MasterKeyBytes()
	require.NoError(t, err)
	require.False(t, defaulted)
	require.Len(t, key, 32)
}

func TestMasterKeyRequiredInProduction(t *testing.T) {
	t.Setenv("HUGI_ENV", "production")
	_, err := Load("hugi-test")
	require.ErrorContains(t, err, "HUGI_MASTER_KEY")
}
