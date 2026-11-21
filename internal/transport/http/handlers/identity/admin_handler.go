package identity

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/nuriansyah/hugimethara/internal/modules/identity/model"
	identitydto "github.com/nuriansyah/hugimethara/internal/modules/identity/model/dto"
	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/transport/http/dto"
)

func bodyRequired() error { return problem.BadRequest("A JSON body is required.") }

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func apiMember(m model.OrgMember, withGroups bool) dto.Member {
	roles := m.Roles
	joined := m.JoinedAt
	ms := dto.MemberMembershipStatus(m.MembershipStatus)
	out := dto.Member{Id: m.User.ID, Email: m.User.Email, DisplayName: m.User.DisplayName, Status: m.User.Status,
		IsOwner: m.IsOwner, Roles: &roles, JoinedAt: &joined, MembershipStatus: &ms}
	if withGroups {
		gs := make([]dto.GroupRef, len(m.Groups))
		for i, g := range m.Groups {
			gs[i] = dto.GroupRef{Id: g.ID, Name: g.Name}
		}
		out.Groups = &gs
	}
	return out
}

// @Summary Members of the organization
// @ID listUsers
// @Tags identity
// @Success 200 {object} dto.ListUsers200JSONResponse "Organization members."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/users [get]
func (h *HTTP) ListUsers(ctx context.Context, _ dto.ListUsersRequestObject) (dto.ListUsersResponseObject, error) {
	ms, err := h.svc.ListMembers(ctx)
	if err != nil {
		return nil, err
	}
	out := dto.ListUsers200JSONResponse{Data: make([]dto.Member, len(ms))}
	for i, m := range ms {
		out.Data[i] = apiMember(m, false)
	}
	return out, nil
}

// @Summary One member, with their groups
// @ID getUser
// @Tags identity
// @Param user_id path string true "user_id"
// @Success 200 {object} dto.Member "The member."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/users/{user_id} [get]
func (h *HTTP) GetUser(ctx context.Context, req dto.GetUserRequestObject) (dto.GetUserResponseObject, error) {
	m, err := h.svc.GetMember(ctx, req.UserId)
	if err != nil {
		return nil, err
	}
	return dto.GetUser200JSONResponse(apiMember(m, true)), nil
}

// @Summary Suspend or reactivate a member
// @ID updateUser
// @Tags identity
// @Param user_id path string true "user_id"
// @Param body body dto.UserUpdate true "Request body"
// @Success 200 {object} dto.Member "Updated."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/users/{user_id} [patch]
func (h *HTTP) UpdateUser(ctx context.Context, req dto.UpdateUserRequestObject) (dto.UpdateUserResponseObject, error) {
	if req.Body == nil {
		return nil, bodyRequired()
	}
	m, err := h.svc.SetMemberStatus(ctx, req.UserId, string(req.Body.Status))
	if err != nil {
		return nil, err
	}
	return dto.UpdateUser200JSONResponse(apiMember(m, false)), nil
}

// @Summary Remove a member from the organization
// @ID removeUser
// @Tags identity
// @Param user_id path string true "user_id"
// @Success 204 "Removed."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/users/{user_id} [delete]
func (h *HTTP) RemoveUser(ctx context.Context, req dto.RemoveUserRequestObject) (dto.RemoveUserResponseObject, error) {
	if err := h.svc.RemoveMember(ctx, req.UserId); err != nil {
		return nil, err
	}
	return dto.RemoveUser204Response{}, nil
}

func apiInvitation(inv model.Invitation) dto.Invitation {
	out := dto.Invitation{Id: inv.ID, Email: inv.Email, Status: dto.InvitationStatus(inv.Status),
		CreatedAt: inv.CreatedAt, ExpiresAt: inv.ExpiresAt, AcceptedAt: inv.AcceptedAt,
		Workspaces: make([]dto.InvitationWorkspace, len(inv.Workspaces))}
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
		out.Workspaces[i] = dto.InvitationWorkspace{WorkspaceId: w.WorkspaceID, WorkspaceName: w.WorkspaceName,
			WorkspaceSlug: &slug, RoleId: w.RoleID, RoleName: w.RoleName}
	}
	return out
}

