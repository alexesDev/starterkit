// Package metrics exposes Prometheus counters and gauges.
//
// Counters are incremented by use cases through their own Env. Gauges are the
// opposite: nobody increments them, a background loop asks the database and
// sets them. See docs/metrics-and-shutdown.md.
package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Config struct {
	Addr           string
	Path           string
	Namespace      string
	UpdateInterval time.Duration
}

func DefaultConfig() Config {
	return Config{
		Addr:           "127.0.0.1:7302",
		Path:           "/metrics",
		Namespace:      "starterkit",
		UpdateInterval: 30 * time.Second,
	}
}

//go:generate go tool github.com/matryer/moq -out mocks_test.go -pkg metrics_test . Env

type Env interface {
	TableRowCounts(ctx context.Context) (map[string]int64, error)
}

type Metrics struct {
	env      Env
	config   Config
	registry *prometheus.Registry

	signIns       prometheus.Counter
	adminDenied   *prometheus.CounterVec
	usersBanned   prometheus.Counter
	adminChanged  *prometheus.CounterVec
	jobsFinished  *prometheus.CounterVec
	notifications *prometheus.CounterVec

	tableRows *prometheus.GaugeVec
}

func New(env Env, config Config) *Metrics {
	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewGoCollector())
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	m := &Metrics{
		env:      env,
		config:   config,
		registry: registry,

		signIns: counter(config, "sign_ins_total", "Successful sign-ins."),
		adminDenied: counterVec(config, "admin_denied_total",
			"Refused crossings of the admin boundary.", "operation"),
		usersBanned: counter(config, "users_banned_total", "Users banned."),
		adminChanged: counterVec(config, "admin_changed_total",
			"Admin access granted or revoked.", "action"),
		jobsFinished: counterVec(config, "jobs_finished_total",
			"Background jobs that ran to completion.", "job", "outcome"),
		notifications: counterVec(config, "notifications_total",
			"Notices handed to the notifier.", "kind"),

		tableRows: gaugeVec(config, "table_rows", "Rows currently in each table.", "table"),
	}

	registry.MustRegister(
		m.signIns, m.adminDenied, m.usersBanned,
		m.adminChanged, m.jobsFinished, m.notifications, m.tableRows,
	)

	return m
}

func counter(config Config, name, help string) prometheus.Counter {
	return prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: config.Namespace, Name: name, Help: help,
	})
}

func counterVec(config Config, name, help string, labels ...string) *prometheus.CounterVec {
	return prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: config.Namespace, Name: name, Help: help,
	}, labels)
}

func gaugeVec(config Config, name, help string, labels ...string) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: config.Namespace, Name: name, Help: help,
	}, labels)
}

func (m *Metrics) Config() Config {
	return m.config
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

func (m *Metrics) Run(ctx context.Context, onError func(error)) error {
	ticker := time.NewTicker(m.config.UpdateInterval)
	defer ticker.Stop()

	for {
		err := m.Update(ctx)
		if err != nil && onError != nil {
			onError(err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (m *Metrics) Update(ctx context.Context) error {
	counts, err := m.env.TableRowCounts(ctx)
	if err != nil {
		return err
	}

	m.tableRows.Reset()

	for table, count := range counts {
		m.tableRows.WithLabelValues(table).Set(float64(count))
	}

	return nil
}

func (m *Metrics) MetricsIncreaseSignIn(_ context.Context) {
	m.signIns.Inc()
}

func (m *Metrics) MetricsIncreaseAdminDenied(_ context.Context, operation string) {
	m.adminDenied.WithLabelValues(operation).Inc()
}

func (m *Metrics) MetricsIncreaseUserBanned(_ context.Context) {
	m.usersBanned.Inc()
}

func (m *Metrics) MetricsIncreaseAdminChanged(_ context.Context, action string) {
	m.adminChanged.WithLabelValues(action).Inc()
}

func (m *Metrics) MetricsIncreaseNotification(_ context.Context, kind string) {
	m.notifications.WithLabelValues(kind).Inc()
}

func (m *Metrics) MetricsIncreaseJobFinished(_ context.Context, job string, err error) {
	outcome := "ok"
	if err != nil {
		outcome = "error"
	}

	m.jobsFinished.WithLabelValues(job, outcome).Inc()
}
