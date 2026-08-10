package banuser

import (
	"context"
	"fmt"
	"strings"
	"time"

	ozzo "github.com/go-ozzo/ozzo-validation/v4"

	"starterkit/internal/db"
	"starterkit/internal/dbtime"
	"starterkit/internal/graph/model"
)

//go:generate go tool github.com/matryer/moq -out mocks_test.go -pkg banuser_test . Env

type Env interface {
	DBGetUserByID(ctx context.Context, id int64) (db.User, error)
	DBInsertUserBan(ctx context.Context, arg db.DBInsertUserBanParams) (db.UserBan, error)
	CurrentUserID(ctx context.Context) *int64
	EnqueueUserBannedNotice(ctx context.Context, userID int64) error
	Now() time.Time
	MetricsIncreaseUserBanned(ctx context.Context)
}

type (
	Input   = model.BanUserInput
	Payload = model.BanUserOrErrorPayload
)

func validate(input *Input) *model.ErrorPayload {
	return model.NewOzzoError(ozzo.ValidateStruct(input,
		ozzo.Field(&input.UserID, ozzo.Required, ozzo.Min(int64(1))),
		ozzo.Field(&input.Reason, ozzo.Required, ozzo.Length(1, 500)),
	))
}

func Resolve(ctx context.Context, env Env, input Input) (Payload, error) {
	input.Reason = strings.TrimSpace(input.Reason)

	invalid := validate(&input)
	if invalid != nil {
		return invalid, nil
	}

	actor := env.CurrentUserID(ctx)
	if actor != nil && *actor == input.UserID {
		return &model.ErrorPayload{Message: "you cannot ban yourself"}, nil
	}

	_, err := env.DBGetUserByID(ctx, input.UserID)
	if err != nil {
		if db.IsNotFound(err) {
			return &model.ErrorPayload{Message: "user not found"}, nil
		}

		return nil, fmt.Errorf("failed to load user: %w", err)
	}

	banParams := db.DBInsertUserBanParams{
		UserID:    input.UserID,
		BannedBy:  actor,
		Reason:    input.Reason,
		CreatedAt: dbtime.At(env.Now()),
	}

	_, err = env.DBInsertUserBan(ctx, banParams)
	if err != nil {
		return nil, fmt.Errorf("failed to insert user ban: %w", err)
	}

	err = env.EnqueueUserBannedNotice(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to enqueue the ban notice: %w", err)
	}

	env.MetricsIncreaseUserBanned(ctx)

	return &model.BanUserPayload{UserID: input.UserID}, nil
}
