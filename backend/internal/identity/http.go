package identity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/nuriansyah/hugimethara/backend/internal/api/apigen"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/httpx"
	"github.com/nuriansyah/hugimethara/backend/internal/principal"
)

// CSRFHeader carries the CSRF token on state-changing requests.
const CSRFHeader = "X-CSRF-Token"

// Cookies names and attributes. With Secure, the __Host- prefix pins the
// cookies to this exact host, HTTPS and path "/".
type Cookies struct{ Secure bool }

func (c Cookies) SessionName() string { return c.prefix() + "hugi_session" }
func (c Cookies) CSRFName() string    { return c.prefix() + "hugi_csrf" }
func (c Cookies) prefix() string {
	if c.Secure {
		return "__Host-"
	}
	return ""
}

// Secure is config-driven: false only for plain-HTTP local development, and
// config.Load refuses it in production. The CSRF cookie is deliberately not
// HttpOnly — the page must read it to echo it in X-CSRF-Token.
//
//nolint:gosec // G124: see above
func (c Cookies) issue(g SessionGrant) []*http.Cookie {
	return []*http.Cookie{
		{Name: c.SessionName(), Value: g.Token, Path: "/", Expires: g.ExpiresAt, HttpOnly: true, Secure: c.Secure, SameSite: http.SameSiteLaxMode},
		{Name: c.CSRFName(), Value: g.CSRFToken, Path: "/", Expires: g.ExpiresAt, HttpOnly: false, Secure: c.Secure, SameSite: http.SameSiteLaxMode},
	}
}

//nolint:gosec // G124: same reasoning as issue
func (c Cookies) clear() []*http.Cookie {
	return []*http.Cookie{
		{Name: c.SessionName(), Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(0, 0), HttpOnly: true, Secure: c.Secure, SameSite: http.SameSiteLaxMode},
		{Name: c.CSRFName(), Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(0, 0), Secure: c.Secure, SameSite: http.SameSiteLaxMode},
	}
}

// HTTP implements the auth slice of apigen.StrictServerInterface plus the
// session middleware.
type HTTP struct {
	svc     *Service
	cookies Cookies
}

func NewHTTP(svc *Service, cookies Cookies) *HTTP { return &HTTP{svc: svc, cookies: cookies} }

// Middleware resolves the session cookie into a principal and enforces the
// CSRF token on state-changing requests made with a session.
func (h *HTTP) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(h.cookies.SessionName())
		if err != nil || c.Value == "" {
			next.ServeHTTP(w, r)
			return
		}
		p, csrfHash, err := h.svc.Authenticate(r.Context(), c.Value)
		if errors.Is(err, errNoSession) {
			// Stale or revoked cookie: drop it and continue unauthenticated.
			for _, ck := range h.cookies.clear() {
				http.SetCookie(w, ck)
			}
			next.ServeHTTP(w, r)
			return
		}
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		if !httpx.IsSafeMethod(r.Method) && !VerifyCSRF(r.Header.Get(CSRFHeader), csrfHash) {
			httpx.WriteError(w, r, httpx.NewProblem(http.StatusForbidden, "csrf_failed", "Request blocked",
				"The CSRF token is missing or invalid. Reload the page and try again."))
			return
		}
		next.ServeHTTP(w, r.WithContext(principal.With(r.Context(), p)))
	})
}

// sessionResponse writes JSON plus session cookies; it satisfies the
// generated response interfaces for signup and login.
type sessionResponse struct {
	status  int
	body    any
	cookies []*http.Cookie
}

