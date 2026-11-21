package services

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/nuriansyah/hugimethara/internal/infrastructure/security/ratelimit"
	authzmodel "github.com/nuriansyah/hugimethara/internal/modules/authorization/model"
	authzsvc "github.com/nuriansyah/hugimethara/internal/modules/authorization/services"
	auditmodel "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/model"
	auditsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/services"
	"github.com/nuriansyah/hugimethara/internal/modules/webhook/model"
	"github.com/nuriansyah/hugimethara/internal/modules/webhook/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/webhook/repository"
	"github.com/nuriansyah/hugimethara/internal/platform/clock"
	secretbox "github.com/nuriansyah/hugimethara/internal/platform/crypto"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/platform/id"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

// List page bounds.
const (
	defaultListLimit = 50
	maxListLimit     = 200
)

// Limiter is the rate limiter the ingress uses (ratelimit.Limiter).
type Limiter interface {
	Allow(ctx context.Context, rule ratelimit.Rule, key string) error
}

// Config tunes the API-side service.
type Config struct {
	Limiter    Limiter // nil: no ingress rate limit (tests)
	Registerer prometheus.Registerer
	Clock      clock.Clock
	IDs        id.Generator
}

// WebhookService implements the control plane and the ingress. It runs as
// hugi_app; every tenant transaction is RLS-bound.
type WebhookService struct {
	pool    *sqlx.DB
	az      authzsvc.Authorizer
	sealer  *secretbox.Key
	cfg     Config
	ingress *prometheus.CounterVec
}

// NewService builds the webhook service. sealer protects signing secrets at rest.
func NewService(pool *sqlx.DB, az authzsvc.Authorizer, sealer *secretbox.Key, cfg Config) *WebhookService {
	if cfg.Clock == nil {
		cfg.Clock = clock.System{}
	}
	if cfg.IDs == nil {
		cfg.IDs = id.V7{}
	}
	s := &WebhookService{pool: pool, az: az, sealer: sealer, cfg: cfg,
		ingress: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "hugi_webhook_ingress_total",
			Help: "Inbound webhook requests, by result."}, []string{"result"})}
	if cfg.Registerer != nil {
		cfg.Registerer.MustRegister(s.ingress)
	}
	return s
}

// Sealing binds each ciphertext to its row, so it cannot be moved to another.
func endpointAAD(org, id uuid.UUID) []byte {
	return []byte("webhook_endpoint|" + org.String() + "|" + id.String())
}

func subscriptionAAD(org, id uuid.UUID) []byte {
	return []byte("webhook_subscription|" + org.String() + "|" + id.String())
}

func (s *WebhookService) principal(ctx context.Context) (principal.Principal, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return principal.Principal{}, authzmodel.Unauthenticated()
	}
	return p, nil
}

func (s *WebhookService) authorize(ctx context.Context, p principal.Principal, action string, workspaceID uuid.UUID, id string) error {
	_, err := authzsvc.Require(ctx, s.az, action, authzmodel.Resource{
		Type: model.ResourceType, ID: id, OrgID: p.OrgID, WorkspaceID: &workspaceID,
	})
	return err
}

func (s *WebhookService) audit(ctx context.Context, tx *sqlx.Tx, p principal.Principal, action string,
	workspaceID uuid.UUID, resourceID uuid.UUID, details map[string]any) error {
	return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: action, ResourceType: model.ResourceType,
		ResourceID: resourceID.String(), WorkspaceID: &workspaceID, Details: details})
}

func validation(field, code, msg string) error {
	return problem.Validation(problem.FieldError{Field: field, Code: code, Message: msg})
}

func clampLimit(n int) int {
	switch {
	case n <= 0:
		return defaultListLimit
	case n > maxListLimit:
		return maxListLimit
	}
	return n
}

func checkName(name, desc string) (string, string, error) {
	name, desc = strings.TrimSpace(name), strings.TrimSpace(desc)
	if name == "" || len([]rune(name)) > 100 {
		return "", "", validation("name", "invalid_length", "Name must be 1–100 characters.")
	}
	if len([]rune(desc)) > 500 {
		return "", "", validation("description", "invalid_length", "Description must be at most 500 characters.")
	}
	return name, desc, nil
}

