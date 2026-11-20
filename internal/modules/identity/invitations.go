package identity

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/modules/authorization"
	"github.com/nuriansyah/hugimethara/internal/modules/governance/audit"
	"github.com/nuriansyah/hugimethara/internal/modules/organization"
	"github.com/nuriansyah/hugimethara/internal/modules/workflow/event"
	"github.com/nuriansyah/hugimethara/internal/platform/database"
	"github.com/nuriansyah/hugimethara/internal/platform/httpx"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
	"github.com/nuriansyah/hugimethara/internal/platform/ratelimit"
)

// Invitation limits.
const (
	DefaultInvitationDays = 7
	MaxInvitationDays     = 30
	maxInvitationGrants   = 50
)

var (
	ruleInviteOrg = ratelimit.Rule{Name: "invite_org", Max: 100, Window: time.Hour}
	ruleAcceptIP  = ratelimit.Rule{Name: "invite_accept_ip", Max: 20, Window: 5 * time.Minute}
)

// RoleRef names a role.
type RoleRef struct {
	ID   uuid.UUID
	Key  string
	Name string
}

// InvitationWorkspace is a workspace an invitee joins, with their role there.
type InvitationWorkspace struct {
	WorkspaceID   uuid.UUID `json:"workspace_id"`
	WorkspaceName string    `json:"workspace_name"`
	WorkspaceSlug string    `json:"workspace_slug"`
	RoleID        uuid.UUID `json:"role_id"`
	RoleName      string    `json:"role_name"`
}

// Invitation is an invitation to join the organization.
type Invitation struct {
	ID            uuid.UUID
	Email         string
	Status        string // pending | accepted | revoked | expired
	OrgRole       *RoleRef
	Workspaces    []InvitationWorkspace
	InvitedByName string
	CreatedAt     time.Time
	ExpiresAt     time.Time
	AcceptedAt    *time.Time
}

type invitationRow struct {
	ID            uuid.UUID      `db:"id"`
	Email         string         `db:"email"`
	CreatedAt     time.Time      `db:"created_at"`
	ExpiresAt     time.Time      `db:"expires_at"`
	AcceptedAt    sql.NullTime   `db:"accepted_at"`
	Status        string         `db:"status"`
	OrgRoleID     uuid.NullUUID  `db:"org_role_id"`
	OrgRoleKey    sql.NullString `db:"org_role_key"`
	OrgRoleName   sql.NullString `db:"org_role_name"`
	InvitedByName sql.NullString `db:"invited_by_name"`
	Workspaces    []byte         `db:"workspaces"`
}

func (r invitationRow) toModel() Invitation {
	inv := Invitation{ID: r.ID, Email: r.Email, Status: r.Status, InvitedByName: r.InvitedByName.String,
		CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt, Workspaces: []InvitationWorkspace{}}
	if r.OrgRoleID.Valid {
		inv.OrgRole = &RoleRef{ID: r.OrgRoleID.UUID, Key: r.OrgRoleKey.String, Name: r.OrgRoleName.String}
	}
	if r.AcceptedAt.Valid {
		t := r.AcceptedAt.Time
		inv.AcceptedAt = &t
	}
	_ = json.Unmarshal(r.Workspaces, &inv.Workspaces)
	return inv
}

// WorkspaceGrant is a workspace role an invitation hands out.
type WorkspaceGrant struct {
	WorkspaceID uuid.UUID `db:"workspace_id"`
	RoleID      uuid.UUID `db:"role_id"`
}

// InvitationInput creates an invitation.
type InvitationInput struct {
	Email         string
	OrgRoleID     *uuid.UUID
	Workspaces    []WorkspaceGrant
	ExpiresInDays int
}

// CreatedInvitation carries the link token, shown once.
type CreatedInvitation struct {
	Invitation Invitation
	Token      string
}

// AcceptPath is the web route of the invitation page.
func AcceptPath(token string) string { return "/invite/" + token }

// ListInvitations lists invitations, open ones only unless all (user:invite).
func (s *Service) ListInvitations(ctx context.Context, all bool) ([]Invitation, error) {
	p, err := s.require(ctx, "invite", UserResource, "")
	if err != nil {
		return nil, err
	}
	var rows []invitationRow
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, q.Get("ListInvitations"), !all, nil)
	}); err != nil {
		return nil, err
	}
	out := make([]Invitation, len(rows))
	for i, r := range rows {
		out[i] = r.toModel()
	}
	return out, nil
}

