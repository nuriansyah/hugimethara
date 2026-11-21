package services

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnvironmentPolicyParsesAllowlistAndFailsClosed(t *testing.T) {
	service := &ServiceRegistryService{}
	policy, err := service.environmentPolicy([]byte(`["127.0.0.1/32"]`))
	require.NoError(t, err)
	require.True(t, policy.Permits(netip.MustParseAddr("127.0.0.1")))
	require.False(t, policy.Permits(netip.MustParseAddr("10.0.0.1")))

	for _, invalid := range [][]byte{nil, []byte(`null`), []byte(`["not-a-cidr"]`)} {
		_, err := service.environmentPolicy(invalid)
		require.Error(t, err)
	}
}
