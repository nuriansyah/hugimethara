package services

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/infrastructure/persistence/outbox"
	authzmodel "github.com/nuriansyah/hugimethara/internal/modules/authorization/model"
	authzdto "github.com/nuriansyah/hugimethara/internal/modules/authorization/model/dto"
	authzsvc "github.com/nuriansyah/hugimethara/internal/modules/authorization/services"
	auditmodel "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/model"
	auditsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/services"
	entmodel "github.com/nuriansyah/hugimethara/internal/modules/governance/entitlement/model"
	entsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/entitlement/services"
	"github.com/nuriansyah/hugimethara/internal/modules/organization/model"
	"github.com/nuriansyah/hugimethara/internal/modules/organization/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/organization/repository"
	"github.com/nuriansyah/hugimethara/internal/modules/organization/repository/commands"
	"github.com/nuriansyah/hugimethara/internal/modules/organization/repository/queries"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

// ListWorkspaces returns the workspaces the caller may see: all of them for
// principals with org-wide scope, otherwise only those they belong to.
func (s *OrganizationService) ListWorkspaces(ctx context.Context, includeArchived bool) ([]model.Workspace, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return nil, authzmodel.Unauthenticated()
	}
	scope, err := s.scopes.WorkspaceScope(ctx, p, "read")
	if err != nil {
		return nil, err
	}
	if scope.None() {
		return []model.Workspace{}, nil
	}
	var ids any
	if !scope.All {
		ids = scope.WorkspaceIDs
	}
	var rows []repository.WorkspaceRow
	if err := s.tx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, queries.SQL.Get("ListWorkspaces"), includeArchived, ids)
	}); err != nil {
		return nil, err
	}
	out := make([]model.Workspace, len(rows))
	for i, r := range rows {
		out[i] = r.ToModel()
	}
	return out, nil
}

// loadWorkspace fetches by slug and authorizes action on it. Unknown and
// forbidden-to-read workspaces both yield 404 (no existence oracle).
func (s *OrganizationService) loadWorkspace(ctx context.Context, tx *sqlx.Tx, p principal.Principal, slug, action string) (repository.WorkspaceRow, error) {
	var w repository.WorkspaceRow
	if err := tx.GetContext(ctx, &w, queries.SQL.Get("GetWorkspaceBySlug"), strings.ToLower(slug)); err != nil {
		if db.IsNoRows(err) {
			return w, model.ErrWorkspaceNotFound
		}
		return w, err
	}
	res := authzmodel.Resource{Type: model.WorkspaceResource, ID: w.ID.String(), OrgID: p.OrgID, WorkspaceID: &w.ID}
	if action != "read" {
		// Callers who cannot even read the workspace learn nothing about it.
		if d, err := s.az.Check(ctx, authzdto.AuthorizationInput{Principal: p, Action: "read", Resource: res}); err != nil {
			return w, err
		} else if !d.Allowed {
			return w, model.ErrWorkspaceNotFound
		}
	}
	if _, err := authzsvc.Require(ctx, s.az, action, res); err != nil {
		var pr *problem.Problem
		if errors.As(err, &pr) && pr.Status == 403 && action == "read" {
			return w, model.ErrWorkspaceNotFound
		}
		return w, err
	}
	return w, nil
}

// GetWorkspace returns a workspace by slug.
func (s *OrganizationService) GetWorkspace(ctx context.Context, slug string) (model.Workspace, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return model.Workspace{}, authzmodel.Unauthenticated()
	}
	var w repository.WorkspaceRow
	err := s.tx(ctx, p, func(tx *sqlx.Tx) error {
		var err error
		w, err = s.loadWorkspace(ctx, tx, p, slug, "read")
		return err
	})
	return w.ToModel(), err
}

func (s *OrganizationService) validateWorkspaceInput(in *dto.WorkspaceInput, create bool) error {
	in.Name, in.Description = strings.TrimSpace(in.Name), strings.TrimSpace(in.Description)
	var slugErr *problem.FieldError
	if create {
		in.Slug = strings.ToLower(strings.TrimSpace(in.Slug))
		slugErr = model.ValidateShortSlug("slug", in.Slug)
	}
	return model.Collect(slugErr, model.ValidateText("name", in.Name, 1, 100, "Name"),
		model.ValidateText("description", in.Description, 0, 500, "Description"))
}