// @Summary Invitations, newest first
// @ID listInvitations
// @Tags identity
// @Param status query string false "status"
// @Success 200 {object} dto.ListInvitations200JSONResponse "Invitations."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/invitations [get]
func (h *HTTP) ListInvitations(ctx context.Context, req dto.ListInvitationsRequestObject) (dto.ListInvitationsResponseObject, error) {
	all := req.Params.Status != nil && *req.Params.Status == dto.ListInvitationsParamsStatusAll
	invs, err := h.svc.ListInvitations(ctx, all)
	if err != nil {
		return nil, err
	}
	out := dto.ListInvitations200JSONResponse{Data: make([]dto.Invitation, len(invs))}
	for i, inv := range invs {
		out.Data[i] = apiInvitation(inv)
	}
	return out, nil
}

// @Summary Invite someone by email
// @ID createInvitation
// @Tags identity
// @Param body body dto.InvitationCreate true "Request body"
// @Success 201 {object} dto.InvitationCreated "Invited."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/invitations [post]
func (h *HTTP) CreateInvitation(ctx context.Context, req dto.CreateInvitationRequestObject) (dto.CreateInvitationResponseObject, error) {
	if req.Body == nil {
		return nil, bodyRequired()
	}
	in := identitydto.InvitationInput{Email: req.Body.Email, OrgRoleID: req.Body.OrgRoleId}
	if req.Body.ExpiresInDays != nil {
		in.ExpiresInDays = *req.Body.ExpiresInDays
	}
	if req.Body.Workspaces != nil {
		for _, w := range *req.Body.Workspaces {
			in.Workspaces = append(in.Workspaces, model.WorkspaceGrant{WorkspaceID: w.WorkspaceId, RoleID: w.RoleId})
		}
	}
	c, err := h.svc.CreateInvitation(ctx, in)
	if err != nil {
		return nil, err
	}
	return dto.CreateInvitation201JSONResponse{Invitation: apiInvitation(c.Invitation), Token: c.Token, AcceptPath: model.AcceptPath(c.Token)}, nil
}

// @Summary Revoke an open invitation
// @ID revokeInvitation
// @Tags identity
// @Param invitation_id path string true "invitation_id"
// @Success 204 "Revoked."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/invitations/{invitation_id} [delete]
func (h *HTTP) RevokeInvitation(ctx context.Context, req dto.RevokeInvitationRequestObject) (dto.RevokeInvitationResponseObject, error) {
	if err := h.svc.RevokeInvitation(ctx, req.InvitationId); err != nil {
		return nil, err
	}
	return dto.RevokeInvitation204Response{}, nil
}

// @Summary What an invitation link is for (public)
// @ID getInvitationPreview
// @Tags identity
// @Param token path string true "token"
// @Success 200 {object} dto.InvitationPreview "The invitation."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 410 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Router /v1/hugimethara/invitation-tokens/{token} [get]
func (h *HTTP) GetInvitationPreview(ctx context.Context, req dto.GetInvitationPreviewRequestObject) (dto.GetInvitationPreviewResponseObject, error) {
	pv, err := h.svc.PreviewInvitation(ctx, req.Token)
	if err != nil {
		return nil, err
	}
	out := dto.GetInvitationPreview200JSONResponse{Email: pv.Email, ExpiresAt: pv.ExpiresAt, AccountExists: pv.AccountExists,
		Organization: dto.OrganizationSummary{Id: pv.Organization.ID, Slug: pv.Organization.Slug, Name: pv.Organization.Name}}
	if pv.InvitedByName != "" {
		n := pv.InvitedByName
		out.InvitedByName = &n
	}
	return out, nil
}

func (r sessionResponse) VisitAcceptInvitationResponse(w http.ResponseWriter) error {
	return r.visit(w)
}

// @Summary Accept an invitation and sign in to the organization (public, rate-limited)
// @ID acceptInvitation
// @Tags identity
// @Param token path string true "token"
// @Param body body dto.InvitationAccept false "Request body"
// @Success 200 {object} dto.AuthResponse "Joined and signed in."
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 410 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 429 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Router /v1/hugimethara/invitation-tokens/{token}/accept [post]
func (h *HTTP) AcceptInvitation(ctx context.Context, req dto.AcceptInvitationRequestObject) (dto.AcceptInvitationResponseObject, error) {
	var in identitydto.AcceptInput
	if req.Body != nil {
		in = identitydto.AcceptInput{DisplayName: str(req.Body.DisplayName), Password: str(req.Body.Password)}
	}
	g, err := h.svc.AcceptInvitation(ctx, req.Token, in)
	if err != nil {
		return nil, err
	}
	return sessionResponse{status: http.StatusOK, body: authResponse(g), cookies: h.cookies.issue(g)}, nil
}

