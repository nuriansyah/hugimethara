package organization

import (
	"context"

	"github.com/google/uuid"

	"github.com/nuriansyah/hugimethara/internal/platform/httpx"
	"github.com/nuriansyah/hugimethara/internal/transport/http/apigen"
)

func bodyRequired() error { return httpx.BadRequest("A JSON body is required.") }

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Departments ---------------------------------------------------------------

func apiDepartment(d Department) apigen.Department {
	return apigen.Department{Id: d.ID, Slug: d.Slug, Name: d.Name, Description: d.Description,
		WorkspaceCount: d.WorkspaceCount, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt}
}

func (h *HTTP) ListDepartments(ctx context.Context, _ apigen.ListDepartmentsRequestObject) (apigen.ListDepartmentsResponseObject, error) {
	ds, err := h.svc.ListDepartments(ctx)
	if err != nil {
		return nil, err
	}
	out := apigen.ListDepartments200JSONResponse{Data: make([]apigen.Department, len(ds))}
	for i, d := range ds {
		out.Data[i] = apiDepartment(d)
	}
	return out, nil
}

func (h *HTTP) CreateDepartment(ctx context.Context, req apigen.CreateDepartmentRequestObject) (apigen.CreateDepartmentResponseObject, error) {
	if req.Body == nil {
		return nil, bodyRequired()
	}
	d, err := h.svc.CreateDepartment(ctx, DepartmentInput{Slug: req.Body.Slug, Name: req.Body.Name, Description: str(req.Body.Description)})
	if err != nil {
		return nil, err
	}
	return apigen.CreateDepartment201JSONResponse(apiDepartment(d)), nil
}

func (h *HTTP) UpdateDepartment(ctx context.Context, req apigen.UpdateDepartmentRequestObject) (apigen.UpdateDepartmentResponseObject, error) {
	if req.Body == nil {
		return nil, bodyRequired()
	}
	d, err := h.svc.UpdateDepartment(ctx, req.DepartmentId, DepartmentInput{Name: req.Body.Name, Description: str(req.Body.Description)})
	if err != nil {
		return nil, err
	}
	return apigen.UpdateDepartment200JSONResponse(apiDepartment(d)), nil
}

func (h *HTTP) DeleteDepartment(ctx context.Context, req apigen.DeleteDepartmentRequestObject) (apigen.DeleteDepartmentResponseObject, error) {
	if err := h.svc.DeleteDepartment(ctx, req.DepartmentId); err != nil {
		return nil, err
	}
	return apigen.DeleteDepartment204Response{}, nil
}

// Environments ----------------------------------------------------------------

func apiEnvironment(e Environment) apigen.Environment {
	return apigen.Environment{Id: e.ID, Slug: e.Slug, Name: e.Name, IsProduction: e.IsProduction,
		EgressAllowlist: e.EgressAllowlist, CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt}
}

func strs(p *[]string) []string {
	if p == nil {
		return nil
	}
	return *p
}

func (h *HTTP) ListEnvironments(ctx context.Context, _ apigen.ListEnvironmentsRequestObject) (apigen.ListEnvironmentsResponseObject, error) {
	es, err := h.svc.ListEnvironments(ctx)
	if err != nil {
		return nil, err
	}
	out := apigen.ListEnvironments200JSONResponse{Data: make([]apigen.Environment, len(es))}
	for i, e := range es {
		out.Data[i] = apiEnvironment(e)
	}
	return out, nil
}

func (h *HTTP) CreateEnvironment(ctx context.Context, req apigen.CreateEnvironmentRequestObject) (apigen.CreateEnvironmentResponseObject, error) {
	if req.Body == nil {
		return nil, bodyRequired()
	}
	in := EnvironmentInput{Slug: req.Body.Slug, Name: req.Body.Name, EgressAllowlist: strs(req.Body.EgressAllowlist)}
	if req.Body.IsProduction != nil {
		in.IsProduction = *req.Body.IsProduction
	}
	e, err := h.svc.CreateEnvironment(ctx, in)
	if err != nil {
		return nil, err
	}
	return apigen.CreateEnvironment201JSONResponse(apiEnvironment(e)), nil
}

func (h *HTTP) UpdateEnvironment(ctx context.Context, req apigen.UpdateEnvironmentRequestObject) (apigen.UpdateEnvironmentResponseObject, error) {
	if req.Body == nil {
		return nil, bodyRequired()
	}
	e, err := h.svc.UpdateEnvironment(ctx, req.EnvironmentId, EnvironmentInput{
		Name: req.Body.Name, IsProduction: req.Body.IsProduction, EgressAllowlist: strs(req.Body.EgressAllowlist)})
	if err != nil {
		return nil, err
	}
	return apigen.UpdateEnvironment200JSONResponse(apiEnvironment(e)), nil
}

func (h *HTTP) DeleteEnvironment(ctx context.Context, req apigen.DeleteEnvironmentRequestObject) (apigen.DeleteEnvironmentResponseObject, error) {
	if err := h.svc.DeleteEnvironment(ctx, req.EnvironmentId); err != nil {
		return nil, err
	}
	return apigen.DeleteEnvironment204Response{}, nil
}

// Workspaces ------------------------------------------------------------------

