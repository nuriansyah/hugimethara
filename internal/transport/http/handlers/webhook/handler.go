// Package webhook adapts the webhook use cases to the HTTP API: management
// operations through the operation table, and the raw ingress handler that
// senders POST to. It holds no business rules.
package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/nuriansyah/hugimethara/internal/modules/webhook/model"
	whdto "github.com/nuriansyah/hugimethara/internal/modules/webhook/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/webhook/services"
	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
	"github.com/nuriansyah/hugimethara/internal/transport/http/dto"
)

// HTTP implements the webhook slice of the API.
type HTTP struct{ svc services.Service }

// NewHTTP returns the webhook HTTP adapter.
func NewHTTP(svc services.Service) *HTTP { return &HTTP{svc: svc} }

func str(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func wireConfig(c model.HMACConfig) dto.WebhookVerifierConfig {
	return dto.WebhookVerifierConfig{SignatureHeader: str(c.SignatureHeader), Encoding: str(c.Encoding),
		Prefix: str(c.Prefix), TimestampHeader: str(c.TimestampHeader), IdHeader: str(c.IDHeader)}
}

func modelConfig(c *dto.WebhookVerifierConfig) model.HMACConfig {
	if c == nil {
		return model.HMACConfig{}
	}
	v := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	return model.HMACConfig{SignatureHeader: v(c.SignatureHeader), Encoding: v(c.Encoding), Prefix: v(c.Prefix),
		TimestampHeader: v(c.TimestampHeader), IDHeader: v(c.IdHeader)}
}

func apiEndpoint(e model.Endpoint, secret string) dto.WebhookEndpoint {
	return dto.WebhookEndpoint{Id: e.ID, WorkspaceId: e.WorkspaceID, EnvironmentId: e.EnvironmentID, Name: e.Name,
		Description: e.Description, Verifier: string(e.Verifier), VerifierConfig: wireConfig(e.VerifierConfig),
		PublicId: e.PublicID, IngressPath: e.IngressPath(), SecretHint: e.SecretHint,
		ToleranceSeconds: e.ToleranceSeconds, RateLimitPerMinute: e.RateLimitPerMinute, Enabled: e.Enabled,
		ReceivedCount: e.ReceivedCount, RejectedCount: e.RejectedCount, LastReceivedAt: e.LastReceivedAt,
		LastRejectedAt: e.LastRejectedAt, LastRejectedReason: e.LastRejectedReason,
		SecretRotatedAt: e.SecretRotatedAt, CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt, Secret: str(secret)}
}

func decode(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("webhook: decode stored payload: %w", err)
	}
	return v, nil
}

func apiEvent(e model.Event) (dto.WebhookEvent, error) {
	payload, err := decode(e.Payload)
	if err != nil {
		return dto.WebhookEvent{}, err
	}
	headers := e.Headers
	if headers == nil {
		headers = map[string]string{}
	}
	return dto.WebhookEvent{Id: e.ID, WorkspaceId: e.WorkspaceID, EndpointId: e.EndpointID,
		ProviderEventId: str(e.ProviderEventID), EventType: e.EventType, ContentType: e.ContentType, Headers: headers,
		Payload: payload, RawBody: str(e.RawBody), PayloadSize: e.PayloadSize, SourceIp: e.SourceIP,
		ReceivedAt: e.ReceivedAt}, nil
}

func apiSubscription(s model.Subscription, secret string) dto.WebhookSubscription {
	return dto.WebhookSubscription{Id: s.ID, WorkspaceId: s.WorkspaceID, EnvironmentId: s.EnvironmentID, Name: s.Name,
		Url: s.URL, EventTypes: s.EventTypes, SecretHint: s.SecretHint, Enabled: s.Enabled,
		DisabledReason: s.DisabledReason, FailingSince: s.FailingSince, LastSuccessAt: s.LastSuccessAt,
		CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt, Secret: str(secret)}
}

