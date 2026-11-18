// Package appconfig resolves configuration from flags, then STARTERKIT_* env
// vars, then defaults. Flags win, so a one-off override never needs the
// environment edited.
package appconfig

import (
	"errors"
	"flag"
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"starterkit/internal/clientip"
	"starterkit/internal/dbmate/sqlite"
	"starterkit/internal/metrics"
	"starterkit/internal/notifier"
)

type Config struct {
	Addr      string
	PublicURL string
	BasePath  string

	DatabaseURL  string
	DatabaseFile string

	OIDCIssuer   string
	OIDCClientID string
	SignOutURL   string

	BootstrapAdmin string
	AuditRetention time.Duration
	AuditPurgeSpec string

	NotifierFrom notifier.Address

	TrustedProxies []netip.Prefix

	Metrics metrics.Config

	ShutdownDelay   time.Duration
	ShutdownTimeout time.Duration

	LogLevel string
	DevMode  bool
}

func Get() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{}
	fs := flag.NewFlagSet("starterkit", flag.ContinueOnError)

	fs.StringVar(&cfg.Addr, "addr", env("STARTERKIT_ADDR", "127.0.0.1:7301"), "HTTP listen address")
	fs.StringVar(&cfg.PublicURL, "public-url", env("STARTERKIT_PUBLIC_URL", "http://panel.sk.localbox"), "public origin the browser sees")
	fs.StringVar(
		&cfg.BasePath,
		"base-path",
		env("STARTERKIT_BASE_PATH", "/_system/starterkit"),
		"URL prefix the panel's routes live under; the root belongs to the SPA",
	)
	fs.StringVar(&cfg.DatabaseURL, "database-url", env("DATABASE_URL", "sqlite:data.sqlite3"), "sqlite database url, shared with dbmate")
	fs.StringVar(&cfg.OIDCIssuer, "oidc-issuer", env("STARTERKIT_OIDC_ISSUER", "http://dex.sk.localbox/dex"), "OIDC issuer URL")
	fs.StringVar(&cfg.OIDCClientID, "oidc-client-id", env("STARTERKIT_OIDC_CLIENT_ID", "starterkit"), "audience the gate's ID token must carry")
	fs.StringVar(&cfg.SignOutURL, "sign-out-url", env("STARTERKIT_SIGN_OUT_URL", "/oauth2/sign_out"), "the gate's sign-out endpoint the client is sent to")
	fs.StringVar(&cfg.BootstrapAdmin, "bootstrap-admin", env("STARTERKIT_BOOTSTRAP_ADMIN", ""), "email promoted to admin while the admins table is empty")
	fs.StringVar(&cfg.LogLevel, "log-level", env("STARTERKIT_LOG_LEVEL", "info"), "debug|info|warn|error")
	fs.BoolVar(&cfg.DevMode, "dev", envBool("STARTERKIT_DEV_MODE", false), "serve the GraphQL playground, log human-readable")
	fs.DurationVar(
		&cfg.AuditRetention,
		"audit-retention",
		envDuration("STARTERKIT_AUDIT_RETENTION", 30*24*time.Hour),
		"how long audit_log rows are kept; 0 disables the purge",
	)
	fs.StringVar(&cfg.AuditPurgeSpec, "audit-purge-spec", env("STARTERKIT_AUDIT_PURGE_SPEC", "0 4 * * *"), "cron spec for the audit retention sweep")

	notifierFrom := fs.String("notifier-from", env("STARTERKIT_NOTIFIER_FROM", "noreply@example.invalid"), "sender address every notice is delivered from")
	trusted := fs.String(
		"trusted-proxies",
		env("STARTERKIT_TRUSTED_PROXIES", "127.0.0.1/32,::1/128"),
		"CIDRs whose X-Forwarded-For is believed; empty means never believe it",
	)

	registerRuntimeFlags(fs, cfg)

	err := fs.Parse(os.Args[1:])
	if err != nil {
		return nil, fmt.Errorf("failed to parse flags: %w", err)
	}

	cfg.NotifierFrom = notifier.Address(*notifierFrom)
	cfg.TrustedProxies = clientip.Parse(*trusted)

	parsed, err := url.Parse(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DATABASE_URL: %w", err)
	}

	cfg.DatabaseFile = sqlite.ConnectionString(parsed)

	err = cfg.validate()
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func registerRuntimeFlags(fs *flag.FlagSet, cfg *Config) {
	cfg.Metrics = metrics.DefaultConfig()

	fs.StringVar(&cfg.Metrics.Addr, "metrics-addr", env("STARTERKIT_METRICS_ADDR", cfg.Metrics.Addr), "listener for the Prometheus endpoint")
	fs.DurationVar(
		&cfg.Metrics.UpdateInterval,
		"metrics-interval",
		envDuration("STARTERKIT_METRICS_INTERVAL", cfg.Metrics.UpdateInterval),
		"how often gauges are refreshed",
	)
	fs.DurationVar(
		&cfg.ShutdownDelay,
		"shutdown-delay",
		envDuration("STARTERKIT_SHUTDOWN_DELAY", 0),
		"keep serving this long after /readyz starts failing",
	)
	fs.DurationVar(
		&cfg.ShutdownTimeout,
		"shutdown-timeout",
		envDuration("STARTERKIT_SHUTDOWN_TIMEOUT", 15*time.Second),
		"how long in-flight requests get to finish",
	)
}

func (c *Config) validate() error {
	if !strings.HasPrefix(c.BasePath, "/") || strings.HasSuffix(c.BasePath, "/") {
		return errors.New("base path must start with / and must not end with /: " + c.BasePath)
	}

	if c.OIDCIssuer == "" {
		return errors.New("oidc issuer is empty: set STARTERKIT_OIDC_ISSUER")
	}

	if c.OIDCClientID == "" {
		return errors.New("oidc client id is empty: set STARTERKIT_OIDC_CLIENT_ID to the audience the gate's token carries")
	}

	if c.NotifierFrom == "" {
		return errors.New("notifier sender is empty: set STARTERKIT_NOTIFIER_FROM")
	}

	return nil
}

func env(key, fallback string) string {
	val, ok := os.LookupEnv(key)
	if !ok || val == "" {
		return fallback
	}

	return val
}

func envBool(key string, fallback bool) bool {
	val, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	parsed, err := strconv.ParseBool(val)
	if err != nil {
		return fallback
	}

	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	val, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	parsed, err := time.ParseDuration(val)
	if err != nil {
		return fallback
	}

	return parsed
}
