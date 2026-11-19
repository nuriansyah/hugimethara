package organization

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/backend/internal/audit"
	"github.com/nuriansyah/hugimethara/backend/internal/authz"
	"github.com/nuriansyah/hugimethara/backend/internal/entitlement"
	"github.com/nuriansyah/hugimethara/backend/internal/event"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/db"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/httpx"
	"github.com/nuriansyah/hugimethara/backend/internal/principal"
)

// Workspace is a team space inside an organization.
type Workspace struct {
	ID             uuid.UUID
	Slug           string
	Name           string
	Description    string
	DepartmentID   *uuid.UUID
	DepartmentName string
	ArchivedAt     *time.Time
	MemberCount    int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// WorkspaceInput creates or updates a workspace (Slug is create-only).
type WorkspaceInput struct {
	Slug         string
	Name         string
	Description  string
	DepartmentID *uuid.UUID
}

// Member is an organization member as seen from a workspace or the org.
type Member struct {
	ID          uuid.UUID
	Email       string
	DisplayName string
	Status      string
	IsOwner     bool
	Roles       []string  // org-level roles (org list) or workspace roles (workspace list)
	JoinedAt    time.Time // org memberships
	AddedAt     time.Time // workspace memberships
}

func (r workspaceRow) toModel() Workspace {
	w := Workspace{ID: r.ID, Slug: r.Slug, Name: r.Name, Description: r.Description,
		DepartmentName: r.DepartmentName.String, MemberCount: r.MemberCount, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
	if r.DepartmentID.Valid {
		id := r.DepartmentID.UUID
		w.DepartmentID = &id
	}
	if r.ArchivedAt.Valid {
		t := r.ArchivedAt.Time
		w.ArchivedAt = &t
	}
	return w
}

var errWorkspaceNotFound = httpx.NotFound("Workspace not found.")

// ListWorkspaces returns the workspaces the caller may see: all of them for
// principals with org-wide scope, otherwise only those they belong to.
func (s *Service) ListWorkspaces(ctx context.Context, includeArchived bool) ([]Workspace, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return nil, authz.Unauthenticated()
	}
	scope, err := s.scopes.WorkspaceScope(ctx, p, "read")
	if err != nil {
		return nil, err
	}
	if scope.None() {
		return []Workspace{}, nil
	}
	var ids any
	if !scope.All {
		ids = scope.WorkspaceIDs
	}
	var rows []workspaceRow
	if err := s.tx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, q.Get("ListWorkspaces"), includeArchived, ids)
	}); err != nil {
		return nil, err
	}
	out := make([]Workspace, len(rows))
	for i, r := range rows {
		out[i] = r.toModel()
	}
	return out, nil
}

// loadWorkspace fetches by slug and authorizes action on it. Unknown and
// forbidden-to-read workspaces both yield 404 (no existence oracle).
func (s *Service) loadWorkspace(ctx context.Context, tx *sqlx.Tx, p principal.Principal, slug, action string) (workspaceRow, error) {
	var w workspaceRow
	if err := tx.GetContext(ctx, &w, q.Get("GetWorkspaceBySlug"), strings.ToLower(slug)); err != nil {
		if db.IsNoRows(err) {
			return w, errWorkspaceNotFound
		}
		return w, err
	}
	res := authz.Resource{Type: WorkspaceResource, ID: w.ID.String(), OrgID: p.OrgID, WorkspaceID: &w.ID}
	if action != "read" {
		// Callers who cannot even read the workspace learn nothing about it.
		if d, err := s.az.Check(ctx, authz.AuthorizationInput{Principal: p, Action: "read", Resource: res}); err != nil {
			return w, err
		} else if !d.Allowed {
			return w, errWorkspaceNotFound
		}
	}
	if _, err := authz.Require(ctx, s.az, action, res); err != nil {
		var pr *httpx.Problem
		if errors.As(err, &pr) && pr.Status == 403 && action == "read" {
			return w, errWorkspaceNotFound
		}
		return w, err
	}
	return w, nil
}

// GetWorkspace returns a workspace by slug.
func (s *Service) GetWorkspace(ctx context.Context, slug string) (Workspace, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return Workspace{}, authz.Unauthenticated()
	}
	var w workspaceRow
	err := s.tx(ctx, p, func(tx *sqlx.Tx) error {
		var err error
		w, err = s.loadWorkspace(ctx, tx, p, slug, "read")
		return err
	})
	return w.toModel(), err
}

func (s *Service) validateWorkspaceInput(in *WorkspaceInput, create bool) error {
	in.Name, in.Description = strings.TrimSpace(in.Name), strings.TrimSpace(in.Description)
	var slugErr *httpx.FieldError
	if create {
		in.Slug = strings.ToLower(strings.TrimSpace(in.Slug))
		slugErr = validateShortSlug("slug", in.Slug)
	}
	return collect(slugErr, validateText("name", in.Name, 1, 100, "Name"),
		validateText("description", in.Description, 0, 500, "Description"))
}

func nullable(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return *id
}