func apiDelivery(d model.Delivery) (dto.WebhookDelivery, error) {
	payload, err := decode(d.Payload)
	if err != nil {
		return dto.WebhookDelivery{}, err
	}
	out := dto.WebhookDelivery{Id: d.ID, WorkspaceId: d.WorkspaceID, SubscriptionId: d.SubscriptionID,
		SubscriptionName: d.SubscriptionName, EventId: d.EventID, EventType: d.EventType, Payload: payload,
		ReplayOf: d.ReplayOf, Status: string(d.Status), Attempts: d.Attempts, MaxAttempts: model.MaxAttempts,
		NextAttemptAt: d.NextAttemptAt, LastAttemptAt: d.LastAttemptAt, ResponseStatus: d.ResponseStatus,
		ResponseExcerpt: d.ResponseExcerpt, LatencyMs: d.LatencyMS, Error: d.Error, CreatedAt: d.CreatedAt,
		FinishedAt: d.FinishedAt}
	if d.Status != model.DeliveryPending {
		out.NextAttemptAt = nil
	}
	for _, a := range d.AttemptLog {
		out.AttemptLog = append(out.AttemptLog, dto.WebhookDeliveryAttempt{Attempt: a.Attempt,
			ResponseStatus: a.ResponseStatus, ResponseExcerpt: a.ResponseExcerpt, LatencyMs: a.LatencyMS,
			Error: a.Error, AttemptedAt: a.AttemptedAt})
	}
	return out, nil
}

func required() error { return problem.BadRequest("A request body is required.") }

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

// ---- inbound endpoints ----

// @Summary List inbound webhook endpoints
// @ID listWebhooks
// @Tags webhooks
// @Param workspace_id query string true "Workspace ID"
// @Success 200 {object} dto.WebhookEndpoints
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/webhooks [get]
func (h *HTTP) ListWebhooks(ctx context.Context, req dto.ListWebhooksRequestObject) (dto.ListWebhooksResponseObject, error) {
	rows, err := h.svc.ListEndpoints(ctx, req.Params.WorkspaceId)
	if err != nil {
		return nil, err
	}
	out := dto.WebhookEndpoints{Data: make([]dto.WebhookEndpoint, len(rows))}
	for i, e := range rows {
		out.Data[i] = apiEndpoint(e, "")
	}
	return dto.ListWebhooks200JSONResponse(out), nil
}

// @Summary Create an inbound webhook endpoint
// @Description Senders POST to /hooks/{public_id}. For standard_webhooks and hmac_sha256 the signing secret is generated when omitted and returned once in `secret`; stripe, github and slack need the secret the provider issued.
// @ID createWebhook
// @Tags webhooks
// @Param body body dto.WebhookEndpointCreate true "Endpoint"
// @Success 201 {object} dto.WebhookEndpoint
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/webhooks [post]
func (h *HTTP) CreateWebhook(ctx context.Context, req dto.CreateWebhookRequestObject) (dto.CreateWebhookResponseObject, error) {
	if req.Body == nil {
		return nil, required()
	}
	b := req.Body
	e, secret, err := h.svc.CreateEndpoint(ctx, whdto.CreateEndpointInput{WorkspaceID: b.WorkspaceId,
		EnvironmentID: b.EnvironmentId, Name: b.Name, Description: deref(b.Description),
		Verifier: model.Scheme(deref(b.Verifier)), VerifierConfig: modelConfig(b.VerifierConfig), Secret: deref(b.Secret),
		ToleranceSeconds: deref(b.ToleranceSeconds), RateLimitPerMinute: deref(b.RateLimitPerMinute)})
	if err != nil {
		return nil, err
	}
	return dto.CreateWebhook201JSONResponse(apiEndpoint(e, secret)), nil
}

// @Summary Get an inbound webhook endpoint
// @ID getWebhook
// @Tags webhooks
// @Param webhook_id path string true "Endpoint ID"
// @Success 200 {object} dto.WebhookEndpoint
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/webhooks/{webhook_id} [get]
func (h *HTTP) GetWebhook(ctx context.Context, req dto.GetWebhookRequestObject) (dto.GetWebhookResponseObject, error) {
	e, err := h.svc.GetEndpoint(ctx, req.WebhookId)
	if err != nil {
		return nil, err
	}
	return dto.GetWebhook200JSONResponse(apiEndpoint(e, "")), nil
}

