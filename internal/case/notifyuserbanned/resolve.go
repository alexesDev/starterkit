package notifyuserbanned

import (
	"context"
	"fmt"

	"starterkit/internal/db"
	"starterkit/internal/notifier"
)

//go:generate go tool github.com/matryer/moq -out mocks_test.go -pkg notifyuserbanned_test . Env

type Env interface {
	DBGetUserByID(ctx context.Context, id int64) (db.User, error)
	DBGetUserBan(ctx context.Context, userID int64) (db.UserBan, error)
	NotifyUserBanned(ctx context.Context, to notifier.Address, reason string) error
}

type Input struct {
	UserID int64 `json:"userId"`
}

// Resolve tells a banned person why. It runs from the queue, after the
// transaction that wrote the ban has committed, so both the user and the ban
// may be gone by the time it runs: an unban between the enqueue and the run is
// the answer, not a failure, and retrying would not bring the row back.
func Resolve(ctx context.Context, env Env, input Input) error {
	user, err := env.DBGetUserByID(ctx, input.UserID)
	if err != nil {
		if db.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("failed to load user %d: %w", input.UserID, err)
	}

	ban, err := env.DBGetUserBan(ctx, input.UserID)
	if err != nil {
		if db.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("failed to load the ban of user %d: %w", input.UserID, err)
	}

	err = env.NotifyUserBanned(ctx, notifier.Address(user.Email), ban.Reason)
	if err != nil {
		return fmt.Errorf("failed to notify user %d: %w", input.UserID, err)
	}

	return nil
}
