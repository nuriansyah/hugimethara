package services

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/infrastructure/persistence/outbox"
	authzmodel "github.com/nuriansyah/hugimethara/internal/modules/authorization/model"
	authzsvc "github.com/nuriansyah/hugimethara/internal/modules/authorization/services"
	auditmodel "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/model"
	auditsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/services"
	identitymodel "github.com/nuriansyah/hugimethara/internal/modules/identity/model"
	"github.com/nuriansyah/hugimethara/internal/modules/identity/repository"
	"github.com/nuriansyah/hugimethara/internal/modules/identity/repository/commands"
	"github.com/nuriansyah/hugimethara/internal/modules/identity/repository/queries"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	apperrors "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

// require authorizes action on an organization-level resource of this domain.
func (s *IdentityService) require(ctx context.Context, action, resourceType, id string) (principal.Principal, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return p, authzmodel.Unauthenticated()
	}
	return authzsvc.Require(ctx, s.az, action, authzmodel.Resource{Type: resourceType, ID: id, OrgID: p.OrgID})
}

func (s *IdentityService) tenantTx(ctx context.Context, p principal.Principal, fn func(*sqlx.Tx) error) error {
	return db.InTenantTx(ctx, s.pool, p.OrgID, fn)
}

// ListMembers lists the organization's members (user:read).
func (s *IdentityService) ListMembers(ctx context.Context) ([]identitymodel.OrgMember, error) {
	p, err := s.require(ctx, "read", identitymodel.UserResource, "")
	if err != nil {
		return nil, err
	}
	var rows []repository.OrgMemberRow
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error { return tx.SelectContext(ctx, &rows, queries.SQL.Get("ListMembers")) }); err != nil {
		return nil, err
	}
	out := make([]identitymodel.OrgMember, len(rows))
	for i, r := range rows {
		out[i] = r.ToModel()
	}
	return out, nil
}

// GetMember returns one member with their groups (user:read).
func (s *IdentityService) GetMember(ctx context.Context, id uuid.UUID) (identitymodel.OrgMember, error) {
	p, err := s.require(ctx, "read", identitymodel.UserResource, id.String())
	if err != nil {
		return identitymodel.OrgMember{}, err
	}
	var row repository.OrgMemberRow
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error { return tx.GetContext(ctx, &row, queries.SQL.Get("GetMemberDetail"), id) })
	if db.IsNoRows(err) {
		return identitymodel.OrgMember{}, identitymodel.ErrMemberNotFound
	}
	return row.ToModel(), err
}

// SetMemberStatus suspends or reactivates a member (user:update). Suspension
// ends the member's sessions in this organization and removes their access
// (grants require an active membership). Nobody can suspend themselves or the
// last owner.
func (s *IdentityService) SetMemberStatus(ctx context.Context, id uuid.UUID, status string) (identitymodel.OrgMember, error) {
	if status != identitymodel.MembershipActive && status != identitymodel.MembershipSuspended {
		return identitymodel.OrgMember{}, apperrors.Validation(apperrors.FieldError{Field: "status", Code: "invalid", Message: "Status must be active or suspended."})
	}
	p, err := s.require(ctx, "update", identitymodel.UserResource, id.String())
	if err != nil {
		return identitymodel.OrgMember{}, err
	}
	if id == p.ID && status == identitymodel.MembershipSuspended {
		return identitymodel.OrgMember{}, apperrors.NewProblem(http.StatusConflict, "cannot_suspend_self", "Conflict", "You cannot suspend yourself.")
	}
	var row repository.OrgMemberRow
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		if err := tx.GetContext(ctx, &row, queries.SQL.Get("GetMemberDetail"), id); err != nil {
			if db.IsNoRows(err) {
				return identitymodel.ErrMemberNotFound
			}
			return err
		}
		if row.MembershipStatus == status {
			return nil
		}
		if status == identitymodel.MembershipSuspended {
			if err := authzsvc.EnsureOwnerRemainsWithout(ctx, tx, id); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, commands.SQL.Get("RevokeAllUserSessions"), id); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("SetMembershipStatus"), id, status); err != nil {
			return err
		}
		if err := authzsvc.BumpVersion(ctx, tx, p.OrgID); err != nil {
			return err
		}
		action := "user.suspended"
		if status == identitymodel.MembershipActive {
			action = "user.reactivated"
		}
		if err := auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: action, ResourceType: identitymodel.UserResource,
			ResourceID: id.String(), Details: map[string]any{"email": row.Email}}); err != nil {
			return err
		}
		row.MembershipStatus = status
		return nil
	})
	if err != nil {
		return identitymodel.OrgMember{}, err
	}
	s.acl.Invalidate(ctx, p.OrgID)
	return row.ToModel(), nil
}

// RemoveMember removes a member from the organization (user:remove): their
// role assignments, group memberships and sessions here end. The global
// account is kept (it may belong to other organizations).
func (s *IdentityService) RemoveMember(ctx context.Context, id uuid.UUID) error {
	p, err := s.require(ctx, "remove", identitymodel.UserResource, id.String())
	if err != nil {
		return err
	}
	if id == p.ID {
		return apperrors.NewProblem(http.StatusConflict, "cannot_remove_self", "Conflict", "You cannot remove yourself from the organization.")
	}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		var row repository.OrgMemberRow
		if err := tx.GetContext(ctx, &row, queries.SQL.Get("GetMemberDetail"), id); err != nil {
			if db.IsNoRows(err) {
				return identitymodel.ErrMemberNotFound
			}
			return err
		}
		if err := authzsvc.EnsureOwnerRemainsWithout(ctx, tx, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("RevokeAllUserSessions"), id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("DeleteMembership"), id); err != nil {
			return err
		}
		if err := authzsvc.BumpVersion(ctx, tx, p.OrgID); err != nil {
			return err
		}
		if err := auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "user.removed", ResourceType: identitymodel.UserResource,
			ResourceID: id.String(), Details: map[string]any{"email": row.Email, "roles": row.ToModel().Roles}}); err != nil {
			return err
		}
		_, err := outbox.Publish(ctx, tx, outbox.Event{OrgID: p.OrgID, Subject: "hugi.events.user.removed",
			Payload: map[string]any{"user_id": id}})
		return err
	})
	if err == nil {
		s.acl.Invalidate(ctx, p.OrgID)
	}
	return err
}
