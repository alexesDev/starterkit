package revokemcpkey_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"starterkit/internal/case/revokemcpkey"
	"starterkit/internal/db"
	"starterkit/internal/graph/model"
)

func envRevoking(rows int64) *EnvMock {
	return &EnvMock{
		DBRevokeMCPKeyFunc: func(context.Context, db.DBRevokeMCPKeyParams) (int64, error) { return rows, nil },
		NowFunc:            func() time.Time { return time.Unix(1786000000, 0).UTC() },
	}
}

func TestResolveRevokesAnActiveKeyAtNow(t *testing.T) {
	env := envRevoking(1)

	result, err := revokemcpkey.Resolve(t.Context(), env, revokemcpkey.Input{MCPKeyID: 3})
	require.NoError(t, err)

	success, ok := result.(*model.RevokeMCPKeyPayload)
	require.True(t, ok, "expected success, got %#v", result)
	require.Equal(t, int64(3), success.MCPKeyID)

	calls := env.DBRevokeMCPKeyCalls()
	require.Len(t, calls, 1)
	require.Equal(t, int64(1786000000), calls[0].Arg.RevokedAt.Unix())
}

func TestResolveSaysTheKeyIsAlreadyRevoked(t *testing.T) {
	env := envRevoking(0)
	env.DBGetMCPKeyByIDFunc = func(context.Context, int64) (db.DBGetMCPKeyByIDRow, error) {
		return db.DBGetMCPKeyByIDRow{ID: 3}, nil
	}

	result, err := revokemcpkey.Resolve(t.Context(), env, revokemcpkey.Input{MCPKeyID: 3})
	require.NoError(t, err)

	refusal, ok := result.(*model.ErrorPayload)
	require.True(t, ok, "expected ErrorPayload, got %#v", result)
	require.Contains(t, refusal.Message, "already revoked")
}

func TestResolveSaysAnUnknownKeyIsNotFound(t *testing.T) {
	env := envRevoking(0)
	env.DBGetMCPKeyByIDFunc = func(context.Context, int64) (db.DBGetMCPKeyByIDRow, error) {
		return db.DBGetMCPKeyByIDRow{}, sql.ErrNoRows
	}

	result, err := revokemcpkey.Resolve(t.Context(), env, revokemcpkey.Input{MCPKeyID: 404})
	require.NoError(t, err)

	refusal, ok := result.(*model.ErrorPayload)
	require.True(t, ok, "expected ErrorPayload, got %#v", result)
	require.Contains(t, refusal.Message, "not found")
}
