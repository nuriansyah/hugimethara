package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	authzmodel "github.com/nuriansyah/hugimethara/internal/modules/authorization/model"
	authzsvc "github.com/nuriansyah/hugimethara/internal/modules/authorization/services"
	auditmodel "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/model"
	auditsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/services"
	identitymodel "github.com/nuriansyah/hugimethara/internal/modules/identity/model"
	identitydto "github.com/nuriansyah/hugimethara/internal/modules/identity/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/identity/repository/commands"
	"github.com/nuriansyah/hugimethara/internal/modules/identity/repository/queries"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
)

func getGroup(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (identitymodel.Group, error) {
	var g identitymodel.Group
	err := tx.GetContext(ctx, &g, queries.SQL.Get("GetGroup"), id)
	if db.IsNoRows(err) {
		return g, identitymodel.ErrGroupNotFound
	}
	return g, err
}

// ListGroups lists the organization's groups (group:read).
func (s *IdentityService) ListGroups(ctx context.Context) ([]identitymodel.Group, error) {
	p, err := s.require(ctx, "read", identitymodel.GroupResource, "")
	if err != nil {
		return nil, err
	}
	out := []identitymodel.Group{}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error { return tx.SelectContext(ctx, &out, queries.SQL.Get("ListGroups")) })
	return out, err
}

// GetGroup returns one group (group:read).
func (s *IdentityService) GetGroup(ctx context.Context, id uuid.UUID) (identitymodel.Group, error) {
	p, err := s.require(ctx, "read", identitymodel.GroupResource, id.String())
	if err != nil {
		return identitymodel.Group{}, err
	}
	var g identitymodel.Group
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		var err error
		g, err = getGroup(ctx, tx, id)
		return err
	})
	return g, err
}

// CreateGroup creates an internal group (group:create).
func (s *IdentityService) CreateGroup(ctx context.Context, in identitydto.GroupInput) (identitymodel.Group, error) {
	p, err := s.require(ctx, "create", identitymodel.GroupResource, "")
	if err != nil {
		return identitymodel.Group{}, err
	}
	if err := identitymodel.ValidateNameDesc(&in.Name, &in.Description, "Group"); err != nil {
		return identitymodel.Group{}, err
	}
	id := uuid.Must(uuid.NewV7())
	var g identitymodel.Group
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertGroup"), id, p.OrgID, in.Name, in.Description, p.ID); err != nil {
			if db.IsUniqueViolation(err) {
				return identitymodel.ErrGroupNameTake
			}
			return err
		}
		var err error
		if g, err = getGroup(ctx, tx, id); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "group.created", ResourceType: identitymodel.GroupResource,
			ResourceID: id.String(), Details: map[string]any{"name": in.Name}})
	})
	return g, err
}

// UpdateGroup renames or re-describes a group (group:update).
func (s *IdentityService) UpdateGroup(ctx context.Context, id uuid.UUID, in identitydto.GroupInput) (identitymodel.Group, error) {
	p, err := s.require(ctx, "update", identitymodel.GroupResource, id.String())
	if err != nil {
		return identitymodel.Group{}, err
	}
	if err := identitymodel.ValidateNameDesc(&in.Name, &in.Description, "Group"); err != nil {
		return identitymodel.Group{}, err
	}
	var g identitymodel.Group
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		before, err := getGroup(ctx, tx, id)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("UpdateGroup"), id, in.Name, in.Description); err != nil {
			if db.IsUniqueViolation(err) {
				return identitymodel.ErrGroupNameTake
			}
			return err
		}
		if g, err = getGroup(ctx, tx, id); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "group.updated", ResourceType: identitymodel.GroupResource,
			ResourceID: id.String(), Details: map[string]any{
				"from": map[string]any{"name": before.Name, "description": before.Description},
				"to":   map[string]any{"name": g.Name, "description": g.Description}}})
	})
	return g, err
}