func (r sessionResponse) visit(w http.ResponseWriter) error {
	for _, c := range r.cookies {
		http.SetCookie(w, c)
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r.body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(r.status)
	_, err := w.Write(buf.Bytes())
	return err
}

func (r sessionResponse) VisitCreateOrganizationResponse(w http.ResponseWriter) error {
	return r.visit(w)
}
func (r sessionResponse) VisitLoginResponse(w http.ResponseWriter) error { return r.visit(w) }

type logoutResponse struct{ cookies []*http.Cookie }

func (r logoutResponse) VisitLogoutResponse(w http.ResponseWriter) error {
	for _, c := range r.cookies {
		http.SetCookie(w, c)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *HTTP) CreateOrganization(ctx context.Context, req apigen.CreateOrganizationRequestObject) (apigen.CreateOrganizationResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.BadRequest("A JSON body is required.")
	}
	g, err := h.svc.Signup(ctx, SignupInput{
		OrganizationName: req.Body.OrganizationName, OrganizationSlug: req.Body.OrganizationSlug,
		DisplayName: req.Body.DisplayName, Email: req.Body.Email, Password: req.Body.Password,
	})
	if err != nil {
		return nil, err
	}
	return sessionResponse{status: http.StatusCreated, body: authResponse(g), cookies: h.cookies.issue(g)}, nil
}

func (h *HTTP) Login(ctx context.Context, req apigen.LoginRequestObject) (apigen.LoginResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.BadRequest("A JSON body is required.")
	}
	in := LoginInput{Email: req.Body.Email, Password: req.Body.Password}
	if req.Body.OrganizationSlug != nil {
		in.OrganizationSlug = *req.Body.OrganizationSlug
	}
	g, err := h.svc.Login(ctx, in)
	if err != nil {
		return nil, err
	}
	return sessionResponse{status: http.StatusOK, body: authResponse(g), cookies: h.cookies.issue(g)}, nil
}

func (h *HTTP) Logout(ctx context.Context, _ apigen.LogoutRequestObject) (apigen.LogoutResponseObject, error) {
	if err := h.svc.Logout(ctx); err != nil {
		return nil, err
	}
	return logoutResponse{cookies: h.cookies.clear()}, nil
}

func (h *HTTP) GetMe(ctx context.Context, _ apigen.GetMeRequestObject) (apigen.GetMeResponseObject, error) {
	m, err := h.svc.Me(ctx)
	if err != nil {
		return nil, err
	}
	org, err := h.svc.orgs.Current(ctx)
	if err != nil {
		return nil, err
	}
	p, _ := principal.From(ctx)
	var cur SessionInfo
	sessions, err := h.svc.ListSessions(ctx)
	if err != nil {
		return nil, err
	}
	for _, s := range sessions {
		if s.ID == p.SessionID {
			cur = s
		}
	}
	out := apigen.GetMe200JSONResponse{
		User:         apiUser(m.User),
		Organization: apigen.OrganizationSummary{Id: org.ID, Slug: org.Slug, Name: org.Name},
		Session:      apigen.SessionSummary{Id: p.SessionID, ExpiresAt: cur.ExpiresAt, IdleExpiresAt: cur.IdleExpiresAt},
	}
	out.Membership.IsOwner = m.IsOwner
	out.Membership.JoinedAt = m.JoinedAt
	return out, nil
}

func (h *HTTP) ListMySessions(ctx context.Context, _ apigen.ListMySessionsRequestObject) (apigen.ListMySessionsResponseObject, error) {
	sessions, err := h.svc.ListSessions(ctx)
	if err != nil {
		return nil, err
	}
	out := apigen.ListMySessions200JSONResponse{Data: make([]apigen.SessionInfo, len(sessions))}
	for i, s := range sessions {
		out.Data[i] = apigen.SessionInfo{
			Id: s.ID, Current: s.Current, CreatedAt: s.CreatedAt, LastSeenAt: s.LastSeenAt,
			IdleExpiresAt: s.IdleExpiresAt, ExpiresAt: s.ExpiresAt, Ip: strPtr(s.IP), UserAgent: strPtr(s.UserAgent),
		}
	}
	return out, nil
}

func (h *HTTP) RevokeMySession(ctx context.Context, req apigen.RevokeMySessionRequestObject) (apigen.RevokeMySessionResponseObject, error) {
	if err := h.svc.RevokeSession(ctx, req.SessionId); err != nil {
		return nil, err
	}
	return apigen.RevokeMySession204Response{}, nil
}

func authResponse(g SessionGrant) apigen.AuthResponse {
	return apigen.AuthResponse{
		User:         apiUser(g.User),
		Organization: apigen.OrganizationSummary{Id: g.Organization.ID, Slug: g.Organization.Slug, Name: g.Organization.Name},
		Session:      apigen.SessionSummary{Id: g.SessionID, ExpiresAt: g.ExpiresAt, IdleExpiresAt: g.IdleExpiresAt},
	}
}

func apiUser(u User) apigen.User {
	return apigen.User{Id: u.ID, Email: u.Email, DisplayName: u.DisplayName, Status: apigen.UserStatus(u.Status)}
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
