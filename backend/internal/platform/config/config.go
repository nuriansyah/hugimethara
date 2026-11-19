// Package config loads typed process configuration from HUGI_* environment variables.
// Invalid configuration fails fast at startup.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"

	"github.com/nuriansyah/hugimethara/backend/internal/platform/secretbox"
)

// Config is shared by all binaries. Fields a binary does not use are ignored.
type Config struct {
	Env         string `env:"HUGI_ENV" envDefault:"development"` // development | staging | production
	ServiceName string `env:"HUGI_SERVICE_NAME"`                 // defaults to binary name when empty

	HTTPAddr    string        `env:"HUGI_HTTP_ADDR" envDefault:":8080"`
	MetricsAddr string        `env:"HUGI_METRICS_ADDR" envDefault:":9090"`
	ShutdownTTL time.Duration `env:"HUGI_SHUTDOWN_TIMEOUT" envDefault:"20s"`

	// CORSAllowedOrigins lists trusted browser origins: allowed for credentialed
	// CORS and for state-changing requests (origin check).
	CORSAllowedOrigins []string `env:"HUGI_CORS_ALLOWED_ORIGINS" envSeparator:","`
	// TrustedProxies are CIDRs whose X-Forwarded-For is honoured for client IPs.
	TrustedProxies []string `env:"HUGI_TRUSTED_PROXIES" envSeparator:","`
	// CookieSecure marks session cookies Secure and uses the __Host- prefix.
	// Disable only for plain-HTTP local development.
	CookieSecure bool `env:"HUGI_COOKIE_SECURE" envDefault:"true"`

	// PublicURL is the origin users reach the app at. OIDC redirect URIs and
	// post-login redirects are built from it (the web app proxies /api to this
	// listener, so the browser sees one origin).
	PublicURL string `env:"HUGI_PUBLIC_URL" envDefault:"http://localhost:3000"`
	// MasterKey is a base64-encoded 32-byte key that encrypts identity-provider
	// client secrets and the short-lived OIDC state cookie. Required in
	// production; development falls back to a documented key.
	MasterKey string `env:"HUGI_MASTER_KEY"`

	Log LogConfig

	DatabaseURL        string `env:"HUGI_DATABASE_URL"`         // runtime role (hugi_app), RLS enforced
	MigrateDatabaseURL string `env:"HUGI_MIGRATE_DATABASE_URL"` // owner role, migrations only
	SystemDatabaseURL  string `env:"HUGI_SYSTEM_DATABASE_URL"`  // hugi_system role: cross-tenant platform jobs
	DBMaxConns         int    `env:"HUGI_DB_MAX_CONNS" envDefault:"20"`

	RedisURL string `env:"HUGI_REDIS_URL"`
	NATSURL  string `env:"HUGI_NATS_URL"`

	// OTLPEndpoint enables trace export when set (e.g. "otel-collector:4317").
	OTLPEndpoint string  `env:"OTEL_EXPORTER_OTLP_ENDPOINT"`
	TraceSample  float64 `env:"HUGI_TRACE_SAMPLE_RATIO" envDefault:"1.0"`
}

type LogConfig struct {
	Level  string `env:"HUGI_LOG_LEVEL" envDefault:"info"`  // debug | info | warn | error
	Format string `env:"HUGI_LOG_FORMAT" envDefault:"json"` // json | text
}

// Requirement names a dependency a binary needs configured.
type Requirement int

const (
	NeedDatabase Requirement = iota
	NeedMigrateDatabase
	NeedSystemDatabase
	NeedRedis
	NeedNATS
)

// Load parses the environment and validates the fields the caller requires.
func Load(serviceName string, needs ...Requirement) (Config, error) {
	var c Config
	if err := env.Parse(&c); err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	if c.ServiceName == "" {
		c.ServiceName = serviceName
	}
	if err := c.validate(needs); err != nil {
		return Config{}, err
	}
	return c, nil
}

func (c Config) IsProduction() bool { return c.Env == "production" }

