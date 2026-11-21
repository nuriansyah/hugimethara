package model

import (
	"time"

	"github.com/google/uuid"

	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
)

// Resource types owned by the identity domain.
const (
	UserResource           = "user"
	GroupResource          = "group"
	ServiceAccountResource = "service_account"
)

// Membership statuses.
const (
	MembershipActive    = "active"
	MembershipSuspended = "suspended"
)

// OrgMember is a member of the caller's organization.
type OrgMember struct {
	User             User
	MembershipStatus string
	JoinedAt         time.Time
	IsOwner          bool
	Roles            []string
	Groups           []GroupRef
}

// GroupRef names a group.
type GroupRef struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

var ErrMemberNotFound = problem.NotFound("Member not found.")
