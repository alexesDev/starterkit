package banuser_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"starterkit/internal/case/banuser"
	"starterkit/internal/db"
	"starterkit/internal/graph/model"
)

func acceptingEnv(actor *int64, stored *db.DBInsertUserBanParams) *EnvMock {
	return &EnvMock{
		CurrentUserIDFunc: func(context.Context) *int64 { return actor },
		DBGetUserByIDFunc: func(_ context.Context, id int64) (db.User, error) {
			return db.User{ID: id}, nil
		},
		DBInsertUserBanFunc: func(_ context.Context, arg db.DBInsertUserBanParams) (db.UserBan, error) {
			*stored = arg
			return db.UserBan{UserID: arg.UserID}, nil
		},
		EnqueueUserBannedNoticeFunc:   func(context.Context, int64) error { return nil },
		NowFunc:                       func() time.Time { return time.Unix(1786000000, 0).UTC() },
		MetricsIncreaseUserBannedFunc: func(context.Context) {},
	}
}

func TestResolveRecordsTheBanCountsItAndEnqueuesTheNotice(t *testing.T) {
	var stored db.DBInsertUserBanParams

	actor := int64(1)
	env := acceptingEnv(&actor, &stored)

	result, err := banuser.Resolve(t.Context(), env, banuser.Input{UserID: 2, Reason: "  spam  "})
	require.NoError(t, err)

	success, ok := result.(*model.BanUserPayload)
	require.True(t, ok, "expected success, got %#v", result)

	assert.Equal(t, int64(2), success.UserID)
	assert.Equal(t, "spam", stored.Reason, "the reason was not trimmed")

	if assert.NotNil(t, stored.BannedBy, "the ban was not attributed to the actor") {
		assert.Equal(t, actor, *stored.BannedBy)
	}

	assert.Len(t, env.MetricsIncreaseUserBannedCalls(), 1, "the banned counter was not incremented")
	assert.Len(t, env.EnqueueUserBannedNoticeCalls(), 1, "the notice was not enqueued")
}

func TestResolveRefusesAnEmptyReasonWithoutTouchingTheDatabase(t *testing.T) {
	env := &EnvMock{CurrentUserIDFunc: func(context.Context) *int64 { return nil }}

	result, err := banuser.Resolve(t.Context(), env, banuser.Input{UserID: 2, Reason: "   "})
	require.NoError(t, err, "a missing reason must not be an infrastructure error")

	_, ok := result.(*model.ErrorPayload)
	require.True(t, ok, "expected ErrorPayload, got %#v", result)

	assert.Empty(t, env.DBGetUserByIDCalls(), "a missing reason must not reach the database")
}

func TestResolveRefusesToLetTheActorBanThemselves(t *testing.T) {
	actor := int64(7)
	env := &EnvMock{CurrentUserIDFunc: func(context.Context) *int64 { return &actor }}

	result, err := banuser.Resolve(t.Context(), env, banuser.Input{UserID: 7, Reason: "oops"})
	require.NoError(t, err, "a self-ban must not be an infrastructure error")

	_, ok := result.(*model.ErrorPayload)
	require.True(t, ok, "expected ErrorPayload, got %#v", result)

	assert.Empty(t, env.DBInsertUserBanCalls(), "a self-ban must not be written")
}

func TestResolveReportsAnUnknownUserAsDataNotError(t *testing.T) {
	env := &EnvMock{
		CurrentUserIDFunc: func(context.Context) *int64 { return nil },
		DBGetUserByIDFunc: func(context.Context, int64) (db.User, error) {
			return db.User{}, sql.ErrNoRows
		},
	}

	result, err := banuser.Resolve(t.Context(), env, banuser.Input{UserID: 404, Reason: "spam"})
	require.NoError(t, err, "an unknown user must not be an infrastructure error")

	_, ok := result.(*model.ErrorPayload)
	require.True(t, ok, "expected ErrorPayload, got %#v", result)

	assert.Empty(t, env.DBInsertUserBanCalls(), "no ban may be written for a user that does not exist")
}

func TestResolveBansOnBehalfOfNobodyWhenThereIsNoCurrentUser(t *testing.T) {
	var stored db.DBInsertUserBanParams

	env := acceptingEnv(nil, &stored)

	_, err := banuser.Resolve(t.Context(), env, banuser.Input{UserID: 2, Reason: "spam"})
	require.NoError(t, err)

	assert.Nil(t, stored.BannedBy, "bannedBy should stay nil")
}

func TestResolvePropagatesAUserLookupFailure(t *testing.T) {
	env := &EnvMock{
		CurrentUserIDFunc: func(context.Context) *int64 { return nil },
		DBGetUserByIDFunc: func(context.Context, int64) (db.User, error) {
			return db.User{}, errors.New("database is locked")
		},
	}

	_, err := banuser.Resolve(t.Context(), env, banuser.Input{UserID: 2, Reason: "spam"})
	require.Error(t, err, "a failing lookup must surface as an error, not an ErrorPayload")
}

func TestResolvePropagatesAFailedEnqueueSoTheBanRollsBack(t *testing.T) {
	var stored db.DBInsertUserBanParams

	env := acceptingEnv(nil, &stored)
	env.EnqueueUserBannedNoticeFunc = func(context.Context, int64) error {
		return errors.New("queue is unavailable")
	}

	_, err := banuser.Resolve(t.Context(), env, banuser.Input{UserID: 2, Reason: "spam"})
	require.Error(t, err, "an enqueue that failed inside the transaction must fail the ban")

	assert.Empty(t, env.MetricsIncreaseUserBannedCalls(), "a ban that will roll back must not be counted")
}
