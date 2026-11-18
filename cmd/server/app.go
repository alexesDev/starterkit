package main

import (
	"context"
	"database/sql"
	"sync/atomic"
	"time"

	"maragu.dev/goqite"
	goqitejobs "maragu.dev/goqite/jobs"

	"starterkit/internal/appconfig"
	"starterkit/internal/cronjobs"
	"starterkit/internal/db"
	"starterkit/internal/gateauth"
	"starterkit/internal/logger"
	"starterkit/internal/metrics"
	"starterkit/internal/notifier"
)

type app struct {
	*appState

	*db.Queries
	*db.WriteQueries

	currentTx *sql.Tx
}

type appState struct {
	*metrics.Metrics
	*notifier.Notifier

	config      *appconfig.Config
	buildCommit string
	log         logger.Logger

	conns *db.Set

	ready atomic.Bool

	baseWrite *db.WriteQueries

	gate *gateauth.Client

	queue      *goqite.Queue
	jobRunner  *goqitejobs.Runner
	cronRunner *cronjobs.Runner
}

func (a *app) Logger() logger.Logger {
	return a.log
}

func (a *app) SignOutURL() string {
	return a.config.SignOutURL
}

func (a *app) BuildGitCommit() string {
	return a.buildCommit
}

func (a *app) Now() time.Time {
	return time.Now().UTC()
}

func (a *app) AuditRetention() time.Duration {
	return a.config.Audit.Retention
}

func (a *app) BootstrapAdminEmail() string {
	return a.config.BootstrapAdmin
}

func (a *app) TableRowCounts(ctx context.Context) (map[string]int64, error) {
	return db.TableRowCounts(ctx, a.conns.Read)
}
