package identity

import (
	"net/http"
	"time"

	"github.com/nuriansyah/hugimethara/internal/modules/identity/model"
)

// OIDCStateCookie is the exported cookie name suffix for the flow cookie.
func (c Cookies) OIDCStateName() string { return c.prefix() + "hugi_oidc_state" }

func (c Cookies) oidcState(value string, expires time.Time) *http.Cookie {
	//nolint:gosec // G124: HttpOnly + Secure (except plain-HTTP development) + SameSite=Lax.
	return &http.Cookie{
		Name: c.OIDCStateName(), Value: value, Path: "/", Expires: expires, MaxAge: int(model.OidcStateTTL.Seconds()),
		HttpOnly: true, Secure: c.Secure, SameSite: http.SameSiteLaxMode,
	}
}
