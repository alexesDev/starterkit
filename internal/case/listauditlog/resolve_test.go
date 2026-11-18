package listauditlog_test

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"starterkit/internal/case/listauditlog"
	"starterkit/internal/db"
	"starterkit/internal/page"
)

func rows(top int64, n int) []db.AuditLog {
	out := make([]db.AuditLog, 0, n)
	for i := range n {
		out = append(out, db.AuditLog{ID: top - int64(i)})
	}

	return out
}

func envReturning(found []db.AuditLog, seen *db.DBListAuditLogParams) *EnvMock {
	return &EnvMock{
		PageLimitsFunc: func() page.Limits {
			return page.Limits{Default: 50, Max: 500}
		},
		DBListAuditLogFunc: func(_ context.Context, arg db.DBListAuditLogParams) ([]db.AuditLog, error) {
			*seen = arg
			return found, nil
		},
	}
}

func first(n int64) *int64 {
	return &n
}

func TestResolveAsksTheFirstPageWithTheProbeRow(t *testing.T) {
	var asked db.DBListAuditLogParams

	env := envReturning(rows(10, 3), &asked)

	_, err := listauditlog.Resolve(t.Context(), env, listauditlog.Input{First: first(10)})
	require.NoError(t, err)

	require.Equal(t, db.DBListAuditLogParams{Before: math.MaxInt64, PageLimit: 11}, asked)
}

func TestResolvePagesOnFromTheCursorItIssued(t *testing.T) {
	var asked db.DBListAuditLogParams

	env := envReturning(rows(90, 4), &asked)

	result, err := listauditlog.Resolve(t.Context(), env, listauditlog.Input{First: first(3)})
	require.NoError(t, err)

	require.Equal(t, []int64{90, 89, 88}, entryIDs(result.Nodes))
	require.True(t, result.PageInfo.HasNextPage)
	require.NotNil(t, result.PageInfo.EndCursor)

	_, err = listauditlog.Resolve(t.Context(), env, listauditlog.Input{First: first(3), After: result.PageInfo.EndCursor})
	require.NoError(t, err)

	require.Equal(t, int64(88), asked.Before, "the next page must start below the last row shown")
}

func TestResolveLeavesTheCursorUnsetOnAnEmptyPage(t *testing.T) {
	var asked db.DBListAuditLogParams

	env := envReturning(nil, &asked)

	result, err := listauditlog.Resolve(t.Context(), env, listauditlog.Input{})
	require.NoError(t, err)

	require.Empty(t, result.Nodes)
	require.Nil(t, result.PageInfo.EndCursor)
	require.Len(t, env.DBListAuditLogCalls(), 1)
}

func TestResolveReportsAFailedRead(t *testing.T) {
	boom := errors.New("disk on fire")

	env := &EnvMock{
		PageLimitsFunc: func() page.Limits {
			return page.Limits{Default: 50, Max: 500}
		},
		DBListAuditLogFunc: func(context.Context, db.DBListAuditLogParams) ([]db.AuditLog, error) {
			return nil, boom
		},
	}

	_, err := listauditlog.Resolve(t.Context(), env, listauditlog.Input{})
	require.ErrorIs(t, err, boom)
}

func entryIDs(entries []db.AuditLog) []int64 {
	out := make([]int64, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.ID)
	}

	return out
}
