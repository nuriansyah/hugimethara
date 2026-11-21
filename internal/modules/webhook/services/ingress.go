package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/infrastructure/persistence/outbox"
	"github.com/nuriansyah/hugimethara/internal/infrastructure/security/ratelimit"
	"github.com/nuriansyah/hugimethara/internal/modules/webhook/model"
	"github.com/nuriansyah/hugimethara/internal/modules/webhook/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/webhook/repository"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	logging "github.com/nuriansyah/hugimethara/internal/platform/logger"
)

// MaxEventPayload is the largest body forwarded inline in the
// webhook.received event (and so to workflow triggers and subscribers).
// Larger bodies are stored and flagged payload_truncated.
const MaxEventPayload = 64 << 10

// Headers whose values are credentials or signatures: stored as "[redacted]".
var redactedHeaders = map[string]bool{
	"Authorization": true, "Cookie": true, "Proxy-Authorization": true, "Webhook-Signature": true,
	"Stripe-Signature": true, "X-Hub-Signature": true, "X-Hub-Signature-256": true, "X-Slack-Signature": true,
	"X-Hugi-Token": true,
}

func storedHeaders(h http.Header, extraSecret string) map[string]string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := map[string]string{}
	for _, k := range keys {
		if len(out) >= model.MaxStoredHeaders {
			break
		}
		ck := http.CanonicalHeaderKey(k)
		v := strings.Join(h.Values(k), ", ")
		if redactedHeaders[ck] || (extraSecret != "" && ck == extraSecret) {
			v = "[redacted]"
		}
		if len(v) > 1024 {
			v = v[:1024]
		}
		out[ck] = strings.ToValidUTF8(v, "")
	}
	return out
}

// Receive verifies and records an inbound webhook. Unknown and disabled
// endpoints answer 404 alike; a failed verification is counted on the
// endpoint and answered 401 without storing anything; a repeated provider
// event id is acknowledged as a duplicate and emits nothing.
func (s *WebhookService) Receive(ctx context.Context, in dto.IngressRequest) (dto.IngressResult, error) {
	if len(in.Body) > model.MaxBodyBytes {
		s.ingress.WithLabelValues("too_large").Inc()
		return dto.IngressResult{}, model.ErrBodyTooLarge
	}
	target, err := repository.LookupIngress(ctx, s.pool, in.PublicID)
	if db.IsNoRows(err) || (err == nil && !target.Enabled) {
		s.ingress.WithLabelValues("not_found").Inc()
		return dto.IngressResult{}, model.ErrIngressNotFound
	}
	if err != nil {
		return dto.IngressResult{}, err
	}
	if s.cfg.Limiter != nil {
		rule := ratelimit.Rule{Name: "webhook_ingress", Max: int64(target.RateLimitPerMinute), Window: time.Minute}
		if err := s.cfg.Limiter.Allow(ctx, rule, target.EndpointID.String()); err != nil {
			s.ingress.WithLabelValues("rate_limited").Inc()
			return dto.IngressResult{}, err
		}
	}

	var result dto.IngressResult
	var rejected *model.Rejection
	err = db.InTenantTx(ctx, s.pool, target.OrgID, func(tx *sqlx.Tx) error {
		e, err := repository.GetEndpoint(ctx, tx, target.EndpointID)
		if err != nil {
			return err
		}
		sealed, err := repository.EndpointSecret(ctx, tx, e.ID)
		if err != nil {
			return err
		}
		secret, err := s.sealer.OpenWithAAD(sealed, endpointAAD(target.OrgID, e.ID))
		if err != nil {
			return err
		}
		defer clear(secret)
		v, err := model.Verify(e.Verifier, e.VerifierConfig, secret, model.VerifyRequest{Header: in.Header,
			Body: in.Body, Now: s.cfg.Clock.Now(), Tolerance: e.Tolerance()})
		if errors.As(err, &rejected) {
			// Committed on purpose: the rejection is counted, nothing else stored.
			return repository.RecordRejected(ctx, tx, e.ID, rejected.Reason)
		}
		if err != nil {
			return err
		}
		var body map[string]any
		isJSON := json.Unmarshal(in.Body, &body) == nil && body != nil
		if !isJSON {
			var anyJSON any
			isJSON = json.Unmarshal(in.Body, &anyJSON) == nil
		}
		if e.Verifier == model.SchemeSlack && body["type"] == "url_verification" {
			result.Challenge, _ = body["challenge"].(string)
			return nil
		}
		ev := model.Event{ID: s.cfg.IDs.New(), WorkspaceID: e.WorkspaceID, EndpointID: e.ID,
			ProviderEventID: truncate(v.EventID, 200), EventType: truncate(v.EventType, 200),
			ContentType: truncate(in.ContentType, 200), Headers: storedHeaders(in.Header, e.VerifierConfig.SignatureHeader),
			PayloadSize: len(in.Body), SourceIP: in.SourceIP}
		if isJSON {
			ev.Payload = in.Body
		} else if utf8.Valid(in.Body) {
			ev.RawBody = string(in.Body)
		}
		inserted, err := repository.InsertEvent(ctx, tx, repository.NewEvent{Event: ev, OrgID: target.OrgID})
		if err != nil {
			return err
		}
		if !inserted {
			result.Duplicate = true
			return nil
		}
		result.EventID = ev.ID
		if err := repository.RecordReceived(ctx, tx, e.ID); err != nil {
			return err
		}
		payload := map[string]any{"event_id": ev.ID, "endpoint_id": e.ID, "endpoint_name": e.Name,
			"workspace_id": e.WorkspaceID, "environment_id": e.EnvironmentID, "provider_event_id": ev.ProviderEventID,
			"event_type": ev.EventType, "content_type": ev.ContentType, "received_at": s.cfg.Clock.Now(),
			"request_id": logging.RequestID(ctx)}
		switch {
		case len(in.Body) > MaxEventPayload:
			payload["payload_truncated"] = true
		case isJSON:
			payload["payload"] = json.RawMessage(in.Body)
		default:
			payload["raw_body"] = ev.RawBody
		}
		_, err = outbox.Publish(ctx, tx, outbox.Event{OrgID: target.OrgID,
			Subject: "hugi.events." + model.ReceivedEvent, Payload: payload})
		return err
	})
	switch {
	case err != nil:
		return dto.IngressResult{}, err
	case rejected != nil:
		s.ingress.WithLabelValues("rejected").Inc()
		return dto.IngressResult{}, model.RejectedProblem(rejected.Reason)
	case result.Duplicate:
		s.ingress.WithLabelValues("duplicate").Inc()
	default:
		s.ingress.WithLabelValues("accepted").Inc()
	}
	return result, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.ToValidUTF8(s[:n], "")
}
