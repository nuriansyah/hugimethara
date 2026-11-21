package principal

import (
	"context"
	"net/netip"
)

// ClientInfo describes the caller as seen by the API.
type ClientInfo struct {
	IP        netip.Addr // invalid when unknown
	UserAgent string
}

type clientKey struct{}

// ClientInfoFrom returns the client info stored by the ClientInfoMiddleware.
func ClientInfoFrom(ctx context.Context) ClientInfo {
	ci, _ := ctx.Value(clientKey{}).(ClientInfo)
	return ci
}

// WithClientInfo stores ci in ctx (used by tests and background callers).
func WithClientInfo(ctx context.Context, ci ClientInfo) context.Context {
	return context.WithValue(ctx, clientKey{}, ci)
}