func (s *WebhookService) tx(ctx context.Context, p principal.Principal, fn func(*sqlx.Tx) error) error {
	return db.InTenantTx(ctx, s.pool, p.OrgID, fn)
}

// ---- endpoints ----

// loadEndpoint fetches an endpoint and authorizes action on it.
func (s *WebhookService) loadEndpoint(ctx context.Context, tx *sqlx.Tx, p principal.Principal, action string,
	id uuid.UUID) (model.Endpoint, error) {
	e, err := repository.GetEndpoint(ctx, tx, id)
	if db.IsNoRows(err) {
		return model.Endpoint{}, model.ErrEndpointNotFound
	}
	if err != nil {
		return model.Endpoint{}, err
	}
	return e, s.authorize(ctx, p, action, e.WorkspaceID, id.String())
}

// ListEndpoints lists a workspace's inbound endpoints.
func (s *WebhookService) ListEndpoints(ctx context.Context, workspaceID uuid.UUID) ([]model.Endpoint, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, p, "read", workspaceID, ""); err != nil {
		return nil, err
	}
	var out []model.Endpoint
	err = s.tx(ctx, p, func(tx *sqlx.Tx) error {
		out, err = repository.ListEndpoints(ctx, tx, workspaceID)
		return err
	})
	return out, err
}

// GetEndpoint returns one endpoint.
func (s *WebhookService) GetEndpoint(ctx context.Context, id uuid.UUID) (model.Endpoint, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Endpoint{}, err
	}
	var out model.Endpoint
	err = s.tx(ctx, p, func(tx *sqlx.Tx) error {
		out, err = s.loadEndpoint(ctx, tx, p, "read", id)
		return err
	})
	return out, err
}

func normalizeLimits(tolerance, rate int) (int, int, error) {
	if tolerance == 0 {
		tolerance = int(model.DefaultTolerance / time.Second)
	}
	if rate == 0 {
		rate = 600
	}
	if tolerance < 30 || tolerance > 3600 {
		return 0, 0, validation("tolerance_seconds", "out_of_range", "tolerance_seconds must be between 30 and 3600.")
	}
	if rate < 1 || rate > 10000 {
		return 0, 0, validation("rate_limit_per_minute", "out_of_range", "rate_limit_per_minute must be between 1 and 10000.")
	}
	return tolerance, rate, nil
}

// resolveSecret validates a supplied secret or generates one; generated
// reports whether the caller must be shown it.
func resolveSecret(scheme model.Scheme, supplied string) (secret string, generated bool, err error) {
	supplied = strings.TrimSpace(supplied)
	if supplied == "" {
		if !scheme.GeneratesSecret() {
			return "", false, validation("secret", "required",
				"Paste the signing secret issued by the provider for this scheme.")
		}
		secret, err = model.GenerateSecret(scheme)
		return secret, true, err
	}
	return supplied, false, model.ValidateSecret(scheme, supplied)
}

