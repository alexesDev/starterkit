package createmcpkey

import (
	"context"
	"fmt"
	"strings"
	"time"

	ozzo "github.com/go-ozzo/ozzo-validation/v4"

	"starterkit/internal/db"
	"starterkit/internal/dbtime"
	"starterkit/internal/graph/model"
	"starterkit/internal/mcpkey"
)

//go:generate go tool github.com/matryer/moq -out mocks_test.go -pkg createmcpkey_test . Env

type Env interface {
	CurrentAdmin(ctx context.Context) (int64, error)
	GenerateMCPKey() (mcpkey.Secret, error)
	DBInsertMCPKey(ctx context.Context, arg db.DBInsertMCPKeyParams) (db.MCPKey, error)
	Now() time.Time
}

type (
	Input   = model.CreateMCPKeyInput
	Payload = model.CreateMCPKeyOrErrorPayload
)

func validate(input *Input) *model.ErrorPayload {
	return model.NewOzzoError(ozzo.ValidateStruct(input,
		ozzo.Field(&input.Name, ozzo.Required, ozzo.Length(1, 100)),
	))
}

func Resolve(ctx context.Context, env Env, input Input) (Payload, error) {
	input.Name = strings.TrimSpace(input.Name)

	invalid := validate(&input)
	if invalid != nil {
		return invalid, nil
	}

	owner, err := env.CurrentAdmin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve the key's owner: %w", err)
	}

	secret, err := env.GenerateMCPKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate a key: %w", err)
	}

	insertParams := db.DBInsertMCPKeyParams{
		UserID:    owner,
		Name:      input.Name,
		KeyHash:   secret.Hash(),
		CreatedAt: dbtime.At(env.Now()),
	}

	stored, err := env.DBInsertMCPKey(ctx, insertParams)
	if err != nil {
		return nil, fmt.Errorf("failed to insert the key: %w", err)
	}

	return &model.CreateMCPKeyPayload{MCPKeyID: stored.ID, Key: string(secret)}, nil
}
