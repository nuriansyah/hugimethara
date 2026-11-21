// Package model holds the webhook engine's entities, signature verifiers,
// Standard Webhooks signing, the retry schedule and the event catalog. It
// performs no I/O.
package model

import (
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
)

// ResourceType is the authorization resource owned by this module.
const ResourceType = "webhook"

// Limits.
const (
	MaxBodyBytes     = 1 << 20 // inbound request body cap
	DefaultTolerance = 5 * time.Minute
	MaxExcerptBytes  = 2048 // stored response excerpt per attempt
	MaxStoredHeaders = 50
)

// ReceivedEvent is the platform event emitted for every verified inbound
// webhook (subject hugi.events.webhook.received).
const ReceivedEvent = "webhook.received"

// PingEvent is the type of a test delivery sent from the UI or API.
const PingEvent = "webhook.ping"

var headerNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,99}$`)

func invalidField(field, code, msg string) error {
	return problem.Validation(problem.FieldError{Field: field, Code: code, Message: msg})
}

// Endpoint is an inbound webhook endpoint.
type Endpoint struct {
	ID                 uuid.UUID
	WorkspaceID        uuid.UUID
	EnvironmentID      uuid.UUID
	PublicID           string
	Name               string
	Description        string
	Verifier           Scheme
	VerifierConfig     HMACConfig
	SecretHint         string
	ToleranceSeconds   int
	RateLimitPerMinute int
	Enabled            bool
	ReceivedCount      int64
	RejectedCount      int64
	LastReceivedAt     *time.Time
	LastRejectedAt     *time.Time
	LastRejectedReason string
	SecretRotatedAt    *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// IngressPath is the path senders POST to.
func (e Endpoint) IngressPath() string { return "/hooks/" + e.PublicID }

// Tolerance is the accepted clock skew for timestamped schemes.
func (e Endpoint) Tolerance() time.Duration {
	if e.ToleranceSeconds <= 0 {
		return DefaultTolerance
	}
	return time.Duration(e.ToleranceSeconds) * time.Second
}

// Event is a verified inbound webhook request.
type Event struct {
	ID              uuid.UUID
	WorkspaceID     uuid.UUID
	EndpointID      uuid.UUID
	ProviderEventID string
	EventType       string
	ContentType     string
	Headers         map[string]string
	Payload         json.RawMessage // JSON bodies
	RawBody         string          // non-JSON bodies
	PayloadSize     int
	SourceIP        string
	ReceivedAt      time.Time
}

// Subscription is an outbound subscription to platform events.
type Subscription struct {
	ID             uuid.UUID
	WorkspaceID    uuid.UUID
	EnvironmentID  uuid.UUID
	Name           string
	URL            string
	EventTypes     []string
	SecretHint     string
	Enabled        bool
	DisabledReason string
	FailingSince   *time.Time
	LastSuccessAt  *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// DeliveryStatus is the lifecycle state of a delivery.
type DeliveryStatus string

const (
	DeliveryPending      DeliveryStatus = "pending"
	DeliverySucceeded    DeliveryStatus = "succeeded"
	DeliveryDeadLettered DeliveryStatus = "dead_lettered"
)

// Delivery is one event sent (or to be sent) to one subscription.
type Delivery struct {
	ID               uuid.UUID
	WorkspaceID      uuid.UUID
	SubscriptionID   uuid.UUID
	SubscriptionName string
	EventID          string
	EventType        string
	Payload          json.RawMessage
	ReplayOf         *uuid.UUID
	Status           DeliveryStatus
	Attempts         int
	NextAttemptAt    *time.Time
	LastAttemptAt    *time.Time
	ResponseStatus   *int
	ResponseExcerpt  string
	LatencyMS        *int
	Error            string
	CreatedAt        time.Time
	FinishedAt       *time.Time
	AttemptLog       []Attempt
}

// Attempt is one recorded delivery attempt.
type Attempt struct {
	Attempt         int
	ResponseStatus  *int
	ResponseExcerpt string
	LatencyMS       int
	Error           string
	AttemptedAt     time.Time
}

// RetrySchedule is the wait before each retry: 8 attempts over ≈ 21 hours.
var RetrySchedule = []time.Duration{
	30 * time.Second, 2 * time.Minute, 10 * time.Minute, 30 * time.Minute,
	2 * time.Hour, 6 * time.Hour, 12 * time.Hour,
}

// MaxAttempts is the number of attempts before a delivery is dead-lettered.
var MaxAttempts = len(RetrySchedule) + 1

// NextRetry returns the wait after the attempt-th attempt failed, or false
// when the delivery must be dead-lettered.
func NextRetry(attempt int) (time.Duration, bool) {
	if attempt < 1 || attempt > len(RetrySchedule) {
		return 0, false
	}
	return RetrySchedule[attempt-1], true
}

// AutoDisableAfter disables a subscription whose deliveries have failed
// continuously for this long.
const AutoDisableAfter = 5 * 24 * time.Hour

// DisabledFailing is the disabled_reason of an automatically disabled subscription.
const DisabledFailing = "failing_for_5_days"

// ValidateURL checks an outbound destination. The SSRF guard still applies at
// dial time; this only rejects what can never work.
func ValidateURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	switch {
	case err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "":
		return invalidField("url", "invalid_url", "Must be an absolute http(s) URL.")
	case u.User != nil:
		return invalidField("url", "invalid_url", "Credentials in the URL are not allowed; the payload is signed instead.")
	case len(raw) > 2000:
		return invalidField("url", "too_long", "The URL must be at most 2000 characters.")
	}
	return nil
}

// Domain errors.
var (
	ErrEndpointNotFound     = problem.NewProblem(http.StatusNotFound, "webhook_not_found", "Webhook not found", "No such webhook endpoint.")
	ErrEventNotFound        = problem.NewProblem(http.StatusNotFound, "webhook_event_not_found", "Event not found", "No such webhook event.")
	ErrSubscriptionNotFound = problem.NewProblem(http.StatusNotFound, "webhook_subscription_not_found",
		"Subscription not found", "No such webhook subscription.")
	ErrDeliveryNotFound = problem.NewProblem(http.StatusNotFound, "webhook_delivery_not_found",
		"Delivery not found", "No such webhook delivery.")
	ErrDeliveryInFlight = problem.NewProblem(http.StatusConflict, "delivery_in_progress",
		"Delivery still in progress", "Only a succeeded or dead-lettered delivery can be replayed.")
	ErrSubscriptionDisabled = problem.NewProblem(http.StatusConflict, "subscription_disabled",
		"Subscription disabled", "Enable the subscription before sending to it.")
	// ErrIngressNotFound is returned for unknown or disabled endpoints alike,
	// so the ingress is not an oracle for which ids exist.
	ErrIngressNotFound = problem.NewProblem(http.StatusNotFound, "webhook_not_found", "Not found", "No such webhook endpoint.")
	ErrBodyTooLarge    = problem.NewProblem(http.StatusRequestEntityTooLarge, "payload_too_large",
		"Payload too large", "Webhook bodies are limited to 1 MiB.")
)

// RejectedProblem renders a verification failure for the sender.
func RejectedProblem(reason string) *problem.Problem {
	return problem.NewProblem(http.StatusUnauthorized, reason, "Webhook rejected", "The request signature could not be verified.")
}