// CreateInvitation invites an email address (user:invite). Each workspace
// grant needs workspace:manage_members there and an organization role needs
// role:bind; in every case the caller can only hand out permissions they hold
// at that scope. Re-inviting an address replaces its open invitation.
func (s *Service) CreateInvitation(ctx context.Context, in InvitationInput) (CreatedInvitation, error) {
	p, err := s.require(ctx, "invite", UserResource, "")
	if err != nil {
		return CreatedInvitation{}, err
	}
	in.Email = normalizeEmail(in.Email)
	if in.ExpiresInDays == 0 {
		in.ExpiresInDays = DefaultInvitationDays
	}
	var errs []httpx.FieldError
	if !validEmail(in.Email) {
		errs = append(errs, httpx.FieldError{Field: "email", Code: "invalid_email", Message: "Enter a valid email address."})
	}
	if in.ExpiresInDays < 1 || in.ExpiresInDays > MaxInvitationDays {
		errs = append(errs, httpx.FieldError{Field: "expires_in_days", Code: "out_of_range", Message: "Invitations last 1–30 days."})
	}
	if len(in.Workspaces) > maxInvitationGrants {
		errs = append(errs, httpx.FieldError{Field: "workspaces", Code: "too_many", Message: "Invite to at most 50 workspaces at once."})
	}
	seen := map[uuid.UUID]bool{}
	for _, g := range in.Workspaces {
		if seen[g.WorkspaceID] {
			errs = append(errs, httpx.FieldError{Field: "workspaces", Code: "duplicate", Message: "Each workspace can appear only once."})
			break
		}
		seen[g.WorkspaceID] = true
	}
	if len(errs) > 0 {
		return CreatedInvitation{}, httpx.Validation(errs...)
	}
	if err := s.limiter.Allow(ctx, ruleInviteOrg, p.OrgID.String()); err != nil {
		return CreatedInvitation{}, err
	}
	if in.OrgRoleID != nil {
		if _, err := authz.Require(ctx, s.az, "bind", authz.Resource{Type: authz.RoleResource, OrgID: p.OrgID}); err != nil {
			return CreatedInvitation{}, err
		}
	}
	for _, g := range in.Workspaces {
		ws := g.WorkspaceID
		if _, err := authz.Require(ctx, s.az, "manage_members", authz.Resource{Type: organization.WorkspaceResource,
			ID: ws.String(), OrgID: p.OrgID, WorkspaceID: &ws}); err != nil {
			return CreatedInvitation{}, err
		}
	}

	token, tokenHash := newToken()
	id := uuid.Must(uuid.NewV7())
	var row invitationRow
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		var member bool
		if err := tx.GetContext(ctx, &member, q.Get("MemberEmailExists"), in.Email); err != nil {
			return err
		}
		if member {
			return httpx.NewProblem(http.StatusConflict, "already_member", "Conflict", "This person is already a member of the organization.")
		}
		var orgRole any
		if in.OrgRoleID != nil {
			if _, err := s.acl.CheckGrantRole(ctx, tx, p, *in.OrgRoleID, authz.ScopeRef{Type: authz.ScopeOrg}); err != nil {
				return err
			}
			orgRole = *in.OrgRoleID
		}
		for _, g := range in.Workspaces {
			var w struct {
				ID         uuid.UUID    `db:"id"`
				Name       string       `db:"name"`
				ArchivedAt sql.NullTime `db:"archived_at"`
			}
			if err := tx.GetContext(ctx, &w, q.Get("GetWorkspaceForInvite"), g.WorkspaceID); err != nil {
				if db.IsNoRows(err) {
					return httpx.Validation(httpx.FieldError{Field: "workspaces", Code: "not_found", Message: "Workspace not found."})
				}
				return err
			}
			if w.ArchivedAt.Valid {
				return httpx.Validation(httpx.FieldError{Field: "workspaces", Code: "archived", Message: "“" + w.Name + "” is archived."})
			}
			if _, err := s.acl.CheckGrantRole(ctx, tx, p, g.RoleID, authz.ScopeRef{Type: authz.ScopeWorkspace, WorkspaceID: g.WorkspaceID}); err != nil {
				return err
			}
		}
		replaced, err := execRows(ctx, tx, q.Get("RevokeOpenInvitationsFor"), in.Email)
		if err != nil {
			return err
		}
		expires := s.now().Add(time.Duration(in.ExpiresInDays) * 24 * time.Hour)
		if _, err := tx.ExecContext(ctx, q.Get("InsertInvitation"), id, p.OrgID, in.Email, tokenHash, orgRole, p.ID, expires); err != nil {
			return err
		}
		for _, g := range in.Workspaces {
			if _, err := tx.ExecContext(ctx, q.Get("InsertInvitationWorkspace"), p.OrgID, id, g.WorkspaceID, g.RoleID); err != nil {
				return err
			}
		}
		if err := tx.GetContext(ctx, &row, q.Get("ListInvitations"), false, id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "user.invited", ResourceType: UserResource,
			ResourceID: id.String(), Details: map[string]any{"email": in.Email, "org_role_id": orgRole,
				"workspaces": len(in.Workspaces), "replaced_open_invitation": replaced > 0, "expires_at": expires}})
	})
	if err != nil {
		return CreatedInvitation{}, err
	}
	return CreatedInvitation{Invitation: row.toModel(), Token: token}, nil
}

