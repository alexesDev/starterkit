// Package appconfig assembles the process configuration: the settings that
// belong to the server itself, and each service package's own Config, read by
// internal/confread from flags and STARTERKIT_* variables. See docs/config.md.
package appconfig

import (
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"starterkit/internal/audit"
	"starterkit/internal/clientip"
	"starterkit/internal/confread"
	"starterkit/internal/dbmate/sqlite"
	"starterkit/internal/gateauth"
	"starterkit/internal/logger"
	"starterkit/internal/metrics"
	"starterkit/internal/notifier"
)

const prefix = "STARTERKIT_"

type Config struct {
	Addr      string
	PublicURL string
	BasePath  string

	DatabaseURL  string
	DatabaseFile string

	RepoURL        string
	SignOutURL     string
	BootstrapAdmin string
	DevMode        bool

	Gate     gateauth.Config
	Audit    audit.Config
	Notifier notifier.Config
	Proxies  clientip.Config
	Metrics  metrics.Config
	Log      logger.Config
	Shutdown Shutdown
}

type Shutdown struct {
	Delay   time.Duration
	Timeout time.Duration
}

func (s *Shutdown) Register(r *confread.Reader) {
	*s = Shutdown{Timeout: 15 * time.Second}

	r.Duration(&s.Delay, "shutdown-delay", 0, "keep serving this long after /readyz starts failing")
	r.Duration(&s.Timeout, "shutdown-timeout", time.Second, "how long in-flight requests get to finish")
}

func Get() (*Config, error) {
	_ = godotenv.Load()

	return Parse(os.Args[1:], os.Environ())
}

func Parse(args, environ []string) (*Config, error) {
	cfg := &Config{
		Addr:        "127.0.0.1:7301",
		PublicURL:   "http://panel.sk.localbox",
		BasePath:    "/_system/starterkit",
		DatabaseURL: "sqlite:data.sqlite3",
		SignOutURL:  "/oauth2/sign_out",
	}

	r := confread.New("starterkit", prefix, environ)

	cfg.register(r)
	cfg.Gate.Register(r)
	cfg.Audit.Register(r)
	cfg.Notifier.Register(r)
	cfg.Proxies.Register(r)
	cfg.Metrics.Register(r)
	cfg.Log.Register(r)
	cfg.Shutdown.Register(r)

	r.Parse(args)

	cfg.check(r)
	cfg.Gate.Check(r)
	cfg.Audit.Check(r)
	cfg.Notifier.Check(r)
	cfg.Metrics.Check(r)

	return cfg, r.Err()
}

func (c *Config) register(r *confread.Reader) {
	r.String(&c.Addr, "addr", "HTTP listen address")
	r.String(&c.PublicURL, "public-url", "public origin the browser sees")
	r.String(&c.BasePath, "base-path", "URL prefix the panel's routes live under; the root belongs to the SPA")
	r.String(&c.DatabaseURL, "database-url", "sqlite database url, shared with dbmate")
	r.Key("database-url", "DATABASE_URL")
	r.String(&c.RepoURL, "repo-url", "repository a build's commit links to, such as https://github.com/org/app")
	r.String(&c.SignOutURL, "sign-out-url", "the gate's sign-out endpoint the client is sent to")
	r.String(&c.BootstrapAdmin, "bootstrap-admin", "email promoted to admin while the admins table is empty")
	r.Bool(&c.DevMode, "dev-mode", "serve the GraphQL playground, log human-readable")
}

func (c *Config) check(r *confread.Reader) {
	r.Require("addr", c.Addr != "", "is empty")
	r.Require("public-url", c.PublicURL != "", "is empty")
	r.Require("base-path", strings.HasPrefix(c.BasePath, "/") && !strings.HasSuffix(c.BasePath, "/"), "must start with / and must not end with /")

	r.Require("repo-url", c.RepoURL == "" || isRepoURL(c.RepoURL), "must be an http(s) url that does not end with /")

	parsed, err := url.Parse(c.DatabaseURL)
	r.Require("database-url", err == nil && parsed.Scheme == "sqlite", "must be a sqlite: url")

	if err == nil {
		c.DatabaseFile = sqlite.ConnectionString(parsed)
	}
}

func isRepoURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}

	web := parsed.Scheme == "https" || parsed.Scheme == "http"

	return web && parsed.Host != "" && !strings.HasSuffix(raw, "/")
}
