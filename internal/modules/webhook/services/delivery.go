package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/nuriansyah/hugimethara/internal/infrastructure/security/egress"
	"github.com/nuriansyah/hugimethara/internal/modules/webhook/model"
	"github.com/nuriansyah/hugimethara/internal/modules/webhook/repository"
	"github.com/nuriansyah/hugimethara/internal/platform/clock"
	secretbox "github.com/nuriansyah/hugimethara/internal/platform/crypto"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	"github.com/nuriansyah/hugimethara/internal/platform/id"
)

// DispatcherConfig tunes outbound delivery.
type DispatcherConfig struct {
	Interval    time.Duration // poll interval for due deliveries; default 2s
	Batch       int           // deliveries claimed per pass; default 50
	Concurrency int           // parallel attempts per pass; default 8
	Timeout     time.Duration // per-attempt HTTP timeout; default 15s
	Clock       clock.Clock
	IDs         id.Generator
	Log         *slog.Logger
	Registerer  prometheus.Registerer
}

// Dispatcher fans platform events out to subscriptions and delivers them. It
// connects as hugi_system (cross-tenant claim) and runs in hugi-worker.
type Dispatcher struct {
	pool      *sqlx.DB
	sealer    *secretbox.Key
	cfg       DispatcherConfig
	attempts  *prometheus.CounterVec
	latency   prometheus.Histogram
	fanned    prometheus.Counter
	userAgent string
}

// NewDispatcher builds a dispatcher. pool must connect as hugi_system.
func NewDispatcher(pool *sqlx.DB, sealer *secretbox.Key, cfg DispatcherConfig) *Dispatcher {
	if cfg.Interval <= 0 {
		cfg.Interval = 2 * time.Second
	}
	if cfg.Batch <= 0 {
		cfg.Batch = 50
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 8
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Second
	}
	if cfg.Clock == nil {
		cfg.Clock = clock.System{}
	}
	if cfg.IDs == nil {
		cfg.IDs = id.V7{}
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	d := &Dispatcher{pool: pool, sealer: sealer, cfg: cfg, userAgent: "Hugimethara-Webhooks/1",
		attempts: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "hugi_webhook_delivery_attempts_total",
			Help: "Outbound webhook delivery attempts, by result."}, []string{"result"}),
		latency: prometheus.NewHistogram(prometheus.HistogramOpts{Name: "hugi_webhook_delivery_seconds",
			Help: "Outbound webhook attempt latency.", Buckets: prometheus.DefBuckets}),
		fanned: prometheus.NewCounter(prometheus.CounterOpts{Name: "hugi_webhook_deliveries_enqueued_total",
			Help: "Deliveries enqueued by event fan-out."}),
	}
	if cfg.Registerer != nil {
		cfg.Registerer.MustRegister(d.attempts, d.latency, d.fanned)
	}
	return d
}

// PlatformEvent is an event from the HUGI_EVENTS stream.
type PlatformEvent struct {
	OrgID uuid.UUID
	Name  string // subject without "hugi.events."
	ID    string // stable message id (outbox id)
	Data  []byte
}

// HandleEvent enqueues one delivery per enabled subscription of the event's
// workspace. Idempotent: a redelivered event enqueues nothing new. Events
// without a workspace_id (organization-level) are never delivered.
func (d *Dispatcher) HandleEvent(ctx context.Context, ev PlatformEvent) error {
	if !model.KnownEventType(ev.Name) || ev.ID == "" {
		return nil
	}
	var data map[string]any
	if err := json.Unmarshal(ev.Data, &data); err != nil || data == nil {
		return nil
	}
	ws, _ := data["workspace_id"].(string)
	workspaceID, err := uuid.Parse(ws)
	if err != nil {
		return nil
	}
	return db.InTenantTx(ctx, d.pool, ev.OrgID, func(tx *sqlx.Tx) error {
		subs, err := repository.MatchingSubscriptions(ctx, tx, ev.OrgID, workspaceID, ev.Name)
		if err != nil {
			return err
		}
		for _, s := range subs {
			inserted, err := repository.InsertDelivery(ctx, tx, repository.NewDelivery{ID: d.cfg.IDs.New(),
				OrgID: ev.OrgID, WorkspaceID: s.WorkspaceID, SubscriptionID: s.ID, EventID: ev.ID,
				EventType: ev.Name, Payload: ev.Data})
			if err != nil {
				return err
			}
			if inserted {
				d.fanned.Inc()
			}
		}
		return nil
	})
}

// Run delivers due deliveries until ctx is cancelled.
func (d *Dispatcher) Run(ctx context.Context) error {
	t := time.NewTicker(d.cfg.Interval)
	defer t.Stop()
	for {
		n, err := d.DeliverDue(ctx)
		if err != nil && ctx.Err() == nil {
			d.cfg.Log.ErrorContext(ctx, "webhook delivery pass failed", "err", err)
		}
		if n == d.cfg.Batch && err == nil {
			continue // more work likely waiting
		}
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
	}
}

// DeliverDue claims due deliveries and attempts each once. It returns how
// many were claimed.
func (d *Dispatcher) DeliverDue(ctx context.Context) (int, error) {
	var claimed []repository.ClaimedDelivery
	// The lease outlasts the attempt, so a live worker never loses its claim.
	lease := d.cfg.Timeout + 30*time.Second
	err := db.InTx(ctx, d.pool, func(tx *sqlx.Tx) error {
		var err error
		claimed, err = repository.ClaimDueDeliveries(ctx, tx, d.cfg.Batch, lease)
		return err
	})
	if err != nil {
		return 0, err
	}
	sem := make(chan struct{}, d.cfg.Concurrency)
	var wg sync.WaitGroup
	for _, c := range claimed {
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer func() { <-sem; wg.Done() }()
			if err := d.attempt(ctx, c); err != nil && ctx.Err() == nil {
				d.cfg.Log.ErrorContext(ctx, "webhook delivery attempt not recorded", "delivery_id", c.ID, "err", err)
			}
		}()
	}
	wg.Wait()
	return len(claimed), nil
}