// CreateWorkspace creates a workspace and makes the creator its first member.
func (s *OrganizationService) CreateWorkspace(ctx context.Context, in dto.WorkspaceInput) (model.Workspace, error) {
	p, err := s.authorize(ctx, "create", model.WorkspaceResource, "", nil)
	if err != nil {
		return model.Workspace{}, err
	}
	if err := s.validateWorkspaceInput(&in, true); err != nil {
		return model.Workspace{}, err
	}
	var out repository.WorkspaceRow
	err = s.tx(ctx, p, func(tx *sqlx.Tx) error {
		if err := entsvc.LockFeature(ctx, tx, p.OrgID, entmodel.WorkspacesMax); err != nil {
			return err
		}
		ent, err := entsvc.Load(ctx, tx, p.OrgID)
		if err != nil {
			return err
		}
		if in.DepartmentID != nil {
			if err := ent.Require(entmodel.Departments); err != nil {
				return err
			}
		}
		var active int64
		if err := tx.GetContext(ctx, &active, queries.SQL.Get("CountActiveWorkspaces")); err != nil {
			return err
		}
		if err := ent.CheckLimit(entmodel.WorkspacesMax, active, 1); err != nil {
			return err
		}
		id := uuid.Must(uuid.NewV7())
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertWorkspace"), id, p.OrgID, in.Slug, in.Name, in.Description,
			model.Nullable(in.DepartmentID), p.ID); err != nil {
			switch {
			case db.IsUniqueViolation(err):
				return model.Conflict("slug_taken", "A workspace with this address already exists.")
			case db.IsForeignKeyViolation(err):
				return problem.Validation(problem.FieldError{Field: "department_id", Code: "not_found", Message: "Department not found."})
			}
			return err
		}
		// The creator administers what they create.
		if err := authzsvc.GrantCreatorInTx(ctx, tx, p, id); err != nil {
			return err
		}
		if err := tx.GetContext(ctx, &out, queries.SQL.Get("GetWorkspaceBySlug"), in.Slug); err != nil {
			return err
		}
		if err := auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "workspace.created",
			ResourceType: model.WorkspaceResource, ResourceID: id.String(), WorkspaceID: &id,
			Details: map[string]any{"slug": in.Slug, "name": in.Name}}); err != nil {
			return err
		}
		_, err = outbox.Publish(ctx, tx, outbox.Event{OrgID: p.OrgID, Subject: "hugi.events.workspace.created",
			Payload: map[string]any{"workspace_id": id, "slug": in.Slug}})
		return err
	})
	if err == nil {
		s.acl.Invalidate(ctx, p.OrgID)
	}
	return out.ToModel(), err
}

// UpdateWorkspace changes name, description and department.
func (s *OrganizationService) UpdateWorkspace(ctx context.Context, slug string, in dto.WorkspaceInput) (model.Workspace, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return model.Workspace{}, authzmodel.Unauthenticated()
	}
	if err := s.validateWorkspaceInput(&in, false); err != nil {
		return model.Workspace{}, err
	}
	var out repository.WorkspaceRow
	err := s.tx(ctx, p, func(tx *sqlx.Tx) error {
		before, err := s.loadWorkspace(ctx, tx, p, slug, "update")
		if err != nil {
			return err
		}
		prevDept := before.DepartmentID
		changingDept := (in.DepartmentID == nil) != !prevDept.Valid || (in.DepartmentID != nil && *in.DepartmentID != prevDept.UUID)
		if in.DepartmentID != nil && changingDept {
			ent, err := entsvc.Load(ctx, tx, p.OrgID)
			if err != nil {
				return err
			}
			if err := ent.Require(entmodel.Departments); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("UpdateWorkspace"), before.ID, in.Name, in.Description, model.Nullable(in.DepartmentID)); err != nil {
			if db.IsForeignKeyViolation(err) {
				return problem.Validation(problem.FieldError{Field: "department_id", Code: "not_found", Message: "Department not found."})
			}
			return err
		}
		if err := tx.GetContext(ctx, &out, queries.SQL.Get("GetWorkspaceBySlug"), before.Slug); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "workspace.updated",
			ResourceType: model.WorkspaceResource, ResourceID: before.ID.String(), WorkspaceID: &before.ID,
			Details: map[string]any{
				"from": map[string]any{"name": before.Name, "description": before.Description, "department": before.DepartmentName.String},
				"to":   map[string]any{"name": out.Name, "description": out.Description, "department": out.DepartmentName.String},
			}})
	})
	return out.ToModel(), err
}

