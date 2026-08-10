package makeadmin_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"starterkit/internal/case/makeadmin"
	"starterkit/internal/db"
	"starterkit/internal/graph/model"
)

func envReturning(row db.DBGetAdminUserByIDRow, err error) *EnvMock {
	return &EnvMock{
		DBGetAdminUserByIDFunc: func(context.Context, int64) (db.DBGetAdminUserByIDRow, error) {
			return row, err
		},
		DBInsertAdminFunc:               func(context.Context, db.DBInsertAdminParams) error { return nil },
		NowFunc:                         func() time.Time { return time.Unix(1786000000, 0).UTC() },
		MetricsIncreaseAdminChangedFunc: func(context.Context, string) {},
	}
}

func TestResolveGrantsAdminAndCountsIt(t *testing.T) {
	env := envReturning(db.DBGetAdminUserByIDRow{ID: 2}, nil)

	result, err := makeadmin.Resolve(t.Context(), env, makeadmin.Input{UserID: 2})
	require.NoError(t, err)

	_, ok := result.(*model.MakeAdminPayload)
	require.True(t, ok, "expected success, got %#v", result)

	assert.Len(t, env.DBInsertAdminCalls(), 1)
	assert.Len(t, env.MetricsIncreaseAdminChangedCalls(), 1)
}

func TestResolveRefusesToPromoteABannedUser(t *testing.T) {
	env := envReturning(db.DBGetAdminUserByIDRow{ID: 2, BanReason: "spam"}, nil)

	result, err := makeadmin.Resolve(t.Context(), env, makeadmin.Input{UserID: 2})
	require.NoError(t, err, "a banned user is a domain answer, not an infrastructure error")

	refusal, ok := result.(*model.ErrorPayload)
	require.True(t, ok, "expected ErrorPayload, got %#v", result)

	assert.Equal(t, "user is banned", refusal.Message)
	assert.Empty(t, env.DBInsertAdminCalls())
}

func TestResolveReportsAnUnknownUserAsData(t *testing.T) {
	env := envReturning(db.DBGetAdminUserByIDRow{}, sql.ErrNoRows)

	result, err := makeadmin.Resolve(t.Context(), env, makeadmin.Input{UserID: 404})
	require.NoError(t, err)

	_, ok := result.(*model.ErrorPayload)
	require.True(t, ok, "expected ErrorPayload, got %#v", result)
}

func TestResolveRefusesANonPositiveUserID(t *testing.T) {
	env := envReturning(db.DBGetAdminUserByIDRow{}, nil)

	result, err := makeadmin.Resolve(t.Context(), env, makeadmin.Input{UserID: 0})
	require.NoError(t, err)

	_, ok := result.(*model.ErrorPayload)
	require.True(t, ok, "expected ErrorPayload, got %#v", result)

	assert.Empty(t, env.DBGetAdminUserByIDCalls())
}
