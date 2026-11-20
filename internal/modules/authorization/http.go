package authz

import (
	"context"

	"github.com/google/uuid"

	"github.com/nuriansyah/hugimethara/internal/platform/httpx"
	"github.com/nuriansyah/hugimethara/internal/transport/http/dto"
)

// HTTP implements the access slice of dto.StrictServerInterface.
type HTTP struct{ svc *Service }

func NewHTTP(svc *Service) *HTTP { return &HTTP{svc: svc} }

func apiRole(r Role) dto.Role {
	return dto.Role{Id: r.ID, Key: r.Key, Name: r.Name, Description: r.Description, IsSystem: r.IsSystem,
		Immutable: r.Immutable, Permissions: r.Permissions, BindingCount: r.BindingCount, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

func apiScope(sc ScopeRef) dto.BindingScope {
	out := dto.BindingScope{Type: dto.BindingScopeType(sc.Type)}
	switch sc.Type {
	case ScopeWorkspace:
		id := sc.WorkspaceID
		out.WorkspaceId = &id
	case ScopeResource:
		rt, rid := sc.ResourceType, sc.ResourceID
		out.ResourceType, out.ResourceId = &rt, &rid
	}
	return out
}

func fromAPIScope(in dto.BindingScope) ScopeRef {
	sc := ScopeRef{Type: string(in.Type)}
	if in.WorkspaceId != nil {
		sc.WorkspaceID = *in.WorkspaceId
	}
	if in.ResourceType != nil {
		sc.ResourceType = *in.ResourceType
	}
	if in.ResourceId != nil {
		sc.ResourceID = *in.ResourceId
	}
	return sc
}

func opt(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func apiBinding(b Binding) dto.RoleBinding {
	out := dto.RoleBinding{Id: b.ID, Scope: apiScope(b.Scope), CreatedAt: b.CreatedAt, ExpiresAt: b.ExpiresAt,
		WorkspaceSlug: opt(b.WorkspaceSlug), WorkspaceName: opt(b.WorkspaceName)}
	out.Principal.Type, out.Principal.Id, out.Principal.Name = dto.PrincipalType(b.Principal.Type), b.Principal.ID, b.PrincipalName
	out.Principal.Email = opt(b.PrincipalEmail)
	if b.Principal.Type == PrincipalUser {
		out.User = &struct {
			DisplayName string    `json:"display_name"`
			Email       string    `json:"email"`
			Id          uuid.UUID `json:"id"`
		}{DisplayName: b.PrincipalName, Email: b.PrincipalEmail, Id: b.Principal.ID}
	}
	out.Role.Id, out.Role.Key, out.Role.Name = b.RoleID, b.RoleKey, b.RoleName
	return out
}

// @Summary The caller's effective permissions (for UI gating; the API still authorizes every call)
// @ID getMyPermissions
// @Tags access
// @Success 200 {object} dto.EffectivePermissions "Concrete permissions at organization scope and per workspace."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/me/permissions [get]
func (h *HTTP) GetMyPermissions(ctx context.Context, _ dto.GetMyPermissionsRequestObject) (dto.GetMyPermissionsResponseObject, error) {
	e, err := h.svc.MyPermissions(ctx)
	if err != nil {
		return nil, err
	}
	out := dto.GetMyPermissions200JSONResponse{Organization: e.Organization, Workspaces: map[string][]string{}}
	for id, perms := range e.Workspaces {
		out.Workspaces[id.String()] = perms
	}
	return out, nil
}

// @Summary Permission catalog (resource types and actions)
// @ID listResourceTypes
// @Tags access
// @Success 200 {object} dto.ListResourceTypes200JSONResponse "Catalog."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/resource-types [get]
func (h *HTTP) ListResourceTypes(ctx context.Context, _ dto.ListResourceTypesRequestObject) (dto.ListResourceTypesResponseObject, error) {
	types, err := h.svc.ResourceTypes(ctx)
	if err != nil {
		return nil, err
	}
	out := dto.ListResourceTypes200JSONResponse{Data: make([]dto.ResourceType, len(types))}
	for i, t := range types {
		out.Data[i] = dto.ResourceType{Type: t.Type, Description: t.Description, Actions: t.Actions, SensitiveActions: t.SensitiveActions}
	}
	return out, nil
}

// @Summary Roles of the organization
// @ID listRoles
// @Tags access
// @Success 200 {object} dto.ListRoles200JSONResponse "Roles with permissions."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/roles [get]
func (h *HTTP) ListRoles(ctx context.Context, _ dto.ListRolesRequestObject) (dto.ListRolesResponseObject, error) {
	roles, err := h.svc.ListRoles(ctx)
	if err != nil {
		return nil, err
	}
	out := dto.ListRoles200JSONResponse{Data: make([]dto.Role, len(roles))}
	for i, r := range roles {
		out.Data[i] = apiRole(r)
	}
	return out, nil
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// @Summary Create a custom role
// @ID createRole
// @Tags access
// @Param body body dto.RoleCreate true "Request body"
// @Success 201 {object} dto.Role "Created."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/roles [post]
func (h *HTTP) CreateRole(ctx context.Context, req dto.CreateRoleRequestObject) (dto.CreateRoleResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.BadRequest("A JSON body is required.")
	}
	r, err := h.svc.CreateRole(ctx, RoleInput{Key: req.Body.Key, Name: req.Body.Name, Description: str(req.Body.Description), Permissions: req.Body.Permissions})
	if err != nil {
		return nil, err
	}
	return dto.CreateRole201JSONResponse(apiRole(r)), nil
}

// @Summary A role
// @ID getRole
// @Tags access
// @Param role_id path string true "role_id"
// @Success 200 {object} dto.Role "The role."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/roles/{role_id} [get]
func (h *HTTP) GetRole(ctx context.Context, req dto.GetRoleRequestObject) (dto.GetRoleResponseObject, error) {
	r, err := h.svc.GetRole(ctx, req.RoleId)
	if err != nil {
		return nil, err
	}
	return dto.GetRole200JSONResponse(apiRole(r)), nil
}

// @Summary Replace a role's name, description and permissions
// @ID updateRole
// @Tags access
// @Param role_id path string true "role_id"
// @Param body body dto.RoleUpdate true "Request body"
// @Success 200 {object} dto.Role "Updated."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/roles/{role_id} [put]
func (h *HTTP) UpdateRole(ctx context.Context, req dto.UpdateRoleRequestObject) (dto.UpdateRoleResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.BadRequest("A JSON body is required.")
	}
	r, err := h.svc.UpdateRole(ctx, req.RoleId, RoleInput{Name: req.Body.Name, Description: str(req.Body.Description), Permissions: req.Body.Permissions})
	if err != nil {
		return nil, err
	}
	return dto.UpdateRole200JSONResponse(apiRole(r)), nil
}

// @Summary Delete an unused custom role
// @ID deleteRole
// @Tags access
// @Param role_id path string true "role_id"
// @Success 204 "Deleted."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/roles/{role_id} [delete]
func (h *HTTP) DeleteRole(ctx context.Context, req dto.DeleteRoleRequestObject) (dto.DeleteRoleResponseObject, error) {
	if err := h.svc.DeleteRole(ctx, req.RoleId); err != nil {
		return nil, err
	}
	return dto.DeleteRole204Response{}, nil
}

// @Summary Role assignments
// @ID listRoleBindings
// @Tags access
// @Param user_id query string false "user_id"
// @Param group_id query string false "group_id"
// @Param service_account_id query string false "service_account_id"
// @Param principal_type query string false "principal_type"
// @Param scope_type query string false "scope_type"
// @Param workspace_id query string false "workspace_id"
// @Success 200 {object} dto.ListRoleBindings200JSONResponse "Assignments."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/role-bindings [get]
func (h *HTTP) ListRoleBindings(ctx context.Context, req dto.ListRoleBindingsRequestObject) (dto.ListRoleBindingsResponseObject, error) {
	var f BindingFilter
	if req.Params.PrincipalType != nil {
		f.Principal.Type = string(*req.Params.PrincipalType)
	}
	switch {
	case req.Params.UserId != nil:
		f.Principal = UserRef(*req.Params.UserId)
	case req.Params.GroupId != nil:
		f.Principal = PrincipalRef{Type: PrincipalGroup, ID: *req.Params.GroupId}
	case req.Params.ServiceAccountId != nil:
		f.Principal = PrincipalRef{Type: PrincipalServiceAccount, ID: *req.Params.ServiceAccountId}
	}
	if req.Params.ScopeType != nil {
		f.ScopeType = string(*req.Params.ScopeType)
	}
	if req.Params.WorkspaceId != nil {
		f.WorkspaceID = *req.Params.WorkspaceId
	}
	bs, err := h.svc.ListBindings(ctx, f)
	if err != nil {
		return nil, err
	}
	out := dto.ListRoleBindings200JSONResponse{Data: make([]dto.RoleBinding, len(bs))}
	for i, b := range bs {
		out.Data[i] = apiBinding(b)
	}
	return out, nil
}

// @Summary Assign a role to a member, group or service account at a scope
// @ID createRoleBinding
// @Tags access
// @Param body body dto.RoleBindingCreate true "Request body"
// @Success 201 {object} dto.RoleBinding "Assigned."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/role-bindings [post]
func (h *HTTP) CreateRoleBinding(ctx context.Context, req dto.CreateRoleBindingRequestObject) (dto.CreateRoleBindingResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.BadRequest("A JSON body is required.")
	}
	who, err := principalFrom(req.Body.UserId, req.Body.GroupId, req.Body.ServiceAccountId)
	if err != nil {
		return nil, err
	}
	b, err := h.svc.CreateBinding(ctx, BindingInput{Principal: who, RoleID: req.Body.RoleId,
		Scope: fromAPIScope(req.Body.Scope), ExpiresAt: req.Body.ExpiresAt})
	if err != nil {
		return nil, err
	}
	return dto.CreateRoleBinding201JSONResponse(apiBinding(b)), nil
}

// @Summary Remove a role assignment (the last owner cannot be removed)
// @ID deleteRoleBinding
// @Tags access
// @Param binding_id path string true "binding_id"
// @Success 204 "Removed."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/role-bindings/{binding_id} [delete]
func (h *HTTP) DeleteRoleBinding(ctx context.Context, req dto.DeleteRoleBindingRequestObject) (dto.DeleteRoleBindingResponseObject, error) {
	if err := h.svc.DeleteBinding(ctx, req.BindingId); err != nil {
		return nil, err
	}
	return dto.DeleteRoleBinding204Response{}, nil
}

// @Summary Check whether a member could perform an action (simulation; not logged as a decision)
// @ID evaluateAccess
// @Tags access
// @Param body body dto.EvaluateRequest true "Request body"
// @Success 200 {object} dto.AccessDecision "The decision."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/authorization/evaluate [post]
func (h *HTTP) EvaluateAccess(ctx context.Context, req dto.EvaluateAccessRequestObject) (dto.EvaluateAccessResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.BadRequest("A JSON body is required.")
	}
	res := Resource{Type: req.Body.ResourceType, ID: str(req.Body.ResourceId)}
	if req.Body.WorkspaceId != nil {
		ws := *req.Body.WorkspaceId
		res.WorkspaceID = &ws
	}
	who, err := principalFrom(req.Body.UserId, nil, req.Body.ServiceAccountId)
	if err != nil {
		return nil, err
	}
	d, err := h.svc.Simulate(ctx, who, req.Body.Action, res)
	if err != nil {
		return nil, err
	}
	matched := d.MatchedRules
	if matched == nil {
		matched = []string{}
	}
	return dto.EvaluateAccess200JSONResponse{Allowed: d.Allowed, Reason: d.Reason, MatchedRules: matched, PolicyVersion: d.PolicyVersion}, nil
}

