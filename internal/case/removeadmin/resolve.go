package removeadmin

import (
	"context"
	"fmt"

	ozzo "github.com/go-ozzo/ozzo-validation/v4"

	"starterkit/internal/db"
	"starterkit/internal/graph/model"
)

//go:generate go tool github.com/matryer/moq -out mocks_test.go -pkg removeadmin_test . Env

type Env interface {
	DBDeleteAdmin(ctx context.Context, userID int64) (int64, error)
	DBGetAdminByUserID(ctx context.Context, userID int64) (db.Admin, error)
	CurrentUserID(ctx context.Context) *int64
	MetricsIncreaseAdminChanged(ctx context.Context, action string)
}

type (
	Input   = model.RemoveAdminInput
	Payload = model.RemoveAdminOrErrorPayload
)

func validate(input *Input) *model.ErrorPayload {
	return model.NewOzzoError(ozzo.ValidateStruct(input,
		ozzo.Field(&input.UserID, ozzo.Required, ozzo.Min(int64(1))),
	))
}

func Resolve(ctx context.Context, env Env, input Input) (Payload, error) {
	invalid := validate(&input)
	if invalid != nil {
		return invalid, nil
	}

	actor := env.CurrentUserID(ctx)
	if actor != nil && *actor == input.UserID {
		return &model.ErrorPayload{Message: "you cannot remove your own admin access"}, nil
	}

	removed, err := env.DBDeleteAdmin(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete admin: %w", err)
	}

	if removed == 0 {
		_, err = env.DBGetAdminByUserID(ctx, input.UserID)
		if err == nil {
			return &model.ErrorPayload{Message: "the last admin cannot be removed"}, nil
		}

		if !db.IsNotFound(err) {
			return nil, fmt.Errorf("failed to load admin: %w", err)
		}

		return &model.RemoveAdminPayload{UserID: input.UserID}, nil
	}

	env.MetricsIncreaseAdminChanged(ctx, "remove")

	return &model.RemoveAdminPayload{UserID: input.UserID}, nil
}