// CreateEndpoint creates an inbound endpoint.
func (s *WebhookService) CreateEndpoint(ctx context.Context, in dto.CreateEndpointInput) (model.Endpoint, string, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Endpoint{}, "", err
	}
	if err := s.authorize(ctx, p, "create", in.WorkspaceID, ""); err != nil {
		return model.Endpoint{}, "", err
	}
	name, desc, err := checkName(in.Name, in.Description)
	if err != nil {
		return model.Endpoint{}, "", err
	}
	if in.Verifier == "" {
		in.Verifier = model.SchemeStandardWebhooks
	}
	if !in.Verifier.Valid() {
		return model.Endpoint{}, "", validation("verifier", "invalid_verifier",
			"verifier must be standard_webhooks, hmac_sha256, stripe, github or slack.")
	}
	cfg := model.HMACConfig{}
	if in.Verifier == model.SchemeHMAC {
		if err := in.VerifierConfig.Validate(); err != nil {
			return model.Endpoint{}, "", err
		}
		cfg = in.VerifierConfig.Normalize()
	}
	tolerance, rate, err := normalizeLimits(in.ToleranceSeconds, in.RateLimitPerMinute)
	if err != nil {
		return model.Endpoint{}, "", err
	}
	secret, generated, err := resolveSecret(in.Verifier, in.Secret)
	if err != nil {
		return model.Endpoint{}, "", err
	}
	publicID, err := model.NewPublicID()
	if err != nil {
		return model.Endpoint{}, "", err
	}
	e := model.Endpoint{ID: s.cfg.IDs.New(), WorkspaceID: in.WorkspaceID, EnvironmentID: in.EnvironmentID,
		PublicID: publicID, Name: name, Description: desc, Verifier: in.Verifier, VerifierConfig: cfg,
		SecretHint: model.SecretHint(secret), ToleranceSeconds: tolerance, RateLimitPerMinute: rate, Enabled: true}
	sealed, err := s.sealer.SealWithAAD([]byte(secret), endpointAAD(p.OrgID, e.ID))
	if err != nil {
		return model.Endpoint{}, "", err
	}
	var out model.Endpoint
	err = s.tx(ctx, p, func(tx *sqlx.Tx) error {
		ok, err := repository.EnvironmentExists(ctx, tx, in.EnvironmentID)
		if err != nil {
			return err
		}
		if !ok {
			return validation("environment_id", "not_found", "The environment does not exist.")
		}
		if err := repository.InsertEndpoint(ctx, tx, repository.NewEndpoint{Endpoint: e, OrgID: p.OrgID,
			Sealed: sealed, CreatedBy: p.ID}); err != nil {
			if db.IsForeignKeyViolation(err) {
				return validation("workspace_id", "not_found", "The workspace does not exist.")
			}
			return err
		}
		if out, err = repository.GetEndpoint(ctx, tx, e.ID); err != nil {
			return err
		}
		return s.audit(ctx, tx, p, "webhook.created", e.WorkspaceID, e.ID, map[string]any{
			"name": e.Name, "verifier": e.Verifier, "environment_id": e.EnvironmentID})
	})
	if err != nil {
		return model.Endpoint{}, "", err
	}
	if !generated {
		secret = ""
	}
	return out, secret, nil
}

// UpdateEndpoint edits an endpoint.
func (s *WebhookService) UpdateEndpoint(ctx context.Context, id uuid.UUID, in dto.UpdateEndpointInput) (model.Endpoint, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Endpoint{}, err
	}
	var out model.Endpoint
	err = s.tx(ctx, p, func(tx *sqlx.Tx) error {
		e, err := s.loadEndpoint(ctx, tx, p, "update", id)
		if err != nil {
			return err
		}
		changed := []string{}
		name, desc := e.Name, e.Description
		if in.Name != nil {
			name, changed = *in.Name, append(changed, "name")
		}
		if in.Description != nil {
			desc, changed = *in.Description, append(changed, "description")
		}
		if e.Name, e.Description, err = checkName(name, desc); err != nil {
			return err
		}
		if in.VerifierConfig != nil {
			if e.Verifier != model.SchemeHMAC {
				return validation("verifier_config", "not_applicable", "Only hmac_sha256 endpoints take a verifier_config.")
			}
			if err := in.VerifierConfig.Validate(); err != nil {
				return err
			}
			e.VerifierConfig, changed = in.VerifierConfig.Normalize(), append(changed, "verifier_config")
		}
		tol, rate := e.ToleranceSeconds, e.RateLimitPerMinute
		if in.ToleranceSeconds != nil {
			tol, changed = *in.ToleranceSeconds, append(changed, "tolerance_seconds")
		}
		if in.RateLimitPerMinute != nil {
			rate, changed = *in.RateLimitPerMinute, append(changed, "rate_limit_per_minute")
		}
		if e.ToleranceSeconds, e.RateLimitPerMinute, err = normalizeLimits(tol, rate); err != nil {
			return err
		}
		if in.Enabled != nil {
			e.Enabled, changed = *in.Enabled, append(changed, "enabled")
		}
		if err := repository.UpdateEndpoint(ctx, tx, e); err != nil {
			return err
		}
		if out, err = repository.GetEndpoint(ctx, tx, id); err != nil {
			return err
		}
		return s.audit(ctx, tx, p, "webhook.updated", e.WorkspaceID, id, map[string]any{"changed": changed})
	})
	return out, err
}

