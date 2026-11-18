// Package identity owns users, local credentials and sessions: signup, login,
// logout, session resolution and session management. (Invitations, groups,
// service accounts and OIDC arrive in S4–S5.)
package identity

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nuriansyah/hugimethara/backend/internal/audit"
	"github.com/nuriansyah/hugimethara/backend/internal/authz"
	"github.com/nuriansyah/hugimethara/backend/internal/event"
	"github.com/nuriansyah/hugimethara/backend/internal/identity/identitydb"
	"github.com/nuriansyah/hugimethara/backend/internal/organization"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/db"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/httpx"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/ratelimit"
	"github.com/nuriansyah/hugimethara/backend/internal/principal"
)

// Errors returned to clients.
var (
	ErrInvalidCredentials = httpx.NewProblem(http.StatusUnauthorized, "invalid_credentials", "Sign-in failed",
		"The email or password is incorrect, or the account is temporarily locked after repeated failures.")
	ErrEmailTaken = httpx.NewProblem(http.StatusConflict, "email_taken", "Email already registered",
		"An account with this email already exists. Sign in instead.")
	ErrNoActiveOrganization = httpx.NewProblem(http.StatusForbidden, "no_active_organization", "No active organization",
		"Your account is not an active member of any organization.")
	errNoSession = errors.New("identity: no valid session")
)

// Rate limits (per window). Lockout in Postgres is the backstop if Redis is down.
var (
	ruleSignupIP  = ratelimit.Rule{Name: "signup_ip", Max: 10, Window: time.Hour}
	ruleLoginIP   = ratelimit.Rule{Name: "login_ip", Max: 30, Window: 5 * time.Minute}
	ruleLoginUser = ratelimit.Rule{Name: "login_email", Max: 10, Window: 5 * time.Minute}
)

// Config tunes the identity service.
type Config struct {
	Hasher          *Hasher
	MaxFailedLogins int           // consecutive failures before lockout; default 10
	LockoutDuration time.Duration // default 15m
	TouchInterval   time.Duration // min interval between last_seen updates; default 1m
}

// Service implements authentication use cases.
type Service struct {
	pool    *pgxpool.Pool
	orgs    *organization.Service
	az      authz.Authorizer
	limiter *ratelimit.Limiter
	log     *slog.Logger
	cfg     Config
	now     func() time.Time
}

func NewService(pool *pgxpool.Pool, orgs *organization.Service, az authz.Authorizer, limiter *ratelimit.Limiter, log *slog.Logger, cfg Config) *Service {
	if cfg.Hasher == nil {
		cfg.Hasher = NewHasher(DefaultArgon2)
	}
	if cfg.MaxFailedLogins <= 0 {
		cfg.MaxFailedLogins = 10
	}
	if cfg.LockoutDuration <= 0 {
		cfg.LockoutDuration = 15 * time.Minute
	}
	if cfg.TouchInterval <= 0 {
		cfg.TouchInterval = time.Minute
	}
	return &Service{pool: pool, orgs: orgs, az: az, limiter: limiter, log: log, cfg: cfg, now: time.Now}
}

// User is a person's identity.
type User struct {
	ID          uuid.UUID
	Email       string
	DisplayName string
	Status      string
}

// OrgSummary identifies an organization a user may sign in to.
type OrgSummary struct {
	ID   uuid.UUID
	Slug string
	Name string
}

// SessionGrant is the result of signup/login: secrets for the cookies plus context.
type SessionGrant struct {
	Token         string // session cookie value (shown once)
	CSRFToken     string // CSRF cookie value
	SessionID     uuid.UUID
	ExpiresAt     time.Time
	IdleExpiresAt time.Time
	User          User
	Organization  OrgSummary
}

// SignupInput creates an organization and its first owner.
type SignupInput struct {
	OrganizationName string
	OrganizationSlug string
	DisplayName      string
	Email            string
	Password         string
}

