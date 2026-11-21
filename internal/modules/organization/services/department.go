package services

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

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
)

func (s *OrganizationService) ListDepartments(ctx context.Context) ([]model.Department, error) {
	p, err := s.authorize(ctx, "read", model.DepartmentResource, "", nil)
	if err != nil {
		return nil, err
	}
	var rows []repository.DepartmentRow
	if err := s.tx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, queries.SQL.Get("ListDepartments"))
	}); err != nil {
		return nil, err
	}
	out := make([]model.Department, len(rows))
	for i, r := range rows {
		out[i] = r.ToModel()
	}
	return out, nil
}

func (s *OrganizationService) CreateDepartment(ctx context.Context, in dto.DepartmentInput) (model.Department, error) {
	p, err := s.authorize(ctx, "create", model.DepartmentResource, "", nil)
	if err != nil {
		return model.Department{}, err
	}
	in.Slug = strings.ToLower(strings.TrimSpace(in.Slug))
	in.Name, in.Description = strings.TrimSpace(in.Name), strings.TrimSpace(in.Description)
	if err := model.Collect(model.ValidateShortSlug("slug", in.Slug), model.ValidateText("name", in.Name, 1, 100, "Name"),
		model.ValidateText("description", in.Description, 0, 500, "Description")); err != nil {
		return model.Department{}, err
	}
	var out repository.DepartmentRow
	err = s.tx(ctx, p, func(tx *sqlx.Tx) error {
		ent, err := entsvc.Load(ctx, tx, p.OrgID)
		if err != nil {
			return err
		}
		if err := ent.Require(entmodel.Departments); err != nil {
			return err
		}
		id := uuid.Must(uuid.NewV7())
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertDepartment"), id, p.OrgID, in.Slug, in.Name, in.Description); err != nil {
			if db.IsUniqueViolation(err) {
				return model.Conflict("slug_taken", "A department with this address already exists.")
			}
			return err
		}
		if err := tx.GetContext(ctx, &out, queries.SQL.Get("GetDepartment"), id); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "department.created",
			ResourceType: model.DepartmentResource, ResourceID: id.String(), Details: map[string]any{"slug": in.Slug, "name": in.Name}})
	})
	return out.ToModel(), err
}

func (s *OrganizationService) UpdateDepartment(ctx context.Context, id uuid.UUID, in dto.DepartmentInput) (model.Department, error) {
	p, err := s.authorize(ctx, "update", model.DepartmentResource, id.String(), nil)
	if err != nil {
		return model.Department{}, err
	}
	in.Name, in.Description = strings.TrimSpace(in.Name), strings.TrimSpace(in.Description)
	if err := model.Collect(model.ValidateText("name", in.Name, 1, 100, "Name"),
		model.ValidateText("description", in.Description, 0, 500, "Description")); err != nil {
		return model.Department{}, err
	}
	var out repository.DepartmentRow
	err = s.tx(ctx, p, func(tx *sqlx.Tx) error {
		var before repository.DepartmentRow
		if err := tx.GetContext(ctx, &before, queries.SQL.Get("GetDepartment"), id); err != nil {
			if db.IsNoRows(err) {
				return model.ErrDepartmentNotFound
			}
			return err
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("UpdateDepartment"), id, in.Name, in.Description); err != nil {
			return err
		}
		if err := tx.GetContext(ctx, &out, queries.SQL.Get("GetDepartment"), id); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "department.updated",
			ResourceType: model.DepartmentResource, ResourceID: id.String(),
			Details: map[string]any{"from": map[string]any{"name": before.Name, "description": before.Description},
				"to": map[string]any{"name": in.Name, "description": in.Description}}})
	})
	return out.ToModel(), err
}

// DeleteDepartment removes a department; its workspaces become unassigned.
func (s *OrganizationService) DeleteDepartment(ctx context.Context, id uuid.UUID) error {
	p, err := s.authorize(ctx, "delete", model.DepartmentResource, id.String(), nil)
	if err != nil {
		return err
	}
	return s.tx(ctx, p, func(tx *sqlx.Tx) error {
		var before repository.DepartmentRow
		if err := tx.GetContext(ctx, &before, queries.SQL.Get("GetDepartment"), id); err != nil {
			if db.IsNoRows(err) {
				return model.ErrDepartmentNotFound
			}
			return err
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("DeleteDepartment"), id); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "department.deleted",
			ResourceType: model.DepartmentResource, ResourceID: id.String(),
			Details: map[string]any{"slug": before.Slug, "name": before.Name, "unassigned_workspaces": before.WorkspaceCount}})
	})
}
