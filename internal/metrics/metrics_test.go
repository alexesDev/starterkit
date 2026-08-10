package metrics_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"starterkit/internal/metrics"
)

func scrape(t *testing.T, m *metrics.Metrics) string {
	t.Helper()

	w := httptest.NewRecorder()
	m.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	require.Equal(t, http.StatusOK, w.Code)

	return w.Body.String()
}

func TestUpdatePublishesOneSeriesPerTable(t *testing.T) {
	env := &EnvMock{
		TableRowCountsFunc: func(context.Context) (map[string]int64, error) {
			return map[string]int64{"users": 2, "audit_log": 6}, nil
		},
	}

	m := metrics.New(env, metrics.DefaultConfig())
	require.NoError(t, m.Update(t.Context()))

	body := scrape(t, m)
	assert.Contains(t, body, `starterkit_table_rows{table="users"} 2`)
	assert.Contains(t, body, `starterkit_table_rows{table="audit_log"} 6`)
}

func TestUpdateStopsReportingATableThatIsGone(t *testing.T) {
	counts := map[string]int64{"users": 2, "widgets": 1}

	env := &EnvMock{
		TableRowCountsFunc: func(context.Context) (map[string]int64, error) {
			return counts, nil
		},
	}

	m := metrics.New(env, metrics.DefaultConfig())
	require.NoError(t, m.Update(t.Context()))

	counts = map[string]int64{"users": 2}
	require.NoError(t, m.Update(t.Context()))

	body := scrape(t, m)
	assert.NotContains(t, body, `table="widgets"`, "a dropped table froze at its last value")
}

func TestUpdateSurfacesAFailedRead(t *testing.T) {
	env := &EnvMock{
		TableRowCountsFunc: func(context.Context) (map[string]int64, error) {
			return nil, errors.New("database is locked")
		},
	}

	m := metrics.New(env, metrics.DefaultConfig())
	require.Error(t, m.Update(t.Context()))
}

func TestACounterIsPublishedOnceItMoves(t *testing.T) {
	env := &EnvMock{
		TableRowCountsFunc: func(context.Context) (map[string]int64, error) {
			return map[string]int64{}, nil
		},
	}

	m := metrics.New(env, metrics.DefaultConfig())
	m.MetricsIncreaseNotification(t.Context(), "user_banned")

	body := scrape(t, m)
	assert.Contains(t, body, `starterkit_notifications_total{kind="user_banned"} 1`)
}
