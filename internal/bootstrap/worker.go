// RunWorker runs data-plane background work: it provisions the JetStream
// streams, relays the transactional outbox, executes workflow steps (S10),
// runs the workflow reaper (which also fires delay and task-timeout timers),
// and starts executions from schedule and platform-event triggers (S11).
// Webhook delivery arrives in S12.
package bootstrap

import (
	"encoding/json"

	"context"
	"net/http"
	"os"

	workflowsvc "github.com/nuriansyah/hugimethara/internal/modules/workflow/services"
	"github.com/nuriansyah/hugimethara/internal/platform/id"

	"github.com/google/uuid"

	"github.com/nuriansyah/hugimethara/internal/infrastructure/messaging/bus"
	"github.com/nuriansyah/hugimethara/internal/infrastructure/observability/health"
	"github.com/nuriansyah/hugimethara/internal/infrastructure/observability/telemetry"
	"github.com/nuriansyah/hugimethara/internal/infrastructure/persistence/outbox"
	"github.com/nuriansyah/hugimethara/internal/platform/config"
	"github.com/nuriansyah/hugimethara/internal/platform/config/buildinfo"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	"github.com/nuriansyah/hugimethara/internal/platform/database/migrate"
	logging "github.com/nuriansyah/hugimethara/internal/platform/logger"
	lifecycle "github.com/nuriansyah/hugimethara/internal/platform/shutdown"
)

func RunWorker() error {
	cfg, err := config.Load("hugi-worker", config.NeedSystemDatabase, config.NeedNATS)
	log := logging.New(os.Stdout, cfg.Log.Level, cfg.Log.Format, cfg.ServiceName, cfg.Env)
	if err != nil {
		log.Error("invalid configuration", "err", err)
		return err
	}
	ctx, stop := lifecycle.SignalContext()
	defer stop()

	shutdownTracing, err := telemetry.Setup(ctx, cfg.ServiceName, buildinfo.Version, cfg.Env, cfg.OTLPEndpoint, cfg.TraceSample)
	if err != nil {
		log.Error("telemetry setup failed", "err", err)
		return err
	}
	defer func() { _ = shutdownTracing(context.WithoutCancel(ctx)) }()

	sysPool, err := db.Open(ctx, cfg.SystemDatabaseURL, cfg.DBMaxConns)
	if err != nil {
		log.Error("postgres unavailable", "err", err)
		return err
	}
	defer func() { _ = sysPool.Close() }()

	nb, err := bus.Connect(cfg.NATSURL, cfg.ServiceName, log)
	if err != nil {
		log.Error("nats unavailable", "err", err)
		return err
	}
	defer nb.Close()
	if err := nb.EnsureStreams(ctx); err != nil {
		log.Error("jetstream provisioning failed", "err", err)
		return err
	}

	reg := telemetry.NewRegistry()
	relay := outbox.NewRelay(sysPool, nb.JS, reg, log, outbox.RelayOptions{})

	expressions, err := workflowsvc.NewExpressions()
	if err != nil {
		log.Error("workflow expression environment", "err", err)
		return err
	}
	hostname, _ := os.Hostname()
	engine := workflowsvc.NewEngine(sysPool, workflowsvc.NewRegistry(expressions), workflowsvc.EngineConfig{
		WorkerID: hostname + "/" + id.New().String(), Log: log, Registerer: reg,
	})
	consumeSteps := func(ctx context.Context) error {
		return nb.Consume(ctx, bus.WorkConsumer{Stream: "HUGI_WF", Durable: "hugi-wf-steps", Filter: "hugi.wf.step.>"},
			log, func(ctx context.Context, data []byte) error {
				var msg workflowsvc.StepMessage
				if err := json.Unmarshal(data, &msg); err != nil {
					log.ErrorContext(ctx, "dropping malformed step message", "err", err)
					return nil // poison message: acknowledging is the only way to make progress
				}
				return engine.HandleStep(ctx, msg)
			})
	}

	triggers := workflowsvc.NewTriggerRunner(sysPool, expressions, workflowsvc.TriggerConfig{Log: log, Registerer: reg})
	consumeEvents := func(ctx context.Context) error {
		return nb.ConsumeMessages(ctx, bus.WorkConsumer{Stream: "HUGI_EVENTS", Durable: "hugi-wf-event-triggers",
			Filter: "hugi.events.>", DeliverNew: true}, log, func(ctx context.Context, m bus.Message) error {
			name, ok := workflowsvc.EventFromSubject(m.Subject)
			orgID, err := uuid.Parse(first(m.Header["Hugi-Org-Id"]))
			if !ok || err != nil {
				return nil // not a tenant event: nothing can be triggered by it
			}
			msgID := first(m.Header["Nats-Msg-Id"])
			if msgID == "" {
				log.WarnContext(ctx, "event without message id; event triggers skipped", "subject", m.Subject)
				return nil // no stable id means no idempotency key
			}
			return triggers.HandleEvent(ctx, workflowsvc.Event{OrgID: orgID, Name: name, ID: msgID, Data: m.Data})
		})
	}

	hc := health.New(0)
	hc.Add("postgres", db.Check(sysPool))
	hc.Add("migrations", migrate.Check(sysPool))
	hc.Add("nats", nb.Check)
	probes := http.NewServeMux()
	probes.HandleFunc("GET /healthz", hc.Live)
	probes.HandleFunc("GET /readyz", hc.Ready)

	log.Info("starting", "version", buildinfo.Version, "commit", buildinfo.Commit)
	err = lifecycle.Run(ctx,
		relay.Run,
		consumeSteps,
		engine.RunReaper,
		triggers.RunScheduler,
		consumeEvents,
		lifecycle.HTTPServer("probes", cfg.HTTPAddr, probes, cfg.ShutdownTTL, log),
		lifecycle.MetricsServer(cfg.MetricsAddr, reg, cfg.ShutdownTTL, log),
	)
	if err != nil {
		log.Error("stopped with error", "err", err)
		return err
	}
	log.Info("stopped")
	return nil
}

func first(v []string) string {
	if len(v) == 0 {
		return ""
	}
	return v[0]
}
