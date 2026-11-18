// Command hugi-worker runs data-plane background work. In S0 it provisions the
// JetStream streams and relays the transactional outbox; workflow execution,
// timers and webhook delivery are added by later slices.
package main

import (
	"context"
	"net/http"
	"os"

	"github.com/nuriansyah/hugimethara/backend/internal/event"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/buildinfo"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/bus"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/config"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/db"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/health"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/lifecycle"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/logging"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/migrate"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/telemetry"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
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
	defer sysPool.Close()

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
	relay := event.NewRelay(sysPool, nb.JS, reg, log, event.RelayOptions{})

	hc := health.New(0)
	hc.Add("postgres", func(ctx context.Context) error { return sysPool.Ping(ctx) })
	hc.Add("migrations", migrate.Check(sysPool))
	hc.Add("nats", nb.Check)
	probes := http.NewServeMux()
	probes.HandleFunc("GET /healthz", hc.Live)
	probes.HandleFunc("GET /readyz", hc.Ready)

	log.Info("starting", "version", buildinfo.Version, "commit", buildinfo.Commit)
	err = lifecycle.Run(ctx,
		relay.Run,
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
