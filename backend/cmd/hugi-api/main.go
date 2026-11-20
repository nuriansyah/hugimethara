// Command hugi-api serves the Hugimethara REST API (control plane and
// synchronous data-plane endpoints).
package main

import (
	"context"
	"os"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/backend/internal/api"
	"github.com/nuriansyah/hugimethara/backend/internal/audit"
	"github.com/nuriansyah/hugimethara/backend/internal/authz"
	"github.com/nuriansyah/hugimethara/backend/internal/connector"
	"github.com/nuriansyah/hugimethara/backend/internal/entitlement"
	"github.com/nuriansyah/hugimethara/backend/internal/identity"
	"github.com/nuriansyah/hugimethara/backend/internal/organization"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/buildinfo"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/bus"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/cache"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/config"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/db"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/egress"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/health"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/lifecycle"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/logging"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/migrate"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/ratelimit"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/secretbox"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/telemetry"
	secretvault "github.com/nuriansyah/hugimethara/backend/internal/secret"
	"github.com/nuriansyah/hugimethara/backend/internal/servicereg"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load("hugi-api", config.NeedDatabase, config.NeedRedis, config.NeedNATS)
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

	pool, err := db.Open(ctx, cfg.DatabaseURL, cfg.DBMaxConns)
	if err != nil {
		log.Error("postgres unavailable", "err", err)
		return err
	}
	defer func() { _ = pool.Close() }()

	rdb, err := cache.Open(ctx, cfg.RedisURL)
	if err != nil {
		log.Error("redis unavailable", "err", err)
		return err
	}
	defer func() { _ = rdb.Close() }()

	nb, err := bus.Connect(cfg.NATSURL, cfg.ServiceName, log)
	if err != nil {
		log.Error("nats unavailable", "err", err)
		return err
	}
	defer nb.Close()

	hc := health.New(0)
	hc.Add("postgres", db.Check(pool))
	hc.Add("migrations", migrate.Check(pool))
	hc.Add("redis", cache.Check(rdb))
	hc.Add("nats", nb.Check)

	reg := telemetry.NewRegistry()
	az := authz.NewRBAC(pool, rdb, log)
	acl := authz.NewService(pool, az, auditAdapter)
	limiter := ratelimit.New(rdb, log)
	masterKey, defaulted, err := cfg.MasterKeyBytes()
	if err != nil {
		log.Error("invalid master key", "err", err)
		return err
	}
	if defaulted {
		log.Warn("HUGI_MASTER_KEY is not set; using the development key. Set it in every real deployment.")
	}
	sealer, err := secretbox.New(masterKey)
	if err != nil {
		log.Error("master key unusable", "err", err)
		return err
	}
	secretSvc, err := secretvault.NewService(pool, az, masterKey, cfg.MasterKeyID)
	if err != nil {
		log.Error("secret vault setup failed", "err", err)
		return err
	}
	connectorSvc, err := connector.NewService(pool, az)
	if err != nil {
		log.Error("connector service setup failed", "err", err)
		return err
	}
	serviceEgress, err := egress.ParsePolicy(nil)
	if err != nil {
		log.Error("service registry egress policy invalid", "err", err)
		return err
	}
	serviceSvc := servicereg.NewService(pool, az, serviceEgress)
	orgSvc := organization.NewService(pool, az, acl)
	idSvc := identity.NewService(pool, orgSvc, az, acl, limiter, log, identity.Config{
		PublicURL: cfg.PublicURL, Sealer: sealer,
	})
	router, err := api.NewRouter(api.Deps{
		Env: cfg.Env, TrustedOrigins: cfg.CORSAllowedOrigins, TrustedProxies: cfg.TrustedProxyPrefixes(),
		Health: hc, Registerer: reg,
		Identity:     identity.NewHTTP(idSvc, identity.Cookies{Secure: cfg.CookieSecure}),
		Organization: organization.NewHTTP(orgSvc),
		Audit:        audit.NewHTTP(audit.NewService(pool, az)),
		Entitlement:  entitlement.NewHTTP(entitlement.NewService(pool, az, usageCounters())),
		Access:       authz.NewHTTP(acl),
		Secrets:      secretvault.NewHTTP(secretSvc),
		Connectors:   connector.NewHTTP(connectorSvc),
		Services:     servicereg.NewHTTP(serviceSvc),
	})
	if err != nil {
		log.Error("router setup failed", "err", err)
		return err
	}

	log.Info("starting", "version", buildinfo.Version, "commit", buildinfo.Commit)
	err = lifecycle.Run(ctx,
		lifecycle.HTTPServer("api", cfg.HTTPAddr, router, cfg.ShutdownTTL, log),
		lifecycle.MetricsServer(cfg.MetricsAddr, reg, cfg.ShutdownTTL, log),
	)
	if err != nil {
		log.Error("stopped with error", "err", err)
		return err
	}
	log.Info("stopped")
	return nil
}

// usageCounters collects per-feature usage counters from the domains that own them.
func usageCounters() map[string]entitlement.UsageFunc {
	out := map[string]entitlement.UsageFunc{}
	for k, fn := range organization.UsageCounters() {
		out[k] = fn
	}
	return out
}

// auditAdapter lets authz record audit events without importing the audit
// domain (which itself depends on authz).
func auditAdapter(ctx context.Context, tx *sqlx.Tx, org uuid.UUID, action, resourceType, resourceID string, details map[string]any) error {
	return audit.Record(ctx, tx, audit.Event{OrgID: org, Action: action, ResourceType: resourceType,
		ResourceID: resourceID, Details: details})
}
