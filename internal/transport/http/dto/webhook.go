package dto

import (
	"net/http"
	"time"

	"github.com/google/uuid"
)

// ---- S12: webhook engine ----

// WebhookVerifierConfig configures the generic hmac_sha256 verifier.
type WebhookVerifierConfig struct {
	SignatureHeader *string `json:"signature_header,omitempty"`
	Encoding        *string `json:"encoding,omitempty" enums:"hex,base64"`
	Prefix          *string `json:"prefix,omitempty"`
	TimestampHeader *string `json:"timestamp_header,omitempty"`
	IdHeader        *string `json:"id_header,omitempty"`
}

type WebhookEndpoint struct {
	Id                 uuid.UUID             `json:"id" binding:"required"`
	WorkspaceId        uuid.UUID             `json:"workspace_id" binding:"required"`
	EnvironmentId      uuid.UUID             `json:"environment_id" binding:"required"`
	Name               string                `json:"name" binding:"required"`
	Description        string                `json:"description" binding:"required"`
	Verifier           string                `json:"verifier" binding:"required" enums:"standard_webhooks,hmac_sha256,stripe,github,slack,n8n,make,zapier"`
	VerifierConfig     WebhookVerifierConfig `json:"verifier_config" binding:"required"`
	PublicId           string                `json:"public_id" binding:"required"`
	IngressPath        string                `json:"ingress_path" binding:"required"`
	SecretHint         string                `json:"secret_hint" binding:"required"`
	ToleranceSeconds   int                   `json:"tolerance_seconds" binding:"required"`
	RateLimitPerMinute int                   `json:"rate_limit_per_minute" binding:"required"`
	Enabled            bool                  `json:"enabled" binding:"required"`
	ReceivedCount      int64                 `json:"received_count" binding:"required"`
	RejectedCount      int64                 `json:"rejected_count" binding:"required"`
	LastReceivedAt     *time.Time            `json:"last_received_at,omitempty"`
	LastRejectedAt     *time.Time            `json:"last_rejected_at,omitempty"`
	LastRejectedReason string                `json:"last_rejected_reason" binding:"required"`
	SecretRotatedAt    *time.Time            `json:"secret_rotated_at,omitempty"`
	CreatedAt          time.Time             `json:"created_at" binding:"required"`
	UpdatedAt          time.Time             `json:"updated_at" binding:"required"`
	// Secret is returned once, on create or rotate, when the platform generated it.
	Secret *string `json:"secret,omitempty"`
}

type WebhookEndpoints struct {
	Data []WebhookEndpoint `json:"data" binding:"required"`
}

type WebhookEndpointCreate struct {
	WorkspaceId        uuid.UUID              `json:"workspace_id" binding:"required"`
	EnvironmentId      uuid.UUID              `json:"environment_id" binding:"required"`
	Name               string                 `json:"name" binding:"required"`
	Description        *string                `json:"description,omitempty"`
	Verifier           *string                `json:"verifier,omitempty" enums:"standard_webhooks,hmac_sha256,stripe,github,slack,n8n,make,zapier"`
	VerifierConfig     *WebhookVerifierConfig `json:"verifier_config,omitempty"`
	Secret             *string                `json:"secret,omitempty"`
	ToleranceSeconds   *int                   `json:"tolerance_seconds,omitempty"`
	RateLimitPerMinute *int                   `json:"rate_limit_per_minute,omitempty"`
}

type WebhookEndpointUpdate struct {
	Name               *string                `json:"name,omitempty"`
	Description        *string                `json:"description,omitempty"`
	VerifierConfig     *WebhookVerifierConfig `json:"verifier_config,omitempty"`
	ToleranceSeconds   *int                   `json:"tolerance_seconds,omitempty"`
	RateLimitPerMinute *int                   `json:"rate_limit_per_minute,omitempty"`
	Enabled            *bool                  `json:"enabled,omitempty"`
}

type WebhookSecretRotate struct {
	// Secret is required for the provider presets (stripe, github, slack) and generated when omitted otherwise.
	Secret *string `json:"secret,omitempty"`
}

type WebhookEvent struct {
	Id              uuid.UUID         `json:"id" binding:"required"`
	WorkspaceId     uuid.UUID         `json:"workspace_id" binding:"required"`
	EndpointId      uuid.UUID         `json:"endpoint_id" binding:"required"`
	ProviderEventId *string           `json:"provider_event_id,omitempty"`
	EventType       string            `json:"event_type" binding:"required"`
	ContentType     string            `json:"content_type" binding:"required"`
	Headers         map[string]string `json:"headers" binding:"required"`
	Payload         interface{}       `json:"payload,omitempty"`
	RawBody         *string           `json:"raw_body,omitempty"`
	PayloadSize     int               `json:"payload_size" binding:"required"`
	SourceIp        string            `json:"source_ip" binding:"required"`
	ReceivedAt      time.Time         `json:"received_at" binding:"required"`
}

