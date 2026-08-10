// Package notifier delivers a notice to a person.
//
// It is the kit's worked example of a service object: it declares its own Env,
// takes it at construction, and is embedded into `app`, so its methods are
// promoted and a case reaches them through its own Env like any other. The
// delivery here writes to the log; replacing that body with an SMTP dial is
// the whole change, and no case moves. See docs/env-pattern.md.
package notifier

import (
	"context"
	"errors"

	"starterkit/internal/logger"
)

type Address string

type Origin string

type Kind string

const KindUserBanned Kind = "user_banned"

type Env interface {
	Logger() logger.Logger
	MetricsIncreaseNotification(ctx context.Context, kind string)
}

type Notifier struct {
	env    Env
	from   Address
	origin Origin
}

func New(env Env, from Address, origin Origin) *Notifier {
	return &Notifier{env: env, from: from, origin: origin}
}

func (n *Notifier) NotifyUserBanned(ctx context.Context, to Address, reason string) error {
	return n.deliver(ctx, KindUserBanned, to,
		"your access to "+string(n.origin)+" was revoked: "+reason)
}

func (n *Notifier) deliver(ctx context.Context, kind Kind, to Address, body string) error {
	if to == "" {
		return errors.New("cannot deliver " + string(kind) + " to an empty address")
	}

	n.env.Logger().Info("notice delivered",
		"kind", string(kind),
		"from", string(n.from),
		"to", string(to),
		"body", body)

	n.env.MetricsIncreaseNotification(ctx, string(kind))

	return nil
}