// Signup registers a new organization with the caller as owner and signs them in.
func (s *Service) Signup(ctx context.Context, in SignupInput) (SessionGrant, error) {
	ci := httpx.ClientInfoFrom(ctx)
	if err := s.limiter.Allow(ctx, ruleSignupIP, ci.IP.String()); err != nil {
		return SessionGrant{}, err
	}
	in.Email = normalizeEmail(in.Email)
	in.OrganizationSlug = strings.ToLower(strings.TrimSpace(in.OrganizationSlug))
	in.OrganizationName = strings.TrimSpace(in.OrganizationName)
	in.DisplayName = strings.TrimSpace(in.DisplayName)

	var errs []httpx.FieldError
	if fe := organization.ValidateName("organization_name", in.OrganizationName); fe != nil {
		errs = append(errs, *fe)
	}
	if fe := organization.ValidateSlug("organization_slug", in.OrganizationSlug); fe != nil {
		errs = append(errs, *fe)
	}
	if n := len([]rune(in.DisplayName)); n < 1 || n > 100 {
		errs = append(errs, httpx.FieldError{Field: "display_name", Code: "invalid_length", Message: "Name must be 1–100 characters."})
	}
	if !validEmail(in.Email) {
		errs = append(errs, httpx.FieldError{Field: "email", Code: "invalid_email", Message: "Enter a valid email address."})
	}
	errs = append(errs, ValidatePassword("password", in.Password, in.Email)...)
	if len(errs) > 0 {
		return SessionGrant{}, httpx.Validation(errs...)
	}

	hash, err := s.cfg.Hasher.Hash(in.Password)
	if err != nil {
		return SessionGrant{}, err
	}
	orgID, userID := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	user := User{ID: userID, Email: in.Email, DisplayName: in.DisplayName, Status: "active"}
	org := OrgSummary{ID: orgID, Slug: in.OrganizationSlug, Name: in.OrganizationName}
	actor := &audit.Actor{Type: string(principal.User), ID: userID}

	var grant SessionGrant
	err = db.InTenantTx(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		if err := s.orgs.Create(ctx, tx, organization.NewOrganization{ID: orgID, Slug: org.Slug, Name: org.Name}); err != nil {
			return err
		}
		q := identitydb.New(tx)
		if err := q.InsertUser(ctx, identitydb.InsertUserParams{ID: userID, Email: user.Email, DisplayName: user.DisplayName}); err != nil {
			if isUniqueViolation(err) {
				return ErrEmailTaken
			}
			return fmt.Errorf("identity: insert user: %w", err)
		}
		if err := q.InsertMembership(ctx, identitydb.InsertMembershipParams{OrgID: orgID, UserID: userID, IsOwner: true}); err != nil {
			return fmt.Errorf("identity: insert membership: %w", err)
		}
		if err := q.InsertCredentials(ctx, identitydb.InsertCredentialsParams{UserID: userID, PasswordHash: hash}); err != nil {
			return fmt.Errorf("identity: insert credentials: %w", err)
		}
		if err := audit.Record(ctx, tx, audit.Event{
			OrgID: orgID, Action: "organization.created", ResourceType: organization.ResourceType,
			ResourceID: orgID.String(), Actor: actor,
			Details: map[string]any{"slug": org.Slug, "name": org.Name, "owner_email": user.Email},
		}); err != nil {
			return err
		}
		if _, err := event.Publish(ctx, tx, event.Event{
			OrgID: orgID, Subject: "hugi.events.organization.created",
			Payload: map[string]any{"organization_id": orgID, "slug": org.Slug, "owner_user_id": userID},
		}); err != nil {
			return err
		}
		grant, err = s.issueSession(ctx, tx, user, org, organization.DefaultSettings(), "signup")
		return err
	})
	if err != nil {
		return SessionGrant{}, err
	}
	s.log.InfoContext(ctx, "organization created", "org_id", orgID, "user_id", userID)
	return grant, nil
}

// LoginInput authenticates with email and password.
type LoginInput struct {
	Email            string
	Password         string
	OrganizationSlug string // required when the user belongs to several orgs
}

// OrganizationChoiceRequired is returned when the user must pick an organization.
type OrganizationChoiceRequired struct{ Choices []OrgSummary }

func (e *OrganizationChoiceRequired) Error() string { return "organization choice required" }

