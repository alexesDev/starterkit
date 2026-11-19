package deadletterjobs

import (
	"context"
	"fmt"

	"starterkit/internal/db"
	"starterkit/internal/jobs"
	"starterkit/internal/logger"
)

//go:generate go tool github.com/matryer/moq -out mocks_test.go -pkg deadletterjobs_test . Env

type Env interface {
	DBListQueuedJobs(ctx context.Context, queue string) ([]db.DBListQueuedJobsRow, error)
	DBMoveJobToQueue(ctx context.Context, arg db.DBMoveJobToQueueParams) error
	JobRegistered(name string) bool
	Logger() logger.Logger
}

type Input struct {
	Queue     string
	DeadQueue string
}

const (
	reasonUnregistered = "unregistered"
	reasonUndecodable  = "undecodable"
)

func Resolve(ctx context.Context, env Env, input Input) error {
	rows, err := env.DBListQueuedJobs(ctx, input.Queue)
	if err != nil {
		return fmt.Errorf("failed to list queued jobs: %w", err)
	}

	for _, row := range rows {
		reason := reasonUnregistered

		name, decodeErr := jobs.Name(row.Body)
		if decodeErr != nil {
			reason = reasonUndecodable
		}

		if decodeErr == nil && env.JobRegistered(name) {
			continue
		}

		moveParams := db.DBMoveJobToQueueParams{Queue: input.DeadQueue, ID: row.ID}

		err = env.DBMoveJobToQueue(ctx, moveParams)
		if err != nil {
			return fmt.Errorf("failed to set aside job %s: %w", row.ID, err)
		}

		env.Logger().Warn("set aside a queued job no handler can run",
			"id", row.ID,
			"job", name,
			"reason", reason,
			"queue", input.DeadQueue)
	}

	return nil
}
