package identity

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/backend/internal/audit"
	"github.com/nuriansyah/hugimethara/backend/internal/authz"
	"github.com/nuriansyah/hugimethara/backend/internal/event"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/db"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/httpx"
	"github.com/nuriansyah/hugimethara/backend/internal/principal"
)

// Resource types owned by the identity domain.
const (
	UserResource           = "user"
	GroupResource          = "group"
	ServiceAccountResource = "service_account"
)

// Membership statuses.
const (
	MembershipActive    = "active"
	MembershipSuspended = "suspended"
)

// OrgMember is a member of the caller's organization.
type OrgMember struct {
	User             User
	MembershipStatus string
	JoinedAt         time.Time
	IsOwner          bool
	Roles            []string
	Groups           []GroupRef
}

// GroupRef names a group.
type GroupRef struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type orgMemberRow struct {
	ID               uuid.UUID `db:"id"`
	Email            string    `db:"email"`
	DisplayName      string    `db:"display_name"`
	Status           string    `db:"status"`
	MembershipStatus string    `db:"membership_status"`
	JoinedAt         time.Time `db:"joined_at"`
	IsOwner          bool      `db:"is_owner"`
	Roles            []byte    `db:"roles"`
	Groups           []byte    `db:"groups"`
}

func (r orgMemberRow) toModel() OrgMember {
	m := OrgMember{User: User{ID: r.ID, Email: r.Email, DisplayName: r.DisplayName, Status: r.Status},
		MembershipStatus: r.MembershipStatus, JoinedAt: r.JoinedAt, IsOwner: r.IsOwner, Roles: []string{}, Groups: []GroupRef{}}
	_ = json.Unmarshal(r.Roles, &m.Roles)
	_ = json.Unmarshal(r.Groups, &m.Groups)
	return m
}

var errMemberNotFound = httpx.NotFound("Member not found.")

// require authorizes action on an organization-level resource of this domain.
func (s *Service) require(ctx context.Context, action, resourceType, id string) (principal.Principal, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return p, authz.Unauthenticated()
	}
	return authz.Require(ctx, s.az, action, authz.Resource{Type: resourceType, ID: id, OrgID: p.OrgID})
}

func (s *Service) tenantTx(ctx context.Context, p principal.Principal, fn func(*sqlx.Tx) error) error {
	return db.InTenantTx(ctx, s.pool, p.OrgID, fn)
}

// ListMembers lists the organization's members (user:read).
func (s *Service) ListMembers(ctx context.Context) ([]OrgMember, error) {
	p, err := s.require(ctx, "read", UserResource, "")
	if err != nil {
		return nil, err
	}
	var rows []orgMemberRow
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error { return tx.SelectContext(ctx, &rows, q.Get("ListMembers")) }); err != nil {
		return nil, err
	}
	out := make([]OrgMember, len(rows))
	for i, r := range rows {
		out[i] = r.toModel()
	}
	return out, nil
}

// GetMember returns one member with their groups (user:read).
func (s *Service) GetMember(ctx context.Context, id uuid.UUID) (OrgMember, error) {
	p, err := s.require(ctx, "read", UserResource, id.String())
	if err != nil {
		return OrgMember{}, err
	}
	var row orgMemberRow
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error { return tx.GetContext(ctx, &row, q.Get("GetMemberDetail"), id) })
	if db.IsNoRows(err) {
		return OrgMember{}, errMemberNotFound
	}
	return row.toModel(), err
}

// SetMemberStatus suspends or reactivates a member (user:update). Suspension
// ends the member's sessions in this organization and removes their access
// (grants require an active membership). Nobody can suspend themselves or the
// last owner.
func (s *Service) SetMemberStatus(ctx context.Context, id uuid.UUID, status string) (OrgMember, error) {
	if status != MembershipActive && status != MembershipSuspended {
		return OrgMember{}, httpx.Validation(httpx.FieldError{Field: "status", Code: "invalid", Message: "Status must be active or suspended."})
	}
	p, err := s.require(ctx, "update", UserResource, id.String())
	if err != nil {
		return OrgMember{}, err
	}
	if id == p.ID && status == MembershipSuspended {
		return OrgMember{}, httpx.NewProblem(http.StatusConflict, "cannot_suspend_self", "Conflict", "You cannot suspend yourself.")
	}
	var row orgMemberRow
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		if err := tx.GetContext(ctx, &row, q.Get("GetMemberDetail"), id); err != nil {
			if db.IsNoRows(err) {
				return errMemberNotFound
			}
			return err
		}
		if row.MembershipStatus == status {
			return nil
		}
		if status == MembershipSuspended {
			if err := authz.EnsureOwnerRemainsWithout(ctx, tx, id); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, q.Get("RevokeAllUserSessions"), id); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, q.Get("SetMembershipStatus"), id, status); err != nil {
			return err
		}
		if err := authz.BumpVersion(ctx, tx, p.OrgID); err != nil {
			return err
		}
		action := "user.suspended"
		if status == MembershipActive {
			action = "user.reactivated"
		}
		if err := audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: action, ResourceType: UserResource,
			ResourceID: id.String(), Details: map[string]any{"email": row.Email}}); err != nil {
			return err
		}
		row.MembershipStatus = status
		return nil
	})
	if err != nil {
		return OrgMember{}, err
	}
	s.acl.Invalidate(ctx, p.OrgID)
	return row.toModel(), nil
}

// RemoveMember removes a member from the organization (user:remove): their
// role assignments, group memberships and sessions here end. The global
// account is kept (it may belong to other organizations).
func (s *Service) RemoveMember(ctx context.Context, id uuid.UUID) error {
	p, err := s.require(ctx, "remove", UserResource, id.String())
	if err != nil {
		return err
	}
	if id == p.ID {
		return httpx.NewProblem(http.StatusConflict, "cannot_remove_self", "Conflict", "You cannot remove yourself from the organization.")
	}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		var row orgMemberRow
		if err := tx.GetContext(ctx, &row, q.Get("GetMemberDetail"), id); err != nil {
			if db.IsNoRows(err) {
				return errMemberNotFound
			}
			return err
		}
		if err := authz.EnsureOwnerRemainsWithout(ctx, tx, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, q.Get("RevokeAllUserSessions"), id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, q.Get("DeleteMembership"), id); err != nil {
			return err
		}
		if err := authz.BumpVersion(ctx, tx, p.OrgID); err != nil {
			return err
		}
		if err := audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "user.removed", ResourceType: UserResource,
			ResourceID: id.String(), Details: map[string]any{"email": row.Email, "roles": row.toModel().Roles}}); err != nil {
			return err
		}
		_, err := event.Publish(ctx, tx, event.Event{OrgID: p.OrgID, Subject: "hugi.events.user.removed",
			Payload: map[string]any{"user_id": id}})
		return err
	})
	if err == nil {
		s.acl.Invalidate(ctx, p.OrgID)
	}
	return err
}
