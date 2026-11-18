package notifier_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

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

func logConfig() notifier.Config {
	return notifier.Config{
		From:    "noreply@example.invalid",
		Origin:  "https://panel.example",
		Timeout: time.Second,
	}
}

func relayConfig(endpoint string, timeout time.Duration) notifier.Config {
	config := logConfig()
	config.Endpoint = notifier.Endpoint(endpoint)
	config.Timeout = timeout

	return config
}

type relayStub struct {
	mu        sync.Mutex
	delivered map[string]struct{}
	keys      []string
	stall     bool
}

func newRelayStub() *relayStub {
	return &relayStub{delivered: map[string]struct{}{}}
}

func (r *relayStub) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var message struct {
		To string `json:"to"`
	}

	err := json.NewDecoder(req.Body).Decode(&message)
	if err != nil || message.To == "" {
		http.Error(w, "not a notice", http.StatusBadRequest)
		return
	}

	key := req.Header.Get("Idempotency-Key")

	r.mu.Lock()
	r.keys = append(r.keys, key)
	_, seen := r.delivered[key]

	if !seen {
		r.delivered[key] = struct{}{}
	}

	stall := r.stall && !seen
	r.mu.Unlock()

	if stall {
		<-req.Context().Done()
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (r *relayStub) deliveries() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return len(r.delivered)
}

func TestNotifyUserBannedCountsWhatItDelivered(t *testing.T) {
	env := &envStub{}
	sender := notifier.New(env, logConfig())

	require.NoError(t, sender.NotifyUserBanned(t.Context(), notifier.NewIdempotencyKey(), "user@example.com", "spam"))
	assert.Equal(t, []string{string(notifier.KindUserBanned)}, env.kinds)
}

func TestDeliveryToNobodyIsAFailureRatherThanASilentDrop(t *testing.T) {
	env := &envStub{}
	sender := notifier.New(env, logConfig())

	require.Error(t, sender.NotifyUserBanned(t.Context(), notifier.NewIdempotencyKey(), "", "spam"))
	assert.Empty(t, env.kinds, "a notice that went nowhere must not be counted as sent")
}

func TestDeliveryWithoutAKeyIsRefusedBeforeItLeaves(t *testing.T) {
	relay := newRelayStub()
	server := httptest.NewServer(relay)
	t.Cleanup(server.Close)

	sender := notifier.New(&envStub{}, relayConfig(server.URL, time.Second))

	require.Error(t, sender.NotifyUserBanned(t.Context(), "", "user@example.com", "spam"))
	assert.Empty(t, relay.keys, "a notice without a key must not reach the relay")
}

func TestARetryAfterATimeoutCarriesTheSameKeyAndDeliversOnce(t *testing.T) {
	relay := newRelayStub()
	relay.stall = true

	server := httptest.NewServer(relay)
	t.Cleanup(server.Close)

	env := &envStub{}
	sender := notifier.New(env, relayConfig(server.URL, 100*time.Millisecond))
	key := notifier.NewIdempotencyKey()

	err := sender.NotifyUserBanned(t.Context(), key, "user@example.com", "spam")
	require.Error(t, err, "a timed-out post must not count as delivered")
	assert.Contains(t, err.Error(), "unknown")
	assert.Empty(t, env.kinds)
	assert.Equal(t, 1, relay.deliveries(), "the relay accepted the notice before the response was lost")

	require.NoError(t, sender.NotifyUserBanned(t.Context(), key, "user@example.com", "spam"))

	assert.Equal(t, []string{string(key), string(key)}, relay.keys, "the retry must resend the key the first attempt carried")
	assert.Equal(t, 1, relay.deliveries(), "the retry must not create a second notice")
	assert.Equal(t, []string{string(notifier.KindUserBanned)}, env.kinds)
}

func TestAnUnexpectedRefusalCarriesOneCappedLineOfTheBody(t *testing.T) {
	body := strings.Repeat("x", 500) + "\nsecond line with someone else's data"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, body, http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	sender := notifier.New(&envStub{}, relayConfig(server.URL, time.Second))

	err := sender.NotifyUserBanned(t.Context(), notifier.NewIdempotencyKey(), "user@example.com", "spam")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 500: "+strings.Repeat("x", 200))
	assert.NotContains(t, err.Error(), strings.Repeat("x", 201))
	assert.NotContains(t, err.Error(), "second line")
}

func TestAnExpectedRefusalCarriesNoBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "a request with this key is in progress for user@example.com", http.StatusConflict)
	}))
	t.Cleanup(server.Close)

	sender := notifier.New(&envStub{}, relayConfig(server.URL, time.Second))

	err := sender.NotifyUserBanned(t.Context(), notifier.NewIdempotencyKey(), "user@example.com", "spam")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 409")
	assert.NotContains(t, err.Error(), "in progress")
}
