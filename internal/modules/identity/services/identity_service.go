package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/infrastructure/persistence/outbox"
	"github.com/nuriansyah/hugimethara/internal/infrastructure/security/ratelimit"
	authzmodel "github.com/nuriansyah/hugimethara/internal/modules/authorization/model"
	authzsvc "github.com/nuriansyah/hugimethara/internal/modules/authorization/services"
	auditmodel "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/model"
	auditsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/services"
	entsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/entitlement/services"
	identitymodel "github.com/nuriansyah/hugimethara/internal/modules/identity/model"
	identitydto "github.com/nuriansyah/hugimethara/internal/modules/identity/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/identity/repository"
	"github.com/nuriansyah/hugimethara/internal/modules/identity/repository/commands"
	"github.com/nuriansyah/hugimethara/internal/modules/identity/repository/queries"
	orgmodel "github.com/nuriansyah/hugimethara/internal/modules/organization/model"
	orgdto "github.com/nuriansyah/hugimethara/internal/modules/organization/model/dto"
	orgsvc "github.com/nuriansyah/hugimethara/internal/modules/organization/services"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
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

	// PublicURL is the browser-visible origin; OIDC redirects are built from it.
	PublicURL string
	// Sealer protects client secrets and the OIDC state cookie. Required for
	// OIDC; startup fails fast in production when the key is missing.
	Sealer interface {
		Seal([]byte) ([]byte, error)
		Open([]byte) ([]byte, error)
	}
}

// IdentityService implements authentication use cases.
type IdentityService struct {
	pool    *sqlx.DB
	orgs    *orgsvc.OrganizationService
	az      authzsvc.Authorizer
	acl     *authzsvc.AuthorizationService
	limiter *ratelimit.Limiter
	log     *slog.Logger
	cfg     Config
	now     func() time.Time

	discoveryMu sync.Mutex
	discovery   map[string]discoveryEntry
}

