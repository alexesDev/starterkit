package deletemcpkey

import (
	"context"
	"fmt"

	ozzo "github.com/go-ozzo/ozzo-validation/v4"

	"starterkit/internal/graph/model"
)

//go:generate go tool github.com/matryer/moq -out mocks_test.go -pkg deletemcpkey_test . Env

type Env interface {
	DBDeleteMCPKey(ctx context.Context, id int64) (int64, error)
}

type (
	Input   = model.DeleteMCPKeyInput
	Payload = model.DeleteMCPKeyOrErrorPayload
)

func validate(input *Input) *model.ErrorPayload {
	return model.NewOzzoError(ozzo.ValidateStruct(input,
		ozzo.Field(&input.MCPKeyID, ozzo.Required, ozzo.Min(int64(1))),
	))
}

func Resolve(ctx context.Context, env Env, input Input) (Payload, error) {
	invalid := validate(&input)
	if invalid != nil {
		return invalid, nil
	}

	deleted, err := env.DBDeleteMCPKey(ctx, input.MCPKeyID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete the key: %w", err)
	}

	if deleted == 0 {
		return &model.ErrorPayload{Message: "key not found"}, nil
	}

	return &model.DeleteMCPKeyPayload{MCPKeyID: input.MCPKeyID}, nil
}