// DeleteEndpoint removes an endpoint and its received events.
func (s *WebhookService) DeleteEndpoint(ctx context.Context, id uuid.UUID) error {
	p, err := s.principal(ctx)
	if err != nil {
		return err
	}
	return s.tx(ctx, p, func(tx *sqlx.Tx) error {
		e, err := s.loadEndpoint(ctx, tx, p, "delete", id)
		if err != nil {
			return err
		}
		if err := repository.DeleteEndpoint(ctx, tx, id); err != nil {
			return err
		}
		return s.audit(ctx, tx, p, "webhook.deleted", e.WorkspaceID, id, map[string]any{"name": e.Name})
	})
}

// RotateEndpointSecret replaces the signing secret. The old one stops
// verifying immediately.
func (s *WebhookService) RotateEndpointSecret(ctx context.Context, id uuid.UUID, in dto.RotateSecretInput) (model.Endpoint, string, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Endpoint{}, "", err
	}
	var out model.Endpoint
	var secret string
	var generated bool
	err = s.tx(ctx, p, func(tx *sqlx.Tx) error {
		e, err := s.loadEndpoint(ctx, tx, p, "update", id)
		if err != nil {
			return err
		}
		if secret, generated, err = resolveSecret(e.Verifier, in.Secret); err != nil {
			return err
		}
		sealed, err := s.sealer.SealWithAAD([]byte(secret), endpointAAD(p.OrgID, id))
		if err != nil {
			return err
		}
		if err := repository.RotateEndpointSecret(ctx, tx, id, sealed, model.SecretHint(secret)); err != nil {
			return err
		}
		if out, err = repository.GetEndpoint(ctx, tx, id); err != nil {
			return err
		}
		return s.audit(ctx, tx, p, "webhook.secret_rotated", e.WorkspaceID, id, nil)
	})
	if err != nil {
		return model.Endpoint{}, "", err
	}
	if !generated {
		secret = ""
	}
	return out, secret, nil
}

// ListEvents lists inbound events of a workspace, or of one endpoint.
func (s *WebhookService) ListEvents(ctx context.Context, f dto.EventFilter) ([]model.Event, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	var out []model.Event
	err = s.tx(ctx, p, func(tx *sqlx.Tx) error {
		if f.EndpointID != nil {
			e, err := s.loadEndpoint(ctx, tx, p, "read", *f.EndpointID)
			if err != nil {
				return err
			}
			f.WorkspaceID = e.WorkspaceID
		} else {
			if f.WorkspaceID == uuid.Nil {
				return validation("workspace_id", "required", "Pass workspace_id or endpoint_id.")
			}
			if err := s.authorize(ctx, p, "read", f.WorkspaceID, ""); err != nil {
				return err
			}
		}
		out, err = repository.ListEvents(ctx, tx, f.WorkspaceID, f.EndpointID, clampLimit(f.Limit))
		return err
	})
	return out, err
}

// GetEvent returns one inbound event with its headers and body.
func (s *WebhookService) GetEvent(ctx context.Context, id uuid.UUID) (model.Event, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Event{}, err
	}
	var out model.Event
	err = s.tx(ctx, p, func(tx *sqlx.Tx) error {
		out, err = repository.GetEvent(ctx, tx, id)
		if db.IsNoRows(err) {
			return model.ErrEventNotFound
		}
		if err != nil {
			return err
		}
		return s.authorize(ctx, p, "read", out.WorkspaceID, out.EndpointID.String())
	})
	return out, err
}

// ---- subscriptions ----

// EventTypes returns the subscribable event catalog.
func (s *WebhookService) EventTypes(ctx context.Context) ([]model.EventType, error) {
	if _, err := s.principal(ctx); err != nil {
		return nil, err
	}
	return model.Catalog(), nil
}

