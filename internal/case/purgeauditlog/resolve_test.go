package purgeauditlog_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"starterkit/internal/case/purgeauditlog"
	"starterkit/internal/dbtime"
)

func now() time.Time {
	return time.Date(2026, time.August, 10, 4, 0, 0, 0, time.UTC)
}

func TestResolveDeletesEverythingOlderThanTheRetention(t *testing.T) {
	var cutoff dbtime.Stamp

	env := &EnvMock{
		AuditRetentionFunc: func() time.Duration { return 30 * 24 * time.Hour },
		NowFunc:            now,
		DBDeleteAuditLogBeforeFunc: func(_ context.Context, before dbtime.Stamp) error {
			cutoff = before
			return nil
		},
	}

	require.NoError(t, purgeauditlog.Resolve(t.Context(), env))
	assert.True(t, now().Add(-30*24*time.Hour).Equal(cutoff.Time))
}

func TestResolveDoesNothingWhenRetentionIsDisabled(t *testing.T) {
	env := &EnvMock{
		AuditRetentionFunc: func() time.Duration { return 0 },
		NowFunc:            now,
	}

	require.NoError(t, purgeauditlog.Resolve(t.Context(), env))
	assert.Empty(t, env.DBDeleteAuditLogBeforeCalls(), "retention 0 must not delete anything")
}

func TestResolveDoesNothingWhenRetentionIsNegative(t *testing.T) {
	env := &EnvMock{
		AuditRetentionFunc: func() time.Duration { return -time.Hour },
		NowFunc:            now,
	}

	require.NoError(t, purgeauditlog.Resolve(t.Context(), env))
	assert.Empty(t, env.DBDeleteAuditLogBeforeCalls())
}
