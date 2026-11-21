// Package services holds the webhook use cases: endpoint and subscription
// management (control plane, hugi_app), the inbound ingress, and the
// outbound fan-out and delivery dispatcher (data plane, hugi_system).
package services

import (
	"context"

	"github.com/google/uuid"

	"github.com/nuriansyah/hugimethara/internal/modules/webhook/model"
	"github.com/nuriansyah/hugimethara/internal/modules/webhook/model/dto"
)

// EndpointUseCases manage inbound endpoints and inspect what they received.
type EndpointUseCases interface {
	ListEndpoints(ctx context.Context, workspaceID uuid.UUID) ([]model.Endpoint, error)
	GetEndpoint(ctx context.Context, id uuid.UUID) (model.Endpoint, error)
	// CreateEndpoint returns the generated signing secret once ("" when the
	// caller supplied it).
	CreateEndpoint(ctx context.Context, in dto.CreateEndpointInput) (model.Endpoint, string, error)
	UpdateEndpoint(ctx context.Context, id uuid.UUID, in dto.UpdateEndpointInput) (model.Endpoint, error)
	DeleteEndpoint(ctx context.Context, id uuid.UUID) error
	RotateEndpointSecret(ctx context.Context, id uuid.UUID, in dto.RotateSecretInput) (model.Endpoint, string, error)
	ListEvents(ctx context.Context, f dto.EventFilter) ([]model.Event, error)
	GetEvent(ctx context.Context, id uuid.UUID) (model.Event, error)
}

// SubscriptionUseCases manage outbound subscriptions and their deliveries.
type SubscriptionUseCases interface {
	EventTypes(ctx context.Context) ([]model.EventType, error)
	ListSubscriptions(ctx context.Context, workspaceID uuid.UUID) ([]model.Subscription, error)
	GetSubscription(ctx context.Context, id uuid.UUID) (model.Subscription, error)
	// CreateSubscription returns the generated signing secret once.
	CreateSubscription(ctx context.Context, in dto.CreateSubscriptionInput) (model.Subscription, string, error)
	UpdateSubscription(ctx context.Context, id uuid.UUID, in dto.UpdateSubscriptionInput) (model.Subscription, error)
	DeleteSubscription(ctx context.Context, id uuid.UUID) error
	RotateSubscriptionSecret(ctx context.Context, id uuid.UUID) (model.Subscription, string, error)
	// PingSubscription enqueues a webhook.ping test delivery.
	PingSubscription(ctx context.Context, id uuid.UUID) (model.Delivery, error)
	ListDeliveries(ctx context.Context, f dto.DeliveryFilter) ([]model.Delivery, error)
	GetDelivery(ctx context.Context, id uuid.UUID) (model.Delivery, error)
	// ReplayDelivery enqueues a new delivery of a finished one's event, linked
	// to it by replay_of.
	ReplayDelivery(ctx context.Context, id uuid.UUID) (model.Delivery, error)
}

// IngressUseCases receive inbound webhooks (unauthenticated; the signature
// is the credential).
type IngressUseCases interface {
	Receive(ctx context.Context, in dto.IngressRequest) (dto.IngressResult, error)
}

// Service is the complete API-side surface of the webhook module.
type Service interface {
	EndpointUseCases
	SubscriptionUseCases
	IngressUseCases
}

var _ Service = (*WebhookService)(nil)
