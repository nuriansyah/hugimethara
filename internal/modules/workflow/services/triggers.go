package services

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/nuriansyah/hugimethara/internal/modules/workflow/model"
	"github.com/nuriansyah/hugimethara/internal/modules/workflow/repository"
	"github.com/nuriansyah/hugimethara/internal/platform/clock"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/platform/id"
)

// TriggerConfig tunes the trigger runner.
type TriggerConfig struct {
	Interval   time.Duration // schedule poll interval; default 10s
	Batch      int           // due schedules per pass; default 100
	Clock      clock.Clock
	IDs        id.Generator
	Log        *slog.Logger
	Registerer prometheus.Registerer
}

// TriggerRunner starts executions from schedule and platform-event triggers.
// It connects as hugi_system and binds the tenant per trigger, exactly like
// the engine does per step.
type TriggerRunner struct {
	pool  *sqlx.DB
	expr  *Expressions
	cfg   TriggerConfig
	fired *prometheus.CounterVec
}

// NewTriggerRunner builds a runner. pool must connect as hugi_system.
func NewTriggerRunner(pool *sqlx.DB, expr *Expressions, cfg TriggerConfig) *TriggerRunner {
	if cfg.Interval <= 0 {
		cfg.Interval = 10 * time.Second
	}
	if cfg.Batch <= 0 {
		cfg.Batch = 100
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
	r := &TriggerRunner{pool: pool, expr: expr, cfg: cfg,
		fired: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "hugi_workflow_triggers_total",
			Help: "Trigger activations, by trigger type and result."}, []string{"type", "result"})}
	if cfg.Registerer != nil {
		cfg.Registerer.MustRegister(r.fired)
	}
	return r
}

