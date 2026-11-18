// Package notifier delivers a notice to a person.
//
// It is the kit's worked example of a service object: it declares its own Env,
// takes it at construction, and is embedded into `app`, so its methods are
// promoted and a case reaches them through its own Env like any other. The
// delivery here writes to the log; replacing that body with an SMTP dial is
// the whole change, and no case moves. Every notice carries the idempotency
// key its enqueue minted, so a real delivery hands it to the other side. See
// docs/env-pattern.md and docs/jobs-and-cron.md.
package notifier

import (
	"context"
	"crypto/rand"
	"errors"

	"starterkit/internal/confread"
	"starterkit/internal/logger"
)

type Address string

type Origin string

type Kind string

type IdempotencyKey string

const KindUserBanned Kind = "user_banned"

type Config struct {
	From   Address
	Origin Origin
}

func DefaultConfig() Config {
	return Config{From: "noreply@example.invalid"}
}

func (c *Config) Register(r *confread.Reader) {
	*c = DefaultConfig()

	setFrom := func(raw string) error {
		c.From = Address(raw)

		return nil
	}

	r.Func("notifier-from", string(c.From), "sender address every notice is delivered from", setFrom)
}

func (c *Config) Check(r *confread.Reader) {
	r.Require("notifier-from", c.From != "", "is empty")
}

type Env interface {
	Logger() logger.Logger
	MetricsIncreaseNotification(ctx context.Context, kind string)
}

type Notifier struct {
	env    Env
	config Config
}

func New(env Env, config Config) *Notifier {
	return &Notifier{env: env, config: config}
}

func NewIdempotencyKey() IdempotencyKey {
	return IdempotencyKey(rand.Text())
}

func (n *Notifier) NotifyUserBanned(ctx context.Context, key IdempotencyKey, to Address, reason string) error {
	return n.deliver(ctx, KindUserBanned, key, to,
		"your access to "+string(n.config.Origin)+" was revoked: "+reason)
}

func (n *Notifier) deliver(ctx context.Context, kind Kind, key IdempotencyKey, to Address, body string) error {
	if to == "" {
		return errors.New("cannot deliver " + string(kind) + " to an empty address")
	}

	if key == "" {
		return errors.New("cannot deliver " + string(kind) + " without an idempotency key")
	}

	n.env.Logger().Info("notice delivered",
		"kind", string(kind),
		"key", string(key),
		"from", string(n.config.From),
		"to", string(to),
		"body", body)

	n.env.MetricsIncreaseNotification(ctx, string(kind))

	return nil
}
