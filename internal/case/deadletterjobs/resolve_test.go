package deadletterjobs_test

import (
	"bytes"
	"context"
	"encoding/gob"
	"testing"

	"github.com/stretchr/testify/require"

	"starterkit/internal/case/deadletterjobs"
	"starterkit/internal/db"
	"starterkit/internal/logger"
)

type envelope struct {
	Name    string
	Message []byte
}

func body(t *testing.T, name string) []byte {
	t.Helper()

	var buf bytes.Buffer

	err := gob.NewEncoder(&buf).Encode(envelope{Name: name, Message: []byte("{}")})
	require.NoError(t, err)

	return buf.Bytes()
}

func TestResolveSetsAsideWhatNoHandlerCanRun(t *testing.T) {
	rows := []db.DBListQueuedJobsRow{
		{ID: "m_known", Body: body(t, "purge-audit-log")},
		{ID: "m_renamed", Body: body(t, "purge-old-audit")},
		{ID: "m_garbage", Body: []byte("not gob")},
	}

	var moved []db.DBMoveJobToQueueParams

	env := &EnvMock{
		DBListQueuedJobsFunc: func(_ context.Context, queue string) ([]db.DBListQueuedJobsRow, error) {
			require.Equal(t, "jobs", queue)
			return rows, nil
		},
		DBMoveJobToQueueFunc: func(_ context.Context, arg db.DBMoveJobToQueueParams) error {
			moved = append(moved, arg)
			return nil
		},
		JobRegisteredFunc: func(name string) bool { return name == "purge-audit-log" },
		LoggerFunc:        logger.Discard,
	}

	input := deadletterjobs.Input{Queue: "jobs", DeadQueue: "jobs-dead"}

	require.NoError(t, deadletterjobs.Resolve(t.Context(), env, input))
	require.Equal(t, []db.DBMoveJobToQueueParams{
		{Queue: "jobs-dead", ID: "m_renamed"},
		{Queue: "jobs-dead", ID: "m_garbage"},
	}, moved)
}

func TestResolveLeavesARegisteredQueueAlone(t *testing.T) {
	env := &EnvMock{
		DBListQueuedJobsFunc: func(context.Context, string) ([]db.DBListQueuedJobsRow, error) {
			return []db.DBListQueuedJobsRow{{ID: "m_known", Body: body(t, "purge-audit-log")}}, nil
		},
		JobRegisteredFunc: func(string) bool { return true },
		LoggerFunc:        logger.Discard,
	}

	require.NoError(t, deadletterjobs.Resolve(t.Context(), env, deadletterjobs.Input{Queue: "jobs", DeadQueue: "jobs-dead"}))
	require.Empty(t, env.DBMoveJobToQueueCalls())
}
