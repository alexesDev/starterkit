package notifyuserbanned_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"starterkit/internal/case/notifyuserbanned"
	"starterkit/internal/db"
	"starterkit/internal/notifier"
)

func TestResolveTellsTheBannedPersonWhy(t *testing.T) {
	var (
		sent   notifier.Address
		reason string
	)

	env := &EnvMock{
		DBGetUserByIDFunc: func(_ context.Context, id int64) (db.User, error) {
			return db.User{ID: id, Email: "user@example.com"}, nil
		},
		DBGetUserBanFunc: func(_ context.Context, userID int64) (db.UserBan, error) {
			return db.UserBan{UserID: userID, Reason: "spam"}, nil
		},
		NotifyUserBannedFunc: func(_ context.Context, to notifier.Address, why string) error {
			sent = to
			reason = why

			return nil
		},
	}

	require.NoError(t, notifyuserbanned.Resolve(t.Context(), env, notifyuserbanned.Input{UserID: 2}))

	assert.Equal(t, notifier.Address("user@example.com"), sent)
	assert.Equal(t, "spam", reason)
}

func TestResolveIsDoneWhenTheUserIsGone(t *testing.T) {
	env := &EnvMock{
		DBGetUserByIDFunc: func(context.Context, int64) (db.User, error) {
			return db.User{}, sql.ErrNoRows
		},
	}

	require.NoError(t, notifyuserbanned.Resolve(t.Context(), env, notifyuserbanned.Input{UserID: 2}))
	assert.Empty(t, env.NotifyUserBannedCalls())
}

func TestResolveIsDoneWhenTheBanWasLiftedBeforeTheJobRan(t *testing.T) {
	env := &EnvMock{
		DBGetUserByIDFunc: func(_ context.Context, id int64) (db.User, error) {
			return db.User{ID: id, Email: "user@example.com"}, nil
		},
		DBGetUserBanFunc: func(context.Context, int64) (db.UserBan, error) {
			return db.UserBan{}, sql.ErrNoRows
		},
	}

	require.NoError(t, notifyuserbanned.Resolve(t.Context(), env, notifyuserbanned.Input{UserID: 2}))
	assert.Empty(t, env.NotifyUserBannedCalls(), "an unbanned user must not be told they are banned")
}

func TestResolvePropagatesADeliveryFailureSoTheJobRetries(t *testing.T) {
	env := &EnvMock{
		DBGetUserByIDFunc: func(_ context.Context, id int64) (db.User, error) {
			return db.User{ID: id, Email: "user@example.com"}, nil
		},
		DBGetUserBanFunc: func(_ context.Context, userID int64) (db.UserBan, error) {
			return db.UserBan{UserID: userID, Reason: "spam"}, nil
		},
		NotifyUserBannedFunc: func(context.Context, notifier.Address, string) error {
			return errors.New("smtp is down")
		},
	}

	require.Error(t, notifyuserbanned.Resolve(t.Context(), env, notifyuserbanned.Input{UserID: 2}))
}