// @Summary Update an inbound webhook endpoint
// @ID updateWebhook
// @Tags webhooks
// @Param webhook_id path string true "Endpoint ID"
// @Param body body dto.WebhookEndpointUpdate true "Changes"
// @Success 200 {object} dto.WebhookEndpoint
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/webhooks/{webhook_id} [patch]
func (h *HTTP) UpdateWebhook(ctx context.Context, req dto.UpdateWebhookRequestObject) (dto.UpdateWebhookResponseObject, error) {
	if req.Body == nil {
		return nil, required()
	}
	b := req.Body
	in := whdto.UpdateEndpointInput{Name: b.Name, Description: b.Description, ToleranceSeconds: b.ToleranceSeconds,
		RateLimitPerMinute: b.RateLimitPerMinute, Enabled: b.Enabled}
	if b.VerifierConfig != nil {
		cfg := modelConfig(b.VerifierConfig)
		in.VerifierConfig = &cfg
	}
	e, err := h.svc.UpdateEndpoint(ctx, req.WebhookId, in)
	if err != nil {
		return nil, err
	}
	return dto.UpdateWebhook200JSONResponse(apiEndpoint(e, "")), nil
}

// @Summary Delete an inbound webhook endpoint
// @Description Removes the endpoint and the events it received. Workflows bound to it stop being triggered.
// @ID deleteWebhook
// @Tags webhooks
// @Param webhook_id path string true "Endpoint ID"
// @Success 204
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/webhooks/{webhook_id} [delete]
func (h *HTTP) DeleteWebhook(ctx context.Context, req dto.DeleteWebhookRequestObject) (dto.DeleteWebhookResponseObject, error) {
	if err := h.svc.DeleteEndpoint(ctx, req.WebhookId); err != nil {
		return nil, err
	}
	return dto.DeleteWebhook204Response{}, nil
}

// @Summary Rotate an endpoint's signing secret
// @Description The old secret stops verifying immediately. A generated secret is returned once in `secret`.
// @ID rotateWebhookSecret
// @Tags webhooks
// @Param webhook_id path string true "Endpoint ID"
// @Param body body dto.WebhookSecretRotate false "Provider-issued secret (stripe, github, slack)"
// @Success 200 {object} dto.WebhookEndpoint
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/webhooks/{webhook_id}/rotate-secret [post]
func (h *HTTP) RotateWebhookSecret(ctx context.Context, req dto.RotateWebhookSecretRequestObject) (dto.RotateWebhookSecretResponseObject, error) {
	in := whdto.RotateSecretInput{}
	if req.Body != nil {
		in.Secret = deref(req.Body.Secret)
	}
	e, secret, err := h.svc.RotateEndpointSecret(ctx, req.WebhookId, in)
	if err != nil {
		return nil, err
	}
	return dto.RotateWebhookSecret200JSONResponse(apiEndpoint(e, secret)), nil
}

// @Summary List received webhook events
// @Description Verified requests only (rejected ones are counted on the endpoint, never stored). Bodies are omitted; fetch one event for its payload.
// @ID listWebhookEvents
// @Tags webhooks
// @Param workspace_id query string false "Workspace ID (or endpoint_id)"
// @Param endpoint_id query string false "Endpoint ID"
// @Param limit query int false "Page size (max 200)"
// @Success 200 {object} dto.WebhookEvents
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/webhook-events [get]
func (h *HTTP) ListWebhookEvents(ctx context.Context, req dto.ListWebhookEventsRequestObject) (dto.ListWebhookEventsResponseObject, error) {
	f := whdto.EventFilter{EndpointID: req.Params.EndpointId, Limit: deref(req.Params.Limit)}
	if req.Params.WorkspaceId != nil {
		f.WorkspaceID = *req.Params.WorkspaceId
	}
	rows, err := h.svc.ListEvents(ctx, f)
	if err != nil {
		return nil, err
	}
	out := dto.WebhookEvents{Data: make([]dto.WebhookEvent, 0, len(rows))}
	for _, e := range rows {
		x, err := apiEvent(e)
		if err != nil {
			return nil, err
		}
		out.Data = append(out.Data, x)
	}
	return dto.ListWebhookEvents200JSONResponse(out), nil
}

// @Summary Get a received webhook event with its headers and body
// @ID getWebhookEvent
// @Tags webhooks
// @Param event_id path string true "Event ID"
// @Success 200 {object} dto.WebhookEvent
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/webhook-events/{event_id} [get]
func (h *HTTP) GetWebhookEvent(ctx context.Context, req dto.GetWebhookEventRequestObject) (dto.GetWebhookEventResponseObject, error) {
	e, err := h.svc.GetEvent(ctx, req.EventId)
	if err != nil {
		return nil, err
	}
	out, err := apiEvent(e)
	if err != nil {
		return nil, err
	}
	return dto.GetWebhookEvent200JSONResponse(out), nil
}

