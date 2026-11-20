package identity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/nuriansyah/hugimethara/internal/platform/httpx"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
	"github.com/nuriansyah/hugimethara/internal/transport/http/dto"
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

// HTTP implements the auth slice of dto.StrictServerInterface plus the
// session middleware.
type HTTP struct {
	svc     *Service
	cookies Cookies
}

func NewHTTP(svc *Service, cookies Cookies) *HTTP { return &HTTP{svc: svc, cookies: cookies} }

// Middleware resolves the session cookie into a principal and enforces the
// CSRF token on state-changing requests made with a session.
//
// A request carrying an Authorization header is authenticated by API key only
// (service accounts). Bearer credentials are never sent automatically by
// browsers, so those requests need no CSRF token.
func (h *HTTP) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The OIDC callback is public and needs the browser-bound flow cookie;
		// the generated strict handler has no *http.Request, so carry it here.
		if ck, err := r.Cookie(h.cookies.OIDCStateName()); err == nil && ck.Value != "" {
			r = r.WithContext(withOIDCStateCookie(r.Context(), ck.Value))
		}
		if authz := r.Header.Get("Authorization"); authz != "" {
			key, ok := strings.CutPrefix(authz, "Bearer ")
			if !ok {
				httpx.WriteError(w, r, ErrAPIKeyRejected)
				return
			}
			p, err := h.svc.AuthenticateAPIKey(r.Context(), strings.TrimSpace(key))
			if errors.Is(err, errInvalidAPIKey) {
				httpx.WriteError(w, r, ErrAPIKeyRejected)
				return
			}
			if err != nil {
				httpx.WriteError(w, r, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(principal.With(r.Context(), p)))
			return
		}
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

// @Summary Sign up — create an organization and its first owner, and sign in
// @ID createOrganization
// @Tags auth
// @Param body body dto.SignupRequest true "Request body"
// @Success 201 {object} dto.AuthResponse "Organization created; the caller is signed in."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 429 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Router /v1/hugimethara/organizations [post]
func (h *HTTP) CreateOrganization(ctx context.Context, req dto.CreateOrganizationRequestObject) (dto.CreateOrganizationResponseObject, error) {
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

// @Summary Sign in with email and password
// @ID login
// @Tags auth
// @Param body body dto.LoginRequest true "Request body"
// @Success 200 {object} dto.AuthResponse "Signed in."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.OrganizationChoiceProblem "The user must choose an organization."
// @Failure 429 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Router /v1/hugimethara/auth/login [post]
func (h *HTTP) Login(ctx context.Context, req dto.LoginRequestObject) (dto.LoginResponseObject, error) {
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

// @Summary Sign out (revoke the current session)
// @ID logout
// @Tags auth
// @Success 204 "Signed out; cookies cleared."
// @Produce json
// @Router /v1/hugimethara/auth/logout [post]
func (h *HTTP) Logout(ctx context.Context, _ dto.LogoutRequestObject) (dto.LogoutResponseObject, error) {
	if err := h.svc.Logout(ctx); err != nil {
		return nil, err
	}
	return logoutResponse{cookies: h.cookies.clear()}, nil
}

// @Summary The signed-in user, their organization and session
// @ID getMe
// @Tags auth
// @Success 200 {object} dto.Me "Current identity."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/me [get]
func (h *HTTP) GetMe(ctx context.Context, _ dto.GetMeRequestObject) (dto.GetMeResponseObject, error) {
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
	out := dto.GetMe200JSONResponse{
		User:         apiUser(m.User),
		Organization: dto.OrganizationSummary{Id: org.ID, Slug: org.Slug, Name: org.Name},
		Session:      dto.SessionSummary{Id: p.SessionID, ExpiresAt: cur.ExpiresAt, IdleExpiresAt: cur.IdleExpiresAt},
	}
	out.Membership.IsOwner = m.IsOwner
	out.Membership.JoinedAt = m.JoinedAt
	return out, nil
}

// @Summary The caller's active sessions in this organization
// @ID listMySessions
// @Tags auth
// @Success 200 {object} dto.ListMySessions200JSONResponse "Active sessions, most recently used first."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/me/sessions [get]
func (h *HTTP) ListMySessions(ctx context.Context, _ dto.ListMySessionsRequestObject) (dto.ListMySessionsResponseObject, error) {
	sessions, err := h.svc.ListSessions(ctx)
	if err != nil {
		return nil, err
	}
	out := dto.ListMySessions200JSONResponse{Data: make([]dto.SessionInfo, len(sessions))}
	for i, s := range sessions {
		out.Data[i] = dto.SessionInfo{
			Id: s.ID, Current: s.Current, CreatedAt: s.CreatedAt, LastSeenAt: s.LastSeenAt,
			IdleExpiresAt: s.IdleExpiresAt, ExpiresAt: s.ExpiresAt, Ip: strPtr(s.IP), UserAgent: strPtr(s.UserAgent),
		}
	}
	return out, nil
}

// @Summary Revoke one of the caller's sessions
// @ID revokeMySession
// @Tags auth
// @Param session_id path string true "session_id"
// @Success 204 "Session revoked."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/me/sessions/{session_id} [delete]
func (h *HTTP) RevokeMySession(ctx context.Context, req dto.RevokeMySessionRequestObject) (dto.RevokeMySessionResponseObject, error) {
	if err := h.svc.RevokeSession(ctx, req.SessionId); err != nil {
		return nil, err
	}
	return dto.RevokeMySession204Response{}, nil
}

func authResponse(g SessionGrant) dto.AuthResponse {
	return dto.AuthResponse{
		User:         apiUser(g.User),
		Organization: dto.OrganizationSummary{Id: g.Organization.ID, Slug: g.Organization.Slug, Name: g.Organization.Name},
		Session:      dto.SessionSummary{Id: g.SessionID, ExpiresAt: g.ExpiresAt, IdleExpiresAt: g.IdleExpiresAt},
	}
}

func apiUser(u User) dto.User {
	return dto.User{Id: u.ID, Email: u.Email, DisplayName: u.DisplayName, Status: dto.UserStatus(u.Status)}
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
