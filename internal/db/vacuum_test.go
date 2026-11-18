package db_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"starterkit/internal/db"
)

func TestTheDatabaseReclaimsFreePagesIncrementally(t *testing.T) {
	conns, err := db.Setup(t.Context(), filepath.Join(t.TempDir(), "test.sqlite3"))
	require.NoError(t, err)

	t.Cleanup(conns.Close)

	var mode int

	err = conns.Read.QueryRowContext(t.Context(), `pragma auto_vacuum`).Scan(&mode)
	require.NoError(t, err)
	require.Equal(t, 2, mode, "auto_vacuum is not incremental, so a purge never shrinks the file")

	require.NoError(t, db.ReclaimFreePages(t.Context(), conns.Write))
}
