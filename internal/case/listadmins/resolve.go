package listadmins

import (
	"context"
	"fmt"

	"starterkit/internal/db"
	"starterkit/internal/graph/model"
	"starterkit/internal/page"
)

//go:generate go tool github.com/matryer/moq -out mocks_test.go -pkg listadmins_test . Env

type Env interface {
	PageLimits() page.Limits
	DBListAdmins(ctx context.Context, arg db.DBListAdminsParams) ([]db.DBListAdminsRow, error)
}

type (
	Input  = page.Args
	Result = model.AdminsConnection
)

func Resolve(ctx context.Context, env Env, input Input) (*Result, error) {
	window := env.PageLimits().Window(input)

	rows, err := env.DBListAdmins(ctx, db.DBListAdminsParams{Before: window.Before, PageLimit: window.Probe()})
	if err != nil {
		return nil, fmt.Errorf("failed to list admins: %w", err)
	}

	nodes, info := page.Cut(window, rows, adminID)

	return &Result{Nodes: nodes, PageInfo: info}, nil
}

func adminID(row db.DBListAdminsRow) int64 {
	return row.UserID
}