// RunScheduler fires due schedules until ctx is cancelled.
func (r *TriggerRunner) RunScheduler(ctx context.Context) error {
	t := time.NewTicker(r.cfg.Interval)
	defer t.Stop()
	for {
		if _, err := r.FireDueSchedules(ctx); err != nil && ctx.Err() == nil {
			r.cfg.Log.ErrorContext(ctx, "workflow scheduler pass failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
	}
}

// FireDueSchedules starts one execution per due schedule trigger and moves
// it to its next activation. Missed activations (worker down) collapse into
// one run: the next fire time is computed from now, never replayed. The
// idempotency key is the scheduled instant, so two workers can never start
// the same activation twice.
func (r *TriggerRunner) FireDueSchedules(ctx context.Context) (int, error) {
	due, err := repository.DueSchedules(ctx, r.pool, r.cfg.Batch)
	if err != nil {
		return 0, err
	}
	started := 0
	for _, d := range due {
		err := db.InTenantTx(ctx, r.pool, d.OrgID, func(tx *sqlx.Tx) error {
			s, ok, err := repository.LockDueSchedule(ctx, tx, d.ID)
			if err != nil || !ok {
				return err
			}
			now := r.cfg.Clock.Now()
			sched, err := model.ParseScheduleTrigger(repository.TriggerConfig(s.Config))
			if err != nil {
				r.fired.WithLabelValues(model.TypeTriggerSchedule, "invalid").Inc()
				r.cfg.Log.WarnContext(ctx, "stopping invalid schedule", "trigger_id", s.ID, "err", err)
				return repository.SetNextFire(ctx, tx, s.ID, nil)
			}
			from := now
			if s.NextFireAt.After(from) {
				from = s.NextFireAt
			}
			var next *time.Time
			if n := sched.Cron.Next(from, sched.Location); !n.IsZero() {
				next = &n
			}
			if err := repository.SetNextFire(ctx, tx, s.ID, next); err != nil {
				return err
			}
			if !s.Enabled || s.WorkflowStatus != "active" {
				r.fired.WithLabelValues(model.TypeTriggerSchedule, "skipped").Inc()
				return nil
			}
			v, err := repository.GetVersion(ctx, tx, s.VersionID)
			if err != nil {
				return err
			}
			input := sched.Input
			if input == nil {
				input = map[string]any{}
			}
			raw, err := json.Marshal(input)
			if err != nil {
				return err
			}
			at := s.NextFireAt.UTC().Format(time.RFC3339)
			_, created, err := startExecution(ctx, tx, r.cfg.IDs, startRequest{OrgID: s.OrgID,
				WorkspaceID: s.WorkspaceID, WorkflowID: s.WorkflowID, EnvironmentID: s.EnvironmentID, Version: v,
				TriggerType: model.TypeTriggerSchedule, TriggerRef: at, IdempotencyKey: "schedule:" + at,
				Input: raw, Now: now})
			return r.outcome(ctx, model.TypeTriggerSchedule, s.WorkflowID, created, &started, err)
		})
		if err != nil && ctx.Err() == nil {
			r.cfg.Log.ErrorContext(ctx, "schedule trigger failed", "trigger_id", d.ID, "err", err)
		}
	}
	return started, nil
}

// outcome records a start attempt. A refusal by a limit is logged and counted
// but not an error: the trigger is consumed (the schedule advances, the event
// is acknowledged) rather than retried forever.
func (r *TriggerRunner) outcome(ctx context.Context, typ string, workflowID uuid.UUID, created bool, started *int,
	err error) error {
	var p *problem.Problem
	switch {
	case errors.As(err, &p) && p.Code == model.ReasonConcurrencyLimit:
		r.fired.WithLabelValues(typ, "limited").Inc()
		r.cfg.Log.WarnContext(ctx, "trigger refused by concurrency limit", "type", typ, "workflow_id", workflowID)
		return nil
	case err != nil:
		return err
	case created:
		*started++
		r.fired.WithLabelValues(typ, "started").Inc()
	default:
		r.fired.WithLabelValues(typ, "duplicate").Inc()
	}
	return nil
}

// Event is a platform event delivered from the HUGI_EVENTS stream.
type Event struct {
	OrgID uuid.UUID
	Name  string // subject without the "hugi.events." prefix
	ID    string // stable per event (the outbox message id)
	Data  []byte
}

// EventFromSubject derives the event name from a subject.
func EventFromSubject(subject string) (string, bool) {
	const prefix = "hugi.events."
	if !strings.HasPrefix(subject, prefix) {
		return "", false
	}
	return strings.TrimPrefix(subject, prefix), true
}

// HandleEvent starts the executions of every enabled trigger.event listening
// to ev in its organization. It is idempotent per (workflow, event id). Two
// guards stop runaway chains: a workflow is never started by its own events,
// and an event chain deeper than MaxTriggerChainDepth starts nothing.
func (r *TriggerRunner) HandleEvent(ctx context.Context, ev Event) error {
	var data map[string]any
	if err := json.Unmarshal(ev.Data, &data); err != nil || data == nil {
		return nil // not an object payload: no trigger can match it
	}
	if ev.Name == WebhookReceivedEvent {
		return r.handleWebhook(ctx, ev, data)
	}
	depth := 0
	if d, ok := data["trigger_depth"].(float64); ok {
		depth = int(d)
	}
	source, _ := data["workflow_id"].(string)
	started := 0
	return db.InTenantTx(ctx, r.pool, ev.OrgID, func(tx *sqlx.Tx) error {
		triggers, err := repository.EventTriggers(ctx, tx, ev.OrgID, ev.Name)
		if err != nil {
			return err
		}
		for _, t := range triggers {
			if source == t.WorkflowID.String() {
				continue
			}
			if depth+1 > model.MaxTriggerChainDepth {
				r.fired.WithLabelValues(model.TypeTriggerEvent, "chain_too_deep").Inc()
				r.cfg.Log.WarnContext(ctx, "event trigger chain too deep; not starting", "workflow_id", t.WorkflowID,
					"event", ev.Name, "depth", depth)
				continue
			}
			input := map[string]any{"event": ev.Name, "data": data}
			cfg, err := model.ParseEventTrigger(repository.TriggerConfig(t.Config))
			if err != nil {
				continue
			}
			if strings.TrimSpace(cfg.Condition) != "" {
				ok, err := r.expr.EvalBool(cfg.Condition, map[string]any{"input": input, "trigger": input})
				if err != nil {
					r.fired.WithLabelValues(model.TypeTriggerEvent, "condition_error").Inc()
					r.cfg.Log.WarnContext(ctx, "event trigger condition failed", "workflow_id", t.WorkflowID, "err", err)
					continue
				}
				if !ok {
					r.fired.WithLabelValues(model.TypeTriggerEvent, "filtered").Inc()
					continue
				}
			}
			v, err := repository.GetVersion(ctx, tx, t.VersionID)
			if err != nil {
				return err
			}
			raw, err := json.Marshal(input)
			if err != nil {
				return err
			}
			if len(raw) > model.MaxInlinePayload {
				r.fired.WithLabelValues(model.TypeTriggerEvent, "too_large").Inc()
				continue
			}
			_, created, err := startExecution(ctx, tx, r.cfg.IDs, startRequest{OrgID: t.OrgID,
				WorkspaceID: t.WorkspaceID, WorkflowID: t.WorkflowID, EnvironmentID: t.EnvironmentID, Version: v,
				TriggerType: model.TypeTriggerEvent, TriggerRef: ev.ID, IdempotencyKey: "event:" + ev.ID,
				Input: raw, TriggerDepth: depth + 1, Now: r.cfg.Clock.Now()})
			if err := r.outcome(ctx, model.TypeTriggerEvent, t.WorkflowID, created, &started, err); err != nil {
				return err
			}
		}
		return nil
	})
}

// WebhookReceivedEvent is emitted by the webhook ingress for every verified
// inbound request.
const WebhookReceivedEvent = "webhook.received"

// handleWebhook starts the executions of every enabled trigger.webhook bound
// to the endpoint that received the event, in the endpoint's workspace and
// environment. It is idempotent per (workflow, webhook event id): a replayed
// stream message or a re-delivered event starts nothing new.
func (r *TriggerRunner) handleWebhook(ctx context.Context, ev Event, data map[string]any) error {
	str := func(k string) string { s, _ := data[k].(string); return s }
	eventID, endpointID := str("event_id"), str("endpoint_id")
	environmentID, err1 := uuid.Parse(str("environment_id"))
	workspaceID, err2 := uuid.Parse(str("workspace_id"))
	if eventID == "" || endpointID == "" || err1 != nil || err2 != nil {
		return nil
	}
	input := map[string]any{"event_id": eventID, "endpoint_id": endpointID,
		"provider_event_id": data["provider_event_id"], "event_type": data["event_type"],
		"content_type": data["content_type"], "received_at": data["received_at"]}
	for _, k := range []string{"payload", "raw_body", "payload_truncated"} {
		if v, ok := data[k]; ok {
			input[k] = v
		}
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return err
	}
	started := 0
	return db.InTenantTx(ctx, r.pool, ev.OrgID, func(tx *sqlx.Tx) error {
		triggers, err := repository.WebhookTriggers(ctx, tx, ev.OrgID, endpointID, environmentID, workspaceID)
		if err != nil {
			return err
		}
		for _, t := range triggers {
			cfg, err := model.ParseWebhookTrigger(repository.TriggerConfig(t.Config))
			if err != nil {
				continue
			}
			if strings.TrimSpace(cfg.Condition) != "" {
				ok, err := r.expr.EvalBool(cfg.Condition, map[string]any{"input": input, "trigger": input})
				if err != nil {
					r.fired.WithLabelValues(model.TypeTriggerWebhook, "condition_error").Inc()
					r.cfg.Log.WarnContext(ctx, "webhook trigger condition failed", "workflow_id", t.WorkflowID, "err", err)
					continue
				}
				if !ok {
					r.fired.WithLabelValues(model.TypeTriggerWebhook, "filtered").Inc()
					continue
				}
			}
			if len(raw) > model.MaxInlinePayload {
				r.fired.WithLabelValues(model.TypeTriggerWebhook, "too_large").Inc()
				r.cfg.Log.WarnContext(ctx, "webhook payload too large for a workflow input; not starting",
					"workflow_id", t.WorkflowID, "event_id", eventID)
				continue
			}
			v, err := repository.GetVersion(ctx, tx, t.VersionID)
			if err != nil {
				return err
			}
			_, created, err := startExecution(ctx, tx, r.cfg.IDs, startRequest{OrgID: t.OrgID,
				WorkspaceID: t.WorkspaceID, WorkflowID: t.WorkflowID, EnvironmentID: t.EnvironmentID, Version: v,
				TriggerType: model.TypeTriggerWebhook, TriggerRef: eventID, IdempotencyKey: "webhook:" + eventID,
				Input: raw, Now: r.cfg.Clock.Now()})
			if err := r.outcome(ctx, model.TypeTriggerWebhook, t.WorkflowID, created, &started, err); err != nil {
				return err
			}
		}
		return nil
	})
}
