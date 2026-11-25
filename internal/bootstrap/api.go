// Package bootstrap wires the API process.
package bootstrap

import (
	"context"
	"os"

	analyticssvc "github.com/nuriansyah/hugimethara/internal/modules/analytics/services"
	dashboardsvc "github.com/nuriansyah/hugimethara/internal/modules/dashboard/services"
	dataplatformsvc "github.com/nuriansyah/hugimethara/internal/modules/data_platform/services"
	overviewsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/overview/services"
	webhooksvc "github.com/nuriansyah/hugimethara/internal/modules/webhook/services"
	workflowsvc "github.com/nuriansyah/hugimethara/internal/modules/workflow/services"
	analyticshandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/analytics"
	dashboardhandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/dashboard"
	dataplatformhandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/data_platform"
	overviewhandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/overview"
	webhookhandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/webhook"
	workflowhandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/workflow"

	clickhouse "github.com/nuriansyah/hugimethara/internal/infrastructure/analytics/clickhouse"
	"github.com/nuriansyah/hugimethara/internal/infrastructure/messaging/bus"
	"github.com/nuriansyah/hugimethara/internal/infrastructure/observability/health"
	"github.com/nuriansyah/hugimethara/internal/infrastructure/observability/telemetry"
	"github.com/nuriansyah/hugimethara/internal/infrastructure/persistence/cache"
	"github.com/nuriansyah/hugimethara/internal/infrastructure/security/egress"
	"github.com/nuriansyah/hugimethara/internal/infrastructure/security/ratelimit"
	compositionsvc "github.com/nuriansyah/hugimethara/internal/modules/api_composition/services"
	authzsvc "github.com/nuriansyah/hugimethara/internal/modules/authorization/services"
	auditsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/services"
	entsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/entitlement/services"
	identitysvc "github.com/nuriansyah/hugimethara/internal/modules/identity/services"
	connectorsvc "github.com/nuriansyah/hugimethara/internal/modules/integration/connector/services"
	mappingsvc "github.com/nuriansyah/hugimethara/internal/modules/integration/mapping/services"
	secretssvc "github.com/nuriansyah/hugimethara/internal/modules/integration/secrets/services"
	serviceregsvc "github.com/nuriansyah/hugimethara/internal/modules/integration/service_registry/services"
	orgsvc "github.com/nuriansyah/hugimethara/internal/modules/organization/services"
	"github.com/nuriansyah/hugimethara/internal/platform/config"
	"github.com/nuriansyah/hugimethara/internal/platform/config/buildinfo"
	secretbox "github.com/nuriansyah/hugimethara/internal/platform/crypto"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	"github.com/nuriansyah/hugimethara/internal/platform/database/migrate"
	logging "github.com/nuriansyah/hugimethara/internal/platform/logger"
	lifecycle "github.com/nuriansyah/hugimethara/internal/platform/shutdown"
	api "github.com/nuriansyah/hugimethara/internal/transport/http"
	compositionhandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/api_composition"
	audithandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/audit"
	authzhandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/authorization"
	connectorhandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/connector"
	enthandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/entitlement"
	identityhandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/identity"
	mappinghandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/mapping"
	orghandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/organization"
	secretshandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/secrets"
	servicereghandler "github.com/nuriansyah/hugimethara/internal/transport/http/handlers/service_registry"
)

func RunAPI() error {
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
	az := authzsvc.NewRBAC(pool, rdb, log)
	acl := authzsvc.NewService(pool, az, auditAdapter)
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
	secretSvc, err := secretssvc.NewService(pool, az, masterKey, cfg.MasterKeyID)
	if err != nil {
		log.Error("secret vault setup failed", "err", err)
		return err
	}
	opener, err := secretssvc.NewOpener(masterKey, cfg.MasterKeyID)
	if err != nil {
		log.Error("secret opener setup failed", "err", err)
		return err
	}
	ch, err := clickhouse.New(clickhouse.Config{URL: cfg.ClickHouseURL, User: cfg.ClickHouseUser,
		Password: cfg.ClickHousePassword, Database: cfg.ClickHouseDatabase})
	if err != nil {
		log.Error("clickhouse client setup failed", "err", err)
		return err
	}
	dataPlatformSvc, err := dataplatformsvc.NewService(pool, az, opener, ch)
	if err != nil {
		log.Error("data platform service setup failed", "err", err)
		return err
	}
	analyticsSvc, err := analyticssvc.NewService(pool, az, ch, dataPlatformSvc)
	if err != nil {
		log.Error("analytics service setup failed", "err", err)
		return err
	}
	dashboardSvc, err := dashboardsvc.NewService(pool, pool, az, analyticsSvc)
	if err != nil {
		log.Error("dashboard service setup failed", "err", err)
		return err
	}
	connectorSvc, err := connectorsvc.NewService(pool, az)
	if err != nil {
		log.Error("connector service setup failed", "err", err)
		return err
	}
	serviceEgress, err := egress.ParsePolicy(nil)
	if err != nil {
		log.Error("service registry egress policy invalid", "err", err)
		return err
	}
	serviceSvc := serviceregsvc.NewService(pool, az, serviceEgress)
	mappingSvc := mappingsvc.NewService(pool, az)
	compositionSvc := compositionsvc.NewService(pool, az, serviceSvc)
	expressions, err := workflowsvc.NewExpressions()
	if err != nil {
		log.Error("workflow expression environment", "err", err)
		return err
	}
	workflowSvc := workflowsvc.NewService(pool, az, expressions)
	webhookSvc := webhooksvc.NewService(pool, az, sealer, webhooksvc.Config{Limiter: limiter, Registerer: reg})
	orgSvc := orgsvc.NewService(pool, az, acl)
	idSvc := identitysvc.NewService(pool, orgSvc, az, acl, limiter, log, identitysvc.Config{
		PublicURL: cfg.PublicURL, Sealer: sealer,
	})
	router, err := api.NewRouter(api.Deps{
		Env: cfg.Env, TrustedOrigins: cfg.CORSAllowedOrigins, TrustedProxies: cfg.TrustedProxyPrefixes(),
		Health: hc, Registerer: reg,
		Identity:     identityhandler.NewHTTP(idSvc, identityhandler.Cookies{Secure: cfg.CookieSecure}),
		Organization: orghandler.NewHTTP(orgSvc),
		Audit:        audithandler.NewHTTP(auditsvc.NewService(pool, az)),
		Entitlement:  enthandler.NewHTTP(entsvc.NewService(pool, az, usageCounters())),
		Access:       authzhandler.NewHTTP(acl),
		Secrets:      secretshandler.NewHTTP(secretSvc),
		Connectors:   connectorhandler.NewHTTP(connectorSvc),
		Services:     servicereghandler.NewHTTP(serviceSvc),
		Mappings:     mappinghandler.NewHTTP(mappingSvc),
		Compositions: compositionhandler.NewHTTP(compositionSvc),
		Workflows:    workflowhandler.NewHTTP(workflowSvc),
		Webhooks:     webhookhandler.NewHTTP(webhookSvc),
		Overview:     overviewhandler.NewHTTP(overviewsvc.NewService(pool, az)),
		DataPlatform: dataplatformhandler.NewHTTP(dataPlatformSvc),
		Analytics:    analyticshandler.NewHTTP(analyticsSvc),
		Dashboards:   dashboardhandler.NewHTTP(dashboardSvc),
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