// Problem renders the error with the list of organizations to choose from.
func (e *OrganizationChoiceRequired) Problem() *httpx.Problem {
	p := httpx.NewProblem(http.StatusConflict, "organization_required", "Choose an organization",
		"Your account belongs to several organizations. Choose one to sign in to.")
	choices := make([]map[string]any, len(e.Choices))
	for i, c := range e.Choices {
		choices[i] = map[string]any{"slug": c.Slug, "name": c.Name}
	}
	p.Extensions = map[string]any{"organizations": choices}
	return p
}

// Login verifies credentials and creates a session in the chosen organization.
func (s *Service) Login(ctx context.Context, in LoginInput) (SessionGrant, error) {
	ci := httpx.ClientInfoFrom(ctx)
	in.Email = normalizeEmail(in.Email)
	in.OrganizationSlug = strings.ToLower(strings.TrimSpace(in.OrganizationSlug))
	if err := s.limiter.Allow(ctx, ruleLoginIP, ci.IP.String()); err != nil {
		return SessionGrant{}, err
	}
	if err := s.limiter.Allow(ctx, ruleLoginUser, digest(in.Email)); err != nil {
		return SessionGrant{}, err
	}
	if !validEmail(in.Email) || in.Password == "" || len(in.Password) > 4*MaxPasswordLen {
		s.cfg.Hasher.BurnTime(in.Password)
		return SessionGrant{}, ErrInvalidCredentials
	}

	var (
		rec     loginRecord
		choices []orgChoice
		ok      bool
	)
	err := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		rec, err = loginLookup(ctx, tx, in.Email)
		if errors.Is(err, pgx.ErrNoRows) {
			s.cfg.Hasher.BurnTime(in.Password)
			return ErrInvalidCredentials
		}
		if err != nil {
			return err
		}
		locked := rec.LockedUntil.Valid && rec.LockedUntil.Time.After(s.now())
		if locked {
			s.cfg.Hasher.BurnTime(in.Password)
		} else {
			ok, err = s.cfg.Hasher.Verify(in.Password, rec.PasswordHash)
			if err != nil {
				return err
			}
			if err := recordAttempt(ctx, tx, rec.UserID, ok, s.cfg.MaxFailedLogins, s.cfg.LockoutDuration); err != nil {
				return err
			}
		}
		choices, err = userOrganizations(ctx, tx, rec.UserID)
		return err
	})
	if err != nil {
		return SessionGrant{}, err
	}

	target, choiceErr := chooseOrg(choices, in.OrganizationSlug)
	if !ok || rec.UserStatus != "active" {
		if target != nil {
			reason := "invalid_password"
			if rec.UserStatus != "active" {
				reason = "user_disabled"
			} else if rec.LockedUntil.Valid && rec.LockedUntil.Time.After(s.now()) {
				reason = "locked"
			}
			s.auditStandalone(ctx, audit.Event{
				OrgID: target.ID, Action: "auth.login", ResourceType: "user", ResourceID: rec.UserID.String(),
				Outcome: audit.Failure, Actor: &audit.Actor{Type: "anonymous"}, Details: map[string]any{"reason": reason},
			})
		}
		return SessionGrant{}, ErrInvalidCredentials
	}
	if choiceErr != nil {
		return SessionGrant{}, choiceErr
	}

	user := User{ID: rec.UserID, Email: in.Email, DisplayName: rec.DisplayName, Status: rec.UserStatus}
	org := OrgSummary{ID: target.ID, Slug: target.Slug, Name: target.Name}
	var grant SessionGrant
	err = db.InTenantTx(ctx, s.pool, org.ID, func(tx pgx.Tx) error {
		var err error
		grant, err = s.issueSession(ctx, tx, user, org, target.Settings, "password")
		return err
	})
	return grant, err
}

// chooseOrg picks the target org. It returns (nil, err) when the user must
// choose or has no active membership.
func chooseOrg(choices []orgChoice, slug string) (*orgChoice, error) {
	if len(choices) == 0 {
		return nil, ErrNoActiveOrganization
	}
	if slug != "" {
		for i := range choices {
			if choices[i].Slug == slug {
				return &choices[i], nil
			}
		}
	} else if len(choices) == 1 {
		return &choices[0], nil
	}
	out := make([]OrgSummary, len(choices))
	for i, c := range choices {
		out[i] = OrgSummary{ID: c.ID, Slug: c.Slug, Name: c.Name}
	}
	return nil, &OrganizationChoiceRequired{Choices: out}
}

