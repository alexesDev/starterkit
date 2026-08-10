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

// app is the IO spine. Every Env interface in the system is satisfied by this
// one struct, and the compiler is what proves it.
//
// The two-level split is load-bearing: app is shallow-copied to make a
// transaction-scoped env, so only the fields that must change per transaction
// live here. Everything process-lifetime lives behind the *appState pointer and
// is shared by every copy. See docs/env-pattern.md.
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

// Now is UTC because SQLite writes current_timestamp in UTC and the audit purge
// compares against it.
func (a *app) Now() time.Time {
	return time.Now().UTC()
}

func (a *app) AuditRetention() time.Duration {
	return a.config.AuditRetention
}

func (a *app) BootstrapAdminEmail() string {
	return a.config.BootstrapAdmin
}

// TableRowCounts reads through the read pool, so it never contends with an open
// write transaction.
func (a *app) TableRowCounts(ctx context.Context) (map[string]int64, error) {
	return db.TableRowCounts(ctx, a.conns.Read)
}
