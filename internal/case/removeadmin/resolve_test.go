package removeadmin_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"starterkit/internal/case/removeadmin"
	"starterkit/internal/db"
	"starterkit/internal/graph/model"
)

func TestResolveRevokesTheGrantAndCountsIt(t *testing.T) {
	env := &EnvMock{
		CurrentUserIDFunc:               func(context.Context) *int64 { return nil },
		DBDeleteAdminFunc:               func(context.Context, int64) (int64, error) { return 1, nil },
		MetricsIncreaseAdminChangedFunc: func(context.Context, string) {},
	}

	result, err := removeadmin.Resolve(t.Context(), env, removeadmin.Input{UserID: 2})
	require.NoError(t, err)

	_, ok := result.(*model.RemoveAdminPayload)
	require.True(t, ok, "expected success, got %#v", result)

	assert.Len(t, env.MetricsIncreaseAdminChangedCalls(), 1)
}

func TestResolveRefusesToRemoveTheLastAdmin(t *testing.T) {
	env := &EnvMock{
		CurrentUserIDFunc: func(context.Context) *int64 { return nil },
		DBDeleteAdminFunc: func(context.Context, int64) (int64, error) { return 0, nil },
		DBGetAdminByUserIDFunc: func(_ context.Context, userID int64) (db.Admin, error) {
			return db.Admin{UserID: userID}, nil
		},
	}

	result, err := removeadmin.Resolve(t.Context(), env, removeadmin.Input{UserID: 2})
	require.NoError(t, err, "the last admin is a domain answer, not an infrastructure error")

	refusal, ok := result.(*model.ErrorPayload)
	require.True(t, ok, "expected ErrorPayload, got %#v", result)

	assert.Equal(t, "the last admin cannot be removed", refusal.Message)
	assert.Empty(t, env.MetricsIncreaseAdminChangedCalls(), "a refused removal must not be counted")
}

func TestResolveTreatsAUserWhoWasNeverAnAdminAsAlreadyDone(t *testing.T) {
	env := &EnvMock{
		CurrentUserIDFunc: func(context.Context) *int64 { return nil },
		DBDeleteAdminFunc: func(context.Context, int64) (int64, error) { return 0, nil },
		DBGetAdminByUserIDFunc: func(context.Context, int64) (db.Admin, error) {
			return db.Admin{}, sql.ErrNoRows
		},
	}

	result, err := removeadmin.Resolve(t.Context(), env, removeadmin.Input{UserID: 2})
	require.NoError(t, err)

	_, ok := result.(*model.RemoveAdminPayload)
	require.True(t, ok, "expected success, got %#v", result)
}

func TestResolveRefusesSelfRemoval(t *testing.T) {
	actor := int64(7)
	env := &EnvMock{CurrentUserIDFunc: func(context.Context) *int64 { return &actor }}

	result, err := removeadmin.Resolve(t.Context(), env, removeadmin.Input{UserID: 7})
	require.NoError(t, err)

	_, ok := result.(*model.ErrorPayload)
	require.True(t, ok, "expected ErrorPayload, got %#v", result)

	assert.Empty(t, env.DBDeleteAdminCalls(), "self-removal must not reach the delete")
}
