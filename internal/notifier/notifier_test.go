package notifier_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"starterkit/internal/logger"
	"starterkit/internal/notifier"
)

type envStub struct {
	kinds []string
}

func (e *envStub) Logger() logger.Logger {
	return logger.Discard()
}

func (e *envStub) MetricsIncreaseNotification(_ context.Context, kind string) {
	e.kinds = append(e.kinds, kind)
}

func TestNotifyUserBannedCountsWhatItDelivered(t *testing.T) {
	env := &envStub{}
	sender := notifier.New(env, "noreply@example.invalid", "https://panel.example")

	require.NoError(t, sender.NotifyUserBanned(t.Context(), "user@example.com", "spam"))
	assert.Equal(t, []string{string(notifier.KindUserBanned)}, env.kinds)
}

func TestDeliveryToNobodyIsAFailureRatherThanASilentDrop(t *testing.T) {
	env := &envStub{}
	sender := notifier.New(env, "noreply@example.invalid", "https://panel.example")

	require.Error(t, sender.NotifyUserBanned(t.Context(), "", "spam"))
	assert.Empty(t, env.kinds, "a notice that went nowhere must not be counted as sent")
}
