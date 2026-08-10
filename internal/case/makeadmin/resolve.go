package makeadmin

import (
	"context"
	"fmt"
	"time"

	ozzo "github.com/go-ozzo/ozzo-validation/v4"

	"starterkit/internal/db"
	"starterkit/internal/dbtime"
	"starterkit/internal/graph/model"
)

//go:generate go tool github.com/matryer/moq -out mocks_test.go -pkg makeadmin_test . Env

type Env interface {
	DBGetAdminUserByID(ctx context.Context, id int64) (db.DBGetAdminUserByIDRow, error)
	DBInsertAdmin(ctx context.Context, arg db.DBInsertAdminParams) error
	MetricsIncreaseAdminChanged(ctx context.Context, action string)
	Now() time.Time
}

type (
	Input   = model.MakeAdminInput
	Payload = model.MakeAdminOrErrorPayload
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

	user, err := env.DBGetAdminUserByID(ctx, input.UserID)
	if err != nil {
		if db.IsNotFound(err) {
			return &model.ErrorPayload{Message: "user not found"}, nil
		}

		return nil, fmt.Errorf("failed to load user: %w", err)
	}

	if user.BanReason != "" {
		return &model.ErrorPayload{Message: "user is banned"}, nil
	}

	adminParams := db.DBInsertAdminParams{UserID: input.UserID, CreatedAt: dbtime.At(env.Now())}

	err = env.DBInsertAdmin(ctx, adminParams)
	if err != nil {
		return nil, fmt.Errorf("failed to insert admin: %w", err)
	}

	env.MetricsIncreaseAdminChanged(ctx, "make")

	return &model.MakeAdminPayload{UserID: input.UserID}, nil
}