// DeleteGroup deletes a group with its memberships and role assignments (group:delete).
func (s *IdentityService) DeleteGroup(ctx context.Context, id uuid.UUID) error {
	p, err := s.require(ctx, "delete", identitymodel.GroupResource, id.String())
	if err != nil {
		return err
	}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		g, err := getGroup(ctx, tx, id)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("DeleteGroup"), id); err != nil {
			return err
		}
		if err := authzsvc.BumpVersion(ctx, tx, p.OrgID); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "group.deleted", ResourceType: identitymodel.GroupResource,
			ResourceID: id.String(), Details: map[string]any{"name": g.Name, "member_count": g.MemberCount}})
	})
	if err == nil {
		s.acl.Invalidate(ctx, p.OrgID)
	}
	return err
}

// ListGroupMembers lists a group's members (group:read).
func (s *IdentityService) ListGroupMembers(ctx context.Context, id uuid.UUID) ([]identitymodel.GroupMember, error) {
	p, err := s.require(ctx, "read", identitymodel.GroupResource, id.String())
	if err != nil {
		return nil, err
	}
	out := []identitymodel.GroupMember{}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		if _, err := getGroup(ctx, tx, id); err != nil {
			return err
		}
		return tx.SelectContext(ctx, &out, queries.SQL.Get("ListGroupMembers"), id)
	})
	return out, err
}

// AddGroupMember adds an organization member to a group (group:manage_members).
// Joining a group hands out the group's roles, so the caller must be able to
// grant every one of them (no escalation through groups).
func (s *IdentityService) AddGroupMember(ctx context.Context, groupID, userID uuid.UUID) error {
	p, err := s.require(ctx, "manage_members", identitymodel.GroupResource, groupID.String())
	if err != nil {
		return err
	}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		g, err := getGroup(ctx, tx, groupID)
		if err != nil {
			return err
		}
		if g.Source != "internal" {
			return identitymodel.ErrGroupIdP
		}
		if err := s.acl.CheckCanHandOut(ctx, tx, p, authzmodel.PrincipalRef{Type: authzmodel.PrincipalGroup, ID: groupID}); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, commands.SQL.Get("InsertGroupMember"), p.OrgID, groupID, userID, p.ID)
		if db.IsForeignKeyViolation(err) {
			return problem.Validation(problem.FieldError{Field: "user_id", Code: "not_a_member", Message: "This user is not a member of the organization."})
		}
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil // already a member
		}
		if err := authzsvc.BumpVersion(ctx, tx, p.OrgID); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "group.member_added", ResourceType: identitymodel.GroupResource,
			ResourceID: groupID.String(), Details: map[string]any{"user_id": userID, "group": g.Name}})
	})
	if err == nil {
		s.acl.Invalidate(ctx, p.OrgID)
	}
	return err
}

// RemoveGroupMember removes a member from a group (group:manage_members).
func (s *IdentityService) RemoveGroupMember(ctx context.Context, groupID, userID uuid.UUID) error {
	p, err := s.require(ctx, "manage_members", identitymodel.GroupResource, groupID.String())
	if err != nil {
		return err
	}
	err = s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		g, err := getGroup(ctx, tx, groupID)
		if err != nil {
			return err
		}
		if g.Source != "internal" {
			return identitymodel.ErrGroupIdP
		}
		n, err := execRows(ctx, tx, commands.SQL.Get("DeleteGroupMember"), groupID, userID)
		if err != nil {
			return err
		}
		if n == 0 {
			return problem.NotFound("This user is not a member of the group.")
		}
		if err := authzsvc.BumpVersion(ctx, tx, p.OrgID); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "group.member_removed", ResourceType: identitymodel.GroupResource,
			ResourceID: groupID.String(), Details: map[string]any{"user_id": userID, "group": g.Name}})
	})
	if err == nil {
		s.acl.Invalidate(ctx, p.OrgID)
	}
	return err
}