func apiGroup(g model.Group) dto.Group {
	return dto.Group{Id: g.ID, Name: g.Name, Description: g.Description, Source: dto.GroupSource(g.Source),
		MemberCount: g.MemberCount, CreatedAt: g.CreatedAt, UpdatedAt: g.UpdatedAt}
}

// @Summary Groups
// @ID listGroups
// @Tags identity
// @Success 200 {object} dto.ListGroups200JSONResponse "Groups."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/groups [get]
func (h *HTTP) ListGroups(ctx context.Context, _ dto.ListGroupsRequestObject) (dto.ListGroupsResponseObject, error) {
	gs, err := h.svc.ListGroups(ctx)
	if err != nil {
		return nil, err
	}
	out := dto.ListGroups200JSONResponse{Data: make([]dto.Group, len(gs))}
	for i, g := range gs {
		out.Data[i] = apiGroup(g)
	}
	return out, nil
}

// @Summary Create a group
// @ID createGroup
// @Tags identity
// @Param body body dto.GroupInput true "Request body"
// @Success 201 {object} dto.Group "Created."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/groups [post]
func (h *HTTP) CreateGroup(ctx context.Context, req dto.CreateGroupRequestObject) (dto.CreateGroupResponseObject, error) {
	if req.Body == nil {
		return nil, bodyRequired()
	}
	g, err := h.svc.CreateGroup(ctx, identitydto.GroupInput{Name: req.Body.Name, Description: str(req.Body.Description)})
	if err != nil {
		return nil, err
	}
	return dto.CreateGroup201JSONResponse(apiGroup(g)), nil
}

// @Summary One group
// @ID getGroup
// @Tags identity
// @Param group_id path string true "group_id"
// @Success 200 {object} dto.Group "The group."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/groups/{group_id} [get]
func (h *HTTP) GetGroup(ctx context.Context, req dto.GetGroupRequestObject) (dto.GetGroupResponseObject, error) {
	g, err := h.svc.GetGroup(ctx, req.GroupId)
	if err != nil {
		return nil, err
	}
	return dto.GetGroup200JSONResponse(apiGroup(g)), nil
}

// @Summary Rename or describe a group
// @ID updateGroup
// @Tags identity
// @Param group_id path string true "group_id"
// @Param body body dto.GroupInput true "Request body"
// @Success 200 {object} dto.Group "Updated."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/groups/{group_id} [patch]
func (h *HTTP) UpdateGroup(ctx context.Context, req dto.UpdateGroupRequestObject) (dto.UpdateGroupResponseObject, error) {
	if req.Body == nil {
		return nil, bodyRequired()
	}
	g, err := h.svc.UpdateGroup(ctx, req.GroupId, identitydto.GroupInput{Name: req.Body.Name, Description: str(req.Body.Description)})
	if err != nil {
		return nil, err
	}
	return dto.UpdateGroup200JSONResponse(apiGroup(g)), nil
}

// @Summary Delete a group and its role assignments
// @ID deleteGroup
// @Tags identity
// @Param group_id path string true "group_id"
// @Success 204 "Deleted."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/groups/{group_id} [delete]
func (h *HTTP) DeleteGroup(ctx context.Context, req dto.DeleteGroupRequestObject) (dto.DeleteGroupResponseObject, error) {
	if err := h.svc.DeleteGroup(ctx, req.GroupId); err != nil {
		return nil, err
	}
	return dto.DeleteGroup204Response{}, nil
}

