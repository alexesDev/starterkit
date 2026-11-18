package db_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"starterkit/internal/db"
)

func scratch(t *testing.T, file string) *sql.DB {
	t.Helper()

	conn, err := sql.Open("sqlite", file+"?_pragma=busy_timeout(0)&_pragma=journal_mode(WAL)")
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = conn.Close()
	})

	return conn
}

func TestAUniqueViolationIsRecognisedThroughWrapping(t *testing.T) {
	conn := scratch(t, filepath.Join(t.TempDir(), "unique.sqlite3"))

	_, err := conn.ExecContext(t.Context(), `create table t (id integer primary key, name text unique)`)
	require.NoError(t, err)

	_, err = conn.ExecContext(t.Context(), `insert into t (id, name) values (1, 'a')`)
	require.NoError(t, err)

	_, err = conn.ExecContext(t.Context(), `insert into t (id, name) values (1, 'b')`)
	require.True(t, db.IsUniqueViolation(fmt.Errorf("failed to insert: %w", err)), "primary key: %v", err)

	_, err = conn.ExecContext(t.Context(), `insert into t (id, name) values (2, 'a')`)
	require.True(t, db.IsUniqueViolation(err), "unique column: %v", err)
	require.False(t, db.IsBusy(err))
}

func TestALockedWriterIsBusyAndNothingElse(t *testing.T) {
	file := filepath.Join(t.TempDir(), "busy.sqlite3")
	holder := scratch(t, file)
	waiter := scratch(t, file)

	_, err := holder.ExecContext(t.Context(), `create table t (id integer primary key)`)
	require.NoError(t, err)

	held, err := holder.Conn(t.Context())
	require.NoError(t, err)

	defer held.Close()

	_, err = held.ExecContext(t.Context(), `begin immediate`)
	require.NoError(t, err)

	_, err = waiter.ExecContext(context.Background(), `insert into t (id) values (1)`)
	require.True(t, db.IsBusy(err), "a write behind an open writer: %v", err)
	require.False(t, db.IsUniqueViolation(err))

	require.False(t, db.IsBusy(errors.New("database is locked")), "a bare string is not a driver error")
}