type WebhookEvents struct {
	Data []WebhookEvent `json:"data" binding:"required"`
}

type WebhookSubscription struct {
	Id             uuid.UUID  `json:"id" binding:"required"`
	WorkspaceId    uuid.UUID  `json:"workspace_id" binding:"required"`
	EnvironmentId  uuid.UUID  `json:"environment_id" binding:"required"`
	Name           string     `json:"name" binding:"required"`
	Url            string     `json:"url" binding:"required"`
	EventTypes     []string   `json:"event_types" binding:"required"`
	SecretHint     string     `json:"secret_hint" binding:"required"`
	Enabled        bool       `json:"enabled" binding:"required"`
	DisabledReason string     `json:"disabled_reason" binding:"required"`
	FailingSince   *time.Time `json:"failing_since,omitempty"`
	LastSuccessAt  *time.Time `json:"last_success_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at" binding:"required"`
	UpdatedAt      time.Time  `json:"updated_at" binding:"required"`
	// Secret (whsec_…) is returned once, on create or rotate.
	Secret *string `json:"secret,omitempty"`
}

type WebhookSubscriptions struct {
	Data []WebhookSubscription `json:"data" binding:"required"`
}

type WebhookSubscriptionCreate struct {
	WorkspaceId   uuid.UUID `json:"workspace_id" binding:"required"`
	EnvironmentId uuid.UUID `json:"environment_id" binding:"required"`
	Name          string    `json:"name" binding:"required"`
	Url           string    `json:"url" binding:"required"`
	EventTypes    []string  `json:"event_types" binding:"required"`
}

type WebhookSubscriptionUpdate struct {
	Name       *string  `json:"name,omitempty"`
	Url        *string  `json:"url,omitempty"`
	EventTypes []string `json:"event_types,omitempty"`
	Enabled    *bool    `json:"enabled,omitempty"`
}

type WebhookDeliveryAttempt struct {
	Attempt         int       `json:"attempt" binding:"required"`
	ResponseStatus  *int      `json:"response_status,omitempty"`
	ResponseExcerpt string    `json:"response_excerpt" binding:"required"`
	LatencyMs       int       `json:"latency_ms" binding:"required"`
	Error           string    `json:"error" binding:"required"`
	AttemptedAt     time.Time `json:"attempted_at" binding:"required"`
}

type WebhookDelivery struct {
	Id               uuid.UUID                `json:"id" binding:"required"`
	WorkspaceId      uuid.UUID                `json:"workspace_id" binding:"required"`
	SubscriptionId   uuid.UUID                `json:"subscription_id" binding:"required"`
	SubscriptionName string                   `json:"subscription_name" binding:"required"`
	EventId          string                   `json:"event_id" binding:"required"`
	EventType        string                   `json:"event_type" binding:"required"`
	Payload          interface{}              `json:"payload,omitempty"`
	ReplayOf         *uuid.UUID               `json:"replay_of,omitempty"`
	Status           string                   `json:"status" binding:"required" enums:"pending,succeeded,dead_lettered"`
	Attempts         int                      `json:"attempts" binding:"required"`
	MaxAttempts      int                      `json:"max_attempts" binding:"required"`
	NextAttemptAt    *time.Time               `json:"next_attempt_at,omitempty"`
	LastAttemptAt    *time.Time               `json:"last_attempt_at,omitempty"`
	ResponseStatus   *int                     `json:"response_status,omitempty"`
	ResponseExcerpt  string                   `json:"response_excerpt" binding:"required"`
	LatencyMs        *int                     `json:"latency_ms,omitempty"`
	Error            string                   `json:"error" binding:"required"`
	CreatedAt        time.Time                `json:"created_at" binding:"required"`
	FinishedAt       *time.Time               `json:"finished_at,omitempty"`
	AttemptLog       []WebhookDeliveryAttempt `json:"attempt_log,omitempty"`
}

type WebhookDeliveries struct {
	Data []WebhookDelivery `json:"data" binding:"required"`
}

type EventType struct {
	Type        string                 `json:"type" binding:"required"`
	Description string                 `json:"description" binding:"required"`
	Example     map[string]interface{} `json:"example" binding:"required"`
}

type EventTypes struct {
	Data []EventType `json:"data" binding:"required"`
}

