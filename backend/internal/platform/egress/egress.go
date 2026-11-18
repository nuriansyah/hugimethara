// Package egress provides the only HTTP client allowed to call user-supplied
// destinations. It blocks SSRF targets (loopback, private, link-local,
// metadata, multicast…) at dial time, after DNS resolution, so DNS rebinding
// cannot bypass the check. Admin-approved destinations are allowed through an
// explicit allowlist (needed for internal services in the Service Registry).
package egress

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"syscall"
	"time"
)

// ErrBlockedDestination is returned (wrapped) when a dial targets a denied address.
var ErrBlockedDestination = errors.New("egress: destination address is not allowed")

// Ranges denied unless explicitly allowlisted.
var deniedPrefixes = mustPrefixes(
	"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16",
	"172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.168.0.0/16", "198.18.0.0/15",
	"198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4", "255.255.255.255/32",
	"::/128", "::1/128", "64:ff9b::/96", "100::/64", "2001:db8::/32", "fc00::/7", "fe80::/10", "ff00::/8",
)

// Policy decides which addresses may be dialed.
type Policy struct {
	// Allow lists CIDRs that are permitted even if they fall in a denied range.
	Allow []netip.Prefix
}

// ParsePolicy builds a Policy from CIDR strings (single IPs are accepted as /32 or /128).
func ParsePolicy(allow []string) (Policy, error) {
	var p Policy
	for _, s := range allow {
		s = strings.TrimSpace(s)
		if !strings.Contains(s, "/") {
			addr, err := netip.ParseAddr(s)
			if err != nil {
				return Policy{}, fmt.Errorf("egress: invalid allowlist entry %q", s)
			}
			p.Allow = append(p.Allow, netip.PrefixFrom(addr, addr.BitLen()))
			continue
		}
		pfx, err := netip.ParsePrefix(s)
		if err != nil {
			return Policy{}, fmt.Errorf("egress: invalid allowlist entry %q", s)
		}
		p.Allow = append(p.Allow, pfx.Masked())
	}
	return p, nil
}

// Permits reports whether addr may be dialed under this policy.
func (p Policy) Permits(addr netip.Addr) bool {
	addr = addr.Unmap()
	for _, a := range p.Allow {
		if a.Contains(addr) {
			return true
		}
	}
	if !addr.IsValid() {
		return false
	}
	for _, d := range deniedPrefixes {
		if d.Contains(addr) {
			return false
		}
	}
	return true
}

// Options configures a client.
type Options struct {
	Policy           Policy
	Timeout          time.Duration // total request timeout; default 30s
	MaxRedirects     int           // default 5
	MaxResponseBytes int64         // default 10 MiB; enforced by LimitBody
}

// NewClient returns an *http.Client whose every connection (including redirect
// hops and proxies, which are disabled) is checked against the policy.
func NewClient(o Options) *http.Client {
	if o.Timeout == 0 {
		o.Timeout = 30 * time.Second
	}
	if o.MaxRedirects == 0 {
		o.MaxRedirects = 5
	}
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			ap, err := netip.ParseAddrPort(address)
			if err != nil {
				return fmt.Errorf("%w: %s", ErrBlockedDestination, address)
			}
			if !o.Policy.Permits(ap.Addr()) {
				return fmt.Errorf("%w: %s", ErrBlockedDestination, ap.Addr())
			}
			return nil
		},
	}
	tr := &http.Transport{
		Proxy:                 nil, // a proxy would bypass the dial-time check
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: o.Timeout,
	}
	return &http.Client{
		Transport: tr,
		Timeout:   o.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= o.MaxRedirects {
				return fmt.Errorf("egress: stopped after %d redirects", o.MaxRedirects)
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("egress: redirect to unsupported scheme %q", req.URL.Scheme)
			}
			return nil
		},
	}
}

// ErrResponseTooLarge is returned by a LimitBody reader once the cap is exceeded.
var ErrResponseTooLarge = errors.New("egress: response body exceeds size limit")

// LimitBody wraps a response body so reads fail after max bytes.
func LimitBody(body io.ReadCloser, max int64) io.ReadCloser {
	if max <= 0 {
		max = 10 << 20
	}
	return &limitedBody{rc: body, remaining: max}
}

type limitedBody struct {
	rc        io.ReadCloser
	remaining int64
}

func (l *limitedBody) Read(p []byte) (int, error) {
	if l.remaining < 0 {
		return 0, ErrResponseTooLarge
	}
	if int64(len(p)) > l.remaining+1 {
		p = p[:l.remaining+1]
	}
	n, err := l.rc.Read(p)
	l.remaining -= int64(n)
	if l.remaining < 0 {
		return n - int(-l.remaining), ErrResponseTooLarge
	}
	return n, err
}

func (l *limitedBody) Close() error { return l.rc.Close() }

// IsBlocked reports whether err was caused by the SSRF guard.
func IsBlocked(err error) bool { return errors.Is(err, ErrBlockedDestination) }

func mustPrefixes(ss ...string) []netip.Prefix {
	out := make([]netip.Prefix, len(ss))
	for i, s := range ss {
		out[i] = netip.MustParsePrefix(s)
	}
	return out
}