// CreateWorkspace creates a workspace and makes the creator its first member.
func (s *Service) CreateWorkspace(ctx context.Context, in WorkspaceInput) (Workspace, error) {
	p, err := s.authorize(ctx, "create", WorkspaceResource, "", nil)
	if err != nil {
		return Workspace{}, err
	}
	if err := s.validateWorkspaceInput(&in, true); err != nil {
		return Workspace{}, err
	}
	var out workspaceRow
	err = s.tx(ctx, p, func(tx *sqlx.Tx) error {
		if err := entitlement.LockFeature(ctx, tx, p.OrgID, entitlement.WorkspacesMax); err != nil {
			return err
		}
		ent, err := entitlement.Load(ctx, tx, p.OrgID)
		if err != nil {
			return err
		}
		if in.DepartmentID != nil {
			if err := ent.Require(entitlement.Departments); err != nil {
				return err
			}
		}
		var active int64
		if err := tx.GetContext(ctx, &active, q.Get("CountActiveWorkspaces")); err != nil {
			return err
		}
		if err := ent.CheckLimit(entitlement.WorkspacesMax, active, 1); err != nil {
			return err
		}
		id := uuid.Must(uuid.NewV7())
		if _, err := tx.ExecContext(ctx, q.Get("InsertWorkspace"), id, p.OrgID, in.Slug, in.Name, in.Description,
			nullable(in.DepartmentID), p.ID); err != nil {
			switch {
			case db.IsUniqueViolation(err):
				return conflict("slug_taken", "A workspace with this address already exists.")
			case db.IsForeignKeyViolation(err):
				return httpx.Validation(httpx.FieldError{Field: "department_id", Code: "not_found", Message: "Department not found."})
			}
			return err
		}
		// The creator administers what they create.
		if err := authz.GrantCreatorInTx(ctx, tx, p.OrgID, p.ID, id); err != nil {
			return err
		}
		if err := tx.GetContext(ctx, &out, q.Get("GetWorkspaceBySlug"), in.Slug); err != nil {
			return err
		}
		if err := audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "workspace.created",
			ResourceType: WorkspaceResource, ResourceID: id.String(), WorkspaceID: &id,
			Details: map[string]any{"slug": in.Slug, "name": in.Name}}); err != nil {
			return err
		}
		_, err = event.Publish(ctx, tx, event.Event{OrgID: p.OrgID, Subject: "hugi.events.workspace.created",
			Payload: map[string]any{"workspace_id": id, "slug": in.Slug}})
		return err
	})
	if err == nil {
		s.acl.Invalidate(ctx, p.OrgID)
	}
	return out.toModel(), err
}

// UpdateWorkspace changes name, description and department.
func (s *Service) UpdateWorkspace(ctx context.Context, slug string, in WorkspaceInput) (Workspace, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return Workspace{}, authz.Unauthenticated()
	}
	if err := s.validateWorkspaceInput(&in, false); err != nil {
		return Workspace{}, err
	}
	var out workspaceRow
	err := s.tx(ctx, p, func(tx *sqlx.Tx) error {
		before, err := s.loadWorkspace(ctx, tx, p, slug, "update")
		if err != nil {
			return err
		}
		prevDept := before.DepartmentID
		changingDept := (in.DepartmentID == nil) != !prevDept.Valid || (in.DepartmentID != nil && *in.DepartmentID != prevDept.UUID)
		if in.DepartmentID != nil && changingDept {
			ent, err := entitlement.Load(ctx, tx, p.OrgID)
			if err != nil {
				return err
			}
			if err := ent.Require(entitlement.Departments); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, q.Get("UpdateWorkspace"), before.ID, in.Name, in.Description, nullable(in.DepartmentID)); err != nil {
			if db.IsForeignKeyViolation(err) {
				return httpx.Validation(httpx.FieldError{Field: "department_id", Code: "not_found", Message: "Department not found."})
			}
			return err
		}
		if err := tx.GetContext(ctx, &out, q.Get("GetWorkspaceBySlug"), before.Slug); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "workspace.updated",
			ResourceType: WorkspaceResource, ResourceID: before.ID.String(), WorkspaceID: &before.ID,
			Details: map[string]any{
				"from": map[string]any{"name": before.Name, "description": before.Description, "department": before.DepartmentName.String},
				"to":   map[string]any{"name": out.Name, "description": out.Description, "department": out.DepartmentName.String},
			}})
	})
	return out.toModel(), err
}

