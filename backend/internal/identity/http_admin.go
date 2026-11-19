package identity

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/nuriansyah/hugimethara/backend/internal/api/apigen"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/httpx"
)

func bodyRequired() error { return httpx.BadRequest("A JSON body is required.") }

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// Members ---------------------------------------------------------------------

func apiMember(m OrgMember, withGroups bool) apigen.Member {
	roles := m.Roles
	joined := m.JoinedAt
	ms := apigen.MemberMembershipStatus(m.MembershipStatus)
	out := apigen.Member{Id: m.User.ID, Email: m.User.Email, DisplayName: m.User.DisplayName, Status: m.User.Status,
		IsOwner: m.IsOwner, Roles: &roles, JoinedAt: &joined, MembershipStatus: &ms}
	if withGroups {
		gs := make([]apigen.GroupRef, len(m.Groups))
		for i, g := range m.Groups {
			gs[i] = apigen.GroupRef{Id: g.ID, Name: g.Name}
		}
		out.Groups = &gs
	}
	return out
}

func (h *HTTP) ListUsers(ctx context.Context, _ apigen.ListUsersRequestObject) (apigen.ListUsersResponseObject, error) {
	ms, err := h.svc.ListMembers(ctx)
	if err != nil {
		return nil, err
	}
	out := apigen.ListUsers200JSONResponse{Data: make([]apigen.Member, len(ms))}
	for i, m := range ms {
		out.Data[i] = apiMember(m, false)
	}
	return out, nil
}

func (h *HTTP) GetUser(ctx context.Context, req apigen.GetUserRequestObject) (apigen.GetUserResponseObject, error) {
	m, err := h.svc.GetMember(ctx, req.UserId)
	if err != nil {
		return nil, err
	}
	return apigen.GetUser200JSONResponse(apiMember(m, true)), nil
}

func (h *HTTP) UpdateUser(ctx context.Context, req apigen.UpdateUserRequestObject) (apigen.UpdateUserResponseObject, error) {
	if req.Body == nil {
		return nil, bodyRequired()
	}
	m, err := h.svc.SetMemberStatus(ctx, req.UserId, string(req.Body.Status))
	if err != nil {
		return nil, err
	}
	return apigen.UpdateUser200JSONResponse(apiMember(m, false)), nil
}

func (h *HTTP) RemoveUser(ctx context.Context, req apigen.RemoveUserRequestObject) (apigen.RemoveUserResponseObject, error) {
	if err := h.svc.RemoveMember(ctx, req.UserId); err != nil {
		return nil, err
	}
	return apigen.RemoveUser204Response{}, nil
}

// Invitations -------------------------------------------------------------------

func apiInvitation(inv Invitation) apigen.Invitation {
	out := apigen.Invitation{Id: inv.ID, Email: inv.Email, Status: apigen.InvitationStatus(inv.Status),
		CreatedAt: inv.CreatedAt, ExpiresAt: inv.ExpiresAt, AcceptedAt: inv.AcceptedAt,
		Workspaces: make([]apigen.InvitationWorkspace, len(inv.Workspaces))}
	if inv.InvitedByName != "" {
		n := inv.InvitedByName
		out.InvitedByName = &n
	}
	if inv.OrgRole != nil {
		out.OrgRole = &struct {
			Id   uuid.UUID `json:"id"`
			Key  string    `json:"key"`
			Name string    `json:"name"`
		}{Id: inv.OrgRole.ID, Key: inv.OrgRole.Key, Name: inv.OrgRole.Name}
	}
	for i, w := range inv.Workspaces {
		slug := w.WorkspaceSlug
		out.Workspaces[i] = apigen.InvitationWorkspace{WorkspaceId: w.WorkspaceID, WorkspaceName: w.WorkspaceName,
			WorkspaceSlug: &slug, RoleId: w.RoleID, RoleName: w.RoleName}
	}
	return out
}