func apiWorkspace(w Workspace) apigen.Workspace {
	return apigen.Workspace{Id: w.ID, Slug: w.Slug, Name: w.Name, Description: w.Description,
		DepartmentId: w.DepartmentID, DepartmentName: strPtr(w.DepartmentName), ArchivedAt: w.ArchivedAt,
		MemberCount: w.MemberCount, CreatedAt: w.CreatedAt, UpdatedAt: w.UpdatedAt}
}

func (h *HTTP) ListWorkspaces(ctx context.Context, req apigen.ListWorkspacesRequestObject) (apigen.ListWorkspacesResponseObject, error) {
	include := req.Params.IncludeArchived != nil && *req.Params.IncludeArchived
	ws, err := h.svc.ListWorkspaces(ctx, include)
	if err != nil {
		return nil, err
	}
	out := apigen.ListWorkspaces200JSONResponse{Data: make([]apigen.Workspace, len(ws))}
	for i, w := range ws {
		out.Data[i] = apiWorkspace(w)
	}
	return out, nil
}

func (h *HTTP) CreateWorkspace(ctx context.Context, req apigen.CreateWorkspaceRequestObject) (apigen.CreateWorkspaceResponseObject, error) {
	if req.Body == nil {
		return nil, bodyRequired()
	}
	w, err := h.svc.CreateWorkspace(ctx, WorkspaceInput{Slug: req.Body.Slug, Name: req.Body.Name,
		Description: str(req.Body.Description), DepartmentID: req.Body.DepartmentId})
	if err != nil {
		return nil, err
	}
	return apigen.CreateWorkspace201JSONResponse(apiWorkspace(w)), nil
}

func (h *HTTP) GetWorkspace(ctx context.Context, req apigen.GetWorkspaceRequestObject) (apigen.GetWorkspaceResponseObject, error) {
	w, err := h.svc.GetWorkspace(ctx, req.Workspace)
	if err != nil {
		return nil, err
	}
	return apigen.GetWorkspace200JSONResponse(apiWorkspace(w)), nil
}

// UpdateWorkspace replaces name, description and department (absent = cleared).
func (h *HTTP) UpdateWorkspace(ctx context.Context, req apigen.UpdateWorkspaceRequestObject) (apigen.UpdateWorkspaceResponseObject, error) {
	if req.Body == nil {
		return nil, bodyRequired()
	}
	w, err := h.svc.UpdateWorkspace(ctx, req.Workspace, WorkspaceInput{Name: req.Body.Name,
		Description: str(req.Body.Description), DepartmentID: req.Body.DepartmentId})
	if err != nil {
		return nil, err
	}
	return apigen.UpdateWorkspace200JSONResponse(apiWorkspace(w)), nil
}

func (h *HTTP) ArchiveWorkspace(ctx context.Context, req apigen.ArchiveWorkspaceRequestObject) (apigen.ArchiveWorkspaceResponseObject, error) {
	w, err := h.svc.SetWorkspaceArchived(ctx, req.Workspace, true)
	if err != nil {
		return nil, err
	}
	return apigen.ArchiveWorkspace200JSONResponse(apiWorkspace(w)), nil
}

func (h *HTTP) RestoreWorkspace(ctx context.Context, req apigen.RestoreWorkspaceRequestObject) (apigen.RestoreWorkspaceResponseObject, error) {
	w, err := h.svc.SetWorkspaceArchived(ctx, req.Workspace, false)
	if err != nil {
		return nil, err
	}
	return apigen.RestoreWorkspace200JSONResponse(apiWorkspace(w)), nil
}

func apiMembers(ms []Member) []apigen.Member {
	out := make([]apigen.Member, len(ms))
	for i, m := range ms {
		roles := m.Roles
		am := apigen.Member{Id: m.ID, Email: m.Email, DisplayName: m.DisplayName, Status: m.Status, IsOwner: m.IsOwner, Roles: &roles}
		t := m.AddedAt
		am.AddedAt = &t
		out[i] = am
	}
	return out
}

func (h *HTTP) ListWorkspaceMembers(ctx context.Context, req apigen.ListWorkspaceMembersRequestObject) (apigen.ListWorkspaceMembersResponseObject, error) {
	ms, err := h.svc.ListWorkspaceMembers(ctx, req.Workspace)
	if err != nil {
		return nil, err
	}
	return apigen.ListWorkspaceMembers200JSONResponse{Data: apiMembers(ms)}, nil
}

func (h *HTTP) AddWorkspaceMember(ctx context.Context, req apigen.AddWorkspaceMemberRequestObject) (apigen.AddWorkspaceMemberResponseObject, error) {
	if req.Body == nil || req.Body.UserId == uuid.Nil {
		return nil, httpx.Validation(httpx.FieldError{Field: "user_id", Code: "required", Message: "Choose a user."})
	}
	if err := h.svc.AddWorkspaceMember(ctx, req.Workspace, req.Body.UserId, str(req.Body.RoleKey)); err != nil {
		return nil, err
	}
	return apigen.AddWorkspaceMember204Response{}, nil
}

func (h *HTTP) RemoveWorkspaceMember(ctx context.Context, req apigen.RemoveWorkspaceMemberRequestObject) (apigen.RemoveWorkspaceMemberResponseObject, error) {
	if err := h.svc.RemoveWorkspaceMember(ctx, req.Workspace, req.UserId); err != nil {
		return nil, err
	}
	return apigen.RemoveWorkspaceMember204Response{}, nil
}