// ---- outbound subscriptions ----

// @Summary Platform event catalog
// @Description The events an outbound subscription can receive. Each carries a workspace_id; a subscription receives only its workspace's events.
// @ID listEventTypes
// @Tags webhooks
// @Success 200 {object} dto.EventTypes
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/event-types [get]
func (h *HTTP) ListEventTypes(ctx context.Context, _ dto.ListEventTypesRequestObject) (dto.ListEventTypesResponseObject, error) {
	rows, err := h.svc.EventTypes(ctx)
	if err != nil {
		return nil, err
	}
	out := dto.EventTypes{Data: make([]dto.EventType, len(rows))}
	for i, t := range rows {
		out.Data[i] = dto.EventType{Type: t.Type, Description: t.Description, Example: t.Example}
	}
	return dto.ListEventTypes200JSONResponse(out), nil
}

// @Summary List outbound webhook subscriptions
// @ID listWebhookSubscriptions
// @Tags webhooks
// @Param workspace_id query string true "Workspace ID"
// @Success 200 {object} dto.WebhookSubscriptions
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/webhook-subscriptions [get]
func (h *HTTP) ListWebhookSubscriptions(ctx context.Context, req dto.ListWebhookSubscriptionsRequestObject) (dto.ListWebhookSubscriptionsResponseObject, error) {
	rows, err := h.svc.ListSubscriptions(ctx, req.Params.WorkspaceId)
	if err != nil {
		return nil, err
	}
	out := dto.WebhookSubscriptions{Data: make([]dto.WebhookSubscription, len(rows))}
	for i, s := range rows {
		out.Data[i] = apiSubscription(s, "")
	}
	return dto.ListWebhookSubscriptions200JSONResponse(out), nil
}

// @Summary Create an outbound webhook subscription
// @Description Deliveries are signed with Standard Webhooks headers using the whsec_ secret returned once in `secret`. Failed deliveries retry after 30s, 2m, 10m, 30m, 2h, 6h and 12h, then are dead-lettered.
// @ID createWebhookSubscription
// @Tags webhooks
// @Param body body dto.WebhookSubscriptionCreate true "Subscription"
// @Success 201 {object} dto.WebhookSubscription
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/webhook-subscriptions [post]
func (h *HTTP) CreateWebhookSubscription(ctx context.Context, req dto.CreateWebhookSubscriptionRequestObject) (dto.CreateWebhookSubscriptionResponseObject, error) {
	if req.Body == nil {
		return nil, required()
	}
	b := req.Body
	s, secret, err := h.svc.CreateSubscription(ctx, whdto.CreateSubscriptionInput{WorkspaceID: b.WorkspaceId,
		EnvironmentID: b.EnvironmentId, Name: b.Name, URL: b.Url, EventTypes: b.EventTypes})
	if err != nil {
		return nil, err
	}
	return dto.CreateWebhookSubscription201JSONResponse(apiSubscription(s, secret)), nil
}

// @Summary Get an outbound webhook subscription
// @ID getWebhookSubscription
// @Tags webhooks
// @Param subscription_id path string true "Subscription ID"
// @Success 200 {object} dto.WebhookSubscription
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/webhook-subscriptions/{subscription_id} [get]
func (h *HTTP) GetWebhookSubscription(ctx context.Context, req dto.GetWebhookSubscriptionRequestObject) (dto.GetWebhookSubscriptionResponseObject, error) {
	s, err := h.svc.GetSubscription(ctx, req.SubscriptionId)
	if err != nil {
		return nil, err
	}
	return dto.GetWebhookSubscription200JSONResponse(apiSubscription(s, "")), nil
}