// SetWorkspaceArchived archives (true) or restores (false) a workspace.
// Restoring counts against the workspace quota again.
func (s *OrganizationService) SetWorkspaceArchived(ctx context.Context, slug string, archived bool) (model.Workspace, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return model.Workspace{}, authzmodel.Unauthenticated()
	}
	action, verb := "archive", "workspace.archived"
	if !archived {
		verb = "workspace.restored"
	}
	var out repository.WorkspaceRow
	err := s.tx(ctx, p, func(tx *sqlx.Tx) error {
		w, err := s.loadWorkspace(ctx, tx, p, slug, action)
		if err != nil {
			return err
		}
		if !archived {
			if err := entsvc.LockFeature(ctx, tx, p.OrgID, entmodel.WorkspacesMax); err != nil {
				return err
			}
			ent, err := entsvc.Load(ctx, tx, p.OrgID)
			if err != nil {
				return err
			}
			var active int64
			if err := tx.GetContext(ctx, &active, queries.SQL.Get("CountActiveWorkspaces")); err != nil {
				return err
			}
			if err := ent.CheckLimit(entmodel.WorkspacesMax, active, 1); err != nil {
				return err
			}
		}
		n, err := model.RowsAffected(tx.ExecContext(ctx, commands.SQL.Get("SetWorkspaceArchived"), w.ID, archived))
		if err != nil {
			return err
		}
		if n == 0 {
			state := "active"
			if archived {
				state = "archived"
			}
			return model.Conflict("already_"+state, "The workspace is already "+state+".")
		}
		if err := tx.GetContext(ctx, &out, queries.SQL.Get("GetWorkspaceBySlug"), w.Slug); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: verb,
			ResourceType: model.WorkspaceResource, ResourceID: w.ID.String(), WorkspaceID: &w.ID})
	})
	return out.ToModel(), err
}

// ListWorkspaceMembers lists members of a workspace the caller can read.
func (s *OrganizationService) ListWorkspaceMembers(ctx context.Context, slug string) ([]model.Member, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return nil, authzmodel.Unauthenticated()
	}
	var rows []repository.MemberRow
	err := s.tx(ctx, p, func(tx *sqlx.Tx) error {
		w, err := s.loadWorkspace(ctx, tx, p, slug, "read")
		if err != nil {
			return err
		}
		return tx.SelectContext(ctx, &rows, queries.SQL.Get("ListWorkspaceMembers"), w.ID)
	})
	return repository.MembersFrom(rows), err
}

// AddWorkspaceMember gives an organization member a role in a workspace
// (idempotent). Requires workspace:manage_members, and the caller cannot grant
// a role with permissions they do not hold in that workspace.
func (s *OrganizationService) AddWorkspaceMember(ctx context.Context, slug string, userID uuid.UUID, roleKey string) error {
	p, ok := principal.From(ctx)
	if !ok {
		return authzmodel.Unauthenticated()
	}
	if roleKey == "" {
		roleKey = model.DefaultWorkspaceRole
	}
	err := s.tx(ctx, p, func(tx *sqlx.Tx) error {
		w, err := s.loadWorkspace(ctx, tx, p, slug, "manage_members")
		if err != nil {
			return err
		}
		if w.ArchivedAt.Valid {
			return model.Conflict("workspace_archived", "Restore the workspace before changing its members.")
		}
		id, err := s.acl.BindInTx(ctx, tx, p, authzdto.BindingInput{Principal: authzmodel.UserRef(userID), RoleKey: roleKey,
			Scope: authzmodel.ScopeRef{Type: authzmodel.ScopeWorkspace, WorkspaceID: w.ID}})
		if err != nil || id == uuid.Nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "workspace.member_added",
			ResourceType: model.WorkspaceResource, ResourceID: w.ID.String(), WorkspaceID: &w.ID,
			Details: map[string]any{"user_id": userID, "role": roleKey}})
	})
	if err == nil {
		s.acl.Invalidate(ctx, p.OrgID)
	}
	return err
}

// RemoveWorkspaceMember removes all of a user's roles in a workspace.
func (s *OrganizationService) RemoveWorkspaceMember(ctx context.Context, slug string, userID uuid.UUID) error {
	p, ok := principal.From(ctx)
	if !ok {
		return authzmodel.Unauthenticated()
	}
	err := s.tx(ctx, p, func(tx *sqlx.Tx) error {
		w, err := s.loadWorkspace(ctx, tx, p, slug, "manage_members")
		if err != nil {
			return err
		}
		if w.ArchivedAt.Valid {
			return model.Conflict("workspace_archived", "Restore the workspace before changing its members.")
		}
		n, err := s.acl.RemoveWorkspaceAccessInTx(ctx, tx, p.OrgID, w.ID, userID)
		if err != nil {
			return err
		}
		if n == 0 {
			return problem.NotFound("This user is not a member of the workspace.")
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "workspace.member_removed",
			ResourceType: model.WorkspaceResource, ResourceID: w.ID.String(), WorkspaceID: &w.ID,
			Details: map[string]any{"user_id": userID}})
	})
	if err == nil {
		s.acl.Invalidate(ctx, p.OrgID)
	}
	return err
}
