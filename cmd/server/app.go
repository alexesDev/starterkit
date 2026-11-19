package main

import (
	"context"
	"database/sql"
	"sync/atomic"
	"time"

	"maragu.dev/goqite"
	goqitejobs "maragu.dev/goqite/jobs"

	"starterkit/internal/appconfig"
	"starterkit/internal/audit"
	"starterkit/internal/buildinfo"
	"starterkit/internal/cronjobs"
	"starterkit/internal/db"
	"starterkit/internal/gateauth"
	"starterkit/internal/logger"
	"starterkit/internal/metrics"
	"starterkit/internal/notifier"
	"starterkit/internal/page"
)

type app struct {
	*appState

	*db.Queries
	*db.WriteQueries

	currentTx    *sql.Tx
	pendingAudit *audit.Buffer
}

type appState struct {
	*metrics.Metrics
	*notifier.Notifier

	config *appconfig.Config
	build  buildinfo.Info
	log    logger.Logger

	conns *db.Set

	ready atomic.Bool

	baseWrite *db.WriteQueries

	gate *gateauth.Client

	queue      *goqite.Queue
	jobNames   map[string]struct{}
	jobRunner  *goqitejobs.Runner
	cronRunner *cronjobs.Runner
}

func (a *app) Logger() logger.Logger {
	return a.log
}

func (a *app) SignOutURL() string {
	return a.config.SignOutURL
}

func (a *app) Build() buildinfo.Info {
	return a.build
}

func (a *app) Now() time.Time {
	return time.Now().UTC()
}

func (a *app) AuditRetention() time.Duration {
	return a.config.Audit.Retention
}

func (a *app) PageLimits() page.Limits {
	return a.config.Page
}

func (a *app) BootstrapAdminEmail() string {
	return a.config.BootstrapAdmin
}

func (a *app) TableRowCounts(ctx context.Context) (map[string]int64, error) {
	return db.TableRowCounts(ctx, a.conns.Read)
}