// WebhookReceipt answers a webhook sender.
type WebhookReceipt struct {
	Status string     `json:"status" binding:"required" enums:"accepted,duplicate"`
	Id     *uuid.UUID `json:"id,omitempty"`
}

// ---- request objects ----

type ListWebhooksParams struct {
	WorkspaceId uuid.UUID `form:"workspace_id" json:"workspace_id"`
}
type ListWebhooksRequestObject struct{ Params ListWebhooksParams }
type CreateWebhookRequestObject struct{ Body *WebhookEndpointCreate }
type GetWebhookRequestObject struct{ WebhookId uuid.UUID }
type UpdateWebhookRequestObject struct {
	WebhookId uuid.UUID
	Body      *WebhookEndpointUpdate
}
type DeleteWebhookRequestObject struct{ WebhookId uuid.UUID }
type RotateWebhookSecretRequestObject struct {
	WebhookId uuid.UUID
	Body      *WebhookSecretRotate
}
type ListWebhookEventsParams struct {
	WorkspaceId *uuid.UUID `form:"workspace_id,omitempty" json:"workspace_id,omitempty"`
	EndpointId  *uuid.UUID `form:"endpoint_id,omitempty" json:"endpoint_id,omitempty"`
	Limit       *int       `form:"limit,omitempty" json:"limit,omitempty"`
}
type ListWebhookEventsRequestObject struct{ Params ListWebhookEventsParams }
type GetWebhookEventRequestObject struct{ EventId uuid.UUID }
type ListEventTypesRequestObject struct{}
type ListWebhookSubscriptionsParams struct {
	WorkspaceId uuid.UUID `form:"workspace_id" json:"workspace_id"`
}
type ListWebhookSubscriptionsRequestObject struct {
	Params ListWebhookSubscriptionsParams
}
type CreateWebhookSubscriptionRequestObject struct{ Body *WebhookSubscriptionCreate }
type GetWebhookSubscriptionRequestObject struct{ SubscriptionId uuid.UUID }
type UpdateWebhookSubscriptionRequestObject struct {
	SubscriptionId uuid.UUID
	Body           *WebhookSubscriptionUpdate
}
type DeleteWebhookSubscriptionRequestObject struct{ SubscriptionId uuid.UUID }
type RotateWebhookSubscriptionSecretRequestObject struct{ SubscriptionId uuid.UUID }
type PingWebhookSubscriptionRequestObject struct{ SubscriptionId uuid.UUID }
type ListWebhookDeliveriesParams struct {
	WorkspaceId    *uuid.UUID `form:"workspace_id,omitempty" json:"workspace_id,omitempty"`
	SubscriptionId *uuid.UUID `form:"subscription_id,omitempty" json:"subscription_id,omitempty"`
	Status         *string    `form:"status,omitempty" json:"status,omitempty"`
	Limit          *int       `form:"limit,omitempty" json:"limit,omitempty"`
}
type ListWebhookDeliveriesRequestObject struct{ Params ListWebhookDeliveriesParams }
type GetWebhookDeliveryRequestObject struct{ DeliveryId uuid.UUID }
type ReplayWebhookDeliveryRequestObject struct{ DeliveryId uuid.UUID }

// ---- responses ----

type ListWebhooksResponseObject interface {
	VisitListWebhooksResponse(http.ResponseWriter) error
}

type ListWebhooks200JSONResponse WebhookEndpoints

func (r ListWebhooks200JSONResponse) VisitListWebhooksResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 200, r)
}

type CreateWebhookResponseObject interface {
	VisitCreateWebhookResponse(http.ResponseWriter) error
}

type CreateWebhook201JSONResponse WebhookEndpoint

func (r CreateWebhook201JSONResponse) VisitCreateWebhookResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 201, r)
}

type GetWebhookResponseObject interface {
	VisitGetWebhookResponse(http.ResponseWriter) error
}

type GetWebhook200JSONResponse WebhookEndpoint

func (r GetWebhook200JSONResponse) VisitGetWebhookResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 200, r)
}

type UpdateWebhookResponseObject interface {
	VisitUpdateWebhookResponse(http.ResponseWriter) error
}

type UpdateWebhook200JSONResponse WebhookEndpoint

func (r UpdateWebhook200JSONResponse) VisitUpdateWebhookResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 200, r)
}

type DeleteWebhookResponseObject interface {
	VisitDeleteWebhookResponse(http.ResponseWriter) error
}

type DeleteWebhook204Response struct{}

func (DeleteWebhook204Response) VisitDeleteWebhookResponse(w http.ResponseWriter) error {
	w.WriteHeader(http.StatusNoContent)
	return nil
}

type RotateWebhookSecretResponseObject interface {
	VisitRotateWebhookSecretResponse(http.ResponseWriter) error
}

