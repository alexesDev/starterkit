// Package jobs is the typed seam over the SQLite-backed queue.
//
// Register captures the case's Env in a closure, so the compiler proves the
// wiring at the registration site. The generic parameter only unmarshals the
// payload; it is not standing in for a runtime type assertion. See
// docs/jobs-and-cron.md.
package jobs

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
)

type Env interface {
	RegisterJob(id string, handler func(ctx context.Context, body []byte) error)
	MetricsIncreaseJobFinished(ctx context.Context, job string, err error)
}

func Register[P any](env Env, jobID string, handler func(context.Context, P) error) {
	env.RegisterJob(jobID, func(ctx context.Context, body []byte) error {
		var params P

		err := json.Unmarshal(body, &params)
		if err != nil {
			return fmt.Errorf("failed to unmarshal %s params: %w", jobID, err)
		}

		err = handler(ctx, params)
		env.MetricsIncreaseJobFinished(ctx, jobID, err)

		if err != nil {
			return fmt.Errorf("failed to run %s: %w", jobID, err)
		}

		return nil
	})
}

type envelope struct {
	Name    string
	Message []byte
}

func Name(body []byte) (string, error) {
	var decoded envelope

	err := gob.NewDecoder(bytes.NewReader(body)).Decode(&decoded)
	if err != nil {
		return "", fmt.Errorf("failed to decode a queued job: %w", err)
	}

	return decoded.Name, nil
}