// issueSession inserts a session (tx bound to org) and records the sign-in.
func (s *Service) issueSession(ctx context.Context, tx pgx.Tx, u User, org OrgSummary, settings organization.Settings, method string) (SessionGrant, error) {
	token, tokenHash := newToken()
	csrf, csrfHash := newToken()
	now := s.now()
	g := SessionGrant{
		Token: token, CSRFToken: csrf, SessionID: uuid.Must(uuid.NewV7()),
		ExpiresAt: now.Add(settings.AbsoluteTimeout()), User: u, Organization: org,
	}
	g.IdleExpiresAt = minTime(now.Add(settings.IdleTimeout()), g.ExpiresAt)
	ci := httpx.ClientInfoFrom(ctx)
	params := identitydb.InsertSessionParams{
		ID: g.SessionID, OrgID: org.ID, UserID: u.ID, TokenHash: tokenHash, CsrfHash: csrfHash,
		IdleExpiresAt: ts(g.IdleExpiresAt), ExpiresAt: ts(g.ExpiresAt),
		UserAgent: pgtype.Text{String: ci.UserAgent, Valid: ci.UserAgent != ""},
	}
	if ci.IP.IsValid() {
		ip := ci.IP
		params.Ip = &ip
	}
	if err := identitydb.New(tx).InsertSession(ctx, params); err != nil {
		return SessionGrant{}, fmt.Errorf("identity: insert session: %w", err)
	}
	err := audit.Record(ctx, tx, audit.Event{
		OrgID: org.ID, Action: "auth.login", ResourceType: "session", ResourceID: g.SessionID.String(),
		Actor: &audit.Actor{Type: string(principal.User), ID: u.ID}, Details: map[string]any{"method": method},
	})
	return g, err
}

// Authenticate resolves a session token into a principal. It returns the
// session's CSRF hash for the caller to verify on unsafe requests.
func (s *Service) Authenticate(ctx context.Context, token string) (principal.Principal, []byte, error) {
	if token == "" || len(token) > 128 {
		return principal.Principal{}, nil, errNoSession
	}
	var rs resolvedSession
	err := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		rs, err = resolveSession(ctx, tx, hashToken(token))
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return principal.Principal{}, nil, errNoSession
	}
	if err != nil {
		return principal.Principal{}, nil, err
	}
	now := s.now()
	if rs.RevokedAt.Valid || !now.Before(rs.IdleExpiresAt) || !now.Before(rs.ExpiresAt) ||
		rs.UserStatus != "active" || rs.MembershipStatus != "active" || rs.OrgStatus != "active" {
		return principal.Principal{}, nil, errNoSession
	}
	p := principal.Principal{ID: rs.UserID, Type: principal.User, OrgID: rs.OrgID, SessionID: rs.SessionID, IsOwner: rs.IsOwner}

	if now.Sub(rs.LastSeenAt) >= s.cfg.TouchInterval {
		idle := minTime(now.Add(rs.OrgSettings.IdleTimeout()), rs.ExpiresAt)
		err := db.InTenantTx(ctx, s.pool, rs.OrgID, func(tx pgx.Tx) error {
			return identitydb.New(tx).TouchSession(ctx, identitydb.TouchSessionParams{ID: rs.SessionID, IdleExpiresAt: ts(idle)})
		})
		if err != nil {
			s.log.WarnContext(ctx, "session touch failed", "err", err) // non-fatal: the session is still valid
		}
	}
	return p, rs.CSRFHash, nil
}

// VerifyCSRF compares the presented token with the session's stored hash.
func VerifyCSRF(presented string, storedHash []byte) bool {
	if presented == "" || len(storedHash) == 0 {
		return false
	}
	return subtle.ConstantTimeCompare(hashToken(presented), storedHash) == 1
}