func (h *HTTP) ListInvitations(ctx context.Context, req apigen.ListInvitationsRequestObject) (apigen.ListInvitationsResponseObject, error) {
	all := req.Params.Status != nil && *req.Params.Status == apigen.ListInvitationsParamsStatusAll
	invs, err := h.svc.ListInvitations(ctx, all)
	if err != nil {
		return nil, err
	}
	out := apigen.ListInvitations200JSONResponse{Data: make([]apigen.Invitation, len(invs))}
	for i, inv := range invs {
		out.Data[i] = apiInvitation(inv)
	}
	return out, nil
}

func (h *HTTP) CreateInvitation(ctx context.Context, req apigen.CreateInvitationRequestObject) (apigen.CreateInvitationResponseObject, error) {
	if req.Body == nil {
		return nil, bodyRequired()
	}
	in := InvitationInput{Email: req.Body.Email, OrgRoleID: req.Body.OrgRoleId}
	if req.Body.ExpiresInDays != nil {
		in.ExpiresInDays = *req.Body.ExpiresInDays
	}
	if req.Body.Workspaces != nil {
		for _, w := range *req.Body.Workspaces {
			in.Workspaces = append(in.Workspaces, WorkspaceGrant{WorkspaceID: w.WorkspaceId, RoleID: w.RoleId})
		}
	}
	c, err := h.svc.CreateInvitation(ctx, in)
	if err != nil {
		return nil, err
	}
	return apigen.CreateInvitation201JSONResponse{Invitation: apiInvitation(c.Invitation), Token: c.Token, AcceptPath: AcceptPath(c.Token)}, nil
}

func (h *HTTP) RevokeInvitation(ctx context.Context, req apigen.RevokeInvitationRequestObject) (apigen.RevokeInvitationResponseObject, error) {
	if err := h.svc.RevokeInvitation(ctx, req.InvitationId); err != nil {
		return nil, err
	}
	return apigen.RevokeInvitation204Response{}, nil
}

func (h *HTTP) GetInvitationPreview(ctx context.Context, req apigen.GetInvitationPreviewRequestObject) (apigen.GetInvitationPreviewResponseObject, error) {
	pv, err := h.svc.PreviewInvitation(ctx, req.Token)
	if err != nil {
		return nil, err
	}
	out := apigen.GetInvitationPreview200JSONResponse{Email: pv.Email, ExpiresAt: pv.ExpiresAt, AccountExists: pv.AccountExists,
		Organization: apigen.OrganizationSummary{Id: pv.Organization.ID, Slug: pv.Organization.Slug, Name: pv.Organization.Name}}
	if pv.InvitedByName != "" {
		n := pv.InvitedByName
		out.InvitedByName = &n
	}
	return out, nil
}

func (r sessionResponse) VisitAcceptInvitationResponse(w http.ResponseWriter) error {
	return r.visit(w)
}

func (h *HTTP) AcceptInvitation(ctx context.Context, req apigen.AcceptInvitationRequestObject) (apigen.AcceptInvitationResponseObject, error) {
	var in AcceptInput
	if req.Body != nil {
		in = AcceptInput{DisplayName: str(req.Body.DisplayName), Password: str(req.Body.Password)}
	}
	g, err := h.svc.AcceptInvitation(ctx, req.Token, in)
	if err != nil {
		return nil, err
	}
	return sessionResponse{status: http.StatusOK, body: authResponse(g), cookies: h.cookies.issue(g)}, nil
}

// Groups ------------------------------------------------------------------------

func apiGroup(g Group) apigen.Group {
	return apigen.Group{Id: g.ID, Name: g.Name, Description: g.Description, Source: apigen.GroupSource(g.Source),
		MemberCount: g.MemberCount, CreatedAt: g.CreatedAt, UpdatedAt: g.UpdatedAt}
}

func (h *HTTP) ListGroups(ctx context.Context, _ apigen.ListGroupsRequestObject) (apigen.ListGroupsResponseObject, error) {
	gs, err := h.svc.ListGroups(ctx)
	if err != nil {
		return nil, err
	}
	out := apigen.ListGroups200JSONResponse{Data: make([]apigen.Group, len(gs))}
	for i, g := range gs {
		out.Data[i] = apiGroup(g)
	}
	return out, nil
}

