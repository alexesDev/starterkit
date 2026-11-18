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
		key    notifier.IdempotencyKey
	)

	env := &EnvMock{
		DBGetUserByIDFunc: func(_ context.Context, id int64) (db.User, error) {
			return db.User{ID: id, Email: "user@example.com"}, nil
		},
		DBGetUserBanFunc: func(_ context.Context, userID int64) (db.UserBan, error) {
			return db.UserBan{UserID: userID, Reason: "spam"}, nil
		},
		NotifyUserBannedFunc: func(_ context.Context, carried notifier.IdempotencyKey, to notifier.Address, why string) error {
			key = carried
			sent = to
			reason = why

			return nil
		},
	}

	input := notifyuserbanned.Input{UserID: 2, Key: "minted-at-enqueue"}

	require.NoError(t, notifyuserbanned.Resolve(t.Context(), env, input))

	assert.Equal(t, notifier.Address("user@example.com"), sent)
	assert.Equal(t, "spam", reason)
	assert.Equal(t, input.Key, key, "the key minted at the enqueue must reach the delivery, not a fresh one")
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
		NotifyUserBannedFunc: func(context.Context, notifier.IdempotencyKey, notifier.Address, string) error {
			return errors.New("smtp is down")
		},
	}

	require.Error(t, notifyuserbanned.Resolve(t.Context(), env, notifyuserbanned.Input{UserID: 2}))
}

func TestARetryAfterAFailedDeliveryCarriesTheSameKey(t *testing.T) {
	var keys []notifier.IdempotencyKey

	env := &EnvMock{
		DBGetUserByIDFunc: func(_ context.Context, id int64) (db.User, error) {
			return db.User{ID: id, Email: "user@example.com"}, nil
		},
		DBGetUserBanFunc: func(_ context.Context, userID int64) (db.UserBan, error) {
			return db.UserBan{UserID: userID, Reason: "spam"}, nil
		},
		NotifyUserBannedFunc: func(_ context.Context, key notifier.IdempotencyKey, _ notifier.Address, _ string) error {
			keys = append(keys, key)
			if len(keys) == 1 {
				return errors.New("the delivery timed out, its outcome is unknown")
			}

			return nil
		},
	}

	input := notifyuserbanned.Input{UserID: 2, Key: "minted-at-enqueue"}

	require.Error(t, notifyuserbanned.Resolve(t.Context(), env, input), "an unknown outcome must reach the queue so it retries")
	require.NoError(t, notifyuserbanned.Resolve(t.Context(), env, input))

	assert.Equal(t, []notifier.IdempotencyKey{input.Key, input.Key}, keys, "the retry must resend the key, not mint one")
}
