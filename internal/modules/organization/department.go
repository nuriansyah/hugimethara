package organization

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/modules/governance/audit"
	"github.com/nuriansyah/hugimethara/internal/modules/governance/entitlement"
	"github.com/nuriansyah/hugimethara/internal/platform/database"
	"github.com/nuriansyah/hugimethara/internal/platform/httpx"
)

// Department groups workspaces (e.g. Marketing, Finance).
type Department struct {
	ID             uuid.UUID
	Slug           string
	Name           string
	Description    string
	WorkspaceCount int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// DepartmentInput creates or updates a department (Slug is create-only).
type DepartmentInput struct {
	Slug        string
	Name        string
	Description string
}

func (r departmentRow) toModel() Department {
	return Department{ID: r.ID, Slug: r.Slug, Name: r.Name, Description: r.Description,
		WorkspaceCount: r.WorkspaceCount, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

var errDepartmentNotFound = httpx.NotFound("Department not found.")

func (s *Service) ListDepartments(ctx context.Context) ([]Department, error) {
	p, err := s.authorize(ctx, "read", DepartmentResource, "", nil)
	if err != nil {
		return nil, err
	}
	var rows []departmentRow
	if err := s.tx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, q.Get("ListDepartments"))
	}); err != nil {
		return nil, err
	}
	out := make([]Department, len(rows))
	for i, r := range rows {
		out[i] = r.toModel()
	}
	return out, nil
}

func (s *Service) CreateDepartment(ctx context.Context, in DepartmentInput) (Department, error) {
	p, err := s.authorize(ctx, "create", DepartmentResource, "", nil)
	if err != nil {
		return Department{}, err
	}
	in.Slug = strings.ToLower(strings.TrimSpace(in.Slug))
	in.Name, in.Description = strings.TrimSpace(in.Name), strings.TrimSpace(in.Description)
	if err := collect(validateShortSlug("slug", in.Slug), validateText("name", in.Name, 1, 100, "Name"),
		validateText("description", in.Description, 0, 500, "Description")); err != nil {
		return Department{}, err
	}
	var out departmentRow
	err = s.tx(ctx, p, func(tx *sqlx.Tx) error {
		ent, err := entitlement.Load(ctx, tx, p.OrgID)
		if err != nil {
			return err
		}
		if err := ent.Require(entitlement.Departments); err != nil {
			return err
		}
		id := uuid.Must(uuid.NewV7())
		if _, err := tx.ExecContext(ctx, q.Get("InsertDepartment"), id, p.OrgID, in.Slug, in.Name, in.Description); err != nil {
			if db.IsUniqueViolation(err) {
				return conflict("slug_taken", "A department with this address already exists.")
			}
			return err
		}
		if err := tx.GetContext(ctx, &out, q.Get("GetDepartment"), id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "department.created",
			ResourceType: DepartmentResource, ResourceID: id.String(), Details: map[string]any{"slug": in.Slug, "name": in.Name}})
	})
	return out.toModel(), err
}

func (s *Service) UpdateDepartment(ctx context.Context, id uuid.UUID, in DepartmentInput) (Department, error) {
	p, err := s.authorize(ctx, "update", DepartmentResource, id.String(), nil)
	if err != nil {
		return Department{}, err
	}
	in.Name, in.Description = strings.TrimSpace(in.Name), strings.TrimSpace(in.Description)
	if err := collect(validateText("name", in.Name, 1, 100, "Name"),
		validateText("description", in.Description, 0, 500, "Description")); err != nil {
		return Department{}, err
	}
	var out departmentRow
	err = s.tx(ctx, p, func(tx *sqlx.Tx) error {
		var before departmentRow
		if err := tx.GetContext(ctx, &before, q.Get("GetDepartment"), id); err != nil {
			if db.IsNoRows(err) {
				return errDepartmentNotFound
			}
			return err
		}
		if _, err := tx.ExecContext(ctx, q.Get("UpdateDepartment"), id, in.Name, in.Description); err != nil {
			return err
		}
		if err := tx.GetContext(ctx, &out, q.Get("GetDepartment"), id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "department.updated",
			ResourceType: DepartmentResource, ResourceID: id.String(),
			Details: map[string]any{"from": map[string]any{"name": before.Name, "description": before.Description},
				"to": map[string]any{"name": in.Name, "description": in.Description}}})
	})
	return out.toModel(), err
}

// DeleteDepartment removes a department; its workspaces become unassigned.
func (s *Service) DeleteDepartment(ctx context.Context, id uuid.UUID) error {
	p, err := s.authorize(ctx, "delete", DepartmentResource, id.String(), nil)
	if err != nil {
		return err
	}
	return s.tx(ctx, p, func(tx *sqlx.Tx) error {
		var before departmentRow
		if err := tx.GetContext(ctx, &before, q.Get("GetDepartment"), id); err != nil {
			if db.IsNoRows(err) {
				return errDepartmentNotFound
			}
			return err
		}
		if _, err := tx.ExecContext(ctx, q.Get("DeleteDepartment"), id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "department.deleted",
			ResourceType: DepartmentResource, ResourceID: id.String(),
			Details: map[string]any{"slug": before.Slug, "name": before.Name, "unassigned_workspaces": before.WorkspaceCount}})
	})
}