// @Summary Update an outbound webhook subscription
// @Description Re-enabling a subscription that was disabled after failing for 5 days clears its failure streak.
// @ID updateWebhookSubscription
// @Tags webhooks
// @Param subscription_id path string true "Subscription ID"
// @Param body body dto.WebhookSubscriptionUpdate true "Changes"
// @Success 200 {object} dto.WebhookSubscription
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/webhook-subscriptions/{subscription_id} [patch]
func (h *HTTP) UpdateWebhookSubscription(ctx context.Context, req dto.UpdateWebhookSubscriptionRequestObject) (dto.UpdateWebhookSubscriptionResponseObject, error) {
	if req.Body == nil {
		return nil, required()
	}
	b := req.Body
	s, err := h.svc.UpdateSubscription(ctx, req.SubscriptionId, whdto.UpdateSubscriptionInput{Name: b.Name, URL: b.Url,
		EventTypes: b.EventTypes, Enabled: b.Enabled})
	if err != nil {
		return nil, err
	}
	return dto.UpdateWebhookSubscription200JSONResponse(apiSubscription(s, "")), nil
}

// @Summary Delete an outbound webhook subscription
// @Description Removes the subscription and its delivery log; pending deliveries are dropped.
// @ID deleteWebhookSubscription
// @Tags webhooks
// @Param subscription_id path string true "Subscription ID"
// @Success 204
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/webhook-subscriptions/{subscription_id} [delete]
func (h *HTTP) DeleteWebhookSubscription(ctx context.Context, req dto.DeleteWebhookSubscriptionRequestObject) (dto.DeleteWebhookSubscriptionResponseObject, error) {
	if err := h.svc.DeleteSubscription(ctx, req.SubscriptionId); err != nil {
		return nil, err
	}
	return dto.DeleteWebhookSubscription204Response{}, nil
}

// @Summary Rotate a subscription's signing secret
// @Description Pending retries are signed with the new secret. The secret is returned once in `secret`.
// @ID rotateWebhookSubscriptionSecret
// @Tags webhooks
// @Param subscription_id path string true "Subscription ID"
// @Success 200 {object} dto.WebhookSubscription
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/webhook-subscriptions/{subscription_id}/rotate-secret [post]
func (h *HTTP) RotateWebhookSubscriptionSecret(ctx context.Context, req dto.RotateWebhookSubscriptionSecretRequestObject) (dto.RotateWebhookSubscriptionSecretResponseObject, error) {
	s, secret, err := h.svc.RotateSubscriptionSecret(ctx, req.SubscriptionId)
	if err != nil {
		return nil, err
	}
	return dto.RotateWebhookSubscriptionSecret200JSONResponse(apiSubscription(s, secret)), nil
}

// @Summary Send a test delivery
// @Description Enqueues a webhook.ping delivery to the subscription.
// @ID pingWebhookSubscription
// @Tags webhooks
// @Param subscription_id path string true "Subscription ID"
// @Success 202 {object} dto.WebhookDelivery
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/webhook-subscriptions/{subscription_id}/ping [post]
func (h *HTTP) PingWebhookSubscription(ctx context.Context, req dto.PingWebhookSubscriptionRequestObject) (dto.PingWebhookSubscriptionResponseObject, error) {
	d, err := h.svc.PingSubscription(ctx, req.SubscriptionId)
	if err != nil {
		return nil, err
	}
	out, err := apiDelivery(d)
	if err != nil {
		return nil, err
	}
	return dto.PingWebhookSubscription202JSONResponse(out), nil
}

// @Summary Delivery log
// @ID listWebhookDeliveries
// @Tags webhooks
// @Param workspace_id query string false "Workspace ID (or subscription_id)"
// @Param subscription_id query string false "Subscription ID"
// @Param status query string false "pending, succeeded or dead_lettered"
// @Param limit query int false "Page size (max 200)"
// @Success 200 {object} dto.WebhookDeliveries
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 422 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/webhook-deliveries [get]
func (h *HTTP) ListWebhookDeliveries(ctx context.Context, req dto.ListWebhookDeliveriesRequestObject) (dto.ListWebhookDeliveriesResponseObject, error) {
	f := whdto.DeliveryFilter{SubscriptionID: req.Params.SubscriptionId, Status: deref(req.Params.Status),
		Limit: deref(req.Params.Limit)}
	if req.Params.WorkspaceId != nil {
		f.WorkspaceID = *req.Params.WorkspaceId
	}
	rows, err := h.svc.ListDeliveries(ctx, f)
	if err != nil {
		return nil, err
	}
	out := dto.WebhookDeliveries{Data: make([]dto.WebhookDelivery, 0, len(rows))}
	for _, d := range rows {
		x, err := apiDelivery(d)
		if err != nil {
			return nil, err
		}
		out.Data = append(out.Data, x)
	}
	return dto.ListWebhookDeliveries200JSONResponse(out), nil
}

