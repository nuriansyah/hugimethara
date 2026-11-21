package dto

import (
	"github.com/google/uuid"

	"github.com/nuriansyah/hugimethara/internal/modules/identity/model"
)

// InvitationInput creates an invitation.
type InvitationInput struct {
	Email         string
	OrgRoleID     *uuid.UUID
	Workspaces    []model.WorkspaceGrant
	ExpiresInDays int
}

// CreatedInvitation carries the link token, shown once.
type CreatedInvitation struct {
	Invitation model.Invitation
	Token      string
}

// AcceptInput accepts an invitation without a matching session.
type AcceptInput struct {
	DisplayName string // new accounts
	Password    string // new accounts choose one; existing accounts prove it
}
