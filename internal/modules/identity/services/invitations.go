package services

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/infrastructure/persistence/outbox"
	"github.com/nuriansyah/hugimethara/internal/infrastructure/security/ratelimit"
	authzmodel "github.com/nuriansyah/hugimethara/internal/modules/authorization/model"
	authzsvc "github.com/nuriansyah/hugimethara/internal/modules/authorization/services"
	auditmodel "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/model"
	auditsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/services"
	identitymodel "github.com/nuriansyah/hugimethara/internal/modules/identity/model"
	identitydto "github.com/nuriansyah/hugimethara/internal/modules/identity/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/identity/repository"
	"github.com/nuriansyah/hugimethara/internal/modules/identity/repository/commands"
	"github.com/nuriansyah/hugimethara/internal/modules/identity/repository/queries"
	orgmodel "github.com/nuriansyah/hugimethara/internal/modules/organization/model"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

var (
	ruleInviteOrg = ratelimit.Rule{Name: "invite_org", Max: 100, Window: time.Hour}
	ruleAcceptIP  = ratelimit.Rule{Name: "invite_accept_ip", Max: 20, Window: 5 * time.Minute}
)

// ListInvitations lists invitations, open ones only unless all (user:invite).
func (s *IdentityService) ListInvitations(ctx context.Context, all bool) ([]identitymodel.Invitation, error) {
	p, err := s.require(ctx, "invite", identitymodel.UserResource, "")
	if err != nil {
		return nil, err
	}
	var rows []repository.InvitationRow
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, queries.SQL.Get("ListInvitations"), !all, nil)
	}); err != nil {
		return nil, err
	}
	out := make([]identitymodel.Invitation, len(rows))
	for i, r := range rows {
		out[i] = r.ToModel()
	}
	return out, nil
}