// outcome is the classified result of one HTTP attempt.
type outcome struct {
	status    *int
	excerpt   string
	latency   time.Duration
	err       string
	retryable bool
}

func (o outcome) ok() bool { return o.status != nil && *o.status >= 200 && *o.status < 300 }

// Body is the JSON envelope sent to subscribers (Standard Webhooks style).
type Body struct {
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

func (d *Dispatcher) attempt(ctx context.Context, c repository.ClaimedDelivery) error {
	target, err := repository.DeliveryTarget(ctx, d.pool, c.SubscriptionID)
	if err != nil {
		return err
	}
	var o outcome
	if !target.Enabled {
		o = outcome{err: "subscription_disabled"}
	} else {
		o = d.send(ctx, c, target)
	}
	if ctx.Err() != nil {
		return ctx.Err() // shutting down: the lease expires and another worker retries
	}
	now := d.cfg.Clock.Now()
	res := repository.AttemptResult{ID: d.cfg.IDs.New(), OrgID: c.OrgID, DeliveryID: c.ID, Attempt: c.Attempt,
		ResponseStatus: o.status, ResponseExcerpt: o.excerpt, LatencyMS: int(o.latency.Milliseconds()), Error: o.err}
	result := "succeeded"
	switch {
	case o.ok():
		res.Status = model.DeliverySucceeded
	case !o.retryable:
		res.Status, result = model.DeliveryDeadLettered, "dead_lettered"
	default:
		if wait, more := model.NextRetry(c.Attempt); more {
			next := now.Add(wait)
			res.Status, res.NextAttemptAt, result = model.DeliveryPending, &next, "retry_scheduled"
		} else {
			res.Status, result = model.DeliveryDeadLettered, "dead_lettered"
		}
	}
	d.attempts.WithLabelValues(result).Inc()
	if o.latency > 0 {
		d.latency.Observe(o.latency.Seconds())
	}
	return db.InTenantTx(ctx, d.pool, c.OrgID, func(tx *sqlx.Tx) error {
		held, err := repository.RecordAttempt(ctx, tx, res)
		if err != nil || !held {
			return err // a lost claim: the new holder records its own attempt
		}
		if !target.Enabled {
			return nil
		}
		if o.ok() {
			return repository.SubscriptionSucceeded(ctx, tx, c.SubscriptionID)
		}
		since, err := repository.SubscriptionFailed(ctx, tx, c.SubscriptionID)
		if err != nil {
			return err
		}
		if now.Sub(since) >= model.AutoDisableAfter {
			disabled, err := repository.DisableSubscription(ctx, tx, c.SubscriptionID, model.DisabledFailing)
			if err != nil {
				return err
			}
			if disabled {
				d.cfg.Log.WarnContext(ctx, "webhook subscription disabled after failing continuously",
					"subscription_id", c.SubscriptionID, "failing_since", since)
			}
		}
		return nil
	})
}

func (d *Dispatcher) send(ctx context.Context, c repository.ClaimedDelivery, t repository.Target) outcome {
	secret, err := d.sealer.OpenWithAAD(t.Sealed, subscriptionAAD(t.OrgID, t.ID))
	if err != nil {
		return outcome{err: "secret_unavailable"}
	}
	defer clear(secret)
	data := c.Payload
	if len(data) == 0 {
		data = json.RawMessage(`{}`)
	}
	body, err := json.Marshal(Body{Type: c.EventType, Timestamp: c.CreatedAt.UTC(), Data: data})
	if err != nil {
		return outcome{err: "payload_invalid"}
	}
	headers, err := model.SignedHeaders(secret, "msg_"+c.ID.String(), d.cfg.Clock.Now(), body)
	if err != nil {
		return outcome{err: "secret_unavailable"}
	}
	policy, err := egress.ParsePolicy(t.Allowlist)
	if err != nil {
		return outcome{err: "egress_policy_invalid"}
	}
	rctx, cancel := context.WithTimeout(ctx, d.cfg.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(rctx, http.MethodPost, t.URL, bytes.NewReader(body))
	if err != nil {
		return outcome{err: "invalid_url"}
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", d.userAgent)
	client := egress.NewClient(egress.Options{Policy: policy, Timeout: d.cfg.Timeout})
	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)
	if err != nil {
		switch {
		case egress.IsBlocked(err):
			// The environment's egress policy refuses the destination: retrying cannot help.
			return outcome{latency: latency, err: "destination_blocked"}
		case errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "Client.Timeout"):
			return outcome{latency: latency, err: "timeout", retryable: true}
		default:
			return outcome{latency: latency, err: truncate("connection_failed: "+scrub(err.Error(), t.URL), 500), retryable: true}
		}
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, model.MaxExcerptBytes))
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10)) // drain a little so the connection is reusable
	status := resp.StatusCode
	o := outcome{status: &status, excerpt: truncate(strings.ToValidUTF8(string(raw), ""), model.MaxExcerptBytes),
		latency: latency, retryable: true}
	if status < 200 || status >= 300 {
		o.err = "http_" + http.StatusText(status)
		if o.err == "http_" {
			o.err = "http_error"
		}
		o.err = strings.ToLower(strings.ReplaceAll(o.err, " ", "_"))
	}
	return o
}

// scrub removes the destination URL (which may carry tokens in its query)
// from transport error messages.
func scrub(msg, url string) string {
	return strings.ReplaceAll(msg, url, "<url>")
}