// RevokeInvitation revokes an open invitation (user:invite).
func (s *Service) RevokeInvitation(ctx context.Context, id uuid.UUID) error {
	p, err := s.require(ctx, "invite", UserResource, id.String())
	if err != nil {
		return err
	}
	return s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		n, err := execRows(ctx, tx, q.Get("RevokeInvitation"), id)
		if err != nil {
			return err
		}
		if n == 0 {
			var exists bool
			if err := tx.GetContext(ctx, &exists, q.Get("InvitationExists"), id); err != nil {
				return err
			}
			if !exists {
				return httpx.NotFound("Invitation not found.")
			}
			return httpx.NewProblem(http.StatusConflict, "invitation_closed", "Conflict", "This invitation was already accepted or revoked.")
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "user.invitation_revoked", ResourceType: UserResource,
			ResourceID: id.String()})
	})
}

// InvitationPreview is what an invitation link shows before accepting.
type InvitationPreview struct {
	Organization  OrgSummary
	Email         string
	InvitedByName string
	ExpiresAt     time.Time
	AccountExists bool
}

type invitationLookup struct {
	ID          uuid.UUID      `db:"invitation_id"`
	OrgID       uuid.UUID      `db:"org_id"`
	OrgSlug     string         `db:"org_slug"`
	OrgName     string         `db:"org_name"`
	OrgSettings []byte         `db:"org_settings"`
	Email       string         `db:"email"`
	ExpiresAt   time.Time      `db:"expires_at"`
	AcceptedAt  sql.NullTime   `db:"accepted_at"`
	RevokedAt   sql.NullTime   `db:"revoked_at"`
	InviterName sql.NullString `db:"inviter_name"`
	UserExists  bool           `db:"user_exists"`
}

var errInvitationNotFound = httpx.NotFound("This invitation link is not valid. Ask for a new invitation.")

func invitationGone(code, detail string) *httpx.Problem {
	return httpx.NewProblem(http.StatusGone, code, "Invitation no longer valid", detail)
}

func (l invitationLookup) usable(now time.Time) error {
	switch {
	case l.AcceptedAt.Valid:
		return invitationGone("invitation_accepted", "This invitation has already been used. Sign in instead.")
	case l.RevokedAt.Valid:
		return invitationGone("invitation_revoked", "This invitation was withdrawn. Ask for a new one.")
	case !now.Before(l.ExpiresAt):
		return invitationGone("invitation_expired", "This invitation has expired. Ask for a new one.")
	}
	return nil
}

func (s *Service) lookupInvitation(ctx context.Context, token string) (invitationLookup, error) {
	if token == "" || len(token) > 128 {
		return invitationLookup{}, errInvitationNotFound
	}
	var l invitationLookup
	err := db.InTx(ctx, s.pool, func(tx *sqlx.Tx) error { return tx.GetContext(ctx, &l, q.Get("InvitationLookup"), hashToken(token)) })
	if db.IsNoRows(err) {
		return l, errInvitationNotFound
	}
	if err != nil {
		return l, err
	}
	return l, l.usable(s.now())
}

// PreviewInvitation describes an invitation link (public; rate-limited by IP).
func (s *Service) PreviewInvitation(ctx context.Context, token string) (InvitationPreview, error) {
	if err := s.limiter.Allow(ctx, ruleAcceptIP, httpx.ClientInfoFrom(ctx).IP.String()); err != nil {
		return InvitationPreview{}, err
	}
	l, err := s.lookupInvitation(ctx, token)
	if err != nil {
		return InvitationPreview{}, err
	}
	return InvitationPreview{Organization: OrgSummary{ID: l.OrgID, Slug: l.OrgSlug, Name: l.OrgName}, Email: l.Email,
		InvitedByName: l.InviterName.String, ExpiresAt: l.ExpiresAt, AccountExists: l.UserExists}, nil
}

