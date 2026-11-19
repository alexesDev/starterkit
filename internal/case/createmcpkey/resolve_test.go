package createmcpkey_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"starterkit/internal/case/createmcpkey"
	"starterkit/internal/db"
	"starterkit/internal/graph/model"
	"starterkit/internal/mcpkey"
)

const issued = mcpkey.Secret("mcp_issued")

func acceptingEnv(stored *db.DBInsertMCPKeyParams) *EnvMock {
	return &EnvMock{
		CurrentAdminFunc:   func(context.Context) (int64, error) { return 7, nil },
		GenerateMCPKeyFunc: func() (mcpkey.Secret, error) { return issued, nil },
		DBInsertMCPKeyFunc: func(_ context.Context, arg db.DBInsertMCPKeyParams) (db.MCPKey, error) {
			*stored = arg
			return db.MCPKey{ID: 42}, nil
		},
		NowFunc: func() time.Time { return time.Unix(1786000000, 0).UTC() },
	}
}

func TestResolveStoresOnlyTheHashAndReturnsTheKeyOnce(t *testing.T) {
	var stored db.DBInsertMCPKeyParams

	env := acceptingEnv(&stored)

	result, err := createmcpkey.Resolve(t.Context(), env, createmcpkey.Input{Name: "  ci job  "})
	require.NoError(t, err)

	success, ok := result.(*model.CreateMCPKeyPayload)
	require.True(t, ok, "expected success, got %#v", result)

	require.Equal(t, int64(42), success.MCPKeyID)
	require.Equal(t, string(issued), success.Key)

	require.Equal(t, int64(7), stored.UserID, "the key must belong to the admin who made it")
	require.Equal(t, "ci job", stored.Name)
	require.Equal(t, issued.Hash(), stored.KeyHash)
	require.NotContains(t, string(stored.KeyHash), string(issued), "the key itself reached the database")
	require.Equal(t, int64(1786000000), stored.CreatedAt.Unix())
}

func TestResolveRefusesAnEmptyNameWithoutMintingAKey(t *testing.T) {
	env := &EnvMock{}

	result, err := createmcpkey.Resolve(t.Context(), env, createmcpkey.Input{Name: "   "})
	require.NoError(t, err, "an empty name must not be an infrastructure error")

	refusal, ok := result.(*model.ErrorPayload)
	require.True(t, ok, "expected ErrorPayload, got %#v", result)
	require.Equal(t, "name", refusal.ByFields[0].Name)

	require.Empty(t, env.GenerateMCPKeyCalls(), "a key was minted for a refused input")
}

func TestResolvePropagatesAFailedInsert(t *testing.T) {
	var stored db.DBInsertMCPKeyParams

	env := acceptingEnv(&stored)
	env.DBInsertMCPKeyFunc = func(context.Context, db.DBInsertMCPKeyParams) (db.MCPKey, error) {
		return db.MCPKey{}, errors.New("database is locked")
	}

	_, err := createmcpkey.Resolve(t.Context(), env, createmcpkey.Input{Name: "ci"})
	require.Error(t, err, "a failed insert must surface as an error, so the key is never shown")
}
