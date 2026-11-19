package identity

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/backend/internal/audit"
	"github.com/nuriansyah/hugimethara/backend/internal/authz"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/db"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/httpx"
)

// Group is a set of members that roles can be assigned to.
type Group struct {
	ID          uuid.UUID `db:"id"`
	Name        string    `db:"name"`
	Description string    `db:"description"`
	Source      string    `db:"source"` // internal | idp (managed by an identity provider, S5)
	MemberCount int64     `db:"member_count"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// GroupMember is one member of a group.
type GroupMember struct {
	ID          uuid.UUID `db:"id"`
	Email       string    `db:"email"`
	DisplayName string    `db:"display_name"`
	AddedAt     time.Time `db:"added_at"`
}

// GroupInput creates or updates a group.
type GroupInput struct {
	Name        string
	Description string
}

var (
	errGroupNotFound = httpx.NotFound("Group not found.")
	errGroupNameTake = httpx.NewProblem(http.StatusConflict, "name_taken", "Conflict", "A group with this name already exists.")
	errGroupIdP      = httpx.NewProblem(http.StatusConflict, "group_managed_by_idp", "Conflict",
		"This group's members come from your identity provider. Change them there.")
)

func validateNameDesc(name, desc *string, label string) error {
	*name, *desc = strings.TrimSpace(*name), strings.TrimSpace(*desc)
	var errs []httpx.FieldError
	if n := len([]rune(*name)); n < 1 || n > 100 {
		errs = append(errs, httpx.FieldError{Field: "name", Code: "invalid_length", Message: label + " name must be 1–100 characters."})
	}
	if len([]rune(*desc)) > 500 {
		errs = append(errs, httpx.FieldError{Field: "description", Code: "invalid_length", Message: "Description must be at most 500 characters."})
	}
	if len(errs) > 0 {
		return httpx.Validation(errs...)
	}
	return nil
}

func getGroup(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (Group, error) {
	var g Group
	err := tx.GetContext(ctx, &g, q.Get("GetGroup"), id)
	if db.IsNoRows(err) {
		return g, errGroupNotFound
	}
	return g, err
}

// ListGroups lists the organization's groups (group:read).
func (s *Service) ListGroups(ctx context.Context) ([]Group, error) {
	p, err := s.require(ctx, "read", GroupResource, "")
	if err != nil {
		return nil, err
	}
	out := []Group{}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error { return tx.SelectContext(ctx, &out, q.Get("ListGroups")) })
	return out, err
}

// GetGroup returns one group (group:read).
func (s *Service) GetGroup(ctx context.Context, id uuid.UUID) (Group, error) {
	p, err := s.require(ctx, "read", GroupResource, id.String())
	if err != nil {
		return Group{}, err
	}
	var g Group
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		var err error
		g, err = getGroup(ctx, tx, id)
		return err
	})
	return g, err
}

// CreateGroup creates an internal group (group:create).
func (s *Service) CreateGroup(ctx context.Context, in GroupInput) (Group, error) {
	p, err := s.require(ctx, "create", GroupResource, "")
	if err != nil {
		return Group{}, err
	}
	if err := validateNameDesc(&in.Name, &in.Description, "Group"); err != nil {
		return Group{}, err
	}
	id := uuid.Must(uuid.NewV7())
	var g Group
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, q.Get("InsertGroup"), id, p.OrgID, in.Name, in.Description, p.ID); err != nil {
			if db.IsUniqueViolation(err) {
				return errGroupNameTake
			}
			return err
		}
		var err error
		if g, err = getGroup(ctx, tx, id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "group.created", ResourceType: GroupResource,
			ResourceID: id.String(), Details: map[string]any{"name": in.Name}})
	})
	return g, err
}

// UpdateGroup renames or re-describes a group (group:update).
func (s *Service) UpdateGroup(ctx context.Context, id uuid.UUID, in GroupInput) (Group, error) {
	p, err := s.require(ctx, "update", GroupResource, id.String())
	if err != nil {
		return Group{}, err
	}
	if err := validateNameDesc(&in.Name, &in.Description, "Group"); err != nil {
		return Group{}, err
	}
	var g Group
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		before, err := getGroup(ctx, tx, id)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, q.Get("UpdateGroup"), id, in.Name, in.Description); err != nil {
			if db.IsUniqueViolation(err) {
				return errGroupNameTake
			}
			return err
		}
		if g, err = getGroup(ctx, tx, id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "group.updated", ResourceType: GroupResource,
			ResourceID: id.String(), Details: map[string]any{
				"from": map[string]any{"name": before.Name, "description": before.Description},
				"to":   map[string]any{"name": g.Name, "description": g.Description}}})
	})
	return g, err
}

// DeleteGroup deletes a group with its memberships and role assignments (group:delete).
func (s *Service) DeleteGroup(ctx context.Context, id uuid.UUID) error {
	p, err := s.require(ctx, "delete", GroupResource, id.String())
	if err != nil {
		return err
	}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		g, err := getGroup(ctx, tx, id)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, q.Get("DeleteGroup"), id); err != nil {
			return err
		}
		if err := authz.BumpVersion(ctx, tx, p.OrgID); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "group.deleted", ResourceType: GroupResource,
			ResourceID: id.String(), Details: map[string]any{"name": g.Name, "member_count": g.MemberCount}})
	})
	if err == nil {
		s.acl.Invalidate(ctx, p.OrgID)
	}
	return err
}

// ListGroupMembers lists a group's members (group:read).
func (s *Service) ListGroupMembers(ctx context.Context, id uuid.UUID) ([]GroupMember, error) {
	p, err := s.require(ctx, "read", GroupResource, id.String())
	if err != nil {
		return nil, err
	}
	out := []GroupMember{}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		if _, err := getGroup(ctx, tx, id); err != nil {
			return err
		}
		return tx.SelectContext(ctx, &out, q.Get("ListGroupMembers"), id)
	})
	return out, err
}

// AddGroupMember adds an organization member to a group (group:manage_members).
// Joining a group hands out the group's roles, so the caller must be able to
// grant every one of them (no escalation through groups).
func (s *Service) AddGroupMember(ctx context.Context, groupID, userID uuid.UUID) error {
	p, err := s.require(ctx, "manage_members", GroupResource, groupID.String())
	if err != nil {
		return err
	}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		g, err := getGroup(ctx, tx, groupID)
		if err != nil {
			return err
		}
		if g.Source != "internal" {
			return errGroupIdP
		}
		if err := s.acl.CheckCanHandOut(ctx, tx, p, authz.PrincipalRef{Type: authz.PrincipalGroup, ID: groupID}); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, q.Get("InsertGroupMember"), p.OrgID, groupID, userID, p.ID)
		if db.IsForeignKeyViolation(err) {
			return httpx.Validation(httpx.FieldError{Field: "user_id", Code: "not_a_member", Message: "This user is not a member of the organization."})
		}
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil // already a member
		}
		if err := authz.BumpVersion(ctx, tx, p.OrgID); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "group.member_added", ResourceType: GroupResource,
			ResourceID: groupID.String(), Details: map[string]any{"user_id": userID, "group": g.Name}})
	})
	if err == nil {
		s.acl.Invalidate(ctx, p.OrgID)
	}
	return err
}

// RemoveGroupMember removes a member from a group (group:manage_members).
func (s *Service) RemoveGroupMember(ctx context.Context, groupID, userID uuid.UUID) error {
	p, err := s.require(ctx, "manage_members", GroupResource, groupID.String())
	if err != nil {
		return err
	}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		g, err := getGroup(ctx, tx, groupID)
		if err != nil {
			return err
		}
		if g.Source != "internal" {
			return errGroupIdP
		}
		n, err := execRows(ctx, tx, q.Get("DeleteGroupMember"), groupID, userID)
		if err != nil {
			return err
		}
		if n == 0 {
			return httpx.NotFound("This user is not a member of the group.")
		}
		if err := authz.BumpVersion(ctx, tx, p.OrgID); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "group.member_removed", ResourceType: GroupResource,
			ResourceID: groupID.String(), Details: map[string]any{"user_id": userID, "group": g.Name}})
	})
	if err == nil {
		s.acl.Invalidate(ctx, p.OrgID)
	}
	return err
}