func (c Config) validate(needs []Requirement) error {
	var errs []error
	switch c.Env {
	case "development", "staging", "production", "test":
	default:
		errs = append(errs, fmt.Errorf("HUGI_ENV: unknown environment %q", c.Env))
	}
	switch strings.ToLower(c.Log.Level) {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, fmt.Errorf("HUGI_LOG_LEVEL: unknown level %q", c.Log.Level))
	}
	if c.Log.Format != "json" && c.Log.Format != "text" {
		errs = append(errs, fmt.Errorf("HUGI_LOG_FORMAT: must be json or text"))
	}
	if c.TraceSample < 0 || c.TraceSample > 1 {
		errs = append(errs, fmt.Errorf("HUGI_TRACE_SAMPLE_RATIO: must be within [0,1]"))
	}
	for _, o := range c.CORSAllowedOrigins {
		u, err := url.Parse(o)
		if err != nil || u.Scheme == "" || u.Host == "" || u.Path != "" {
			errs = append(errs, fmt.Errorf("HUGI_CORS_ALLOWED_ORIGINS: %q is not an origin (scheme://host[:port])", o))
		}
	}
	for _, cidr := range c.TrustedProxies {
		if _, err := ParsePrefix(cidr); err != nil {
			errs = append(errs, fmt.Errorf("HUGI_TRUSTED_PROXIES: %q is not a CIDR or IP", cidr))
		}
	}
	if c.IsProduction() && !c.CookieSecure {
		errs = append(errs, errors.New("HUGI_COOKIE_SECURE: must be true in production"))
	}
	if u, err := url.Parse(c.PublicURL); err != nil || u.Scheme == "" || u.Host == "" || u.Path != "" {
		errs = append(errs, errors.New("HUGI_PUBLIC_URL: must be an origin (scheme://host[:port])"))
	}
	if c.IsProduction() && c.MasterKey == "" {
		errs = append(errs, errors.New("HUGI_MASTER_KEY: required in production"))
	}
	if _, _, err := c.MasterKeyBytes(); err != nil {
		errs = append(errs, err)
	}
	for _, n := range needs {
		switch n {
		case NeedDatabase:
			errs = append(errs, requireURL("HUGI_DATABASE_URL", c.DatabaseURL, "postgres", "postgresql"))
		case NeedMigrateDatabase:
			errs = append(errs, requireURL("HUGI_MIGRATE_DATABASE_URL", c.MigrateDatabaseURL, "postgres", "postgresql"))
		case NeedSystemDatabase:
			errs = append(errs, requireURL("HUGI_SYSTEM_DATABASE_URL", c.SystemDatabaseURL, "postgres", "postgresql"))
		case NeedRedis:
			errs = append(errs, requireURL("HUGI_REDIS_URL", c.RedisURL, "redis", "rediss"))
		case NeedNATS:
			errs = append(errs, requireURL("HUGI_NATS_URL", c.NATSURL, "nats", "tls"))
		}
	}
	return errors.Join(errs...)
}

// MasterKeyBytes decodes HUGI_MASTER_KEY. When unset outside production it
// returns a stable development key and defaulted=true so the caller can warn.
func (c Config) MasterKeyBytes() (key []byte, defaulted bool, err error) {
	if c.MasterKey == "" {
		if c.IsProduction() {
			return nil, false, errors.New("HUGI_MASTER_KEY: required in production")
		}
		return secretbox.DeriveKey("hugi-development-master-key"), true, nil
	}
	key, err = base64.StdEncoding.DecodeString(c.MasterKey)
	if err != nil {
		return nil, false, errors.New("HUGI_MASTER_KEY: must be base64")
	}
	if len(key) != secretbox.KeySize {
		return nil, false, fmt.Errorf("HUGI_MASTER_KEY: must decode to %d bytes", secretbox.KeySize)
	}
	return key, false, nil
}

// TrustedProxyPrefixes parses TrustedProxies (validated by Load).
func (c Config) TrustedProxyPrefixes() []netip.Prefix {
	out := make([]netip.Prefix, 0, len(c.TrustedProxies))
	for _, s := range c.TrustedProxies {
		if p, err := ParsePrefix(s); err == nil {
			out = append(out, p)
		}
	}
	return out
}

// ParsePrefix accepts a CIDR or a single IP.
func ParsePrefix(s string) (netip.Prefix, error) {
	s = strings.TrimSpace(s)
	if !strings.Contains(s, "/") {
		a, err := netip.ParseAddr(s)
		if err != nil {
			return netip.Prefix{}, err
		}
		return netip.PrefixFrom(a, a.BitLen()), nil
	}
	p, err := netip.ParsePrefix(s)
	return p.Masked(), err
}

func requireURL(name, val string, schemes ...string) error {
	if val == "" {
		return fmt.Errorf("%s: required", name)
	}
	u, err := url.Parse(val)
	if err != nil {
		return fmt.Errorf("%s: invalid URL", name) // never echo the value: it may contain a password
	}
	for _, s := range schemes {
		if u.Scheme == s {
			return nil
		}
	}
	return fmt.Errorf("%s: scheme must be one of %v", name, schemes)
}
