package organization

import (
	"context"

	"github.com/google/uuid"

	"github.com/nuriansyah/hugimethara/internal/platform/httpx"
	"github.com/nuriansyah/hugimethara/internal/transport/http/dto"
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

func apiDepartment(d Department) dto.Department {
	return dto.Department{Id: d.ID, Slug: d.Slug, Name: d.Name, Description: d.Description,
		WorkspaceCount: d.WorkspaceCount, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt}
}

// @Summary List departments
// @ID listDepartments
// @Tags workspace
// @Success 200 {object} dto.ListDepartments200JSONResponse "All departments."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/departments [get]
func (h *HTTP) ListDepartments(ctx context.Context, _ dto.ListDepartmentsRequestObject) (dto.ListDepartmentsResponseObject, error) {
	ds, err := h.svc.ListDepartments(ctx)
	if err != nil {
		return nil, err
	}
	out := dto.ListDepartments200JSONResponse{Data: make([]dto.Department, len(ds))}
	for i, d := range ds {
		out.Data[i] = apiDepartment(d)
	}
	return out, nil
}

// @Summary Create a department
// @ID createDepartment
// @Tags workspace
// @Param body body dto.DepartmentCreate true "Request body"
// @Success 201 {object} dto.Department "Created."
// @Failure 402 {object} dto.PlanProblem "The organization's plan does not allow this (feature_not_in_plan or plan_limit_reached)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/departments [post]
func (h *HTTP) CreateDepartment(ctx context.Context, req dto.CreateDepartmentRequestObject) (dto.CreateDepartmentResponseObject, error) {
	if req.Body == nil {
		return nil, bodyRequired()
	}
	d, err := h.svc.CreateDepartment(ctx, DepartmentInput{Slug: req.Body.Slug, Name: req.Body.Name, Description: str(req.Body.Description)})
	if err != nil {
		return nil, err
	}
	return dto.CreateDepartment201JSONResponse(apiDepartment(d)), nil
}

// @Summary Update a department
// @ID updateDepartment
// @Tags workspace
// @Param department_id path string true "department_id"
// @Param body body dto.DepartmentUpdate true "Request body"
// @Success 200 {object} dto.Department "Updated."
// @Failure 402 {object} dto.PlanProblem "The organization's plan does not allow this (feature_not_in_plan or plan_limit_reached)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/departments/{department_id} [patch]
func (h *HTTP) UpdateDepartment(ctx context.Context, req dto.UpdateDepartmentRequestObject) (dto.UpdateDepartmentResponseObject, error) {
	if req.Body == nil {
		return nil, bodyRequired()
	}
	d, err := h.svc.UpdateDepartment(ctx, req.DepartmentId, DepartmentInput{Name: req.Body.Name, Description: str(req.Body.Description)})
	if err != nil {
		return nil, err
	}
	return dto.UpdateDepartment200JSONResponse(apiDepartment(d)), nil
}

// @Summary Delete a department
// @ID deleteDepartment
// @Tags workspace
// @Param department_id path string true "department_id"
// @Success 204 "Deleted."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/departments/{department_id} [delete]
func (h *HTTP) DeleteDepartment(ctx context.Context, req dto.DeleteDepartmentRequestObject) (dto.DeleteDepartmentResponseObject, error) {
	if err := h.svc.DeleteDepartment(ctx, req.DepartmentId); err != nil {
		return nil, err
	}
	return dto.DeleteDepartment204Response{}, nil
}

// Environments ----------------------------------------------------------------

func apiEnvironment(e Environment) dto.Environment {
	return dto.Environment{Id: e.ID, Slug: e.Slug, Name: e.Name, IsProduction: e.IsProduction,
		EgressAllowlist: e.EgressAllowlist, CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt}
}

func strs(p *[]string) []string {
	if p == nil {
		return nil
	}
	return *p
}

// @Summary List environments
// @ID listEnvironments
// @Tags workspace
// @Success 200 {object} dto.ListEnvironments200JSONResponse "All environments."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/environments [get]
func (h *HTTP) ListEnvironments(ctx context.Context, _ dto.ListEnvironmentsRequestObject) (dto.ListEnvironmentsResponseObject, error) {
	es, err := h.svc.ListEnvironments(ctx)
	if err != nil {
		return nil, err
	}
	out := dto.ListEnvironments200JSONResponse{Data: make([]dto.Environment, len(es))}
	for i, e := range es {
		out.Data[i] = apiEnvironment(e)
	}
	return out, nil
}

// @Summary Create a environment
// @ID createEnvironment
// @Tags workspace
// @Param body body dto.EnvironmentCreate true "Request body"
// @Success 201 {object} dto.Environment "Created."
// @Failure 402 {object} dto.PlanProblem "The organization's plan does not allow this (feature_not_in_plan or plan_limit_reached)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/environments [post]
func (h *HTTP) CreateEnvironment(ctx context.Context, req dto.CreateEnvironmentRequestObject) (dto.CreateEnvironmentResponseObject, error) {
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
	return dto.CreateEnvironment201JSONResponse(apiEnvironment(e)), nil
}

// @Summary Update a environment
// @ID updateEnvironment
// @Tags workspace
// @Param environment_id path string true "environment_id"
// @Param body body dto.EnvironmentUpdate true "Request body"
// @Success 200 {object} dto.Environment "Updated."
// @Failure 402 {object} dto.PlanProblem "The organization's plan does not allow this (feature_not_in_plan or plan_limit_reached)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/environments/{environment_id} [patch]
func (h *HTTP) UpdateEnvironment(ctx context.Context, req dto.UpdateEnvironmentRequestObject) (dto.UpdateEnvironmentResponseObject, error) {
	if req.Body == nil {
		return nil, bodyRequired()
	}
	e, err := h.svc.UpdateEnvironment(ctx, req.EnvironmentId, EnvironmentInput{
		Name: req.Body.Name, IsProduction: req.Body.IsProduction, EgressAllowlist: strs(req.Body.EgressAllowlist)})
	if err != nil {
		return nil, err
	}
	return dto.UpdateEnvironment200JSONResponse(apiEnvironment(e)), nil
}

// @Summary Delete a environment
// @ID deleteEnvironment
// @Tags workspace
// @Param environment_id path string true "environment_id"
// @Success 204 "Deleted."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/environments/{environment_id} [delete]
func (h *HTTP) DeleteEnvironment(ctx context.Context, req dto.DeleteEnvironmentRequestObject) (dto.DeleteEnvironmentResponseObject, error) {
	if err := h.svc.DeleteEnvironment(ctx, req.EnvironmentId); err != nil {
		return nil, err
	}
	return dto.DeleteEnvironment204Response{}, nil
}

// Workspaces ------------------------------------------------------------------

func apiWorkspace(w Workspace) dto.Workspace {
	return dto.Workspace{Id: w.ID, Slug: w.Slug, Name: w.Name, Description: w.Description,
		DepartmentId: w.DepartmentID, DepartmentName: strPtr(w.DepartmentName), ArchivedAt: w.ArchivedAt,
		MemberCount: w.MemberCount, CreatedAt: w.CreatedAt, UpdatedAt: w.UpdatedAt}
}

// @Summary Workspaces visible to the caller
// @ID listWorkspaces
// @Tags workspace
// @Param include_archived query bool false "include_archived"
// @Success 200 {object} dto.ListWorkspaces200JSONResponse "Workspaces."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/workspaces [get]
func (h *HTTP) ListWorkspaces(ctx context.Context, req dto.ListWorkspacesRequestObject) (dto.ListWorkspacesResponseObject, error) {
	include := req.Params.IncludeArchived != nil && *req.Params.IncludeArchived
	ws, err := h.svc.ListWorkspaces(ctx, include)
	if err != nil {
		return nil, err
	}
	out := dto.ListWorkspaces200JSONResponse{Data: make([]dto.Workspace, len(ws))}
	for i, w := range ws {
		out.Data[i] = apiWorkspace(w)
	}
	return out, nil
}

// @Summary Create a workspace (the creator becomes its first member)
// @ID createWorkspace
// @Tags workspace
// @Param body body dto.WorkspaceCreate true "Request body"
// @Success 201 {object} dto.Workspace "Created."
// @Failure 402 {object} dto.PlanProblem "The organization's plan does not allow this (feature_not_in_plan or plan_limit_reached)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/workspaces [post]
func (h *HTTP) CreateWorkspace(ctx context.Context, req dto.CreateWorkspaceRequestObject) (dto.CreateWorkspaceResponseObject, error) {
	if req.Body == nil {
		return nil, bodyRequired()
	}
	w, err := h.svc.CreateWorkspace(ctx, WorkspaceInput{Slug: req.Body.Slug, Name: req.Body.Name,
		Description: str(req.Body.Description), DepartmentID: req.Body.DepartmentId})
	if err != nil {
		return nil, err
	}
	return dto.CreateWorkspace201JSONResponse(apiWorkspace(w)), nil
}

// @Summary A workspace by its address
// @ID getWorkspace
// @Tags workspace
// @Param workspace path string true "Workspace address (slug)."
// @Success 200 {object} dto.Workspace "The workspace."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/workspaces/{workspace} [get]
func (h *HTTP) GetWorkspace(ctx context.Context, req dto.GetWorkspaceRequestObject) (dto.GetWorkspaceResponseObject, error) {
	w, err := h.svc.GetWorkspace(ctx, req.Workspace)
	if err != nil {
		return nil, err
	}
	return dto.GetWorkspace200JSONResponse(apiWorkspace(w)), nil
}

// UpdateWorkspace replaces name, description and department (absent = cleared).

// @Summary Update a workspace
// @ID updateWorkspace
// @Tags workspace
// @Param workspace path string true "Workspace address (slug)."
// @Param body body dto.WorkspaceUpdate true "Request body"
// @Success 200 {object} dto.Workspace "Updated."
// @Failure 402 {object} dto.PlanProblem "The organization's plan does not allow this (feature_not_in_plan or plan_limit_reached)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/workspaces/{workspace} [patch]
func (h *HTTP) UpdateWorkspace(ctx context.Context, req dto.UpdateWorkspaceRequestObject) (dto.UpdateWorkspaceResponseObject, error) {
	if req.Body == nil {
		return nil, bodyRequired()
	}
	w, err := h.svc.UpdateWorkspace(ctx, req.Workspace, WorkspaceInput{Name: req.Body.Name,
		Description: str(req.Body.Description), DepartmentID: req.Body.DepartmentId})
	if err != nil {
		return nil, err
	}
	return dto.UpdateWorkspace200JSONResponse(apiWorkspace(w)), nil
}

// @Summary Archive a workspace (frees a slot in the workspace quota)
// @ID archiveWorkspace
// @Tags workspace
// @Param workspace path string true "Workspace address (slug)."
// @Success 200 {object} dto.Workspace "Archived."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/workspaces/{workspace}/archive [post]
func (h *HTTP) ArchiveWorkspace(ctx context.Context, req dto.ArchiveWorkspaceRequestObject) (dto.ArchiveWorkspaceResponseObject, error) {
	w, err := h.svc.SetWorkspaceArchived(ctx, req.Workspace, true)
	if err != nil {
		return nil, err
	}
	return dto.ArchiveWorkspace200JSONResponse(apiWorkspace(w)), nil
}

// @Summary Restore an archived workspace
// @ID restoreWorkspace
// @Tags workspace
// @Param workspace path string true "Workspace address (slug)."
// @Success 200 {object} dto.Workspace "Restored."
// @Failure 402 {object} dto.PlanProblem "The organization's plan does not allow this (feature_not_in_plan or plan_limit_reached)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/workspaces/{workspace}/restore [post]
func (h *HTTP) RestoreWorkspace(ctx context.Context, req dto.RestoreWorkspaceRequestObject) (dto.RestoreWorkspaceResponseObject, error) {
	w, err := h.svc.SetWorkspaceArchived(ctx, req.Workspace, false)
	if err != nil {
		return nil, err
	}
	return dto.RestoreWorkspace200JSONResponse(apiWorkspace(w)), nil
}

func apiMembers(ms []Member) []dto.Member {
	out := make([]dto.Member, len(ms))
	for i, m := range ms {
		roles := m.Roles
		am := dto.Member{Id: m.ID, Email: m.Email, DisplayName: m.DisplayName, Status: m.Status, IsOwner: m.IsOwner, Roles: &roles}
		t := m.AddedAt
		am.AddedAt = &t
		out[i] = am
	}
	return out
}

// @Summary Members of a workspace
// @ID listWorkspaceMembers
// @Tags workspace
// @Param workspace path string true "Workspace address (slug)."
// @Success 200 {object} dto.ListWorkspaceMembers200JSONResponse "Members."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/workspaces/{workspace}/members [get]
func (h *HTTP) ListWorkspaceMembers(ctx context.Context, req dto.ListWorkspaceMembersRequestObject) (dto.ListWorkspaceMembersResponseObject, error) {
	ms, err := h.svc.ListWorkspaceMembers(ctx, req.Workspace)
	if err != nil {
		return nil, err
	}
	return dto.ListWorkspaceMembers200JSONResponse{Data: apiMembers(ms)}, nil
}

// @Summary Add an organization member to the workspace (idempotent)
// @ID addWorkspaceMember
// @Tags workspace
// @Param workspace path string true "Workspace address (slug)."
// @Param body body map[string]interface{} true "Request body"
// @Success 204 "The user is a member."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/workspaces/{workspace}/members [post]
func (h *HTTP) AddWorkspaceMember(ctx context.Context, req dto.AddWorkspaceMemberRequestObject) (dto.AddWorkspaceMemberResponseObject, error) {
	if req.Body == nil || req.Body.UserId == uuid.Nil {
		return nil, httpx.Validation(httpx.FieldError{Field: "user_id", Code: "required", Message: "Choose a user."})
	}
	if err := h.svc.AddWorkspaceMember(ctx, req.Workspace, req.Body.UserId, str(req.Body.RoleKey)); err != nil {
		return nil, err
	}
	return dto.AddWorkspaceMember204Response{}, nil
}

// @Summary Remove a user from the workspace
// @ID removeWorkspaceMember
// @Tags workspace
// @Param workspace path string true "Workspace address (slug)."
// @Param user_id path string true "user_id"
// @Success 204 "Removed."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/workspaces/{workspace}/members/{user_id} [delete]
func (h *HTTP) RemoveWorkspaceMember(ctx context.Context, req dto.RemoveWorkspaceMemberRequestObject) (dto.RemoveWorkspaceMemberResponseObject, error) {
	if err := h.svc.RemoveWorkspaceMember(ctx, req.Workspace, req.UserId); err != nil {
		return nil, err
	}
	return dto.RemoveWorkspaceMember204Response{}, nil
}
