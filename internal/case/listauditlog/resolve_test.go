package listauditlog_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"starterkit/internal/case/listauditlog"
	"starterkit/internal/db"
)

func rows(n int) []db.AuditLog {
	out := make([]db.AuditLog, 0, n)
	for i := range n {
		out = append(out, db.AuditLog{ID: int64(n - i)})
	}

	return out
}

func envReturning(page []db.AuditLog, seen *db.DBListAuditLogParams) *EnvMock {
	return &EnvMock{
		DBListAuditLogFunc: func(_ context.Context, arg db.DBListAuditLogParams) ([]db.AuditLog, error) {
			*seen = arg
			return page, nil
		},
		DBCountAuditLogFunc: func(context.Context) (int64, error) { return 120, nil },
	}
}

func TestResolveAsksForOneRowMoreThanTheLimit(t *testing.T) {
	var asked db.DBListAuditLogParams

	env := envReturning(rows(3), &asked)

	_, err := listauditlog.Resolve(t.Context(), env, listauditlog.Input{Limit: 10})
	require.NoError(t, err)

	assert.Equal(t, int64(11), asked.PageLimit)
}

func TestResolveTrimsTheProbeRowAndReportsAnotherPage(t *testing.T) {
	var asked db.DBListAuditLogParams

	env := envReturning(rows(4), &asked)

	result, err := listauditlog.Resolve(t.Context(), env, listauditlog.Input{Limit: 3})
	require.NoError(t, err)

	assert.Len(t, result.Nodes, 3, "the probe row leaked into the page")
	assert.True(t, result.PageInfo.HasNextPage)

	if assert.NotNil(t, result.PageInfo.EndCursor) {
		assert.Equal(t, result.Nodes[2].ID, *result.PageInfo.EndCursor)
	}
}

func TestResolveReportsNoNextPageWhenTheProbeFoundNothing(t *testing.T) {
	var asked db.DBListAuditLogParams

	env := envReturning(rows(2), &asked)

	result, err := listauditlog.Resolve(t.Context(), env, listauditlog.Input{Limit: 3})
	require.NoError(t, err)

	assert.Len(t, result.Nodes, 2)
	assert.False(t, result.PageInfo.HasNextPage)
}

func TestResolveClampsAnOversizedLimit(t *testing.T) {
	var asked db.DBListAuditLogParams

	env := envReturning(nil, &asked)

	_, err := listauditlog.Resolve(t.Context(), env, listauditlog.Input{Limit: 100000})
	require.NoError(t, err)

	assert.Equal(t, int64(501), asked.PageLimit)
}

func TestResolveFallsBackToTheDefaultLimit(t *testing.T) {
	var asked db.DBListAuditLogParams

	env := envReturning(nil, &asked)

	_, err := listauditlog.Resolve(t.Context(), env, listauditlog.Input{Limit: 0})
	require.NoError(t, err)

	assert.Equal(t, int64(51), asked.PageLimit)
}

func TestResolveTreatsAMissingCursorAsTheFirstPage(t *testing.T) {
	var asked db.DBListAuditLogParams

	env := envReturning(nil, &asked)

	_, err := listauditlog.Resolve(t.Context(), env, listauditlog.Input{Limit: 10})
	require.NoError(t, err)

	assert.Equal(t, int64(0), asked.Before)
}

func TestResolveLeavesTheCursorUnsetOnAnEmptyPage(t *testing.T) {
	var asked db.DBListAuditLogParams

	env := envReturning(nil, &asked)

	result, err := listauditlog.Resolve(t.Context(), env, listauditlog.Input{Limit: 10})
	require.NoError(t, err)

	assert.Nil(t, result.PageInfo.EndCursor)
	assert.Equal(t, int64(120), result.TotalCount)
}