func (h *HTTP) CreateGroup(ctx context.Context, req apigen.CreateGroupRequestObject) (apigen.CreateGroupResponseObject, error) {
	if req.Body == nil {
		return nil, bodyRequired()
	}
	g, err := h.svc.CreateGroup(ctx, GroupInput{Name: req.Body.Name, Description: str(req.Body.Description)})
	if err != nil {
		return nil, err
	}
	return apigen.CreateGroup201JSONResponse(apiGroup(g)), nil
}

func (h *HTTP) GetGroup(ctx context.Context, req apigen.GetGroupRequestObject) (apigen.GetGroupResponseObject, error) {
	g, err := h.svc.GetGroup(ctx, req.GroupId)
	if err != nil {
		return nil, err
	}
	return apigen.GetGroup200JSONResponse(apiGroup(g)), nil
}

func (h *HTTP) UpdateGroup(ctx context.Context, req apigen.UpdateGroupRequestObject) (apigen.UpdateGroupResponseObject, error) {
	if req.Body == nil {
		return nil, bodyRequired()
	}
	g, err := h.svc.UpdateGroup(ctx, req.GroupId, GroupInput{Name: req.Body.Name, Description: str(req.Body.Description)})
	if err != nil {
		return nil, err
	}
	return apigen.UpdateGroup200JSONResponse(apiGroup(g)), nil
}

func (h *HTTP) DeleteGroup(ctx context.Context, req apigen.DeleteGroupRequestObject) (apigen.DeleteGroupResponseObject, error) {
	if err := h.svc.DeleteGroup(ctx, req.GroupId); err != nil {
		return nil, err
	}
	return apigen.DeleteGroup204Response{}, nil
}

func (h *HTTP) ListGroupMembers(ctx context.Context, req apigen.ListGroupMembersRequestObject) (apigen.ListGroupMembersResponseObject, error) {
	ms, err := h.svc.ListGroupMembers(ctx, req.GroupId)
	if err != nil {
		return nil, err
	}
	out := apigen.ListGroupMembers200JSONResponse{Data: make([]apigen.GroupMember, len(ms))}
	for i, m := range ms {
		out.Data[i] = apigen.GroupMember{Id: m.ID, Email: m.Email, DisplayName: m.DisplayName, AddedAt: m.AddedAt}
	}
	return out, nil
}

func (h *HTTP) AddGroupMember(ctx context.Context, req apigen.AddGroupMemberRequestObject) (apigen.AddGroupMemberResponseObject, error) {
	if req.Body == nil || req.Body.UserId == uuid.Nil {
		return nil, httpx.Validation(httpx.FieldError{Field: "user_id", Code: "required", Message: "Choose a member."})
	}
	if err := h.svc.AddGroupMember(ctx, req.GroupId, req.Body.UserId); err != nil {
		return nil, err
	}
	return apigen.AddGroupMember204Response{}, nil
}

func (h *HTTP) RemoveGroupMember(ctx context.Context, req apigen.RemoveGroupMemberRequestObject) (apigen.RemoveGroupMemberResponseObject, error) {
	if err := h.svc.RemoveGroupMember(ctx, req.GroupId, req.UserId); err != nil {
		return nil, err
	}
	return apigen.RemoveGroupMember204Response{}, nil
}

// Service accounts -------------------------------------------------------------------

func apiServiceAccount(sa ServiceAccount) apigen.ServiceAccount {
	return apigen.ServiceAccount{Id: sa.ID, Name: sa.Name, Description: sa.Description, Disabled: sa.DisabledAt.Valid,
		ActiveKeyCount: sa.ActiveKeyCount, CreatedAt: sa.CreatedAt, UpdatedAt: sa.UpdatedAt}
}

func apiKey(k APIKey) apigen.ApiKey {
	out := apigen.ApiKey{Id: k.ID, Name: k.Name, Prefix: k.Prefix, CreatedAt: k.CreatedAt}
	if k.ExpiresAt.Valid {
		out.ExpiresAt = &k.ExpiresAt.Time
	}
	if k.LastUsedAt.Valid {
		out.LastUsedAt = &k.LastUsedAt.Time
	}
	if k.RevokedAt.Valid {
		out.RevokedAt = &k.RevokedAt.Time
	}
	return out
}

