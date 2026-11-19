package authz

import (
	"context"

	"github.com/google/uuid"

	"github.com/nuriansyah/hugimethara/backend/internal/api/apigen"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/httpx"
)

// HTTP implements the access slice of apigen.StrictServerInterface.
type HTTP struct{ svc *Service }

func NewHTTP(svc *Service) *HTTP { return &HTTP{svc: svc} }

func apiRole(r Role) apigen.Role {
	return apigen.Role{Id: r.ID, Key: r.Key, Name: r.Name, Description: r.Description, IsSystem: r.IsSystem,
		Immutable: r.Immutable, Permissions: r.Permissions, BindingCount: r.BindingCount, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

func apiScope(sc ScopeRef) apigen.BindingScope {
	out := apigen.BindingScope{Type: apigen.BindingScopeType(sc.Type)}
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

func fromAPIScope(in apigen.BindingScope) ScopeRef {
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

func apiBinding(b Binding) apigen.RoleBinding {
	out := apigen.RoleBinding{Id: b.ID, Scope: apiScope(b.Scope), CreatedAt: b.CreatedAt, ExpiresAt: b.ExpiresAt,
		WorkspaceSlug: opt(b.WorkspaceSlug), WorkspaceName: opt(b.WorkspaceName)}
	out.Principal.Type, out.Principal.Id, out.Principal.Name = apigen.PrincipalType(b.Principal.Type), b.Principal.ID, b.PrincipalName
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

func (h *HTTP) GetMyPermissions(ctx context.Context, _ apigen.GetMyPermissionsRequestObject) (apigen.GetMyPermissionsResponseObject, error) {
	e, err := h.svc.MyPermissions(ctx)
	if err != nil {
		return nil, err
	}
	out := apigen.GetMyPermissions200JSONResponse{Organization: e.Organization, Workspaces: map[string][]string{}}
	for id, perms := range e.Workspaces {
		out.Workspaces[id.String()] = perms
	}
	return out, nil
}

func (h *HTTP) ListResourceTypes(ctx context.Context, _ apigen.ListResourceTypesRequestObject) (apigen.ListResourceTypesResponseObject, error) {
	types, err := h.svc.ResourceTypes(ctx)
	if err != nil {
		return nil, err
	}
	out := apigen.ListResourceTypes200JSONResponse{Data: make([]apigen.ResourceType, len(types))}
	for i, t := range types {
		out.Data[i] = apigen.ResourceType{Type: t.Type, Description: t.Description, Actions: t.Actions, SensitiveActions: t.SensitiveActions}
	}
	return out, nil
}

func (h *HTTP) ListRoles(ctx context.Context, _ apigen.ListRolesRequestObject) (apigen.ListRolesResponseObject, error) {
	roles, err := h.svc.ListRoles(ctx)
	if err != nil {
		return nil, err
	}
	out := apigen.ListRoles200JSONResponse{Data: make([]apigen.Role, len(roles))}
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

func (h *HTTP) CreateRole(ctx context.Context, req apigen.CreateRoleRequestObject) (apigen.CreateRoleResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.BadRequest("A JSON body is required.")
	}
	r, err := h.svc.CreateRole(ctx, RoleInput{Key: req.Body.Key, Name: req.Body.Name, Description: str(req.Body.Description), Permissions: req.Body.Permissions})
	if err != nil {
		return nil, err
	}
	return apigen.CreateRole201JSONResponse(apiRole(r)), nil
}

func (h *HTTP) GetRole(ctx context.Context, req apigen.GetRoleRequestObject) (apigen.GetRoleResponseObject, error) {
	r, err := h.svc.GetRole(ctx, req.RoleId)
	if err != nil {
		return nil, err
	}
	return apigen.GetRole200JSONResponse(apiRole(r)), nil
}

func (h *HTTP) UpdateRole(ctx context.Context, req apigen.UpdateRoleRequestObject) (apigen.UpdateRoleResponseObject, error) {
	if req.Body == nil {
		return nil, httpx.BadRequest("A JSON body is required.")
	}
	r, err := h.svc.UpdateRole(ctx, req.RoleId, RoleInput{Name: req.Body.Name, Description: str(req.Body.Description), Permissions: req.Body.Permissions})
	if err != nil {
		return nil, err
	}
	return apigen.UpdateRole200JSONResponse(apiRole(r)), nil
}

func (h *HTTP) DeleteRole(ctx context.Context, req apigen.DeleteRoleRequestObject) (apigen.DeleteRoleResponseObject, error) {
	if err := h.svc.DeleteRole(ctx, req.RoleId); err != nil {
		return nil, err
	}
	return apigen.DeleteRole204Response{}, nil
}

func (h *HTTP) ListRoleBindings(ctx context.Context, req apigen.ListRoleBindingsRequestObject) (apigen.ListRoleBindingsResponseObject, error) {
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
	out := apigen.ListRoleBindings200JSONResponse{Data: make([]apigen.RoleBinding, len(bs))}
	for i, b := range bs {
		out.Data[i] = apiBinding(b)
	}
	return out, nil
}

func (h *HTTP) CreateRoleBinding(ctx context.Context, req apigen.CreateRoleBindingRequestObject) (apigen.CreateRoleBindingResponseObject, error) {
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
	return apigen.CreateRoleBinding201JSONResponse(apiBinding(b)), nil
}

func (h *HTTP) DeleteRoleBinding(ctx context.Context, req apigen.DeleteRoleBindingRequestObject) (apigen.DeleteRoleBindingResponseObject, error) {
	if err := h.svc.DeleteBinding(ctx, req.BindingId); err != nil {
		return nil, err
	}
	return apigen.DeleteRoleBinding204Response{}, nil
}

func (h *HTTP) EvaluateAccess(ctx context.Context, req apigen.EvaluateAccessRequestObject) (apigen.EvaluateAccessResponseObject, error) {
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
	return apigen.EvaluateAccess200JSONResponse{Allowed: d.Allowed, Reason: d.Reason, MatchedRules: matched, PolicyVersion: d.PolicyVersion}, nil
}

func (h *HTTP) ListAccessDecisions(ctx context.Context, req apigen.ListAccessDecisionsRequestObject) (apigen.ListAccessDecisionsResponseObject, error) {
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
	out := apigen.ListAccessDecisions200JSONResponse{Data: make([]apigen.LoggedDecision, len(ds))}
	if next != "" {
		out.NextCursor = &next
	}
	for i, d := range ds {
		ld := apigen.LoggedDecision{Id: d.ID, OccurredAt: d.OccurredAt, PrincipalId: d.PrincipalID, Action: d.Action,
			ResourceType: d.ResourceType, Allowed: d.Allowed, PrincipalType: ptr(apigen.PrincipalType(d.PrincipalType)), Reason: d.Reason, MatchedRules: d.MatchedRules,
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