// @Summary Members of a group
// @ID listGroupMembers
// @Tags identity
// @Param group_id path string true "group_id"
// @Success 200 {object} dto.ListGroupMembers200JSONResponse "Members."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/groups/{group_id}/members [get]
func (h *HTTP) ListGroupMembers(ctx context.Context, req dto.ListGroupMembersRequestObject) (dto.ListGroupMembersResponseObject, error) {
	ms, err := h.svc.ListGroupMembers(ctx, req.GroupId)
	if err != nil {
		return nil, err
	}
	out := dto.ListGroupMembers200JSONResponse{Data: make([]dto.GroupMember, len(ms))}
	for i, m := range ms {
		out.Data[i] = dto.GroupMember{Id: m.ID, Email: m.Email, DisplayName: m.DisplayName, AddedAt: m.AddedAt}
	}
	return out, nil
}

// @Summary Add a member to a group (idempotent)
// @ID addGroupMember
// @Tags identity
// @Param group_id path string true "group_id"
// @Param body body map[string]interface{} true "Request body"
// @Success 204 "The user is a member."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/groups/{group_id}/members [post]
func (h *HTTP) AddGroupMember(ctx context.Context, req dto.AddGroupMemberRequestObject) (dto.AddGroupMemberResponseObject, error) {
	if req.Body == nil || req.Body.UserId == uuid.Nil {
		return nil, problem.Validation(problem.FieldError{Field: "user_id", Code: "required", Message: "Choose a member."})
	}
	if err := h.svc.AddGroupMember(ctx, req.GroupId, req.Body.UserId); err != nil {
		return nil, err
	}
	return dto.AddGroupMember204Response{}, nil
}

// @Summary Remove a member from a group
// @ID removeGroupMember
// @Tags identity
// @Param group_id path string true "group_id"
// @Param user_id path string true "user_id"
// @Success 204 "Removed."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/groups/{group_id}/members/{user_id} [delete]
func (h *HTTP) RemoveGroupMember(ctx context.Context, req dto.RemoveGroupMemberRequestObject) (dto.RemoveGroupMemberResponseObject, error) {
	if err := h.svc.RemoveGroupMember(ctx, req.GroupId, req.UserId); err != nil {
		return nil, err
	}
	return dto.RemoveGroupMember204Response{}, nil
}

func apiServiceAccount(sa model.ServiceAccount) dto.ServiceAccount {
	return dto.ServiceAccount{Id: sa.ID, Name: sa.Name, Description: sa.Description, Disabled: sa.DisabledAt.Valid,
		ActiveKeyCount: sa.ActiveKeyCount, CreatedAt: sa.CreatedAt, UpdatedAt: sa.UpdatedAt}
}