// @Summary Get a delivery with its payload and every attempt
// @ID getWebhookDelivery
// @Tags webhooks
// @Param delivery_id path string true "Delivery ID"
// @Success 200 {object} dto.WebhookDelivery
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/webhook-deliveries/{delivery_id} [get]
func (h *HTTP) GetWebhookDelivery(ctx context.Context, req dto.GetWebhookDeliveryRequestObject) (dto.GetWebhookDeliveryResponseObject, error) {
	d, err := h.svc.GetDelivery(ctx, req.DeliveryId)
	if err != nil {
		return nil, err
	}
	out, err := apiDelivery(d)
	if err != nil {
		return nil, err
	}
	return dto.GetWebhookDelivery200JSONResponse(out), nil
}

// @Summary Replay a delivery
// @Description Creates a new delivery of the same event (replay_of links it to the original). Only succeeded or dead-lettered deliveries can be replayed.
// @ID replayWebhookDelivery
// @Tags webhooks
// @Param delivery_id path string true "Delivery ID"
// @Success 202 {object} dto.WebhookDelivery
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 403 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 409 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Produce json
// @Security sessionCookie
// @Security apiKey
// @Router /v1/hugimethara/webhook-deliveries/{delivery_id}/replay [post]
func (h *HTTP) ReplayWebhookDelivery(ctx context.Context, req dto.ReplayWebhookDeliveryRequestObject) (dto.ReplayWebhookDeliveryResponseObject, error) {
	d, err := h.svc.ReplayDelivery(ctx, req.DeliveryId)
	if err != nil {
		return nil, err
	}
	out, err := apiDelivery(d)
	if err != nil {
		return nil, err
	}
	return dto.ReplayWebhookDelivery202JSONResponse(out), nil
}

// ---- ingress ----

// Ingress receives an inbound webhook. It is mounted outside the session,
// CSRF and origin middleware: the signature is the only credential.
//
// @Summary Receive an inbound webhook
// @Description Unauthenticated; the request must carry a valid signature for the endpoint's verifier. Unknown or disabled endpoints answer 404. Invalid signatures and timestamps outside the tolerance answer 401 (signature_missing, signature_malformed, signature_invalid, timestamp_out_of_tolerance) and nothing is stored. A repeated provider event id answers 200 duplicate and starts nothing. Bodies are limited to 1 MiB; requests are rate-limited per endpoint.
// @ID receiveWebhook
// @Tags webhooks
// @Param public_id path string true "Endpoint public ID"
// @Param body body object true "Provider payload"
// @Success 202 {object} dto.WebhookReceipt
// @Success 200 {object} dto.WebhookReceipt "Duplicate (or the Slack url_verification challenge)"
// @Failure 401 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 404 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 413 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Failure 429 {object} dto.Problem "Error (RFC 9457 problem details)."
// @Accept json
// @Produce json
// @Router /hooks/{public_id} [post]
func (h *HTTP) Ingress(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, model.MaxBodyBytes))
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		problem.WriteError(w, r, model.ErrBodyTooLarge)
		return
	}
	if err != nil {
		problem.WriteError(w, r, problem.BadRequest("The request body could not be read."))
		return
	}
	in := whdto.IngressRequest{PublicID: chi.URLParam(r, "public_id"), Header: r.Header, Body: body,
		ContentType: r.Header.Get("Content-Type")}
	if ip := principal.ClientInfoFrom(r.Context()).IP; ip.IsValid() {
		in.SourceIP = ip.String()
	}
	res, err := h.svc.Receive(r.Context(), in)
	switch {
	case err != nil:
		problem.WriteError(w, r, err)
	case res.Challenge != "":
		problem.WriteJSON(w, http.StatusOK, map[string]string{"challenge": res.Challenge})
	case res.Duplicate:
		problem.WriteJSON(w, http.StatusOK, dto.WebhookReceipt{Status: "duplicate"})
	default:
		id := res.EventID
		problem.WriteJSON(w, http.StatusAccepted, dto.WebhookReceipt{Status: "accepted", Id: &id})
	}
}
