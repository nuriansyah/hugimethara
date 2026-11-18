package egress

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPolicyDeniesSSRFTargets(t *testing.T) {
	p := Policy{}
	for _, s := range []string{
		"127.0.0.1", "10.1.2.3", "172.16.0.1", "192.168.1.1", "169.254.169.254", "100.64.0.1",
		"0.0.0.0", "224.0.0.1", "::1", "fd00::1", "fe80::1", "::ffff:127.0.0.1", "::ffff:169.254.169.254",
	} {
		require.False(t, p.Permits(netip.MustParseAddr(s)), s)
	}
	for _, s := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111"} {
		require.True(t, p.Permits(netip.MustParseAddr(s)), s)
	}
}

func TestAllowlistOverridesDeny(t *testing.T) {
	p, err := ParsePolicy([]string{"10.20.0.0/16", "192.168.1.5"})
	require.NoError(t, err)
	require.True(t, p.Permits(netip.MustParseAddr("10.20.3.4")))
	require.True(t, p.Permits(netip.MustParseAddr("192.168.1.5")))
	require.False(t, p.Permits(netip.MustParseAddr("192.168.1.6")))
	require.False(t, p.Permits(netip.MustParseAddr("169.254.169.254")))

	_, err = ParsePolicy([]string{"not-an-ip"})
	require.Error(t, err)
}

func TestClientBlocksLoopbackIncludingViaRedirect(t *testing.T) {
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "secret metadata")
	}))
	defer internal.Close()

	_, err := NewClient(Options{}).Get(internal.URL)
	require.Error(t, err)
	require.True(t, IsBlocked(err))

	// With loopback allowlisted, the same request goes through.
	allow, _ := ParsePolicy([]string{"127.0.0.1"})
	resp, err := NewClient(Options{Policy: allow}).Get(internal.URL)
	require.NoError(t, err)
	_ = resp.Body.Close()
}

func TestRedirectToBlockedTargetIsBlocked(t *testing.T) {
	// Allow only the redirector's address; its redirect target (a different loopback
	// address) must still be checked on the second dial.
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "should not be reached")
	}))
	defer target.Close()
	redirector := httptest.NewUnstartedServer(http.RedirectHandler(
		strings.Replace(target.URL, "127.0.0.1", "127.0.0.2", 1), http.StatusFound))
	redirector.Start()
	defer redirector.Close()

	allow, _ := ParsePolicy([]string{"127.0.0.1"})
	_, err := NewClient(Options{Policy: allow}).Get(redirector.URL)
	require.Error(t, err)
	require.True(t, IsBlocked(err))
}

func TestLimitBody(t *testing.T) {
	rc := LimitBody(io.NopCloser(strings.NewReader("0123456789")), 5)
	b, err := io.ReadAll(rc)
	require.ErrorIs(t, err, ErrResponseTooLarge)
	require.Equal(t, "01234", string(b))

	rc = LimitBody(io.NopCloser(strings.NewReader("01234")), 5)
	b, err = io.ReadAll(rc)
	require.NoError(t, err)
	require.Equal(t, "01234", string(b))
}