// Logout revokes the caller's current session.
func (s *Service) Logout(ctx context.Context) error {
	p, ok := principal.From(ctx)
	if !ok || p.SessionID == uuid.Nil {
		return nil // already signed out
	}
	return db.InTenantTx(ctx, s.pool, p.OrgID, func(tx pgx.Tx) error {
		n, err := identitydb.New(tx).RevokeUserSession(ctx, identitydb.RevokeUserSessionParams{ID: p.SessionID, UserID: p.ID})
		if err != nil || n == 0 {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{
			OrgID: p.OrgID, Action: "auth.logout", ResourceType: "session", ResourceID: p.SessionID.String(),
		})
	})
}

// Member is the caller's identity within their current organization.
type Member struct {
	User     User
	IsOwner  bool
	JoinedAt time.Time
}

// Me returns the caller's user and membership.
func (s *Service) Me(ctx context.Context) (Member, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return Member{}, authz.Unauthenticated()
	}
	var m Member
	err := db.InTenantTx(ctx, s.pool, p.OrgID, func(tx pgx.Tx) error {
		row, err := identitydb.New(tx).GetMember(ctx, identitydb.GetMemberParams{OrgID: p.OrgID, UserID: p.ID})
		if err != nil {
			return err
		}
		m = Member{
			User:    User{ID: row.ID, Email: row.Email, DisplayName: row.DisplayName, Status: row.Status},
			IsOwner: row.IsOwner, JoinedAt: row.JoinedAt.Time,
		}
		return nil
	})
	return m, err
}

// SessionInfo describes one of the caller's active sessions.
type SessionInfo struct {
	ID            uuid.UUID
	Current       bool
	CreatedAt     time.Time
	LastSeenAt    time.Time
	IdleExpiresAt time.Time
	ExpiresAt     time.Time
	IP            string
	UserAgent     string
}

// ListSessions returns the caller's active sessions in the current organization.
func (s *Service) ListSessions(ctx context.Context) ([]SessionInfo, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return nil, authz.Unauthenticated()
	}
	var out []SessionInfo
	err := db.InTenantTx(ctx, s.pool, p.OrgID, func(tx pgx.Tx) error {
		rows, err := identitydb.New(tx).ListActiveUserSessions(ctx, p.ID)
		if err != nil {
			return err
		}
		out = make([]SessionInfo, len(rows))
		for i, r := range rows {
			si := SessionInfo{
				ID: r.ID, Current: r.ID == p.SessionID, CreatedAt: r.CreatedAt.Time, LastSeenAt: r.LastSeenAt.Time,
				IdleExpiresAt: r.IdleExpiresAt.Time, ExpiresAt: r.ExpiresAt.Time, UserAgent: r.UserAgent.String,
			}
			if r.Ip != nil {
				si.IP = r.Ip.String()
			}
			out[i] = si
		}
		return nil
	})
	return out, err
}

// RevokeSession revokes one of the caller's own sessions.
func (s *Service) RevokeSession(ctx context.Context, id uuid.UUID) error {
	p, ok := principal.From(ctx)
	if !ok {
		return authz.Unauthenticated()
	}
	return db.InTenantTx(ctx, s.pool, p.OrgID, func(tx pgx.Tx) error {
		n, err := identitydb.New(tx).RevokeUserSession(ctx, identitydb.RevokeUserSessionParams{ID: id, UserID: p.ID})
		if err != nil {
			return err
		}
		if n == 0 {
			return httpx.NotFound("Session not found or already ended.")
		}
		return audit.Record(ctx, tx, audit.Event{
			OrgID: p.OrgID, Action: "auth.session_revoked", ResourceType: "session", ResourceID: id.String(),
			Details: map[string]any{"self": id == p.SessionID},
		})
	})
}

func (s *Service) auditStandalone(ctx context.Context, e audit.Event) {
	if err := audit.RecordStandalone(ctx, s.pool, e); err != nil {
		s.log.ErrorContext(ctx, "audit write failed", "action", e.Action, "err", err)
	}
}

func normalizeEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }

func validEmail(e string) bool {
	if len(e) > 254 || strings.ContainsAny(e, " \t\r\n<>") {
		return false
	}
	a, err := mail.ParseAddress(e)
	return err == nil && a.Address == e && strings.Contains(e[strings.LastIndex(e, "@"):], ".")
}

func digest(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:16])
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
