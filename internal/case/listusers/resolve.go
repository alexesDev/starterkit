package listusers

import (
	"context"
	"fmt"

	"starterkit/internal/db"
	"starterkit/internal/graph/model"
	"starterkit/internal/page"
)

//go:generate go tool github.com/matryer/moq -out mocks_test.go -pkg listusers_test . Env

type Env interface {
	PageLimits() page.Limits
	DBListUsers(ctx context.Context, arg db.DBListUsersParams) ([]db.DBListUsersRow, error)
}

type (
	Input  = page.Args
	Result = model.UsersConnection
)

func Resolve(ctx context.Context, env Env, input Input) (*Result, error) {
	window := env.PageLimits().Window(input)

	rows, err := env.DBListUsers(ctx, db.DBListUsersParams{Before: window.Before, PageLimit: window.Probe()})
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	nodes, info := page.Cut(window, rows, userID)

	return &Result{Nodes: nodes, PageInfo: info}, nil
}

func userID(row db.DBListUsersRow) int64 {
	return row.ID
}