// CreateInvitation invites an email address (user:invite). Each workspace
// grant needs workspace:manage_members there and an organization role needs
// role:bind; in every case the caller can only hand out permissions they hold
// at that scope. Re-inviting an address replaces its open invitation.
func (s *IdentityService) CreateInvitation(ctx context.Context, in identitydto.InvitationInput) (identitydto.CreatedInvitation, error) {
	p, err := s.require(ctx, "invite", identitymodel.UserResource, "")
	if err != nil {
		return identitydto.CreatedInvitation{}, err
	}
	in.Email = identitymodel.NormalizeEmail(in.Email)
	if in.ExpiresInDays == 0 {
		in.ExpiresInDays = identitymodel.DefaultInvitationDays
	}
	var errs []problem.FieldError
	if !identitymodel.ValidEmail(in.Email) {
		errs = append(errs, problem.FieldError{Field: "email", Code: "invalid_email", Message: "Enter a valid email address."})
	}
	if in.ExpiresInDays < 1 || in.ExpiresInDays > identitymodel.MaxInvitationDays {
		errs = append(errs, problem.FieldError{Field: "expires_in_days", Code: "out_of_range", Message: "Invitations last 1–30 days."})
	}
	if len(in.Workspaces) > identitymodel.MaxInvitationGrants {
		errs = append(errs, problem.FieldError{Field: "workspaces", Code: "too_many", Message: "Invite to at most 50 workspaces at once."})
	}
	seen := map[uuid.UUID]bool{}
	for _, g := range in.Workspaces {
		if seen[g.WorkspaceID] {
			errs = append(errs, problem.FieldError{Field: "workspaces", Code: "duplicate", Message: "Each workspace can appear only once."})
			break
		}
		seen[g.WorkspaceID] = true
	}
	if len(errs) > 0 {
		return identitydto.CreatedInvitation{}, problem.Validation(errs...)
	}
	if err := s.limiter.Allow(ctx, ruleInviteOrg, p.OrgID.String()); err != nil {
		return identitydto.CreatedInvitation{}, err
	}
	if in.OrgRoleID != nil {
		if _, err := authzsvc.Require(ctx, s.az, "bind", authzmodel.Resource{Type: authzmodel.RoleResource, OrgID: p.OrgID}); err != nil {
			return identitydto.CreatedInvitation{}, err
		}
	}
	for _, g := range in.Workspaces {
		ws := g.WorkspaceID
		if _, err := authzsvc.Require(ctx, s.az, "manage_members", authzmodel.Resource{Type: orgmodel.WorkspaceResource,
			ID: ws.String(), OrgID: p.OrgID, WorkspaceID: &ws}); err != nil {
			return identitydto.CreatedInvitation{}, err
		}
	}

	token, tokenHash := identitymodel.NewToken()
	id := uuid.Must(uuid.NewV7())
	var row repository.InvitationRow
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		var member bool
		if err := tx.GetContext(ctx, &member, queries.SQL.Get("MemberEmailExists"), in.Email); err != nil {
			return err
		}
		if member {
			return problem.NewProblem(http.StatusConflict, "already_member", "Conflict", "This person is already a member of the organization.")
		}
		var orgRole any
		if in.OrgRoleID != nil {
			if _, err := s.acl.CheckGrantRole(ctx, tx, p, *in.OrgRoleID, authzmodel.ScopeRef{Type: authzmodel.ScopeOrg}); err != nil {
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
			if err := tx.GetContext(ctx, &w, queries.SQL.Get("GetWorkspaceForInvite"), g.WorkspaceID); err != nil {
				if db.IsNoRows(err) {
					return problem.Validation(problem.FieldError{Field: "workspaces", Code: "not_found", Message: "Workspace not found."})
				}
				return err
			}
			if w.ArchivedAt.Valid {
				return problem.Validation(problem.FieldError{Field: "workspaces", Code: "archived", Message: "“" + w.Name + "” is archived."})
			}
			if _, err := s.acl.CheckGrantRole(ctx, tx, p, g.RoleID, authzmodel.ScopeRef{Type: authzmodel.ScopeWorkspace, WorkspaceID: g.WorkspaceID}); err != nil {
				return err
			}
		}
		replaced, err := execRows(ctx, tx, commands.SQL.Get("RevokeOpenInvitationsFor"), in.Email)
		if err != nil {
			return err
		}
		expires := s.now().Add(time.Duration(in.ExpiresInDays) * 24 * time.Hour)
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertInvitation"), id, p.OrgID, in.Email, tokenHash, orgRole, p.ID, expires); err != nil {
			return err
		}
		for _, g := range in.Workspaces {
			if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertInvitationWorkspace"), p.OrgID, id, g.WorkspaceID, g.RoleID); err != nil {
				return err
			}
		}
		if err := tx.GetContext(ctx, &row, queries.SQL.Get("ListInvitations"), false, id); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "user.invited", ResourceType: identitymodel.UserResource,
			ResourceID: id.String(), Details: map[string]any{"email": in.Email, "org_role_id": orgRole,
				"workspaces": len(in.Workspaces), "replaced_open_invitation": replaced > 0, "expires_at": expires}})
	})
	if err != nil {
		return identitydto.CreatedInvitation{}, err
	}
	return identitydto.CreatedInvitation{Invitation: row.ToModel(), Token: token}, nil
}

// RevokeInvitation revokes an open invitation (user:invite).
func (s *IdentityService) RevokeInvitation(ctx context.Context, id uuid.UUID) error {
	p, err := s.require(ctx, "invite", identitymodel.UserResource, id.String())
	if err != nil {
		return err
	}
	return s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		n, err := execRows(ctx, tx, commands.SQL.Get("RevokeInvitation"), id)
		if err != nil {
			return err
		}
		if n == 0 {
			var exists bool
			if err := tx.GetContext(ctx, &exists, queries.SQL.Get("InvitationExists"), id); err != nil {
				return err
			}
			if !exists {
				return problem.NotFound("Invitation not found.")
			}
			return problem.NewProblem(http.StatusConflict, "invitation_closed", "Conflict", "This invitation was already accepted or revoked.")
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "user.invitation_revoked", ResourceType: identitymodel.UserResource,
			ResourceID: id.String()})
	})
}

func (s *IdentityService) lookupInvitation(ctx context.Context, token string) (identitymodel.InvitationLookup, error) {
	if token == "" || len(token) > 128 {
		return identitymodel.InvitationLookup{}, identitymodel.ErrInvitationNotFound
	}
	var l identitymodel.InvitationLookup
	err := db.InTx(ctx, s.pool, func(tx *sqlx.Tx) error {
		return tx.GetContext(ctx, &l, queries.SQL.Get("InvitationLookup"), identitymodel.HashToken(token))
	})
	if db.IsNoRows(err) {
		return l, identitymodel.ErrInvitationNotFound
	}
	if err != nil {
		return l, err
	}
	return l, l.Usable(s.now())
}