func (s *WebhookService) loadSubscription(ctx context.Context, tx *sqlx.Tx, p principal.Principal, action string,
	id uuid.UUID) (model.Subscription, error) {
	sub, err := repository.GetSubscription(ctx, tx, id)
	if db.IsNoRows(err) {
		return model.Subscription{}, model.ErrSubscriptionNotFound
	}
	if err != nil {
		return model.Subscription{}, err
	}
	return sub, s.authorize(ctx, p, action, sub.WorkspaceID, id.String())
}

func checkEventTypes(types []string) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(types))
	for _, t := range types {
		t = strings.TrimSpace(t)
		if !model.KnownEventType(t) {
			return nil, validation("event_types", "unknown_event_type", "Unknown event type "+t+"; see GET /event-types.")
		}
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil, validation("event_types", "required", "Subscribe to at least one event type.")
	}
	return out, nil
}

// ListSubscriptions lists a workspace's outbound subscriptions.
func (s *WebhookService) ListSubscriptions(ctx context.Context, workspaceID uuid.UUID) ([]model.Subscription, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, p, "read", workspaceID, ""); err != nil {
		return nil, err
	}
	var out []model.Subscription
	err = s.tx(ctx, p, func(tx *sqlx.Tx) error {
		out, err = repository.ListSubscriptions(ctx, tx, workspaceID)
		return err
	})
	return out, err
}

// GetSubscription returns one subscription.
func (s *WebhookService) GetSubscription(ctx context.Context, id uuid.UUID) (model.Subscription, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Subscription{}, err
	}
	var out model.Subscription
	err = s.tx(ctx, p, func(tx *sqlx.Tx) error {
		out, err = s.loadSubscription(ctx, tx, p, "read", id)
		return err
	})
	return out, err
}

// CreateSubscription creates an outbound subscription with a fresh secret.
func (s *WebhookService) CreateSubscription(ctx context.Context, in dto.CreateSubscriptionInput) (model.Subscription, string, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Subscription{}, "", err
	}
	if err := s.authorize(ctx, p, "create", in.WorkspaceID, ""); err != nil {
		return model.Subscription{}, "", err
	}
	name, _, err := checkName(in.Name, "")
	if err != nil {
		return model.Subscription{}, "", err
	}
	in.URL = strings.TrimSpace(in.URL)
	if err := model.ValidateURL(in.URL); err != nil {
		return model.Subscription{}, "", err
	}
	types, err := checkEventTypes(in.EventTypes)
	if err != nil {
		return model.Subscription{}, "", err
	}
	secret, err := model.GenerateSecret(model.SchemeStandardWebhooks)
	if err != nil {
		return model.Subscription{}, "", err
	}
	sub := model.Subscription{ID: s.cfg.IDs.New(), WorkspaceID: in.WorkspaceID, EnvironmentID: in.EnvironmentID,
		Name: name, URL: in.URL, EventTypes: types, SecretHint: model.SecretHint(secret), Enabled: true}
	sealed, err := s.sealer.SealWithAAD([]byte(secret), subscriptionAAD(p.OrgID, sub.ID))
	if err != nil {
		return model.Subscription{}, "", err
	}
	var out model.Subscription
	err = s.tx(ctx, p, func(tx *sqlx.Tx) error {
		ok, err := repository.EnvironmentExists(ctx, tx, in.EnvironmentID)
		if err != nil {
			return err
		}
		if !ok {
			return validation("environment_id", "not_found", "The environment does not exist.")
		}
		if err := repository.InsertSubscription(ctx, tx, p.OrgID, sub, sealed, p.ID); err != nil {
			if db.IsForeignKeyViolation(err) {
				return validation("workspace_id", "not_found", "The workspace does not exist.")
			}
			return err
		}
		if out, err = repository.GetSubscription(ctx, tx, sub.ID); err != nil {
			return err
		}
		return s.audit(ctx, tx, p, "webhook.subscription_created", sub.WorkspaceID, sub.ID, map[string]any{
			"name": sub.Name, "url": sub.URL, "event_types": sub.EventTypes})
	})
	if err != nil {
		return model.Subscription{}, "", err
	}
	return out, secret, nil
}