func (h *HTTP) ListServiceAccounts(ctx context.Context, _ apigen.ListServiceAccountsRequestObject) (apigen.ListServiceAccountsResponseObject, error) {
	sas, err := h.svc.ListServiceAccounts(ctx)
	if err != nil {
		return nil, err
	}
	out := apigen.ListServiceAccounts200JSONResponse{Data: make([]apigen.ServiceAccount, len(sas))}
	for i, sa := range sas {
		out.Data[i] = apiServiceAccount(sa)
	}
	return out, nil
}

func (h *HTTP) CreateServiceAccount(ctx context.Context, req apigen.CreateServiceAccountRequestObject) (apigen.CreateServiceAccountResponseObject, error) {
	if req.Body == nil {
		return nil, bodyRequired()
	}
	sa, err := h.svc.CreateServiceAccount(ctx, ServiceAccountInput{Name: req.Body.Name, Description: str(req.Body.Description)})
	if err != nil {
		return nil, err
	}
	return apigen.CreateServiceAccount201JSONResponse(apiServiceAccount(sa)), nil
}

func (h *HTTP) GetServiceAccount(ctx context.Context, req apigen.GetServiceAccountRequestObject) (apigen.GetServiceAccountResponseObject, error) {
	sa, err := h.svc.GetServiceAccount(ctx, req.ServiceAccountId)
	if err != nil {
		return nil, err
	}
	return apigen.GetServiceAccount200JSONResponse(apiServiceAccount(sa)), nil
}

func (h *HTTP) UpdateServiceAccount(ctx context.Context, req apigen.UpdateServiceAccountRequestObject) (apigen.UpdateServiceAccountResponseObject, error) {
	if req.Body == nil {
		return nil, bodyRequired()
	}
	sa, err := h.svc.UpdateServiceAccount(ctx, req.ServiceAccountId, ServiceAccountInput{Name: req.Body.Name,
		Description: str(req.Body.Description), Disabled: req.Body.Disabled})
	if err != nil {
		return nil, err
	}
	return apigen.UpdateServiceAccount200JSONResponse(apiServiceAccount(sa)), nil
}

func (h *HTTP) DeleteServiceAccount(ctx context.Context, req apigen.DeleteServiceAccountRequestObject) (apigen.DeleteServiceAccountResponseObject, error) {
	if err := h.svc.DeleteServiceAccount(ctx, req.ServiceAccountId); err != nil {
		return nil, err
	}
	return apigen.DeleteServiceAccount204Response{}, nil
}

func (h *HTTP) ListApiKeys(ctx context.Context, req apigen.ListApiKeysRequestObject) (apigen.ListApiKeysResponseObject, error) {
	ks, err := h.svc.ListAPIKeys(ctx, req.ServiceAccountId)
	if err != nil {
		return nil, err
	}
	out := apigen.ListApiKeys200JSONResponse{Data: make([]apigen.ApiKey, len(ks))}
	for i, k := range ks {
		out.Data[i] = apiKey(k)
	}
	return out, nil
}

func (h *HTTP) CreateApiKey(ctx context.Context, req apigen.CreateApiKeyRequestObject) (apigen.CreateApiKeyResponseObject, error) {
	if req.Body == nil {
		return nil, bodyRequired()
	}
	k, secret, err := h.svc.CreateAPIKey(ctx, req.ServiceAccountId, req.Body.Name, req.Body.ExpiresAt)
	if err != nil {
		return nil, err
	}
	return apigen.CreateApiKey201JSONResponse{Key: apiKey(k), Secret: secret}, nil
}

func (h *HTTP) RevokeApiKey(ctx context.Context, req apigen.RevokeApiKeyRequestObject) (apigen.RevokeApiKeyResponseObject, error) {
	if err := h.svc.RevokeAPIKey(ctx, req.ServiceAccountId, req.KeyId); err != nil {
		return nil, err
	}
	return apigen.RevokeApiKey204Response{}, nil
}