// PreviewInvitation describes an invitation link (public; rate-limited by IP).
func (s *IdentityService) PreviewInvitation(ctx context.Context, token string) (identitymodel.InvitationPreview, error) {
	if err := s.limiter.Allow(ctx, ruleAcceptIP, principal.ClientInfoFrom(ctx).IP.String()); err != nil {
		return identitymodel.InvitationPreview{}, err
	}
	l, err := s.lookupInvitation(ctx, token)
	if err != nil {
		return identitymodel.InvitationPreview{}, err
	}
	return identitymodel.InvitationPreview{Organization: identitymodel.OrgSummary{ID: l.OrgID, Slug: l.OrgSlug, Name: l.OrgName}, Email: l.Email,
		InvitedByName: l.InviterName.String, ExpiresAt: l.ExpiresAt, AccountExists: l.UserExists}, nil
}

// AcceptInvitation joins the invited organization and signs in to it. The
// invitee proves who they are with a session for the invited address, the
// password of the existing account, or by creating the account.
func (s *IdentityService) AcceptInvitation(ctx context.Context, token string, in identitydto.AcceptInput) (identitymodel.SessionGrant, error) {
	if err := s.limiter.Allow(ctx, ruleAcceptIP, principal.ClientInfoFrom(ctx).IP.String()); err != nil {
		return identitymodel.SessionGrant{}, err
	}
	l, err := s.lookupInvitation(ctx, token)
	if err != nil {
		return identitymodel.SessionGrant{}, err
	}

	var (
		user    identitymodel.User
		newHash string // set when creating the account
	)
	if p, ok := principal.From(ctx); ok && p.Type == principal.User && in.Password == "" {
		// Signed in (to any organization) as the invited address.
		var row repository.MemberRow
		err := db.InTenantTx(ctx, s.pool, p.OrgID, func(tx *sqlx.Tx) error {
			return tx.GetContext(ctx, &row, queries.SQL.Get("GetMember"), p.OrgID, p.ID)
		})
		if err != nil {
			return identitymodel.SessionGrant{}, err
		}
		if !strings.EqualFold(row.Email, l.Email) {
			return identitymodel.SessionGrant{}, problem.NewProblem(http.StatusConflict, "signed_in_as_other", "Different account",
				"You are signed in as "+row.Email+", but this invitation is for "+l.Email+". Sign out, or enter the password of the invited account.")
		}
		user = identitymodel.User{ID: row.ID, Email: row.Email, DisplayName: row.DisplayName, Status: row.Status}
	} else {
		user, newHash, err = s.inviteeIdentity(ctx, l.Email, in)
		if err != nil {
			return identitymodel.SessionGrant{}, err
		}
	}

	org := identitymodel.OrgSummary{ID: l.OrgID, Slug: l.OrgSlug, Name: l.OrgName}
	var grant identitymodel.SessionGrant
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
		if err := tx.GetContext(ctx, &inv, commands.SQL.Get("LockInvitation"), l.ID); err != nil {
			return err
		}
		if err := (identitymodel.InvitationLookup{ExpiresAt: inv.ExpiresAt, AcceptedAt: inv.AcceptedAt, RevokedAt: inv.RevokedAt}).Usable(s.now()); err != nil {
			return err // lost a race with another accept or a revoke
		}
		if newHash != "" {
			if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertUser"), user.ID, user.Email, user.DisplayName); err != nil {
				if db.IsUniqueViolation(err) {
					return problem.NewProblem(http.StatusConflict, "account_exists", "Account exists",
						"An account with this email already exists. Enter its password instead.")
				}
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertMembershipIfAbsent"), l.OrgID, user.ID); err != nil {
			return err
		}
		var status string
		if err := tx.GetContext(ctx, &status, queries.SQL.Get("GetMembershipStatus"), user.ID); err != nil {
			return err
		}
		if status != identitymodel.MembershipActive {
			return problem.NewProblem(http.StatusForbidden, "membership_suspended", "Membership suspended",
				"Your membership in this organization is suspended. Ask an administrator to reactivate it.")
		}
		if newHash != "" {
			if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertCredentials"), user.ID, newHash); err != nil {
				return err
			}
		}
		if err := s.applyInvitationGrants(ctx, tx, l.OrgID, inv.ID, user.ID, inv.OrgRoleID, inv.InvitedBy.UUID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("MarkInvitationAccepted"), inv.ID, user.ID); err != nil {
			return err
		}
		actor := &auditmodel.Actor{Type: string(principal.User), ID: user.ID}
		if err := auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: l.OrgID, Action: "user.joined", ResourceType: identitymodel.UserResource,
			ResourceID: user.ID.String(), Actor: actor, Details: map[string]any{"invitation_id": inv.ID,
				"invited_by": inv.InvitedBy.UUID, "new_account": newHash != ""}}); err != nil {
			return err
		}
		if _, err := outbox.Publish(ctx, tx, outbox.Event{OrgID: l.OrgID, Subject: "hugi.events.user.joined",
			Payload: map[string]any{"user_id": user.ID, "invitation_id": inv.ID}}); err != nil {
			return err
		}
		grant, err = s.issueSession(ctx, tx, user, org, orgmodel.ParseSettings(l.OrgSettings), "invitation")
		return err
	})
	if err != nil {
		return identitymodel.SessionGrant{}, err
	}
	s.acl.Invalidate(ctx, l.OrgID)
	return grant, nil
}

