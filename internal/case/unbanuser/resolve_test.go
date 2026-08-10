package unbanuser_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"starterkit/internal/case/unbanuser"
	"starterkit/internal/graph/model"
)

func TestResolveLiftsTheBan(t *testing.T) {
	env := &EnvMock{DBDeleteUserBanFunc: func(context.Context, int64) error { return nil }}

	result, err := unbanuser.Resolve(t.Context(), env, unbanuser.Input{UserID: 2})
	require.NoError(t, err)

	success, ok := result.(*model.UnbanUserPayload)
	require.True(t, ok, "expected success, got %#v", result)

	assert.Equal(t, int64(2), success.UserID)
	assert.Len(t, env.DBDeleteUserBanCalls(), 1)
}

func TestResolveIsIdempotentForAUserWhoWasNotBanned(t *testing.T) {
	env := &EnvMock{DBDeleteUserBanFunc: func(context.Context, int64) error { return nil }}

	result, err := unbanuser.Resolve(t.Context(), env, unbanuser.Input{UserID: 2})
	require.NoError(t, err, "the account is already in the state the caller asked for")

	_, ok := result.(*model.UnbanUserPayload)
	require.True(t, ok, "expected success, got %#v", result)
}

func TestResolveRefusesANonPositiveUserID(t *testing.T) {
	env := &EnvMock{DBDeleteUserBanFunc: func(context.Context, int64) error { return nil }}

	result, err := unbanuser.Resolve(t.Context(), env, unbanuser.Input{UserID: 0})
	require.NoError(t, err)

	_, ok := result.(*model.ErrorPayload)
	require.True(t, ok, "expected ErrorPayload, got %#v", result)

	assert.Empty(t, env.DBDeleteUserBanCalls())
}

func TestResolvePropagatesADeleteFailure(t *testing.T) {
	env := &EnvMock{
		DBDeleteUserBanFunc: func(context.Context, int64) error { return errors.New("database is locked") },
	}

	_, err := unbanuser.Resolve(t.Context(), env, unbanuser.Input{UserID: 2})
	require.Error(t, err)
}
