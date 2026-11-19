package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"maragu.dev/goqite"
	goqitejobs "maragu.dev/goqite/jobs"

	"starterkit/internal/case/makeadmin"
	"starterkit/internal/case/notifyuserbanned"
	"starterkit/internal/db"
	"starterkit/internal/dbtime"
	"starterkit/internal/graph/model"
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
		Name:       jobQueue,
		MaxReceive: jobMaxReceive,
		Timeout:    5 * time.Minute,
	})

	runnerOpts := goqitejobs.NewRunnerOpts{
		Limit:        2,
		PollInterval: 10 * time.Millisecond,
		Queue:        queue,
	}

	state := &appState{
		conns:     conns,
		log:       logger.Discard(),
		queue:     queue,
		jobNames:  map[string]struct{}{},
		jobRunner: goqitejobs.NewRunner(runnerOpts),
	}

	a := &app{
		appState:     state,
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
	require.True(t, db.IsNotFound(err), "the row survived a rollback: %v", err)
}

func TestNestedWithTransactionFails(t *testing.T) {
	a := newTestApp(t)

	err := a.WithTransaction(t.Context(), func(ctx context.Context, env *app) error {
		return env.WithTransaction(ctx, func(context.Context, *app) error {
			require.Fail(t, "the inner transaction should never run")
			return nil
		})
	})

	require.ErrorIs(t, err, ErrNestedTransaction)
}

func TestNestedWithTransactionFailsViaTheOuterEnv(t *testing.T) {
	a := newTestApp(t)

	err := a.WithTransaction(t.Context(), func(ctx context.Context, _ *app) error {
		return a.WithTransaction(ctx, func(context.Context, *app) error {
			require.Fail(t, "the inner transaction should never run")
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
		require.True(t, db.IsNotFound(outsideErr), "an uncommitted write is visible outside the transaction: %v", outsideErr)

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
	require.Nil(t, msg, "the enqueue survived the rollback")
}

func TestAnEnqueueInsideACommittedTransactionSurvives(t *testing.T) {
	a := newTestApp(t)

	err := a.WithTransaction(t.Context(), func(ctx context.Context, env *app) error {
		return env.EnqueueUserBannedNotice(ctx, 1)
	})
	require.NoError(t, err)

	msg, err := a.queue.Receive(t.Context())
	require.NoError(t, err, "the enqueue did not commit with its transaction")
	require.NotNil(t, msg)
}

func enqueuedNotice(t *testing.T, msg *goqite.Message) notifyuserbanned.Input {
	t.Helper()

	var envelope struct {
		Name    string
		Message []byte
	}

	require.NoError(t, gob.NewDecoder(bytes.NewReader(msg.Body)).Decode(&envelope))
	require.Equal(t, jobNotifyUserBanned, envelope.Name)

	var notice notifyuserbanned.Input

	require.NoError(t, json.Unmarshal(envelope.Message, &notice))

	return notice
}

func TestARedeliveredNoticeCarriesTheKeyItWasEnqueuedWith(t *testing.T) {
	a := newTestApp(t)

	err := a.WithTransaction(t.Context(), func(ctx context.Context, env *app) error {
		return env.EnqueueUserBannedNotice(ctx, 1)
	})
	require.NoError(t, err)

	first, err := a.queue.Receive(t.Context())
	require.NoError(t, err)
	require.NotNil(t, first)

	require.NoError(t, a.queue.Extend(t.Context(), first.ID, 0), "failed to make the message visible again, as a failed run does")

	second, err := a.queue.Receive(t.Context())
	require.NoError(t, err)
	require.NotNil(t, second, "the message was not redelivered")

	minted := enqueuedNotice(t, first)
	retried := enqueuedNotice(t, second)

	require.NotEmpty(t, minted.Key, "the enqueue must mint the key")
	require.Equal(t, minted.Key, retried.Key, "a retry must carry the key its first run carried")
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

	require.NotNil(t, identity.LastSigninAt, "an integer time column must read back as a stamp")
	require.True(t, issued.Equal(identity.LastSigninAt.Time), "stored %s, read back %s", issued, identity.LastSigninAt)
}

func TestMakingAnAdminTwiceIsADomainAnswer(t *testing.T) {
	a := newTestApp(t)

	user, err := a.WriteQueries.DBUpsertUserByOIDC(t.Context(), testUser("e"))
	require.NoError(t, err)

	input := makeadmin.Input{UserID: user.ID}

	first, err := a.MakeAdmin(t.Context(), input)
	require.NoError(t, err)
	require.IsType(t, &model.MakeAdminPayload{}, first)

	second, err := a.MakeAdmin(t.Context(), input)
	require.NoError(t, err, "a unique violation surfaced as an infrastructure failure")
	require.Equal(t, &model.ErrorPayload{Message: "user is already an admin"}, second)
}
