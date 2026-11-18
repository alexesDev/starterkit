package graph

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"starterkit/internal/logger"
)

func busyError(t *testing.T) error {
	t.Helper()

	file := filepath.Join(t.TempDir(), "busy.sqlite3") + "?_pragma=busy_timeout(0)&_pragma=journal_mode(WAL)"

	holder, err := sql.Open("sqlite", file)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = holder.Close()
	})

	waiter, err := sql.Open("sqlite", file)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = waiter.Close()
	})

	_, err = holder.ExecContext(t.Context(), `create table t (id integer primary key)`)
	require.NoError(t, err)

	held, err := holder.Conn(t.Context())
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = held.Close()
	})

	_, err = held.ExecContext(t.Context(), `begin immediate`)
	require.NoError(t, err)

	_, err = waiter.ExecContext(t.Context(), `insert into t (id) values (1)`)
	require.Error(t, err)

	return err
}

func TestABusyDatabaseTellsTheCallerToRetry(t *testing.T) {
	present := errorPresenter(logger.Discard())

	presented := present(t.Context(), fmt.Errorf("failed to ban user: %w", busyError(t)))

	require.Equal(t, codeRetry, presented.Extensions["code"])
	require.Contains(t, presented.Message, "the database is busy, retry, ref ")
}

func TestAnyOtherFailureHidesItsCause(t *testing.T) {
	present := errorPresenter(logger.Discard())

	presented := present(t.Context(), errors.New("no such column: secret_token"))

	require.Nil(t, presented.Extensions)
	require.Contains(t, presented.Message, "internal error, ref ")
	require.NotContains(t, presented.Message, "secret_token")
}
