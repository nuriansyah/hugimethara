// Package dto holds the transport-agnostic inputs of the webhook use cases.
package dto

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/nuriansyah/hugimethara/internal/modules/webhook/model"
)

// CreateEndpointInput creates an inbound endpoint. Secret is required for the
// provider presets (stripe, github, slack) and generated when empty otherwise.
type CreateEndpointInput struct {
	WorkspaceID        uuid.UUID
	EnvironmentID      uuid.UUID
	Name               string
	Description        string
	Verifier           model.Scheme
	VerifierConfig     model.HMACConfig
	Secret             string
	ToleranceSeconds   int
	RateLimitPerMinute int
}

// UpdateEndpointInput edits an endpoint; nil fields are unchanged.
type UpdateEndpointInput struct {
	Name               *string
	Description        *string
	VerifierConfig     *model.HMACConfig
	ToleranceSeconds   *int
	RateLimitPerMinute *int
	Enabled            *bool
}

// RotateSecretInput replaces a signing secret (generated when empty and the
// scheme allows it).
type RotateSecretInput struct{ Secret string }

// CreateSubscriptionInput creates an outbound subscription.
type CreateSubscriptionInput struct {
	WorkspaceID   uuid.UUID
	EnvironmentID uuid.UUID
	Name          string
	URL           string
	EventTypes    []string
}

// UpdateSubscriptionInput edits a subscription; nil fields are unchanged.
type UpdateSubscriptionInput struct {
	Name       *string
	URL        *string
	EventTypes []string // nil: unchanged
	Enabled    *bool
}

// EventFilter lists inbound events.
type EventFilter struct {
	WorkspaceID uuid.UUID
	EndpointID  *uuid.UUID
	Limit       int
}

// DeliveryFilter lists deliveries.
type DeliveryFilter struct {
	WorkspaceID    uuid.UUID
	SubscriptionID *uuid.UUID
	Status         string
	Limit          int
}

// IngressRequest is an inbound request at /hooks/{public_id}.
type IngressRequest struct {
	PublicID    string
	Header      http.Header
	Body        []byte
	ContentType string
	SourceIP    string
}

// IngressResult is what the sender is answered.
type IngressResult struct {
	EventID   uuid.UUID
	Duplicate bool
	// Challenge is echoed for a Slack url_verification request.
	Challenge string
}