// AcceptInput accepts an invitation without a matching session.
type AcceptInput struct {
	DisplayName string // new accounts
	Password    string // new accounts choose one; existing accounts prove it
}

// AcceptInvitation joins the invited organization and signs in to it. The
// invitee proves who they are with a session for the invited address, the
// password of the existing account, or by creating the account.
func (s *Service) AcceptInvitation(ctx context.Context, token string, in AcceptInput) (SessionGrant, error) {
	if err := s.limiter.Allow(ctx, ruleAcceptIP, httpx.ClientInfoFrom(ctx).IP.String()); err != nil {
		return SessionGrant{}, err
	}
	l, err := s.lookupInvitation(ctx, token)
	if err != nil {
		return SessionGrant{}, err
	}

	var (
		user    User
		newHash string // set when creating the account
	)
	if p, ok := principal.From(ctx); ok && p.Type == principal.User && in.Password == "" {
		// Signed in (to any organization) as the invited address.
		var row memberRow
		err := db.InTenantTx(ctx, s.pool, p.OrgID, func(tx *sqlx.Tx) error {
			return tx.GetContext(ctx, &row, q.Get("GetMember"), p.OrgID, p.ID)
		})
		if err != nil {
			return SessionGrant{}, err
		}
		if !strings.EqualFold(row.Email, l.Email) {
			return SessionGrant{}, httpx.NewProblem(http.StatusConflict, "signed_in_as_other", "Different account",
				"You are signed in as "+row.Email+", but this invitation is for "+l.Email+". Sign out, or enter the password of the invited account.")
		}
		user = User{ID: row.ID, Email: row.Email, DisplayName: row.DisplayName, Status: row.Status}
	} else {
		user, newHash, err = s.inviteeIdentity(ctx, l.Email, in)
		if err != nil {
			return SessionGrant{}, err
		}
	}

	org := OrgSummary{ID: l.OrgID, Slug: l.OrgSlug, Name: l.OrgName}
	var grant SessionGrant
	err = db.InTenantTx(ctx, s.pool, l.OrgID, func(tx *sqlx.Tx) error {
		var inv struct {
			ID         uuid.UUID     `db:"id"`
			Email      string        `db:"email"`
			OrgRoleID  uuid.NullUUID `db:"org_role_id"`
			InvitedBy  uuid.NullUUID `db:"invited_by"`
			ExpiresAt  time.Time     `db:"expires_at"`
			AcceptedAt sql.NullTime  `db:"accepted_at"`
			RevokedAt  sql.NullTime  `db:"revoked_at"`
		}
		if err := tx.GetContext(ctx, &inv, q.Get("LockInvitation"), l.ID); err != nil {
			return err
		}
		if err := (invitationLookup{ExpiresAt: inv.ExpiresAt, AcceptedAt: inv.AcceptedAt, RevokedAt: inv.RevokedAt}).usable(s.now()); err != nil {
			return err // lost a race with another accept or a revoke
		}
		if newHash != "" {
			if _, err := tx.ExecContext(ctx, q.Get("InsertUser"), user.ID, user.Email, user.DisplayName); err != nil {
				if db.IsUniqueViolation(err) {
					return httpx.NewProblem(http.StatusConflict, "account_exists", "Account exists",
						"An account with this email already exists. Enter its password instead.")
				}
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, q.Get("InsertMembershipIfAbsent"), l.OrgID, user.ID); err != nil {
			return err
		}
		var status string
		if err := tx.GetContext(ctx, &status, q.Get("GetMembershipStatus"), user.ID); err != nil {
			return err
		}
		if status != MembershipActive {
			return httpx.NewProblem(http.StatusForbidden, "membership_suspended", "Membership suspended",
				"Your membership in this organization is suspended. Ask an administrator to reactivate it.")
		}
		if newHash != "" {
			if _, err := tx.ExecContext(ctx, q.Get("InsertCredentials"), user.ID, newHash); err != nil {
				return err
			}
		}
		if err := s.applyInvitationGrants(ctx, tx, l.OrgID, inv.ID, user.ID, inv.OrgRoleID, inv.InvitedBy.UUID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, q.Get("MarkInvitationAccepted"), inv.ID, user.ID); err != nil {
			return err
		}
		actor := &audit.Actor{Type: string(principal.User), ID: user.ID}
		if err := audit.Record(ctx, tx, audit.Event{OrgID: l.OrgID, Action: "user.joined", ResourceType: UserResource,
			ResourceID: user.ID.String(), Actor: actor, Details: map[string]any{"invitation_id": inv.ID,
				"invited_by": inv.InvitedBy.UUID, "new_account": newHash != ""}}); err != nil {
			return err
		}
		if _, err := event.Publish(ctx, tx, event.Event{OrgID: l.OrgID, Subject: "hugi.events.user.joined",
			Payload: map[string]any{"user_id": user.ID, "invitation_id": inv.ID}}); err != nil {
			return err
		}
		grant, err = s.issueSession(ctx, tx, user, org, organization.ParseSettings(l.OrgSettings), "invitation")
		return err
	})
	if err != nil {
		return SessionGrant{}, err
	}
	s.acl.Invalidate(ctx, l.OrgID)
	return grant, nil
}

// inviteeIdentity verifies the existing account's password, or validates the
// details of a new account (returning its password hash).
func (s *Service) inviteeIdentity(ctx context.Context, email string, in AcceptInput) (User, string, error) {
	if in.Password == "" {
		return User{}, "", httpx.Validation(httpx.FieldError{Field: "password", Code: "required", Message: "Enter a password."})
	}
	var (
		rec   loginRecord
		found bool
		ok    bool
	)
	err := db.InTx(ctx, s.pool, func(tx *sqlx.Tx) error {
		var err error
		rec, err = loginLookup(ctx, tx, email)
		if db.IsNoRows(err) {
			return nil
		}
		if err != nil {
			return err
		}
		found = true
		if rec.LockedUntil.Valid && rec.LockedUntil.Time.After(s.now()) {
			s.cfg.Hasher.BurnTime(in.Password)
			return nil
		}
		if ok, err = s.cfg.Hasher.Verify(in.Password, rec.PasswordHash); err != nil {
			return err
		}
		return recordAttempt(ctx, tx, rec.UserID, ok, s.cfg.MaxFailedLogins, s.cfg.LockoutDuration)
	})
	if err != nil {
		return User{}, "", err
	}
	if found {
		if !ok || rec.UserStatus != "active" {
			return User{}, "", ErrInvalidCredentials
		}
		return User{ID: rec.UserID, Email: email, DisplayName: rec.DisplayName, Status: rec.UserStatus}, "", nil
	}

	name := strings.TrimSpace(in.DisplayName)
	var errs []httpx.FieldError
	if n := len([]rune(name)); n < 1 || n > 100 {
		errs = append(errs, httpx.FieldError{Field: "display_name", Code: "invalid_length", Message: "Name must be 1–100 characters."})
	}
	errs = append(errs, ValidatePassword("password", in.Password, email)...)
	if len(errs) > 0 {
		return User{}, "", httpx.Validation(errs...)
	}
	hash, err := s.cfg.Hasher.Hash(in.Password)
	if err != nil {
		return User{}, "", err
	}
	return User{ID: uuid.Must(uuid.NewV7()), Email: email, DisplayName: name, Status: "active"}, hash, nil
}

// applyInvitationGrants binds the baseline member role, the optional
// organization role and the workspace roles (authorized when the invitation
// was created), then bumps the authz version.
func (s *Service) applyInvitationGrants(ctx context.Context, tx *sqlx.Tx, org, invitation, user uuid.UUID, orgRole uuid.NullUUID, by uuid.UUID) error {
	who := authz.UserRef(user)
	if err := authz.BindRoleKeyInTx(ctx, tx, org, who, authz.MemberRoleKey, authz.ScopeRef{Type: authz.ScopeOrg}, by); err != nil {
		return err
	}
	if orgRole.Valid {
		if err := authz.BindPreauthorizedInTx(ctx, tx, org, who, orgRole.UUID, authz.ScopeRef{Type: authz.ScopeOrg}, by); err != nil {
			return err
		}
	}
	var grants []WorkspaceGrant
	if err := tx.SelectContext(ctx, &grants, q.Get("InvitationWorkspaceGrants"), invitation); err != nil {
		return err
	}
	for _, g := range grants {
		if err := authz.BindPreauthorizedInTx(ctx, tx, org, who, g.RoleID,
			authz.ScopeRef{Type: authz.ScopeWorkspace, WorkspaceID: g.WorkspaceID}, by); err != nil {
			return err
		}
	}
	return authz.BumpVersion(ctx, tx, org)
}