// UpdateSubscription edits a subscription. Re-enabling an automatically
// disabled subscription clears its failure streak.
func (s *WebhookService) UpdateSubscription(ctx context.Context, id uuid.UUID, in dto.UpdateSubscriptionInput) (model.Subscription, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Subscription{}, err
	}
	var out model.Subscription
	err = s.tx(ctx, p, func(tx *sqlx.Tx) error {
		sub, err := s.loadSubscription(ctx, tx, p, "update", id)
		if err != nil {
			return err
		}
		changed := []string{}
		if in.Name != nil {
			if sub.Name, _, err = checkName(*in.Name, ""); err != nil {
				return err
			}
			changed = append(changed, "name")
		}
		if in.URL != nil {
			u := strings.TrimSpace(*in.URL)
			if err := model.ValidateURL(u); err != nil {
				return err
			}
			sub.URL, changed = u, append(changed, "url")
		}
		if in.EventTypes != nil {
			if sub.EventTypes, err = checkEventTypes(in.EventTypes); err != nil {
				return err
			}
			changed = append(changed, "event_types")
		}
		if in.Enabled != nil {
			sub.Enabled, changed = *in.Enabled, append(changed, "enabled")
		}
		if err := repository.UpdateSubscription(ctx, tx, sub); err != nil {
			return err
		}
		if out, err = repository.GetSubscription(ctx, tx, id); err != nil {
			return err
		}
		return s.audit(ctx, tx, p, "webhook.subscription_updated", sub.WorkspaceID, id, map[string]any{"changed": changed})
	})
	return out, err
}

// DeleteSubscription removes a subscription and its delivery log.
func (s *WebhookService) DeleteSubscription(ctx context.Context, id uuid.UUID) error {
	p, err := s.principal(ctx)
	if err != nil {
		return err
	}
	return s.tx(ctx, p, func(tx *sqlx.Tx) error {
		sub, err := s.loadSubscription(ctx, tx, p, "delete", id)
		if err != nil {
			return err
		}
		if err := repository.DeleteSubscription(ctx, tx, id); err != nil {
			return err
		}
		return s.audit(ctx, tx, p, "webhook.subscription_deleted", sub.WorkspaceID, id, map[string]any{"name": sub.Name})
	})
}

// RotateSubscriptionSecret replaces the signing secret; pending retries are
// signed with the new one.
func (s *WebhookService) RotateSubscriptionSecret(ctx context.Context, id uuid.UUID) (model.Subscription, string, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Subscription{}, "", err
	}
	secret, err := model.GenerateSecret(model.SchemeStandardWebhooks)
	if err != nil {
		return model.Subscription{}, "", err
	}
	var out model.Subscription
	err = s.tx(ctx, p, func(tx *sqlx.Tx) error {
		sub, err := s.loadSubscription(ctx, tx, p, "update", id)
		if err != nil {
			return err
		}
		sealed, err := s.sealer.SealWithAAD([]byte(secret), subscriptionAAD(p.OrgID, id))
		if err != nil {
			return err
		}
		if err := repository.RotateSubscriptionSecret(ctx, tx, id, sealed, model.SecretHint(secret)); err != nil {
			return err
		}
		if out, err = repository.GetSubscription(ctx, tx, id); err != nil {
			return err
		}
		return s.audit(ctx, tx, p, "webhook.subscription_secret_rotated", sub.WorkspaceID, id, nil)
	})
	if err != nil {
		return model.Subscription{}, "", err
	}
	return out, secret, nil
}

// PingSubscription enqueues a test delivery.
func (s *WebhookService) PingSubscription(ctx context.Context, id uuid.UUID) (model.Delivery, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Delivery{}, err
	}
	var out model.Delivery
	err = s.tx(ctx, p, func(tx *sqlx.Tx) error {
		sub, err := s.loadSubscription(ctx, tx, p, "update", id)
		if err != nil {
			return err
		}
		if !sub.Enabled {
			return model.ErrSubscriptionDisabled
		}
		deliveryID := s.cfg.IDs.New()
		payload, err := json.Marshal(map[string]any{"subscription_id": sub.ID, "workspace_id": sub.WorkspaceID,
			"message": "Test delivery from Hugimethara."})
		if err != nil {
			return err
		}
		if _, err := repository.InsertDelivery(ctx, tx, repository.NewDelivery{ID: deliveryID, OrgID: p.OrgID,
			WorkspaceID: sub.WorkspaceID, SubscriptionID: sub.ID, EventID: "ping-" + deliveryID.String(),
			EventType: model.PingEvent, Payload: payload}); err != nil {
			return err
		}
		out, err = repository.GetDelivery(ctx, tx, deliveryID)
		return err
	})
	return out, err
}