func apiKey(k model.APIKey) dto.ApiKey {
	out := dto.ApiKey{Id: k.ID, Name: k.Name, Prefix: k.Prefix, CreatedAt: k.CreatedAt}
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

// @Summary Service accounts
// @ID listServiceAccounts
// @Tags identity
// @Success 200 {object} dto.ListServiceAccounts200JSONResponse "Service accounts."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/service-accounts [get]
func (h *HTTP) ListServiceAccounts(ctx context.Context, _ dto.ListServiceAccountsRequestObject) (dto.ListServiceAccountsResponseObject, error) {
	sas, err := h.svc.ListServiceAccounts(ctx)
	if err != nil {
		return nil, err
	}
	out := dto.ListServiceAccounts200JSONResponse{Data: make([]dto.ServiceAccount, len(sas))}
	for i, sa := range sas {
		out.Data[i] = apiServiceAccount(sa)
	}
	return out, nil
}

// @Summary Create a service account
// @ID createServiceAccount
// @Tags identity
// @Param body body dto.ServiceAccountCreate true "Request body"
// @Success 201 {object} dto.ServiceAccount "Created."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/service-accounts [post]
func (h *HTTP) CreateServiceAccount(ctx context.Context, req dto.CreateServiceAccountRequestObject) (dto.CreateServiceAccountResponseObject, error) {
	if req.Body == nil {
		return nil, bodyRequired()
	}
	sa, err := h.svc.CreateServiceAccount(ctx, identitydto.ServiceAccountInput{Name: req.Body.Name, Description: str(req.Body.Description)})
	if err != nil {
		return nil, err
	}
	return dto.CreateServiceAccount201JSONResponse(apiServiceAccount(sa)), nil
}

// @Summary One service account
// @ID getServiceAccount
// @Tags identity
// @Param service_account_id path string true "service_account_id"
// @Success 200 {object} dto.ServiceAccount "The service account."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/service-accounts/{service_account_id} [get]
func (h *HTTP) GetServiceAccount(ctx context.Context, req dto.GetServiceAccountRequestObject) (dto.GetServiceAccountResponseObject, error) {
	sa, err := h.svc.GetServiceAccount(ctx, req.ServiceAccountId)
	if err != nil {
		return nil, err
	}
	return dto.GetServiceAccount200JSONResponse(apiServiceAccount(sa)), nil
}

// @Summary Rename, describe, disable or re-enable a service account
// @ID updateServiceAccount
// @Tags identity
// @Param service_account_id path string true "service_account_id"
// @Param body body dto.ServiceAccountUpdate true "Request body"
// @Success 200 {object} dto.ServiceAccount "Updated."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/service-accounts/{service_account_id} [patch]
func (h *HTTP) UpdateServiceAccount(ctx context.Context, req dto.UpdateServiceAccountRequestObject) (dto.UpdateServiceAccountResponseObject, error) {
	if req.Body == nil {
		return nil, bodyRequired()
	}
	sa, err := h.svc.UpdateServiceAccount(ctx, req.ServiceAccountId, identitydto.ServiceAccountInput{Name: req.Body.Name,
		Description: str(req.Body.Description), Disabled: req.Body.Disabled})
	if err != nil {
		return nil, err
	}
	return dto.UpdateServiceAccount200JSONResponse(apiServiceAccount(sa)), nil
}

// @Summary Delete a service account, its keys and role assignments
// @ID deleteServiceAccount
// @Tags identity
// @Param service_account_id path string true "service_account_id"
// @Success 204 "Deleted."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/service-accounts/{service_account_id} [delete]
func (h *HTTP) DeleteServiceAccount(ctx context.Context, req dto.DeleteServiceAccountRequestObject) (dto.DeleteServiceAccountResponseObject, error) {
	if err := h.svc.DeleteServiceAccount(ctx, req.ServiceAccountId); err != nil {
		return nil, err
	}
	return dto.DeleteServiceAccount204Response{}, nil
}

// @Summary API keys of a service account (never the secret)
// @ID listApiKeys
// @Tags identity
// @Param service_account_id path string true "service_account_id"
// @Success 200 {object} dto.ListApiKeys200JSONResponse "Keys."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/service-accounts/{service_account_id}/keys [get]
func (h *HTTP) ListApiKeys(ctx context.Context, req dto.ListApiKeysRequestObject) (dto.ListApiKeysResponseObject, error) {
	ks, err := h.svc.ListAPIKeys(ctx, req.ServiceAccountId)
	if err != nil {
		return nil, err
	}
	out := dto.ListApiKeys200JSONResponse{Data: make([]dto.ApiKey, len(ks))}
	for i, k := range ks {
		out.Data[i] = apiKey(k)
	}
	return out, nil
}

// @Summary Create an API key (the secret is returned once)
// @ID createApiKey
// @Tags identity
// @Param service_account_id path string true "service_account_id"
// @Param body body dto.ApiKeyCreate true "Request body"
// @Success 201 {object} dto.ApiKeyCreated "Created."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/service-accounts/{service_account_id}/keys [post]
func (h *HTTP) CreateApiKey(ctx context.Context, req dto.CreateApiKeyRequestObject) (dto.CreateApiKeyResponseObject, error) {
	if req.Body == nil {
		return nil, bodyRequired()
	}
	k, secret, err := h.svc.CreateAPIKey(ctx, req.ServiceAccountId, req.Body.Name, req.Body.ExpiresAt)
	if err != nil {
		return nil, err
	}
	return dto.CreateApiKey201JSONResponse{Key: apiKey(k), Secret: secret}, nil
}

// @Summary Revoke an API key
// @ID revokeApiKey
// @Tags identity
// @Param service_account_id path string true "service_account_id"
// @Param key_id path string true "key_id"
// @Success 204 "Revoked."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/service-accounts/{service_account_id}/keys/{key_id} [delete]
func (h *HTTP) RevokeApiKey(ctx context.Context, req dto.RevokeApiKeyRequestObject) (dto.RevokeApiKeyResponseObject, error) {
	if err := h.svc.RevokeAPIKey(ctx, req.ServiceAccountId, req.KeyId); err != nil {
		return nil, err
	}
	return dto.RevokeApiKey204Response{}, nil
}
