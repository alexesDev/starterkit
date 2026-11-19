package backup_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"starterkit/internal/backup"
	"starterkit/internal/db"
)

func setUp(t *testing.T) *db.Set {
	t.Helper()

	conns, err := db.Setup(t.Context(), filepath.Join(t.TempDir(), "live.sqlite3"))
	require.NoError(t, err)

	t.Cleanup(conns.Close)

	return conns
}

func at(hour int) time.Time {
	return time.Date(2026, time.November, 18, hour, 30, 0, 0, time.UTC)
}

func TestACopyOpensAsADatabaseWithTheSameSchema(t *testing.T) {
	conns := setUp(t)
	dir := t.TempDir()

	b := backup.New(conns.Snapshot, backup.Config{Dir: dir, Keep: 3})

	path, err := b.BackupDatabase(t.Context(), at(3))
	require.NoError(t, err)
	require.Equal(t, backup.Path(filepath.Join(dir, "starterkit-20261118T033000Z.sqlite3")), path)

	copied, err := sql.Open("sqlite", string(path))
	require.NoError(t, err)

	defer copied.Close()

	var tables int

	err = copied.QueryRowContext(context.Background(), `select count(*) from sqlite_schema where type = 'table' and name = 'users'`).Scan(&tables)
	require.NoError(t, err)
	require.Equal(t, 1, tables)
}

func TestOnlyTheNewestCopiesAreKept(t *testing.T) {
	conns := setUp(t)
	dir := t.TempDir()

	unrelated := filepath.Join(dir, "notes.txt")
	require.NoError(t, os.WriteFile(unrelated, []byte("keep me"), 0o600))

	b := backup.New(conns.Snapshot, backup.Config{Dir: dir, Keep: 2})

	for hour := range 4 {
		_, err := b.BackupDatabase(t.Context(), at(hour))
		require.NoError(t, err)
	}

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	require.ElementsMatch(t, []string{"notes.txt", "starterkit-20261118T023000Z.sqlite3", "starterkit-20261118T033000Z.sqlite3"}, names)
}
