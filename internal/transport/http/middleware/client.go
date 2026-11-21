package middleware

import (
	"net"
	"net/http"
	"net/netip"
	"strings"

	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

// ClientInfoMiddleware records the client IP and user agent. X-Forwarded-For is
// honoured only when the direct peer is a trusted proxy; the client is the
// right-most address that is not itself a trusted proxy.
func ClientInfoMiddleware(trusted []netip.Prefix) func(http.Handler) http.Handler {
	isTrusted := func(a netip.Addr) bool {
		for _, p := range trusted {
			if p.Contains(a) {
				return true
			}
		}
		return false
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ua := r.UserAgent()
			if len(ua) > 512 {
				ua = ua[:512]
			}
			ci := principal.ClientInfo{UserAgent: ua, IP: remoteAddr(r)}
			if ci.IP.IsValid() && isTrusted(ci.IP) {
				hops := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
				for i := len(hops) - 1; i >= 0; i-- {
					a, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
					if err != nil {
						break
					}
					ci.IP = a.Unmap()
					if !isTrusted(ci.IP) {
						break
					}
				}
			}
			next.ServeHTTP(w, r.WithContext(principal.WithClientInfo(r.Context(), ci)))
		})
	}
}

func remoteAddr(r *http.Request) netip.Addr {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	a, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return a.Unmap()
}

// OriginGuard rejects state-changing browser requests whose Origin is neither
// the request's own host nor a trusted web origin (defence against CSRF and
// login CSRF, alongside SameSite cookies and the session CSRF token).
// Requests without an Origin header (non-browser clients) pass.
func OriginGuard(trustedOrigins []string) func(http.Handler) http.Handler {
	trusted := make(map[string]bool, len(trustedOrigins))
	for _, o := range trustedOrigins {
		trusted[strings.ToLower(o)] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if IsSafeMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}
			origin := strings.ToLower(r.Header.Get("Origin"))
			if origin == "" || trusted[origin] || origin == "http://"+strings.ToLower(r.Host) || origin == "https://"+strings.ToLower(r.Host) {
				next.ServeHTTP(w, r)
				return
			}
			problem.WriteError(w, r, problem.NewProblem(http.StatusForbidden, "origin_not_allowed", "Origin not allowed",
				"Cross-site requests are not accepted."))
		})
	}
}

// IsSafeMethod reports whether m is a read-only HTTP method.
func IsSafeMethod(m string) bool {
	return m == http.MethodGet || m == http.MethodHead || m == http.MethodOptions
}
