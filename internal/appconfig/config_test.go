package appconfig_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"starterkit/internal/appconfig"
)

func TestTheDefaultsStartAPanel(t *testing.T) {
	cfg, err := appconfig.Parse(nil, nil)
	require.NoError(t, err)

	require.Equal(t, "127.0.0.1:7301", cfg.Addr)
	require.Equal(t, "data.sqlite3", cfg.DatabaseFile)
	require.Equal(t, 15*time.Second, cfg.Shutdown.Timeout)
	require.Len(t, cfg.Proxies.Trusted, 2)
}

func TestEachServiceReadsItsOwnVariables(t *testing.T) {
	environ := []string{
		"DATABASE_URL=sqlite:/var/lib/panel.sqlite3",
		"STARTERKIT_OIDC_ISSUER=https://id.example.com",
		"STARTERKIT_AUDIT_RETENTION=72h",
		"STARTERKIT_NOTIFIER_FROM=panel@example.com",
		"STARTERKIT_TRUSTED_PROXIES=10.0.0.0/8",
		"STARTERKIT_METRICS_INTERVAL=5s",
		"STARTERKIT_SHUTDOWN_DELAY=3s",
		"STARTERKIT_DEV_MODE=true",
	}

	cfg, err := appconfig.Parse(nil, environ)
	require.NoError(t, err)

	require.Equal(t, "/var/lib/panel.sqlite3", cfg.DatabaseFile)
	require.Equal(t, "https://id.example.com", cfg.Gate.Issuer)
	require.Equal(t, 72*time.Hour, cfg.Audit.Retention)
	require.Equal(t, "panel@example.com", string(cfg.Notifier.From))
	require.Len(t, cfg.Proxies.Trusted, 1)
	require.Equal(t, 5*time.Second, cfg.Metrics.UpdateInterval)
	require.Equal(t, 3*time.Second, cfg.Shutdown.Delay)
	require.True(t, cfg.DevMode)
}

func TestEveryProblemIsNamedInOneError(t *testing.T) {
	environ := []string{
		"STARTERKIT_SHUTDOWN_DELAY=10",
		"STARTERKIT_TRUSTED_PROXIES=127.0.0.1/32,localhost",
		"STARTERKIT_LOG_LEVEL=verbose",
		"STARTERKIT_BASE_PATH=/panel/",
		"STARTERKIT_METRICS_ADR=127.0.0.1:9",
	}

	_, err := appconfig.Parse([]string{"--oidc-client-id="}, environ)
	require.Error(t, err)

	for _, problem := range []string{
		"STARTERKIT_SHUTDOWN_DELAY: must be a duration",
		"STARTERKIT_TRUSTED_PROXIES: entry 2 is not a CIDR",
		"STARTERKIT_LOG_LEVEL: must be debug, info, warn or error",
		"STARTERKIT_BASE_PATH (--base-path): must start with / and must not end with /",
		"STARTERKIT_METRICS_ADR names no setting",
		"STARTERKIT_OIDC_CLIENT_ID (--oidc-client-id): is empty",
	} {
		require.Contains(t, err.Error(), problem)
	}
}
