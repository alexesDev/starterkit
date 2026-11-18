// Package notifier delivers a notice to a person.
//
// It is the kit's worked example of a service object: it declares its own Env,
// takes it at construction, and is embedded into `app`, so its methods are
// promoted and a case reaches them through its own Env like any other. With no
// endpoint configured the delivery writes to the log; with one it posts the
// notice to an HTTP relay, carrying the idempotency key the enqueue minted.
// See docs/env-pattern.md and docs/jobs-and-cron.md.
package notifier

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"starterkit/internal/logger"
)

type Address string

type Origin string

type Kind string

type Endpoint string

type IdempotencyKey string

const KindUserBanned Kind = "user_banned"

const errorBodyLimit = 200

type Env interface {
	Logger() logger.Logger
	MetricsIncreaseNotification(ctx context.Context, kind string)
}

type Config struct {
	From     Address
	Origin   Origin
	Endpoint Endpoint
	Timeout  time.Duration
}

type Notifier struct {
	env    Env
	config Config
	client *http.Client
}

type notice struct {
	Kind Kind    `json:"kind"`
	From Address `json:"from"`
	To   Address `json:"to"`
	Text string  `json:"text"`
}

func New(env Env, config Config) *Notifier {
	return &Notifier{
		env:    env,
		config: config,
		client: &http.Client{Timeout: config.Timeout},
	}
}

func NewIdempotencyKey() IdempotencyKey {
	return IdempotencyKey(rand.Text())
}

func (n *Notifier) NotifyUserBanned(ctx context.Context, key IdempotencyKey, to Address, reason string) error {
	message := notice{
		Kind: KindUserBanned,
		From: n.config.From,
		To:   to,
		Text: "your access to " + string(n.config.Origin) + " was revoked: " + reason,
	}

	return n.deliver(ctx, key, message)
}

func (n *Notifier) deliver(ctx context.Context, key IdempotencyKey, message notice) error {
	if message.To == "" {
		return errors.New("cannot deliver " + string(message.Kind) + " to an empty address")
	}

	if key == "" {
		return errors.New("cannot deliver " + string(message.Kind) + " without an idempotency key")
	}

	if n.config.Endpoint == "" {
		n.env.Logger().Info("notice delivered",
			"kind", string(message.Kind),
			"key", string(key),
			"from", string(message.From),
			"to", string(message.To),
			"body", message.Text)
	} else {
		err := n.post(ctx, key, message)
		if err != nil {
			return err
		}
	}

	n.env.MetricsIncreaseNotification(ctx, string(message.Kind))

	return nil
}

func (n *Notifier) post(ctx context.Context, key IdempotencyKey, message notice) error {
	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal %s: %w", message.Kind, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, string(n.config.Endpoint), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to build the %s request: %w", message.Kind, err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", string(key))

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("outcome of %s %s is unknown, a retry resends the same key: %w", message.Kind, key, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	status := strconv.Itoa(resp.StatusCode)

	switch resp.StatusCode {
	case http.StatusConflict, http.StatusNotFound, http.StatusTooManyRequests:
		return errors.New("the relay refused " + string(message.Kind) + " " + string(key) + " with status " + status)
	default:
		return errors.New("the relay refused " + string(message.Kind) + " " + string(key) + " with status " + status + ": " + errorBodyLine(resp))
	}
}

func errorBodyLine(resp *http.Response) string {
	head, err := io.ReadAll(io.LimitReader(resp.Body, errorBodyLimit*4))
	if err != nil && len(head) == 0 {
		return "unreadable body: " + err.Error()
	}

	line, _, _ := strings.Cut(string(head), "\n")
	line = strings.TrimSuffix(line, "\r")

	if len(line) > errorBodyLimit {
		line = strings.ToValidUTF8(line[:errorBodyLimit], "")
	}

	return line
}