// SetWorkspaceArchived archives (true) or restores (false) a workspace.
// Restoring counts against the workspace quota again.
func (s *Service) SetWorkspaceArchived(ctx context.Context, slug string, archived bool) (Workspace, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return Workspace{}, authz.Unauthenticated()
	}
	action, verb := "archive", "workspace.archived"
	if !archived {
		verb = "workspace.restored"
	}
	var out workspaceRow
	err := s.tx(ctx, p, func(tx *sqlx.Tx) error {
		w, err := s.loadWorkspace(ctx, tx, p, slug, action)
		if err != nil {
			return err
		}
		if !archived {
			if err := entitlement.LockFeature(ctx, tx, p.OrgID, entitlement.WorkspacesMax); err != nil {
				return err
			}
			ent, err := entitlement.Load(ctx, tx, p.OrgID)
			if err != nil {
				return err
			}
			var active int64
			if err := tx.GetContext(ctx, &active, q.Get("CountActiveWorkspaces")); err != nil {
				return err
			}
			if err := ent.CheckLimit(entitlement.WorkspacesMax, active, 1); err != nil {
				return err
			}
		}
		n, err := rowsAffected(tx.ExecContext(ctx, q.Get("SetWorkspaceArchived"), w.ID, archived))
		if err != nil {
			return err
		}
		if n == 0 {
			state := "active"
			if archived {
				state = "archived"
			}
			return conflict("already_"+state, "The workspace is already "+state+".")
		}
		if err := tx.GetContext(ctx, &out, q.Get("GetWorkspaceBySlug"), w.Slug); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: verb,
			ResourceType: WorkspaceResource, ResourceID: w.ID.String(), WorkspaceID: &w.ID})
	})
	return out.toModel(), err
}

// ListWorkspaceMembers lists members of a workspace the caller can read.
func (s *Service) ListWorkspaceMembers(ctx context.Context, slug string) ([]Member, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return nil, authz.Unauthenticated()
	}
	var rows []memberRow
	err := s.tx(ctx, p, func(tx *sqlx.Tx) error {
		w, err := s.loadWorkspace(ctx, tx, p, slug, "read")
		if err != nil {
			return err
		}
		return tx.SelectContext(ctx, &rows, q.Get("ListWorkspaceMembers"), w.ID)
	})
	return membersFrom(rows), err
}

// DefaultWorkspaceRole is assigned when adding a member without a role.
const DefaultWorkspaceRole = "editor"

// AddWorkspaceMember gives an organization member a role in a workspace
// (idempotent). Requires workspace:manage_members, and the caller cannot grant
// a role with permissions they do not hold in that workspace.
func (s *Service) AddWorkspaceMember(ctx context.Context, slug string, userID uuid.UUID, roleKey string) error {
	p, ok := principal.From(ctx)
	if !ok {
		return authz.Unauthenticated()
	}
	if roleKey == "" {
		roleKey = DefaultWorkspaceRole
	}
	err := s.tx(ctx, p, func(tx *sqlx.Tx) error {
		w, err := s.loadWorkspace(ctx, tx, p, slug, "manage_members")
		if err != nil {
			return err
		}
		if w.ArchivedAt.Valid {
			return conflict("workspace_archived", "Restore the workspace before changing its members.")
		}
		id, err := s.acl.BindInTx(ctx, tx, p, authz.BindingInput{UserID: userID, RoleKey: roleKey,
			Scope: authz.ScopeRef{Type: authz.ScopeWorkspace, WorkspaceID: w.ID}})
		if err != nil || id == uuid.Nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "workspace.member_added",
			ResourceType: WorkspaceResource, ResourceID: w.ID.String(), WorkspaceID: &w.ID,
			Details: map[string]any{"user_id": userID, "role": roleKey}})
	})
	if err == nil {
		s.acl.Invalidate(ctx, p.OrgID)
	}
	return err
}

// RemoveWorkspaceMember removes all of a user's roles in a workspace.
func (s *Service) RemoveWorkspaceMember(ctx context.Context, slug string, userID uuid.UUID) error {
	p, ok := principal.From(ctx)
	if !ok {
		return authz.Unauthenticated()
	}
	err := s.tx(ctx, p, func(tx *sqlx.Tx) error {
		w, err := s.loadWorkspace(ctx, tx, p, slug, "manage_members")
		if err != nil {
			return err
		}
		if w.ArchivedAt.Valid {
			return conflict("workspace_archived", "Restore the workspace before changing its members.")
		}
		n, err := s.acl.RemoveWorkspaceAccessInTx(ctx, tx, p.OrgID, w.ID, userID)
		if err != nil {
			return err
		}
		if n == 0 {
			return httpx.NotFound("This user is not a member of the workspace.")
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "workspace.member_removed",
			ResourceType: WorkspaceResource, ResourceID: w.ID.String(), WorkspaceID: &w.ID,
			Details: map[string]any{"user_id": userID}})
	})
	if err == nil {
		s.acl.Invalidate(ctx, p.OrgID)
	}
	return err
}

// ListOrgMembers lists the organization's members (for assigning to workspaces).
func (s *Service) ListOrgMembers(ctx context.Context) ([]Member, error) {
	p, err := s.authorize(ctx, "read", UserResource, "", nil)
	if err != nil {
		return nil, err
	}
	var rows []memberRow
	err = s.tx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, q.Get("ListOrgMembers"))
	})
	return membersFrom(rows), err
}

func membersFrom(rows []memberRow) []Member {
	out := make([]Member, len(rows))
	for i, r := range rows {
		m := Member{ID: r.ID, Email: r.Email, DisplayName: r.DisplayName, Status: r.Status,
			IsOwner: r.IsOwner, JoinedAt: r.JoinedAt, AddedAt: r.AddedAt, Roles: []string{}}
		_ = json.Unmarshal(r.Roles, &m.Roles)
		out[i] = m
	}
	return out
}
