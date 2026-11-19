package revokemcpkey

import (
	"context"
	"fmt"
	"time"

	ozzo "github.com/go-ozzo/ozzo-validation/v4"

	"starterkit/internal/db"
	"starterkit/internal/dbtime"
	"starterkit/internal/graph/model"
)

//go:generate go tool github.com/matryer/moq -out mocks_test.go -pkg revokemcpkey_test . Env

type Env interface {
	DBRevokeMCPKey(ctx context.Context, arg db.DBRevokeMCPKeyParams) (int64, error)
	DBGetMCPKeyByID(ctx context.Context, id int64) (db.DBGetMCPKeyByIDRow, error)
	Now() time.Time
}

type (
	Input   = model.RevokeMCPKeyInput
	Payload = model.RevokeMCPKeyOrErrorPayload
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

	at := dbtime.At(env.Now())
	revokeParams := db.DBRevokeMCPKeyParams{ID: input.MCPKeyID, RevokedAt: &at}

	revoked, err := env.DBRevokeMCPKey(ctx, revokeParams)
	if err != nil {
		return nil, fmt.Errorf("failed to revoke the key: %w", err)
	}

	if revoked == 1 {
		return &model.RevokeMCPKeyPayload{MCPKeyID: input.MCPKeyID}, nil
	}

	_, err = env.DBGetMCPKeyByID(ctx, input.MCPKeyID)
	if db.IsNotFound(err) {
		return &model.ErrorPayload{Message: "key not found"}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load the key: %w", err)
	}

	return &model.ErrorPayload{Message: "the key is already revoked"}, nil
}