func NewService(pool *sqlx.DB, orgs *orgsvc.OrganizationService, az authzsvc.Authorizer, acl *authzsvc.AuthorizationService, limiter *ratelimit.Limiter, log *slog.Logger, cfg Config) *IdentityService {
	if cfg.Hasher == nil {
		cfg.Hasher = NewHasher(identitymodel.DefaultArgon2)
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
	if cfg.PublicURL == "" {
		cfg.PublicURL = "http://localhost:3000"
	}
	return &IdentityService{pool: pool, orgs: orgs, az: az, acl: acl, limiter: limiter, log: log, cfg: cfg,
		now: time.Now, discovery: map[string]discoveryEntry{}}
}

// Signup registers a new organization with the caller as owner and signs them in.
func (s *IdentityService) Signup(ctx context.Context, in identitydto.SignupInput) (identitymodel.SessionGrant, error) {
	ci := principal.ClientInfoFrom(ctx)
	if err := s.limiter.Allow(ctx, ruleSignupIP, ci.IP.String()); err != nil {
		return identitymodel.SessionGrant{}, err
	}
	in.Email = identitymodel.NormalizeEmail(in.Email)
	in.OrganizationSlug = strings.ToLower(strings.TrimSpace(in.OrganizationSlug))
	in.OrganizationName = strings.TrimSpace(in.OrganizationName)
	in.DisplayName = strings.TrimSpace(in.DisplayName)

	var errs []problem.FieldError
	if fe := orgmodel.ValidateName("organization_name", in.OrganizationName); fe != nil {
		errs = append(errs, *fe)
	}
	if fe := orgmodel.ValidateSlug("organization_slug", in.OrganizationSlug); fe != nil {
		errs = append(errs, *fe)
	}
	if n := len([]rune(in.DisplayName)); n < 1 || n > 100 {
		errs = append(errs, problem.FieldError{Field: "display_name", Code: "invalid_length", Message: "Name must be 1–100 characters."})
	}
	if !identitymodel.ValidEmail(in.Email) {
		errs = append(errs, problem.FieldError{Field: "email", Code: "invalid_email", Message: "Enter a valid email address."})
	}
	errs = append(errs, identitymodel.ValidatePassword("password", in.Password, in.Email)...)
	if len(errs) > 0 {
		return identitymodel.SessionGrant{}, problem.Validation(errs...)
	}

	hash, err := s.cfg.Hasher.Hash(in.Password)
	if err != nil {
		return identitymodel.SessionGrant{}, err
	}
	orgID, userID := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	user := identitymodel.User{ID: userID, Email: in.Email, DisplayName: in.DisplayName, Status: "active"}
	org := identitymodel.OrgSummary{ID: orgID, Slug: in.OrganizationSlug, Name: in.OrganizationName}
	actor := &auditmodel.Actor{Type: string(principal.User), ID: userID}

	var grant identitymodel.SessionGrant
	err = db.InTenantTx(ctx, s.pool, orgID, func(tx *sqlx.Tx) error {
		if err := s.orgs.Create(ctx, tx, orgdto.NewOrganization{ID: orgID, Slug: org.Slug, Name: org.Name}); err != nil {
			return err
		}
		if err := entsvc.Provision(ctx, tx, orgID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertUser"), userID, user.Email, user.DisplayName); err != nil {
			if db.IsUniqueViolation(err) {
				return identitymodel.ErrEmailTaken
			}
			return fmt.Errorf("identity: insert user: %w", err)
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertMembership"), orgID, userID); err != nil {
			return fmt.Errorf("identity: insert membership: %w", err)
		}
		if err := authzsvc.SeedOrganization(ctx, tx, orgID, userID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertCredentials"), userID, hash); err != nil {
			return fmt.Errorf("identity: insert credentials: %w", err)
		}
		if err := auditsvc.Record(ctx, tx, auditmodel.Event{
			OrgID: orgID, Action: "organization.created", ResourceType: orgmodel.ResourceType,
			ResourceID: orgID.String(), Actor: actor,
			Details: map[string]any{"slug": org.Slug, "name": org.Name, "owner_email": user.Email},
		}); err != nil {
			return err
		}
		if _, err := outbox.Publish(ctx, tx, outbox.Event{
			OrgID: orgID, Subject: "hugi.events.organization.created",
			Payload: map[string]any{"organization_id": orgID, "slug": org.Slug, "owner_user_id": userID},
		}); err != nil {
			return err
		}
		grant, err = s.issueSession(ctx, tx, user, org, orgmodel.DefaultSettings(), "signup")
		return err
	})
	if err != nil {
		return identitymodel.SessionGrant{}, err
	}
	s.log.InfoContext(ctx, "organization created", "org_id", orgID, "user_id", userID)
	return grant, nil
}

// Login verifies credentials and creates a session in the chosen organization.
func (s *IdentityService) Login(ctx context.Context, in identitydto.LoginInput) (identitymodel.SessionGrant, error) {
	ci := principal.ClientInfoFrom(ctx)
	in.Email = identitymodel.NormalizeEmail(in.Email)
	in.OrganizationSlug = strings.ToLower(strings.TrimSpace(in.OrganizationSlug))
	if err := s.limiter.Allow(ctx, ruleLoginIP, ci.IP.String()); err != nil {
		return identitymodel.SessionGrant{}, err
	}
	if err := s.limiter.Allow(ctx, ruleLoginUser, identitymodel.Digest(in.Email)); err != nil {
		return identitymodel.SessionGrant{}, err
	}
	if !identitymodel.ValidEmail(in.Email) || in.Password == "" || len(in.Password) > 4*identitymodel.MaxPasswordLen {
		s.cfg.Hasher.BurnTime(in.Password)
		return identitymodel.SessionGrant{}, identitymodel.ErrInvalidCredentials
	}

	var (
		rec     identitymodel.LoginRecord
		choices []orgChoice
		ok      bool
	)
	err := db.InTx(ctx, s.pool, func(tx *sqlx.Tx) error {
		var err error
		rec, err = loginLookup(ctx, tx, in.Email)
		if db.IsNoRows(err) {
			s.cfg.Hasher.BurnTime(in.Password)
			return identitymodel.ErrInvalidCredentials
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
		return identitymodel.SessionGrant{}, err
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
			s.auditStandalone(ctx, auditmodel.Event{
				OrgID: target.ID, Action: "auth.login", ResourceType: "user", ResourceID: rec.UserID.String(),
				Outcome: auditmodel.Failure, Actor: &auditmodel.Actor{Type: "anonymous"}, Details: map[string]any{"reason": reason},
			})
		}
		return identitymodel.SessionGrant{}, identitymodel.ErrInvalidCredentials
	}
	if choiceErr != nil {
		return identitymodel.SessionGrant{}, choiceErr
	}

	user := identitymodel.User{ID: rec.UserID, Email: in.Email, DisplayName: rec.DisplayName, Status: rec.UserStatus}
	org := identitymodel.OrgSummary{ID: target.ID, Slug: target.Slug, Name: target.Name}
	var grant identitymodel.SessionGrant
	err = db.InTenantTx(ctx, s.pool, org.ID, func(tx *sqlx.Tx) error {
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
		return nil, identitymodel.ErrNoActiveOrganization
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
	out := make([]identitymodel.OrgSummary, len(choices))
	for i, c := range choices {
		out[i] = identitymodel.OrgSummary{ID: c.ID, Slug: c.Slug, Name: c.Name}
	}
	return nil, &identitymodel.OrganizationChoiceRequired{Choices: out}
}

// issueSession inserts a session (tx bound to org) and records the sign-in.
func (s *IdentityService) issueSession(ctx context.Context, tx *sqlx.Tx, u identitymodel.User, org identitymodel.OrgSummary, settings orgmodel.Settings, method string) (identitymodel.SessionGrant, error) {
	token, tokenHash := identitymodel.NewToken()
	csrf, csrfHash := identitymodel.NewToken()
	now := s.now()
	g := identitymodel.SessionGrant{
		Token: token, CSRFToken: csrf, SessionID: uuid.Must(uuid.NewV7()),
		ExpiresAt: now.Add(settings.AbsoluteTimeout()), User: u, Organization: org,
	}
	g.IdleExpiresAt = identitymodel.MinTime(now.Add(settings.IdleTimeout()), g.ExpiresAt)
	ci := principal.ClientInfoFrom(ctx)
	var ip, ua any
	if ci.IP.IsValid() {
		ip = ci.IP.String()
	}
	if ci.UserAgent != "" {
		ua = ci.UserAgent
	}
	if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertSession"), g.SessionID, org.ID, u.ID, tokenHash, csrfHash,
		g.IdleExpiresAt, g.ExpiresAt, ip, ua); err != nil {
		return identitymodel.SessionGrant{}, fmt.Errorf("identity: insert session: %w", err)
	}
	err := auditsvc.Record(ctx, tx, auditmodel.Event{
		OrgID: org.ID, Action: "auth.login", ResourceType: "session", ResourceID: g.SessionID.String(),
		Actor: &auditmodel.Actor{Type: string(principal.User), ID: u.ID}, Details: map[string]any{"method": method},
	})
	return g, err
}

// Authenticate resolves a session token into a principal. It returns the
// session's CSRF hash for the caller to verify on unsafe requests.
func (s *IdentityService) Authenticate(ctx context.Context, token string) (principal.Principal, []byte, error) {
	if token == "" || len(token) > 128 {
		return principal.Principal{}, nil, identitymodel.ErrNoSession
	}
	var rs resolvedSession
	err := db.InTx(ctx, s.pool, func(tx *sqlx.Tx) error {
		var err error
		rs, err = resolveSession(ctx, tx, identitymodel.HashToken(token))
		return err
	})
	if db.IsNoRows(err) {
		return principal.Principal{}, nil, identitymodel.ErrNoSession
	}
	if err != nil {
		return principal.Principal{}, nil, err
	}
	now := s.now()
	if rs.RevokedAt.Valid || !now.Before(rs.IdleExpiresAt) || !now.Before(rs.ExpiresAt) ||
		rs.UserStatus != "active" || rs.MembershipStatus != "active" || rs.OrgStatus != "active" {
		return principal.Principal{}, nil, identitymodel.ErrNoSession
	}
	p := principal.Principal{ID: rs.UserID, Type: principal.User, OrgID: rs.OrgID, SessionID: rs.SessionID}

	if now.Sub(rs.LastSeenAt) >= s.cfg.TouchInterval {
		idle := identitymodel.MinTime(now.Add(rs.OrgSettings.IdleTimeout()), rs.ExpiresAt)
		err := db.InTenantTx(ctx, s.pool, rs.OrgID, func(tx *sqlx.Tx) error {
			_, err := tx.ExecContext(ctx, commands.SQL.Get("TouchSession"), rs.SessionID, idle)
			return err
		})
		if err != nil {
			s.log.WarnContext(ctx, "session touch failed", "err", err) // non-fatal: the session is still valid
		}
	}
	return p, rs.CSRFHash, nil
}

// Logout revokes the caller's current session.
func (s *IdentityService) Logout(ctx context.Context) error {
	p, ok := principal.From(ctx)
	if !ok || p.SessionID == uuid.Nil {
		return nil // already signed out
	}
	return db.InTenantTx(ctx, s.pool, p.OrgID, func(tx *sqlx.Tx) error {
		n, err := execRows(ctx, tx, commands.SQL.Get("RevokeUserSession"), p.SessionID, p.ID)
		if err != nil || n == 0 {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{
			OrgID: p.OrgID, Action: "auth.logout", ResourceType: "session", ResourceID: p.SessionID.String(),
		})
	})
}

// Me returns the caller's user and membership.
func (s *IdentityService) Me(ctx context.Context) (identitymodel.Member, error) {
	p, err := userPrincipal(ctx)
	if err != nil {
		return identitymodel.Member{}, err
	}
	var m identitymodel.Member
	err = db.InTenantTx(ctx, s.pool, p.OrgID, func(tx *sqlx.Tx) error {
		var row repository.MemberRow
		if err := tx.GetContext(ctx, &row, queries.SQL.Get("GetMember"), p.OrgID, p.ID); err != nil {
			return err
		}
		owner, err := authzsvc.IsOrgOwner(ctx, tx, p.ID)
		if err != nil {
			return err
		}
		m = identitymodel.Member{
			User:    identitymodel.User{ID: row.ID, Email: row.Email, DisplayName: row.DisplayName, Status: row.Status},
			IsOwner: owner, JoinedAt: row.JoinedAt,
		}
		return nil
	})
	return m, err
}

// ListSessions returns the caller's active sessions in the current organization.
func (s *IdentityService) ListSessions(ctx context.Context) ([]identitymodel.SessionInfo, error) {
	p, err := userPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	var out []identitymodel.SessionInfo
	err = db.InTenantTx(ctx, s.pool, p.OrgID, func(tx *sqlx.Tx) error {
		var rows []repository.SessionRow
		if err := tx.SelectContext(ctx, &rows, queries.SQL.Get("ListActiveUserSessions"), p.ID); err != nil {
			return err
		}
		out = make([]identitymodel.SessionInfo, len(rows))
		for i, r := range rows {
			out[i] = identitymodel.SessionInfo{
				ID: r.ID, Current: r.ID == p.SessionID, CreatedAt: r.CreatedAt, LastSeenAt: r.LastSeenAt,
				IdleExpiresAt: r.IdleExpiresAt, ExpiresAt: r.ExpiresAt, UserAgent: r.UserAgent.String, IP: r.IP.String,
			}
		}
		return nil
	})
	return out, err
}

// RevokeSession revokes one of the caller's own sessions.
func (s *IdentityService) RevokeSession(ctx context.Context, id uuid.UUID) error {
	p, err := userPrincipal(ctx)
	if err != nil {
		return err
	}
	return db.InTenantTx(ctx, s.pool, p.OrgID, func(tx *sqlx.Tx) error {
		n, err := execRows(ctx, tx, commands.SQL.Get("RevokeUserSession"), id, p.ID)
		if err != nil {
			return err
		}
		if n == 0 {
			return problem.NotFound("Session not found or already ended.")
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{
			OrgID: p.OrgID, Action: "auth.session_revoked", ResourceType: "session", ResourceID: id.String(),
			Details: map[string]any{"self": id == p.SessionID},
		})
	})
}

func userPrincipal(ctx context.Context) (principal.Principal, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return p, authzmodel.Unauthenticated()
	}
	if p.Type != principal.User {
		return p, identitymodel.ErrNotAUser
	}
	return p, nil
}

func (s *IdentityService) auditStandalone(ctx context.Context, e auditmodel.Event) {
	if err := auditsvc.RecordStandalone(ctx, s.pool, e); err != nil {
		s.log.ErrorContext(ctx, "audit write failed", "action", e.Action, "err", err)
	}
}

// execRows runs a statement and returns the number of affected rows.
func execRows(ctx context.Context, tx *sqlx.Tx, query string, args ...any) (int64, error) {
	res, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// CurrentOrganization returns the caller's organization; identity owns the
// session that names it, organization owns the record.
func (s *IdentityService) CurrentOrganization(ctx context.Context) (orgmodel.Organization, error) {
	return s.orgs.Current(ctx)
}

// PublicURL is the externally visible base URL used to build redirects.
func (s *IdentityService) PublicURL() string { return s.cfg.PublicURL }