// ListDeliveries lists the delivery log of a workspace or subscription.
func (s *WebhookService) ListDeliveries(ctx context.Context, f dto.DeliveryFilter) ([]model.Delivery, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	switch model.DeliveryStatus(f.Status) {
	case "", model.DeliveryPending, model.DeliverySucceeded, model.DeliveryDeadLettered:
	default:
		return nil, validation("status", "invalid_status", "status must be pending, succeeded or dead_lettered.")
	}
	var out []model.Delivery
	err = s.tx(ctx, p, func(tx *sqlx.Tx) error {
		if f.SubscriptionID != nil {
			sub, err := s.loadSubscription(ctx, tx, p, "read", *f.SubscriptionID)
			if err != nil {
				return err
			}
			f.WorkspaceID = sub.WorkspaceID
		} else {
			if f.WorkspaceID == uuid.Nil {
				return validation("workspace_id", "required", "Pass workspace_id or subscription_id.")
			}
			if err := s.authorize(ctx, p, "read", f.WorkspaceID, ""); err != nil {
				return err
			}
		}
		out, err = repository.ListDeliveries(ctx, tx, f.WorkspaceID, f.SubscriptionID, f.Status, clampLimit(f.Limit))
		return err
	})
	return out, err
}

func (s *WebhookService) loadDelivery(ctx context.Context, tx *sqlx.Tx, p principal.Principal, action string,
	id uuid.UUID) (model.Delivery, error) {
	d, err := repository.GetDelivery(ctx, tx, id)
	if db.IsNoRows(err) {
		return model.Delivery{}, model.ErrDeliveryNotFound
	}
	if err != nil {
		return model.Delivery{}, err
	}
	return d, s.authorize(ctx, p, action, d.WorkspaceID, d.SubscriptionID.String())
}

// GetDelivery returns a delivery with its payload and every attempt.
func (s *WebhookService) GetDelivery(ctx context.Context, id uuid.UUID) (model.Delivery, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Delivery{}, err
	}
	var out model.Delivery
	err = s.tx(ctx, p, func(tx *sqlx.Tx) error {
		out, err = s.loadDelivery(ctx, tx, p, "read", id)
		return err
	})
	return out, err
}

// ReplayDelivery re-sends a finished delivery's event as a new delivery.
func (s *WebhookService) ReplayDelivery(ctx context.Context, id uuid.UUID) (model.Delivery, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Delivery{}, err
	}
	var out model.Delivery
	err = s.tx(ctx, p, func(tx *sqlx.Tx) error {
		d, err := s.loadDelivery(ctx, tx, p, "replay", id)
		if err != nil {
			return err
		}
		if d.Status == model.DeliveryPending {
			return model.ErrDeliveryInFlight
		}
		sub, err := repository.GetSubscription(ctx, tx, d.SubscriptionID)
		if err != nil {
			return err
		}
		if !sub.Enabled {
			return model.ErrSubscriptionDisabled
		}
		replayID := s.cfg.IDs.New()
		if _, err := repository.InsertDelivery(ctx, tx, repository.NewDelivery{ID: replayID, OrgID: p.OrgID,
			WorkspaceID: d.WorkspaceID, SubscriptionID: d.SubscriptionID, EventID: d.EventID, EventType: d.EventType,
			Payload: d.Payload, ReplayOf: &d.ID}); err != nil {
			return err
		}
		if err := s.audit(ctx, tx, p, "webhook.delivery_replayed", d.WorkspaceID, replayID, map[string]any{
			"replay_of": d.ID, "subscription_id": d.SubscriptionID, "event_type": d.EventType}); err != nil {
			return err
		}
		out, err = repository.GetDelivery(ctx, tx, replayID)
		return err
	})
	return out, err
}