type RotateWebhookSecret200JSONResponse WebhookEndpoint

func (r RotateWebhookSecret200JSONResponse) VisitRotateWebhookSecretResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 200, r)
}

type ListWebhookEventsResponseObject interface {
	VisitListWebhookEventsResponse(http.ResponseWriter) error
}

type ListWebhookEvents200JSONResponse WebhookEvents

func (r ListWebhookEvents200JSONResponse) VisitListWebhookEventsResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 200, r)
}

type GetWebhookEventResponseObject interface {
	VisitGetWebhookEventResponse(http.ResponseWriter) error
}

type GetWebhookEvent200JSONResponse WebhookEvent

func (r GetWebhookEvent200JSONResponse) VisitGetWebhookEventResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 200, r)
}

type ListEventTypesResponseObject interface {
	VisitListEventTypesResponse(http.ResponseWriter) error
}

type ListEventTypes200JSONResponse EventTypes

func (r ListEventTypes200JSONResponse) VisitListEventTypesResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 200, r)
}

type ListWebhookSubscriptionsResponseObject interface {
	VisitListWebhookSubscriptionsResponse(http.ResponseWriter) error
}

type ListWebhookSubscriptions200JSONResponse WebhookSubscriptions

func (r ListWebhookSubscriptions200JSONResponse) VisitListWebhookSubscriptionsResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 200, r)
}

type CreateWebhookSubscriptionResponseObject interface {
	VisitCreateWebhookSubscriptionResponse(http.ResponseWriter) error
}

type CreateWebhookSubscription201JSONResponse WebhookSubscription

func (r CreateWebhookSubscription201JSONResponse) VisitCreateWebhookSubscriptionResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 201, r)
}

type GetWebhookSubscriptionResponseObject interface {
	VisitGetWebhookSubscriptionResponse(http.ResponseWriter) error
}

type GetWebhookSubscription200JSONResponse WebhookSubscription

func (r GetWebhookSubscription200JSONResponse) VisitGetWebhookSubscriptionResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 200, r)
}

type UpdateWebhookSubscriptionResponseObject interface {
	VisitUpdateWebhookSubscriptionResponse(http.ResponseWriter) error
}

type UpdateWebhookSubscription200JSONResponse WebhookSubscription

func (r UpdateWebhookSubscription200JSONResponse) VisitUpdateWebhookSubscriptionResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 200, r)
}

type DeleteWebhookSubscriptionResponseObject interface {
	VisitDeleteWebhookSubscriptionResponse(http.ResponseWriter) error
}

type DeleteWebhookSubscription204Response struct{}

func (DeleteWebhookSubscription204Response) VisitDeleteWebhookSubscriptionResponse(w http.ResponseWriter) error {
	w.WriteHeader(http.StatusNoContent)
	return nil
}

type RotateWebhookSubscriptionSecretResponseObject interface {
	VisitRotateWebhookSubscriptionSecretResponse(http.ResponseWriter) error
}

type RotateWebhookSubscriptionSecret200JSONResponse WebhookSubscription

func (r RotateWebhookSubscriptionSecret200JSONResponse) VisitRotateWebhookSubscriptionSecretResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 200, r)
}

type PingWebhookSubscriptionResponseObject interface {
	VisitPingWebhookSubscriptionResponse(http.ResponseWriter) error
}

type PingWebhookSubscription202JSONResponse WebhookDelivery

func (r PingWebhookSubscription202JSONResponse) VisitPingWebhookSubscriptionResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 202, r)
}

type ListWebhookDeliveriesResponseObject interface {
	VisitListWebhookDeliveriesResponse(http.ResponseWriter) error
}

type ListWebhookDeliveries200JSONResponse WebhookDeliveries

func (r ListWebhookDeliveries200JSONResponse) VisitListWebhookDeliveriesResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 200, r)
}

type GetWebhookDeliveryResponseObject interface {
	VisitGetWebhookDeliveryResponse(http.ResponseWriter) error
}

type GetWebhookDelivery200JSONResponse WebhookDelivery

func (r GetWebhookDelivery200JSONResponse) VisitGetWebhookDeliveryResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 200, r)
}

type ReplayWebhookDeliveryResponseObject interface {
	VisitReplayWebhookDeliveryResponse(http.ResponseWriter) error
}

type ReplayWebhookDelivery202JSONResponse WebhookDelivery

func (r ReplayWebhookDelivery202JSONResponse) VisitReplayWebhookDeliveryResponse(w http.ResponseWriter) error {
	return writeWorkflowJSON(w, 202, r)
}
