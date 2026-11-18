package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"maragu.dev/goqite"
	goqitejobs "maragu.dev/goqite/jobs"

	"starterkit/internal/appconfig"
	"starterkit/internal/backup"
	"starterkit/internal/buildinfo"
	"starterkit/internal/cronjobs"
	"starterkit/internal/db"
	"starterkit/internal/gateauth"
	"starterkit/internal/logger"
	"starterkit/internal/metrics"
	"starterkit/internal/notifier"
)

//nolint:gochecknoglobals // -X can only target a package-level var.
var (
	GitCommit string
	BuildTime string
)

func main() {
	err := run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	config, err := appconfig.Get()
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}

	if err != nil {
		return err
	}

	log := logger.New(config.Log, config.DevMode)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a, err := newApp(ctx, config, log)
	if err != nil {
		return err
	}
	defer a.conns.Close()

	a.registerJobs()

	err = a.setAsideUnregisteredJobs(ctx)
	if err != nil {
		return err
	}

	err = a.registerCronJobs()
	if err != nil {
		return err
	}

	a.cronRunner.Start()
	defer a.cronRunner.Stop()

	stopJobs := a.startJobRunner()
	defer stopJobs()

	go func() {
		metricsErr := a.Metrics.Run(ctx, func(err error) {
			log.Error("failed to update metrics", "error", err)
		})
		if metricsErr != nil && !errors.Is(metricsErr, context.Canceled) {
			log.Error("metrics updater stopped", "error", metricsErr)
		}
	}()

	metricsSrv := &http.Server{
		Addr:              config.Metrics.Addr,
		Handler:           metricsHandler(a),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		serveErr := metricsSrv.ListenAndServe()
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			log.Error("metrics listener stopped", "error", serveErr)
		}
	}()

	srv := &http.Server{
		Addr:              config.Addr,
		Handler:           a.routes(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}

	drained := make(chan struct{})

	go func() {
		<-ctx.Done()
		a.shutdown(srv, metricsSrv)
		close(drained)
	}()

	log.Info("starterkit listening",
		"addr", config.Addr,
		"metrics", config.Metrics.Addr,
		"public_url", config.PublicURL)

	err = srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("failed to serve: %w", err)
	}

	<-drained

	return nil
}

func (a *app) startJobRunner() func() {
	jobsCtx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})

	go func() {
		a.jobRunner.Start(jobsCtx)
		close(stopped)
	}()

	return func() {
		cancel()
		<-stopped
		a.log.Info("job runner stopped")
	}
}

func metricsHandler(a *app) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET "+a.config.Metrics.Path, a.Metrics.Handler())

	return mux
}

func (a *app) shutdown(srv, metricsSrv *http.Server) {
	a.ready.Store(false)
	a.log.Info("shutting down", "delay", a.config.Shutdown.Delay, "timeout", a.config.Shutdown.Timeout)

	if a.config.Shutdown.Delay > 0 {
		time.Sleep(a.config.Shutdown.Delay)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.config.Shutdown.Timeout)
	defer cancel()

	err := srv.Shutdown(shutdownCtx)
	if err != nil {
		a.log.Error("failed to shut down cleanly", "error", err)
	}

	err = metricsSrv.Shutdown(shutdownCtx)
	if err != nil {
		a.log.Error("failed to shut down the metrics listener", "error", err)
	}
}

func newApp(ctx context.Context, config *appconfig.Config, log logger.Logger) (*app, error) {
	conns, err := db.Setup(ctx, config.DatabaseFile)
	if err != nil {
		return nil, err
	}

	gate, err := gateauth.New(ctx, config.Gate)
	if err != nil {
		conns.Close()
		return nil, err
	}

	build, err := buildinfo.New(GitCommit, BuildTime, config.RepoURL)
	if err != nil {
		conns.Close()
		return nil, err
	}

	queries := db.New(conns.Read)
	writeQueries := db.NewWriteQueries(conns.Write)

	queue := goqite.New(goqite.NewOpts{
		DB:         conns.Queue,
		Name:       jobQueue,
		MaxReceive: jobMaxReceive,
		Timeout:    5 * time.Minute,
	})

	runnerOpts := goqitejobs.NewRunnerOpts{
		Limit:        2,
		Log:          jobLog{log: log},
		PollInterval: 2 * time.Second,
		Queue:        queue,
	}

	a := &app{
		appState: &appState{
			config:     config,
			build:      build,
			log:        log,
			conns:      conns,
			baseWrite:  writeQueries,
			gate:       gate,
			queue:      queue,
			jobNames:   map[string]struct{}{},
			jobRunner:  goqitejobs.NewRunner(runnerOpts),
			cronRunner: cronjobs.New(ctx, log),
		},
		Queries:      queries,
		WriteQueries: writeQueries,
	}

	a.ready.Store(true)
	a.Metrics = metrics.New(a, config.Metrics)
	a.Notifier = notifier.New(a, config.Notifier, notifier.Origin(config.PublicURL))
	a.Backup = backup.New(conns.Snapshot, config.Backup)

	return a, nil
}

type jobLog struct {
	log logger.Logger
}

func (l jobLog) Info(msg string, args ...any) {
	if carriesError(args) {
		l.log.Warn(msg, args...)
		return
	}

	l.log.Debug(msg, args...)
}

func carriesError(args []any) bool {
	for i := 0; i+1 < len(args); i += 2 {
		if args[i] == "error" {
			return true
		}
	}

	return false
}
