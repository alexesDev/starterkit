package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"maragu.dev/goqite"

	"starterkit/internal/db"
	"starterkit/internal/dbtime"
	"starterkit/internal/logger"
	"starterkit/internal/metrics"
)

func testUser(subject string) db.DBUpsertUserByOIDCParams {
	return db.DBUpsertUserByOIDCParams{
		OidcIssuer:  "https://issuer.test",
		OidcSubject: subject,
		Email:       subject + "@example.com",
		CreatedAt:   dbtime.At(time.Date(2026, time.August, 10, 9, 0, 0, 0, time.UTC)),
	}
}

func identityOf(subject string) db.DBGetIdentityByOIDCParams {
	return db.DBGetIdentityByOIDCParams{OidcIssuer: "https://issuer.test", OidcSubject: subject}
}

func newTestApp(t *testing.T) *app {
	t.Helper()

	conns, err := db.Setup(t.Context(), filepath.Join(t.TempDir(), "test.sqlite3"))
	require.NoError(t, err, "failed to set up database")

	t.Cleanup(conns.Close)

	queue := goqite.New(goqite.NewOpts{
		DB:         conns.Queue,
		Name:       "jobs",
		MaxReceive: 3,
		Timeout:    5 * time.Minute,
	})

	a := &app{
		appState:     &appState{conns: conns, log: logger.Discard(), queue: queue},
		Queries:      db.New(conns.Read),
		WriteQueries: db.NewWriteQueries(conns.Write),
	}

	a.baseWrite = a.WriteQueries
	a.Metrics = metrics.New(a, metrics.DefaultConfig())

	return a
}

var errTestRollback = errors.New("the callback failed")

func TestWithTransactionCommits(t *testing.T) {
	a := newTestApp(t)

	err := a.WithTransaction(t.Context(), func(ctx context.Context, env *app) error {
		_, txErr := env.DBUpsertUserByOIDC(ctx, testUser("a"))
		return txErr
	})
	require.NoError(t, err)

	_, err = a.Queries.DBGetIdentityByOIDC(t.Context(), identityOf("a"))
	require.NoError(t, err, "the committed row is not visible")
}

func TestWithTransactionRollsBackWhenTheCallbackFails(t *testing.T) {
	a := newTestApp(t)

	err := a.WithTransaction(t.Context(), func(ctx context.Context, env *app) error {
		_, txErr := env.DBUpsertUserByOIDC(ctx, testUser("b"))
		require.NoError(t, txErr)

		return errTestRollback
	})
	require.ErrorIs(t, err, errTestRollback)

	_, err = a.Queries.DBGetIdentityByOIDC(t.Context(), identityOf("b"))
	assert.True(t, db.IsNotFound(err), "the row survived a rollback: %v", err)
}

func TestNestedWithTransactionFails(t *testing.T) {
	a := newTestApp(t)

	err := a.WithTransaction(t.Context(), func(ctx context.Context, env *app) error {
		return env.WithTransaction(ctx, func(context.Context, *app) error {
			assert.Fail(t, "the inner transaction should never run")
			return nil
		})
	})

	require.ErrorIs(t, err, ErrNestedTransaction)
}

func TestNestedWithTransactionFailsViaTheOuterEnv(t *testing.T) {
	a := newTestApp(t)

	err := a.WithTransaction(t.Context(), func(ctx context.Context, _ *app) error {
		return a.WithTransaction(ctx, func(context.Context, *app) error {
			assert.Fail(t, "the inner transaction should never run")
			return nil
		})
	})

	require.ErrorIs(t, err, ErrNestedTransaction)
}

func TestTheTransactionEnvSeesItsOwnUncommittedWrites(t *testing.T) {
	a := newTestApp(t)

	err := a.WithTransaction(t.Context(), func(ctx context.Context, env *app) error {
		_, txErr := env.DBUpsertUserByOIDC(ctx, testUser("c"))
		require.NoError(t, txErr)

		_, txErr = env.Queries.DBGetIdentityByOIDC(ctx, identityOf("c"))
		require.NoError(t, txErr, "the transaction cannot see its own write")

		_, outsideErr := a.Queries.DBGetIdentityByOIDC(ctx, identityOf("c"))
		assert.True(t, db.IsNotFound(outsideErr), "an uncommitted write is visible outside the transaction: %v", outsideErr)

		return errTestRollback
	})
	require.ErrorIs(t, err, errTestRollback)
}

func TestAnEnqueueInsideATransactionRollsBackWithIt(t *testing.T) {
	a := newTestApp(t)

	err := a.WithTransaction(t.Context(), func(ctx context.Context, env *app) error {
		enqueueErr := env.EnqueueUserBannedNotice(ctx, 1)
		require.NoError(t, enqueueErr)

		return errTestRollback
	})
	require.ErrorIs(t, err, errTestRollback)

	msg, err := a.queue.Receive(t.Context())
	require.NoError(t, err)
	assert.Nil(t, msg, "the enqueue survived the rollback")
}

func TestAnEnqueueInsideACommittedTransactionSurvives(t *testing.T) {
	a := newTestApp(t)

	err := a.WithTransaction(t.Context(), func(ctx context.Context, env *app) error {
		return env.EnqueueUserBannedNotice(ctx, 1)
	})
	require.NoError(t, err)

	msg, err := a.queue.Receive(t.Context())
	require.NoError(t, err, "the enqueue did not commit with its transaction")
	assert.NotNil(t, msg)
}

func TestAStampRoundTripsThroughSQLite(t *testing.T) {
	a := newTestApp(t)

	issued := dbtime.At(time.Date(2026, time.August, 10, 9, 30, 15, 0, time.UTC))

	params := testUser("d")
	params.LastSigninAt = &issued

	_, err := a.WriteQueries.DBUpsertUserByOIDC(t.Context(), params)
	require.NoError(t, err)

	identity, err := a.Queries.DBGetIdentityByOIDC(t.Context(), identityOf("d"))
	require.NoError(t, err)

	if assert.NotNil(t, identity.LastSigninAt, "an integer time column must read back as a stamp") {
		assert.True(t, issued.Equal(identity.LastSigninAt.Time), "stored %s, read back %s", issued, identity.LastSigninAt)
	}
}