// @Summary Logged authorization decisions (every denial and sensitive allow), newest first
// @ID listAccessDecisions
// @Tags access
// @Param allowed query bool false "allowed"
// @Param limit query int false "limit"
// @Param cursor query string false "cursor"
// @Success 200 {object} dto.ListAccessDecisions200JSONResponse "One page."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/authorization/decisions [get]
func (h *HTTP) ListAccessDecisions(ctx context.Context, req dto.ListAccessDecisionsRequestObject) (dto.ListAccessDecisionsResponseObject, error) {
	page := httpx.Page{Limit: httpx.DefaultPageLimit}
	if req.Params.Limit != nil {
		page.Limit = *req.Params.Limit
	}
	if req.Params.Cursor != nil {
		page.Cursor = *req.Params.Cursor
	}
	ds, next, err := h.svc.ListDecisions(ctx, req.Params.Allowed, page)
	if err != nil {
		return nil, err
	}
	out := dto.ListAccessDecisions200JSONResponse{Data: make([]dto.LoggedDecision, len(ds))}
	if next != "" {
		out.NextCursor = &next
	}
	for i, d := range ds {
		ld := dto.LoggedDecision{Id: d.ID, OccurredAt: d.OccurredAt, PrincipalId: d.PrincipalID, Action: d.Action,
			ResourceType: d.ResourceType, Allowed: d.Allowed, PrincipalType: ptr(dto.PrincipalType(d.PrincipalType)), Reason: d.Reason, MatchedRules: d.MatchedRules,
			PrincipalEmail: opt(d.PrincipalEmail.String), ResourceId: opt(d.ResourceID.String), RequestId: opt(d.RequestID.String)}
		if ld.MatchedRules == nil {
			ld.MatchedRules = []string{}
		}
		if d.WorkspaceID.Valid {
			ws := d.WorkspaceID.UUID
			ld.WorkspaceId = &ws
		}
		out.Data[i] = ld
	}
	return out, nil
}

func ptr[T any](v T) *T { return &v }

// principalFrom accepts exactly one principal id.
func principalFrom(user, group, sa *uuid.UUID) (PrincipalRef, error) {
	var out []PrincipalRef
	if user != nil {
		out = append(out, UserRef(*user))
	}
	if group != nil {
		out = append(out, PrincipalRef{Type: PrincipalGroup, ID: *group})
	}
	if sa != nil {
		out = append(out, PrincipalRef{Type: PrincipalServiceAccount, ID: *sa})
	}
	if len(out) != 1 {
		return PrincipalRef{}, httpx.Validation(httpx.FieldError{Field: "principal", Code: "invalid",
			Message: "Give exactly one of user_id, group_id or service_account_id."})
	}
	return out[0], nil
}
