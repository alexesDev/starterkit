package unbanuser

import (
	"context"
	"fmt"

	ozzo "github.com/go-ozzo/ozzo-validation/v4"

	"starterkit/internal/graph/model"
)

//go:generate go tool github.com/matryer/moq -out mocks_test.go -pkg unbanuser_test . Env

type Env interface {
	DBDeleteUserBan(ctx context.Context, userID int64) error
}

type (
	Input   = model.UnbanUserInput
	Payload = model.UnbanUserOrErrorPayload
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

	err := env.DBDeleteUserBan(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete user ban: %w", err)
	}

	return &model.UnbanUserPayload{UserID: input.UserID}, nil
}
