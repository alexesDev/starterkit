package listmcpkeys

import (
	"context"
	"fmt"

	"starterkit/internal/db"
	"starterkit/internal/graph/model"
	"starterkit/internal/page"
)

//go:generate go tool github.com/matryer/moq -out mocks_test.go -pkg listmcpkeys_test . Env

type Env interface {
	PageLimits() page.Limits
	DBListMCPKeys(ctx context.Context, arg db.DBListMCPKeysParams) ([]db.DBListMCPKeysRow, error)
}

type (
	Input  = page.Args
	Result = model.MCPKeysConnection
)

func Resolve(ctx context.Context, env Env, input Input) (*Result, error) {
	window := env.PageLimits().Window(input)

	rows, err := env.DBListMCPKeys(ctx, db.DBListMCPKeysParams{Before: window.Before, PageLimit: window.Probe()})
	if err != nil {
		return nil, fmt.Errorf("failed to list MCP keys: %w", err)
	}

	nodes, info := page.Cut(window, rows, keyID)

	return &Result{Nodes: nodes, PageInfo: info}, nil
}

func keyID(row db.DBListMCPKeysRow) int64 {
	return row.ID
}