// inviteeIdentity verifies the existing account's password, or validates the
// details of a new account (returning its password hash).
func (s *IdentityService) inviteeIdentity(ctx context.Context, email string, in identitydto.AcceptInput) (identitymodel.User, string, error) {
	if in.Password == "" {
		return identitymodel.User{}, "", problem.Validation(problem.FieldError{Field: "password", Code: "required", Message: "Enter a password."})
	}
	var (
		rec   identitymodel.LoginRecord
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
		return identitymodel.User{}, "", err
	}
	if found {
		if !ok || rec.UserStatus != "active" {
			return identitymodel.User{}, "", identitymodel.ErrInvalidCredentials
		}
		return identitymodel.User{ID: rec.UserID, Email: email, DisplayName: rec.DisplayName, Status: rec.UserStatus}, "", nil
	}

	name := strings.TrimSpace(in.DisplayName)
	var errs []problem.FieldError
	if n := len([]rune(name)); n < 1 || n > 100 {
		errs = append(errs, problem.FieldError{Field: "display_name", Code: "invalid_length", Message: "Name must be 1–100 characters."})
	}
	errs = append(errs, identitymodel.ValidatePassword("password", in.Password, email)...)
	if len(errs) > 0 {
		return identitymodel.User{}, "", problem.Validation(errs...)
	}
	hash, err := s.cfg.Hasher.Hash(in.Password)
	if err != nil {
		return identitymodel.User{}, "", err
	}
	return identitymodel.User{ID: uuid.Must(uuid.NewV7()), Email: email, DisplayName: name, Status: "active"}, hash, nil
}

// applyInvitationGrants binds the baseline member role, the optional
// organization role and the workspace roles (authorized when the invitation
// was created), then bumps the authz version.
func (s *IdentityService) applyInvitationGrants(ctx context.Context, tx *sqlx.Tx, org, invitation, user uuid.UUID, orgRole uuid.NullUUID, by uuid.UUID) error {
	who := authzmodel.UserRef(user)
	if err := authzsvc.BindRoleKeyInTx(ctx, tx, org, who, authzmodel.MemberRoleKey, authzmodel.ScopeRef{Type: authzmodel.ScopeOrg}, by); err != nil {
		return err
	}
	if orgRole.Valid {
		if err := authzsvc.BindPreauthorizedInTx(ctx, tx, org, who, orgRole.UUID, authzmodel.ScopeRef{Type: authzmodel.ScopeOrg}, by); err != nil {
			return err
		}
	}
	var grants []identitymodel.WorkspaceGrant
	if err := tx.SelectContext(ctx, &grants, queries.SQL.Get("InvitationWorkspaceGrants"), invitation); err != nil {
		return err
	}
	for _, g := range grants {
		if err := authzsvc.BindPreauthorizedInTx(ctx, tx, org, who, g.RoleID,
			authzmodel.ScopeRef{Type: authzmodel.ScopeWorkspace, WorkspaceID: g.WorkspaceID}, by); err != nil {
			return err
		}
	}
	return authzsvc.BumpVersion(ctx, tx, org)
}
